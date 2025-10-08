package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/config"
)

const (
	ProjectsRoot = "projects"
	TasksRoot    = "tasks"
)

// DefaultSyncAllowList returns the default list of allowed sync paths
var DefaultSyncAllowList = []string{
	"projects/server_projects.json",
	"tasks/server_tasks.json",
	"user_current_tasks.json",
	ProjectsRoot + "/",
	TasksRoot + "/",
}

// InitPaths refreshes the sync roots based on the loaded configuration
func InitPaths() {
	projRoot := ProjectsRoot
	meetingsRoot := TasksRoot

	if config.GlobalConfig != nil {
		if dir := strings.TrimSpace(config.GlobalConfig.Data.ProjectsDir); dir != "" {
			projRoot = dir
		}
		if dir := strings.TrimSpace(config.GlobalConfig.Data.MeetingsDir); dir != "" {
			meetingsRoot = dir
		}
	}

	if projRoot == "" {
		projRoot = ProjectsRoot
	}
	if meetingsRoot == "" {
		meetingsRoot = TasksRoot
	}

	projRoot = filepath.Clean(projRoot)
	meetingsRoot = filepath.Clean(meetingsRoot)

	allow := []string{
		NormalizePath(filepath.Join(projRoot, "server_projects.json")),
		NormalizePath(filepath.Join(meetingsRoot, "server_tasks.json")),
		"user_current_tasks.json",
	}
	if prefix := dirPrefixForAllow(projRoot); prefix != "" {
		allow = append(allow, prefix)
	}
	if prefix := dirPrefixForAllow(meetingsRoot); prefix != "" {
		allow = append(allow, prefix)
	}

	DefaultSyncAllowList = allow
}

func dirPrefixForAllow(dir string) string {
	norm := NormalizePath(dir)
	if norm == "" || norm == "." {
		return ""
	}
	if !strings.HasSuffix(norm, "/") {
		norm += "/"
	}
	return norm
}

// CollectAllowedFiles scans and collects all allowed files for synchronization
func CollectAllowedFiles(allowList []string) ([]SyncFile, error) {
	if allowList == nil {
		InitPaths()
		allowList = DefaultSyncAllowList
	}

	out := []SyncFile{}
	for _, allow := range allowList {
		if strings.HasSuffix(allow, "/") { // directory
			base := strings.TrimSuffix(allow, "/")
			if _, err := os.Stat(base); os.IsNotExist(err) {
				continue
			}
			filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					// Skip per-file edit history folders to reduce noise & size
					if info.Name() == ".history" {
						return filepath.SkipDir
					}
					return nil
				}
				rel := NormalizePath(path)
				if !isAllowedSyncPath(rel, allowList) {
					return nil
				}
				if isIgnoredSyncFile(rel) {
					return nil
				}
				if data, err := os.ReadFile(path); err == nil {
					out = append(out, SyncFile{
						Path:    rel,
						Hash:    HashContent(data),
						Content: string(data),
						Size:    int64(len(data)),
					})
				}
				return nil
			})
		} else { // file
			if isIgnoredSyncFile(allow) {
				continue
			}
			if data, err := os.ReadFile(allow); err == nil {
				out = append(out, SyncFile{
					Path:    NormalizePath(allow),
					Hash:    HashContent(data),
					Content: string(data),
					Size:    int64(len(data)),
				})
			}
		}
	}
	return out, nil
}

// WriteSyncFile writes a sync file to disk with validation
func WriteSyncFile(f SyncFile, allowList []string) error {
	if allowList == nil {
		InitPaths()
		allowList = DefaultSyncAllowList
	}

	norm := NormalizePath(f.Path)
	if !isAllowedSyncPath(norm, allowList) {
		return fmt.Errorf("forbidden path: %s", norm)
	}
	if isIgnoredSyncFile(norm) {
		return fmt.Errorf("ignored file type: %s", norm)
	}
	if strings.HasSuffix(norm, "/") {
		return fmt.Errorf("invalid file path: %s", norm)
	}

	localPath := filepath.FromSlash(norm) // convert to OS-specific path separators
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(localPath, []byte(f.Content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
