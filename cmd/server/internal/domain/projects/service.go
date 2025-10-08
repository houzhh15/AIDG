package projects

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/config"
)

var (
	// ProjectsRoot will be initialized from config
	ProjectsRoot      string
	ProjectsStatePath string
)

// InitPaths initializes the paths from config
func InitPaths() {
	if config.GlobalConfig != nil {
		ProjectsRoot = config.GlobalConfig.Data.ProjectsDir
		ProjectsStatePath = filepath.Join(ProjectsRoot, "server_projects.json")
	} else {
		// Fallback to relative paths
		ProjectsRoot = "projects"
		ProjectsStatePath = "projects/server_projects.json"
	}
}

// IsValidProjectName validates that the project name is safe for filesystem use
func IsValidProjectName(name string) bool {
	if len(name) == 0 || len(name) > 100 {
		return false
	}
	// Allow alphanumeric, spaces, hyphens, underscores
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == ' ' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// FileExists checks if a file exists
func FileExists(p string) bool {
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return true
	}
	return false
}

// SaveProjects persists the project registry to disk
func SaveProjects(reg *ProjectRegistry) error {
	InitPaths()
	if err := os.MkdirAll(ProjectsRoot, 0o755); err != nil {
		return fmt.Errorf("ensure projects root: %w", err)
	}
	reg.mu.Lock()
	defer reg.mu.Unlock()

	list := []persistedProject{}
	for _, p := range reg.m {
		list = append(list, persistedProject{
			ID:          p.ID,
			Name:        p.Name,
			ProductLine: p.ProductLine,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}

	b, err := json.MarshalIndent(gin.H{"projects": list}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal projects: %w", err)
	}

	tmp := ProjectsStatePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write tmp file: %w", err)
	}

	if err := os.Rename(tmp, ProjectsStatePath); err != nil {
		return fmt.Errorf("rename tmp file: %w", err)
	}

	return nil
}

// LoadProjects loads persisted projects into the registry
func LoadProjects(reg *ProjectRegistry) error {
	InitPaths()
	b, err := os.ReadFile(ProjectsStatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no file yet
		}
		return fmt.Errorf("read projects file: %w", err)
	}

	var wrapper struct {
		Projects []persistedProject `json:"projects"`
	}
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return fmt.Errorf("unmarshal projects: %w", err)
	}

	reg.mu.Lock()
	defer reg.mu.Unlock()

	for _, pp := range wrapper.Projects {
		reg.m[pp.ID] = &Project{
			ID:          pp.ID,
			Name:        pp.Name,
			ProductLine: pp.ProductLine,
			CreatedAt:   pp.CreatedAt,
			UpdatedAt:   pp.UpdatedAt,
		}
	}

	return nil
}

// ScanProjectDirs scans project directories to backfill registry entries
func ScanProjectDirs(reg *ProjectRegistry) error {
	InitPaths()
	if err := os.MkdirAll(ProjectsRoot, 0o755); err != nil {
		return fmt.Errorf("ensure projects root: %w", err)
	}

	entries, err := os.ReadDir(ProjectsRoot)
	if err != nil {
		return fmt.Errorf("read projects dir: %w", err)
	}

	added := 0
	reg.mu.Lock()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Skip hidden directories (starting with '.')
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}

		id := e.Name()
		if _, exists := reg.m[id]; exists {
			continue
		}

		info, _ := e.Info()
		created := time.Now()
		if info != nil {
			created = info.ModTime()
		}

		reg.m[id] = &Project{
			ID:        id,
			Name:      id,
			CreatedAt: created,
			UpdatedAt: created,
		}
		added++
	}
	reg.mu.Unlock()

	if added > 0 {
		log.Printf("scanProjectDirs added %d projects from %s\n", added, ProjectsRoot)
		if err := SaveProjects(reg); err != nil {
			return fmt.Errorf("save projects after scan: %w", err)
		}
	}

	return nil
}
