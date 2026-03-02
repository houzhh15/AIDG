package copy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/meetings"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
)

// CollectResource gathers all files for a given resource from disk
func CollectResource(res CopyResource, meetingsReg *meetings.Registry, projectsReg *projects.ProjectRegistry, opts CopyOptions) (*ResourcePayload, error) {
	switch res.Type {
	case "meeting":
		return collectMeeting(res.ID, meetingsReg, opts)
	case "project":
		return collectProject(res.ID, projectsReg, opts)
	case "task":
		return collectTask(res.ProjectID, res.ID, projectsReg, opts)
	default:
		return nil, fmt.Errorf("unknown resource type: %s", res.Type)
	}
}

func collectMeeting(id string, reg *meetings.Registry, opts CopyOptions) (*ResourcePayload, error) {
	task := reg.Get(id)
	if task == nil {
		return nil, fmt.Errorf("meeting task not found: %s", id)
	}

	// Determine the meeting directory
	meetingDir := task.Cfg.OutputDir
	if meetingDir == "" {
		meetingDir = filepath.Join(meetings.MeetingsRoot(), id)
	}

	files, err := collectDirFiles(meetingDir, opts.IncludeAudio)
	if err != nil {
		return nil, fmt.Errorf("collect meeting files: %w", err)
	}

	// Build registry entry (lightweight, without orchestrator)
	type regEntry struct {
		ID        string      `json:"id"`
		Config    interface{} `json:"config"`
		State     interface{} `json:"state"`
		CreatedAt interface{} `json:"created_at"`
	}
	entry := regEntry{
		ID:        task.ID,
		Config:    task.Cfg,
		State:     task.State,
		CreatedAt: task.CreatedAt,
	}

	return &ResourcePayload{
		Type:          "meeting",
		ID:            id,
		RegistryEntry: entry,
		Files:         files,
	}, nil
}

func collectProject(id string, reg *projects.ProjectRegistry, opts CopyOptions) (*ResourcePayload, error) {
	proj := reg.Get(id)
	if proj == nil {
		return nil, fmt.Errorf("project not found: %s", id)
	}

	projectDir := filepath.Join(projects.ProjectsRoot, id)
	files, err := collectDirFiles(projectDir, true) // projects don't have audio concern
	if err != nil {
		return nil, fmt.Errorf("collect project files: %w", err)
	}

	return &ResourcePayload{
		Type:          "project",
		ID:            id,
		RegistryEntry: proj,
		Files:         files,
	}, nil
}

func collectTask(projectID, taskID string, reg *projects.ProjectRegistry, opts CopyOptions) (*ResourcePayload, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required for task resource")
	}
	proj := reg.Get(projectID)
	if proj == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	taskDir := filepath.Join(projects.ProjectsRoot, projectID, "tasks", taskID)
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("task directory not found: %s", taskDir)
	}

	files, err := collectDirFiles(taskDir, true)
	if err != nil {
		return nil, fmt.Errorf("collect task files: %w", err)
	}

	// Read the task entry from tasks.json
	tasksFile := filepath.Join(projects.ProjectsRoot, projectID, "tasks.json")
	var taskEntry interface{}
	if data, err := os.ReadFile(tasksFile); err == nil {
		var wrapper struct {
			Tasks []json.RawMessage `json:"tasks"`
		}
		if json.Unmarshal(data, &wrapper) == nil {
			for _, raw := range wrapper.Tasks {
				var t struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(raw, &t) == nil && t.ID == taskID {
					taskEntry = json.RawMessage(raw)
					break
				}
			}
		}
	}

	return &ResourcePayload{
		Type:          "task",
		ID:            taskID,
		ProjectID:     projectID,
		RegistryEntry: taskEntry,
		Files:         files,
	}, nil
}

// collectDirFiles walks a directory and returns all files as ResourceFile entries
func collectDirFiles(dir string, includeAudio bool) ([]ResourceFile, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil // directory doesn't exist, return empty
	}

	var files []ResourceFile
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}

		if info.IsDir() {
			files = append(files, ResourceFile{
				RelPath: rel,
				IsDir:   true,
			})
			return nil
		}

		// Skip audio files unless requested
		if !includeAudio && isAudioFile(path) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		hash := sha256.Sum256(data)
		files = append(files, ResourceFile{
			RelPath: rel,
			Hash:    hex.EncodeToString(hash[:]),
			Content: base64.StdEncoding.EncodeToString(data),
			Size:    info.Size(),
		})
		return nil
	})

	return files, err
}

func isAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".wav" || ext == ".mp3" || ext == ".ogg" || ext == ".flac" || ext == ".m4a"
}

// WriteResource writes a received resource payload to disk
func WriteResource(res ResourcePayload, mode CopyMode, meetingsReg *meetings.Registry, projectsReg *projects.ProjectRegistry) (*CopyResult, error) {
	switch res.Type {
	case "meeting":
		return writeMeeting(res, mode, meetingsReg)
	case "project":
		return writeProject(res, mode, projectsReg)
	case "task":
		return writeTask(res, mode, projectsReg)
	default:
		return nil, fmt.Errorf("unknown resource type: %s", res.Type)
	}
}

func writeMeeting(res ResourcePayload, mode CopyMode, reg *meetings.Registry) (*CopyResult, error) {
	targetDir := filepath.Join(meetings.MeetingsRoot(), res.ID)
	existing := reg.Get(res.ID)

	status := "created"
	if existing != nil {
		if mode == ModeSkipExisting {
			return &CopyResult{Type: "meeting", ID: res.ID, Status: "skipped"}, nil
		}
		status = "updated"
	}

	written, err := writeFiles(targetDir, res.Files, mode)
	if err != nil {
		return &CopyResult{Type: "meeting", ID: res.ID, Status: "error", Error: err.Error()}, err
	}

	// Update registry from the entry
	if res.RegistryEntry != nil {
		entryBytes, _ := json.Marshal(res.RegistryEntry)
		var taskData struct {
			ID        string          `json:"id"`
			Config    json.RawMessage `json:"config"`
			State     interface{}     `json:"state"`
			CreatedAt interface{}     `json:"created_at"`
		}
		if json.Unmarshal(entryBytes, &taskData) == nil {
			// We only need basic info for the registry; the orchestrator will be created on reload
			// For now, trigger a registry reload after copy
		}
	}

	return &CopyResult{Type: "meeting", ID: res.ID, Status: status, Files: written}, nil
}

func writeProject(res ResourcePayload, mode CopyMode, reg *projects.ProjectRegistry) (*CopyResult, error) {
	targetDir := filepath.Join(projects.ProjectsRoot, res.ID)
	existing := reg.Get(res.ID)

	status := "created"
	if existing != nil {
		if mode == ModeSkipExisting {
			return &CopyResult{Type: "project", ID: res.ID, Status: "skipped"}, nil
		}
		status = "updated"
	}

	written, err := writeFiles(targetDir, res.Files, mode)
	if err != nil {
		return &CopyResult{Type: "project", ID: res.ID, Status: "error", Error: err.Error()}, err
	}

	// Register in project registry
	if res.RegistryEntry != nil {
		entryBytes, _ := json.Marshal(res.RegistryEntry)
		var proj projects.Project
		if json.Unmarshal(entryBytes, &proj) == nil && proj.ID != "" {
			reg.Set(&proj)
			projects.SaveProjects(reg)
		}
	}

	return &CopyResult{Type: "project", ID: res.ID, Status: status, Files: written}, nil
}

func writeTask(res ResourcePayload, mode CopyMode, reg *projects.ProjectRegistry) (*CopyResult, error) {
	if res.ProjectID == "" {
		return nil, fmt.Errorf("project_id required for task")
	}

	// Ensure project dir exists
	projectDir := filepath.Join(projects.ProjectsRoot, res.ProjectID)
	os.MkdirAll(projectDir, 0o755)

	targetDir := filepath.Join(projectDir, "tasks", res.ID)

	// Check if task dir exists
	status := "created"
	if _, err := os.Stat(targetDir); err == nil {
		if mode == ModeSkipExisting {
			return &CopyResult{Type: "task", ID: res.ID, Status: "skipped"}, nil
		}
		status = "updated"
	}

	written, err := writeFiles(targetDir, res.Files, mode)
	if err != nil {
		return &CopyResult{Type: "task", ID: res.ID, Status: "error", Error: err.Error()}, err
	}

	// Merge task entry into tasks.json
	if res.RegistryEntry != nil {
		mergeTaskEntry(projectDir, res.ID, res.RegistryEntry)
	}

	return &CopyResult{Type: "task", ID: res.ID, Status: status, Files: written}, nil
}

// mergeTaskEntry adds or updates a task entry in tasks.json
func mergeTaskEntry(projectDir, taskID string, entry interface{}) {
	tasksFile := filepath.Join(projectDir, "tasks.json")

	var wrapper struct {
		Tasks []json.RawMessage `json:"tasks"`
	}

	if data, err := os.ReadFile(tasksFile); err == nil {
		json.Unmarshal(data, &wrapper)
	}

	entryBytes, _ := json.Marshal(entry)

	// Remove existing entry with same ID
	filtered := make([]json.RawMessage, 0, len(wrapper.Tasks))
	for _, raw := range wrapper.Tasks {
		var t struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &t) == nil && t.ID == taskID {
			continue // skip, will be replaced
		}
		filtered = append(filtered, raw)
	}
	filtered = append(filtered, json.RawMessage(entryBytes))
	wrapper.Tasks = filtered

	b, _ := json.MarshalIndent(wrapper, "", "  ")
	tmp := tasksFile + ".tmp"
	os.WriteFile(tmp, b, 0o644)
	os.Rename(tmp, tasksFile)
}

// writeFiles writes ResourceFile entries to a target directory
func writeFiles(targetDir string, files []ResourceFile, mode CopyMode) (int, error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return 0, fmt.Errorf("create target dir: %w", err)
	}

	written := 0
	for _, f := range files {
		fullPath := filepath.Join(targetDir, filepath.FromSlash(f.RelPath))

		if f.IsDir {
			os.MkdirAll(fullPath, 0o755)
			continue
		}

		// Skip existing files in skip_existing mode
		if mode == ModeSkipExisting {
			if _, err := os.Stat(fullPath); err == nil {
				continue
			}
		}

		// Decode content
		data, err := base64.StdEncoding.DecodeString(f.Content)
		if err != nil {
			return written, fmt.Errorf("decode file %s: %w", f.RelPath, err)
		}

		// Ensure parent directory exists
		os.MkdirAll(filepath.Dir(fullPath), 0o755)

		if err := os.WriteFile(fullPath, data, 0o644); err != nil {
			return written, fmt.Errorf("write file %s: %w", f.RelPath, err)
		}
		written++
	}
	return written, nil
}

// MakeSignature generates an HMAC-SHA256 signature for a copy envelope
func MakeSignature(resources []ResourcePayload, mode CopyMode, secret string) string {
	// Build a summary string for signing
	lines := make([]string, 0)
	for _, r := range resources {
		line := fmt.Sprintf("%s:%s:%d", r.Type, r.ID, len(r.Files))
		lines = append(lines, line)
	}
	sort.Strings(lines)
	payload := strings.Join(lines, "\n") + "|" + string(mode)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks the HMAC-SHA256 signature of a copy envelope
func VerifySignature(resources []ResourcePayload, mode CopyMode, signature, secret string) bool {
	expected := MakeSignature(resources, mode, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}
