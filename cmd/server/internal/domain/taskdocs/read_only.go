package taskdocs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrPendingCommit = errors.New("document commit recovery required")

// ExportReadOnlyWithPath returns the last fully committed snapshot without
// creating metadata, migrating legacy files, or recovering a pending commit.
// A pending commit is reported so callers never expose a potentially partial
// snapshot and can trigger recovery through an explicit write/maintenance path.
func ExportReadOnlyWithPath(basePath string) (string, DocMeta, error) {
	absolute, err := filepath.Abs(basePath)
	if err != nil {
		return "", DocMeta{}, err
	}
	if _, err = os.Stat(pendingPath(absolute)); err == nil {
		return "", DocMeta{}, ErrPendingCommit
	} else if !os.IsNotExist(err) {
		return "", DocMeta{}, err
	}
	var meta DocMeta
	data, err := os.ReadFile(docMetaPathFromBase(absolute))
	if os.IsNotExist(err) {
		return "", meta, nil
	}
	if err != nil {
		return "", meta, err
	}
	if err = json.Unmarshal(data, &meta); err != nil {
		return "", meta, fmt.Errorf("decode document metadata: %w", err)
	}
	compiled, err := os.ReadFile(docCompiledPathFromBase(absolute))
	if os.IsNotExist(err) {
		if meta.Version == 0 {
			return "", meta, nil
		}
		return "", meta, errors.New("compiled document missing")
	}
	if err != nil {
		return "", meta, err
	}
	if meta.ETag != "" && meta.ETag != hashDocContent(string(compiled)) {
		return "", meta, errors.New("compiled document checksum mismatch")
	}
	return string(compiled), meta, nil
}

// ListChunksReadOnlyWithPath lists committed chunks without initialization or
// pending-commit recovery.
func ListChunksReadOnlyWithPath(basePath string) ([]DocChunk, DocMeta, error) {
	_, meta, err := ExportReadOnlyWithPath(basePath)
	if err != nil {
		return nil, DocMeta{}, err
	}
	absolute, err := filepath.Abs(basePath)
	if err != nil {
		return nil, meta, err
	}
	data, err := os.ReadFile(docChunksPathFromBase(absolute))
	if os.IsNotExist(err) {
		return []DocChunk{}, meta, nil
	}
	if err != nil {
		return nil, meta, fmt.Errorf("read chunks: %w", err)
	}
	chunks := make([]DocChunk, 0)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var chunk DocChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return nil, meta, fmt.Errorf("decode chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}
	return chunks, meta, nil
}
