package meetings

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	orchestrator "github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/config"
)

var (
	meetingsRootPath = "./data/meetings"
	tasksRootPath    = meetingsRootPath
	taskStatePath    = filepath.Join(meetingsRootPath, "server_tasks.json")
)

// InitPaths initializes meeting-related directories based on global config
func InitPaths() {
	if config.GlobalConfig != nil {
		if dir := config.GlobalConfig.Data.MeetingsDir; dir != "" {
			meetingsRootPath = dir
		}
	}
	if meetingsRootPath == "" {
		meetingsRootPath = "./data/meetings"
	}
	meetingsRootPath = filepath.Clean(meetingsRootPath)
	tasksRootPath = meetingsRootPath
	taskStatePath = filepath.Join(meetingsRootPath, "server_tasks.json")
}

// MeetingsRoot returns the configured meetings root directory
func MeetingsRoot() string {
	if meetingsRootPath == "" {
		InitPaths()
	}
	return meetingsRootPath
}

// TasksRoot returns the directory used to store meeting task data
func TasksRoot() string {
	if tasksRootPath == "" {
		InitPaths()
	}
	return tasksRootPath
}

// TaskStatePath returns the file path for persisted meeting tasks
func TaskStatePath() string {
	if taskStatePath == "" {
		InitPaths()
	}
	return taskStatePath
}

// SaveTasks persists the task registry to disk
func SaveTasks(reg *Registry) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	list := []persistedTask{}
	for _, t := range reg.m {
		cfgCopy := t.Cfg
		if cfgCopy.HFTokenValue != "" { // avoid writing actual token to disk
			cfgCopy.HFTokenValue = ""
		}
		list = append(list, persistedTask{
			ID:        t.ID,
			Cfg:       cfgCopy,
			State:     t.State,
			CreatedAt: t.CreatedAt,
		})
	}

	b, err := json.MarshalIndent(gin.H{"tasks": list}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}

	statePath := TaskStatePath()
	tmp := statePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write tmp file: %w", err)
	}

	if err := os.Rename(tmp, statePath); err != nil {
		return fmt.Errorf("rename tmp file: %w", err)
	}

	return nil
}

// LoadTasks loads persisted tasks into the registry
func LoadTasks(reg *Registry) error {
	statePath := TaskStatePath()
	b, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no file yet
		}
		return fmt.Errorf("read tasks file: %w", err)
	}

	var wrapper struct {
		Tasks []persistedTask `json:"tasks"`
	}
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return fmt.Errorf("unmarshal tasks: %w", err)
	}

	now := time.Now()
	count := 0
	reg.mu.Lock()
	defer reg.mu.Unlock()

	for _, pt := range wrapper.Tasks {
		// Skip hidden (dot-prefixed) task IDs such as .svn
		if strings.HasPrefix(pt.ID, ".") {
			continue
		}

		// Fix legacy OutputDir paths: tasks/* -> configured meetings root
		if pt.Cfg.OutputDir != "" && strings.HasPrefix(pt.Cfg.OutputDir, "tasks/") {
			taskID := strings.TrimPrefix(pt.Cfg.OutputDir, "tasks/")
			pt.Cfg.OutputDir = filepath.Join(TasksRoot(), taskID)
		}

		// Fix Docker container paths: /app/data/meetings/* -> configured meetings root
		if pt.Cfg.OutputDir != "" && strings.HasPrefix(pt.Cfg.OutputDir, "/app/data/meetings/") {
			taskID := strings.TrimPrefix(pt.Cfg.OutputDir, "/app/data/meetings/")
			pt.Cfg.OutputDir = filepath.Join(TasksRoot(), taskID)
		}

		// ensure output dir exists
		if pt.Cfg.OutputDir != "" {
			os.MkdirAll(pt.Cfg.OutputDir, 0o755)
		}

		// reset any transient running states to Created for safety
		st := pt.State
		if st == orchestrator.StateRunning ||
			st == orchestrator.StateStopping ||
			st == orchestrator.StateDraining {
			st = orchestrator.StateCreated
		}

		if pt.CreatedAt.IsZero() {
			pt.CreatedAt = now
		}

		// backfill SB defaults for legacy persisted tasks missing values
		backfillSBDefaults(&pt.Cfg)

		// Create orchestrator instance for the loaded task
		// This ensures that API endpoints requiring dependency client work correctly
		orch := orchestrator.New(pt.Cfg)

		reg.m[pt.ID] = &Task{
			ID:        pt.ID,
			Cfg:       pt.Cfg,
			Orch:      orch, // Initialize orchestrator
			State:     st,
			CreatedAt: pt.CreatedAt,
		}
		count++
	}

	if count > 0 {
		log.Printf("loaded %d tasks from %s\n", count, statePath)
	}

	return nil
}

// backfillSBDefaults fills in default SpeechBrain config values for legacy tasks
func backfillSBDefaults(cfg *orchestrator.Config) bool {
	// Heuristic: treat as uninitialized only if ALL tunables are zero/false.
	if cfg.SBOverclusterFactor == 0 &&
		cfg.SBMergeThreshold == 0 &&
		cfg.SBMinSegmentMerge == 0 &&
		!cfg.SBReassignAfterMerge &&
		!cfg.SBEnergyVAD &&
		cfg.SBEnergyVADThr == 0 {
		def := orchestrator.DefaultConfig()
		cfg.SBOverclusterFactor = def.SBOverclusterFactor
		cfg.SBMergeThreshold = def.SBMergeThreshold
		cfg.SBMinSegmentMerge = def.SBMinSegmentMerge
		cfg.SBReassignAfterMerge = def.SBReassignAfterMerge
		cfg.SBEnergyVAD = def.SBEnergyVAD
		cfg.SBEnergyVADThr = def.SBEnergyVADThr
		return true
	}
	return false
}

// Document management functions

// SaveDocumentWithHistory saves document content with version history
func SaveDocumentWithHistory(reg *Registry, taskID, docType, content string) error {
	t := reg.Get(taskID)
	if t == nil {
		return fmt.Errorf("task not found")
	}

	filename, ok := DocTypeToFilename[docType]
	if !ok {
		return fmt.Errorf("unknown document type: %s", docType)
	}

	return saveContentWithHistory(t.Cfg.OutputDir, filename, content)
}

// LoadDocument loads document content
func LoadDocument(reg *Registry, taskID, docType string) (string, bool, error) {
	t := reg.Get(taskID)
	if t == nil {
		return "", false, fmt.Errorf("task not found")
	}

	filename, ok := DocTypeToFilename[docType]
	if !ok {
		return "", false, fmt.Errorf("unknown document type: %s", docType)
	}

	// Handle tech-design glob pattern
	if docType == DocTypeTechDesign {
		return loadTechDesignContent(t.Cfg.OutputDir)
	}

	filePath := fmt.Sprintf("%s/%s", t.Cfg.OutputDir, filename)
	b, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil // file doesn't exist, return empty content
		}
		return "", false, fmt.Errorf("read file: %w", err)
	}

	return string(b), true, nil
}

// GetDocumentHistory returns version history for a document
func GetDocumentHistory(reg *Registry, taskID, docType string) ([]ContentHistory, error) {
	t := reg.Get(taskID)
	if t == nil {
		return nil, fmt.Errorf("task not found")
	}

	filename, ok := DocTypeToFilename[docType]
	if !ok {
		return nil, fmt.Errorf("unknown document type: %s", docType)
	}

	return getContentHistory(t.Cfg.OutputDir, filename)
}

// DeleteDocumentHistory removes a specific version from document history
func DeleteDocumentHistory(reg *Registry, taskID, docType string, version int) error {
	t := reg.Get(taskID)
	if t == nil {
		return fmt.Errorf("task not found")
	}

	filename, ok := DocTypeToFilename[docType]
	if !ok {
		return fmt.Errorf("unknown document type: %s", docType)
	}

	return deleteContentHistory(t.Cfg.OutputDir, filename, version)
}

// Helper functions for content management

// saveContentWithHistory saves content with version history
func saveContentWithHistory(taskOutputDir, filename, content string) error {
	filePath := filepath.Join(taskOutputDir, filename)
	historyDir := filepath.Join(taskOutputDir, ".history")
	historyFile := filepath.Join(historyDir, filename+".history.json")

	// Ensure history directory exists
	os.MkdirAll(historyDir, 0755)

	// Load existing history
	var history []ContentHistory
	if data, err := os.ReadFile(historyFile); err == nil {
		json.Unmarshal(data, &history)
	}

	// Read current content if file exists
	var currentContent string
	if data, err := os.ReadFile(filePath); err == nil {
		currentContent = string(data)
	}

	// Only save to history if content is different
	if currentContent != content && currentContent != "" {
		newRecord := ContentHistory{
			Timestamp: time.Now(),
			Content:   currentContent,
			Version:   len(history) + 1,
		}
		history = append(history, newRecord)
		if len(history) > 50 {
			history = history[len(history)-50:]
		}
		historyData, _ := json.MarshalIndent(history, "", "  ")
		_ = os.WriteFile(historyFile, historyData, 0644)
	}
	return os.WriteFile(filePath, []byte(content), 0644)
}

// getContentHistory loads history list for a file
func getContentHistory(taskOutputDir, filename string) ([]ContentHistory, error) {
	historyFile := filepath.Join(taskOutputDir, ".history", filename+".history.json")
	var history []ContentHistory
	data, err := os.ReadFile(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return history, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return history, nil
}

// deleteContentHistory removes a specific version from history
func deleteContentHistory(taskOutputDir, filename string, version int) error {
	history, err := getContentHistory(taskOutputDir, filename)
	if err != nil {
		return err
	}
	if version <= 0 {
		return nil
	}

	// filter out the version
	newHist := make([]ContentHistory, 0, len(history))
	for _, rec := range history {
		if rec.Version != version {
			newHist = append(newHist, rec)
		}
	}

	// reassign versions sequentially
	for i := range newHist {
		newHist[i].Version = i + 1
	}

	historyFile := filepath.Join(taskOutputDir, ".history", filename+".history.json")
	os.MkdirAll(filepath.Dir(historyFile), 0755)
	data, _ := json.MarshalIndent(newHist, "", "  ")
	return os.WriteFile(historyFile, data, 0644)
}

// loadTechDesignContent handles tech-design glob pattern
func loadTechDesignContent(taskOutputDir string) (string, bool, error) {
	// Find tech_design_*.md files
	files, err := filepath.Glob(filepath.Join(taskOutputDir, "tech_design_*.md"))
	if err != nil || len(files) == 0 {
		return "", false, nil
	}

	// Read the first matching file
	b, err := os.ReadFile(files[0])
	if err != nil {
		return "", false, nil
	}
	return string(b), true, nil
}
