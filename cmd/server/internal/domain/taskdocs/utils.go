package taskdocs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
)

// TaskExistsInProject checks whether taskID is registered in the project's
// tasks.json. This prevents accidental doc writes to non-existent task IDs.
func TaskExistsInProject(projectID, taskID string) bool {
	if !safeResourceID(projectID) || !safeResourceID(taskID) {
		return false
	}
	tasksFile := filepath.Join(projects.ProjectsRoot, projectID, "tasks.json")
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		return false
	}
	var tasks []map[string]any
	if json.Unmarshal(data, &tasks) != nil {
		return false
	}
	for _, t := range tasks {
		if id, _ := t["id"].(string); id == taskID {
			return true
		}
	}
	return false
}

// docBaseDir returns the base directory for a document type (internal helper)
func docBaseDir(projectID, taskID, docType string) (string, error) {
	return ResolveDocPath(projects.ProjectsRoot, projectID, taskID, docType)
}

// ResolveDocPath validates identifiers and resolves existing symlinks before
// returning a task document path contained by basePath.
func ResolveDocPath(basePath, projectID, taskID, docType string) (string, error) {
	if !safeResourceID(projectID) || !safeResourceID(taskID) || !safeResourceID(docType) {
		return "", fmt.Errorf("invalid resource identifier")
	}
	root, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, projectID)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("project dir missing")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolvedProject, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedProject)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("project path escapes projects root")
	}
	target := filepath.Join(dir, "tasks", taskID, "docs", docType)
	resolvedTarget, err := resolveExistingPath(target)
	if err != nil {
		return "", err
	}
	rel, err = filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("task document path escapes projects root")
	}
	return target, nil
}

func resolveExistingPath(path string) (string, error) {
	current := filepath.Clean(path)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path), nil
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func safeResourceID(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && len(id) <= 200 && id != "." && id != ".." && filepath.Base(id) == id && !strings.ContainsAny(id, "/\\\x00")
}

// docMetaPath returns the path to meta.json
func docMetaPath(projectID, taskID, docType string) (string, error) {
	b, err := docBaseDir(projectID, taskID, docType)
	if err != nil {
		return "", err
	}
	return filepath.Join(b, "meta.json"), nil
}

// docChunksPath returns the path to chunks.ndjson
func docChunksPath(projectID, taskID, docType string) (string, error) {
	b, err := docBaseDir(projectID, taskID, docType)
	if err != nil {
		return "", err
	}
	return filepath.Join(b, "chunks.ndjson"), nil
}

// DocCompiledPath returns the path to compiled.md (exported for API layer)
func DocCompiledPath(projectID, taskID, docType string) (string, error) {
	b, err := docBaseDir(projectID, taskID, docType)
	if err != nil {
		return "", err
	}
	return filepath.Join(b, "compiled.md"), nil
}

// docCompiledPath is the internal helper (kept for internal use)
func docCompiledPath(projectID, taskID, docType string) (string, error) {
	return DocCompiledPath(projectID, taskID, docType)
}

// initDocMeta creates initial metadata for a new document
// DocDir returns the absolute filesystem path of the per-task doc slot
// (e.g. .../tasks/<taskID>/docs/<docType>). The path is exposed for the
// DELETE handler so it can fully remove the slot directory and all of its
// meta / chunks / compiled artifacts.
func DocDir(projectID, taskID, docType string) (string, error) {
	return docBaseDir(projectID, taskID, docType)
}

// DeleteTaskDocSlot removes the entire per-task doc slot directory
// (meta.json + chunks.ndjson + compiled.md + tag dirs). It is safe to call
// when the directory does not exist — a missing slot is treated as success.
func DeleteTaskDocSlot(projectID, taskID, docType string) error {
	dir, err := docBaseDir(projectID, taskID, docType)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(dir)
}

func initDocMeta(docType string) DocMeta {
	now := time.Now()
	return DocMeta{
		Version:      0,
		LastSequence: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
		DocType:      docType,
		HashWindow:   []string{},
		ChunkCount:   0,
		DeletedCount: 0,
		ETag:         "",
	}
}

// hashDocContent computes SHA256 hash of content
func hashDocContent(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// pushHashWindow adds a hash to the window, maintaining size limit
func pushHashWindow(window []string, h string) []string {
	window = append(window, h)
	if len(window) > DocHashWindowSize {
		window = window[len(window)-DocHashWindowSize:]
	}
	return window
}

// containsHash checks if hash exists in window
func containsHash(window []string, h string) bool {
	for _, x := range window {
		if x == h {
			return true
		}
	}
	return false
}

// LoadOrInitMeta loads metadata or returns initialized structure (exported for API layer)
func LoadOrInitMeta(projectID, taskID, docType string) (DocMeta, error) {
	mp, err := docMetaPath(projectID, taskID, docType)
	if err != nil {
		return DocMeta{}, err
	}

	data, err := os.ReadFile(mp)
	if err != nil {
		if os.IsNotExist(err) {
			return initDocMeta(docType), nil
		}
		return DocMeta{}, fmt.Errorf("read meta: %w", err)
	}

	var meta DocMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return DocMeta{}, fmt.Errorf("decode meta: %w", err)
	}

	return meta, nil
}

// loadOrInitMeta is the internal helper (kept for internal use)
func loadOrInitMeta(projectID, taskID, docType string) (DocMeta, error) {
	return LoadOrInitMeta(projectID, taskID, docType)
}

// writeMetaAtomic atomically writes meta.json
func writeMetaAtomic(projectID, taskID, docType string, meta DocMeta) error {
	mp, err := docMetaPath(projectID, taskID, docType)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(mp), 0755); err != nil {
		return fmt.Errorf("create meta dir: %w", err)
	}

	tmp := mp + ".tmp"
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write tmp meta: %w", err)
	}

	if err := os.Rename(tmp, mp); err != nil {
		return fmt.Errorf("rename meta: %w", err)
	}

	return nil
}

// listChunks reads all chunks from file
func listChunks(projectID, taskID, docType string) ([]DocChunk, DocMeta, error) {
	cp, err := docChunksPath(projectID, taskID, docType)
	if err != nil {
		return nil, DocMeta{}, err
	}

	meta, err := loadOrInitMeta(projectID, taskID, docType)
	if err != nil {
		return nil, DocMeta{}, err
	}

	data, err := os.ReadFile(cp)
	if err != nil {
		if os.IsNotExist(err) {
			return []DocChunk{}, meta, nil
		}
		return nil, meta, fmt.Errorf("read chunks: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	res := []DocChunk{}

	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}

		var ck DocChunk
		if json.Unmarshal([]byte(ln), &ck) == nil {
			res = append(res, ck)
		}
	}

	return res, meta, nil
}
