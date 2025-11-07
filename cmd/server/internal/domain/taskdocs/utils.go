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

// docBaseDir returns the base directory for a document type (internal helper)
func docBaseDir(projectID, taskID, docType string) (string, error) {
	dir := filepath.Join(projects.ProjectsRoot, projectID)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("project dir missing")
	}
	return filepath.Join(dir, "tasks", taskID, "docs", docType), nil
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
		// Corrupted meta, return init
		return initDocMeta(docType), nil
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

	meta, _ := loadOrInitMeta(projectID, taskID, docType)

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
