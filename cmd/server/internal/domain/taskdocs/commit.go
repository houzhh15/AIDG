package taskdocs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const pendingCommitFile = ".pending-commit.json"

type fileState struct {
	Exists bool   `json:"exists"`
	Hash   string `json:"hash"`
}

type pendingCommit struct {
	Protocol        int       `json:"protocol"`
	OperationID     string    `json:"operation_id"`
	BeforeVersion   int       `json:"before_version"`
	TargetVersion   int       `json:"target_version"`
	BeforeChunks    fileState `json:"before_chunks"`
	BeforeCompiled  fileState `json:"before_compiled"`
	BeforeMeta      fileState `json:"before_meta"`
	TargetChunks    []byte    `json:"target_chunks"`
	TargetCompiled  []byte    `json:"target_compiled"`
	TargetMeta      DocMeta   `json:"target_meta"`
	NeedSectionSync bool      `json:"need_section_sync"`
	CreatedAt       time.Time `json:"created_at"`
}

func stateOf(path string) (fileState, []byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fileState{}, nil, nil
	}
	if err != nil {
		return fileState{}, nil, err
	}
	return fileState{Exists: true, Hash: hashDocContent(string(data))}, data, nil
}

func matchesState(path string, before fileState, target []byte) (bool, error) {
	current, data, err := stateOf(path)
	if err != nil {
		return false, err
	}
	targetState := fileState{Exists: true, Hash: hashDocContent(string(target))}
	return current == before || (current == targetState && string(data) == string(target)), nil
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".doc-tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if err = tmp.Chmod(0644); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}

func pendingPath(base string) string { return filepath.Join(base, pendingCommitFile) }

func persistPending(base string, p pendingCommit) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(pendingPath(base), data)
}

func operationReplacesCompiled(op string) bool {
	return op == "replace_full" || (strings.HasPrefix(op, "section_") && strings.HasSuffix(op, "_no_parse"))
}

func compileTargetChunks(data []byte) (string, []DocChunk, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "", nil, nil
	}
	lines := bytes.Split(trimmed, []byte("\n"))
	chunks := make([]DocChunk, 0, len(lines))
	parts := make([]string, 0, len(lines))
	var lastSequence int
	for i, line := range lines {
		var chunk DocChunk
		if json.Unmarshal(line, &chunk) != nil || chunk.Sequence <= 0 || chunk.Hash != hashDocContent(chunk.Content) || (i > 0 && chunk.Sequence <= lastSequence) {
			return "", nil, errors.New("invalid pending chunks")
		}
		lastSequence = chunk.Sequence
		chunks = append(chunks, chunk)
		if !chunk.Active {
			continue
		}
		if operationReplacesCompiled(chunk.Op) {
			parts = []string{chunk.Content}
		} else if chunk.Content != "" {
			parts = append(parts, chunk.Content)
		}
	}
	return strings.Join(parts, "\n"), chunks, nil
}

func recoverPendingWithPath(base string) error {
	path := pendingPath(base)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var p pendingCommit
	if err = json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decode pending commit: %w", err)
	}
	if p.Protocol != 1 || p.OperationID == "" || p.TargetVersion != p.BeforeVersion+1 || p.TargetMeta.Version != p.TargetVersion || p.TargetMeta.ETag != hashDocContent(string(p.TargetCompiled)) {
		return errors.New("invalid pending commit")
	}
	compiled, chunks, compileErr := compileTargetChunks(p.TargetChunks)
	if compileErr != nil {
		return compileErr
	}
	if len(chunks) != p.TargetMeta.ChunkCount {
		return errors.New("pending chunk count mismatch")
	}
	if compiled != string(p.TargetCompiled) {
		return errors.New("pending chunks do not compile to target")
	}
	if len(chunks) == 0 {
		if p.TargetMeta.LastSequence != 0 || p.TargetMeta.ChunkCount != 0 || p.TargetMeta.DeletedCount != 0 || len(p.TargetMeta.HashWindow) != 0 {
			return errors.New("pending empty chunk counters mismatch")
		}
	} else {
		last := chunks[len(chunks)-1]
		var hashes []string
		for _, chunk := range chunks {
			if chunk.Active {
				hashes = pushHashWindow(hashes, chunk.Hash)
			}
		}
		if last.Sequence != p.TargetMeta.LastSequence || p.TargetMeta.DeletedCount < 0 || !slices.Equal(hashes, p.TargetMeta.HashWindow) {
			return errors.New("pending chunk counters mismatch")
		}
	}
	chunksPath := docChunksPathFromBase(base)
	compiledPath := docCompiledPathFromBase(base)
	metaPath := docMetaPathFromBase(base)
	for _, item := range []struct {
		path   string
		before fileState
		target []byte
	}{{chunksPath, p.BeforeChunks, p.TargetChunks}, {compiledPath, p.BeforeCompiled, p.TargetCompiled}} {
		ok, checkErr := matchesState(item.path, item.before, item.target)
		if checkErr != nil {
			return checkErr
		}
		if !ok {
			return fmt.Errorf("pending commit conflicts with %s", filepath.Base(item.path))
		}
	}
	targetMeta, err := json.MarshalIndent(p.TargetMeta, "", "  ")
	if err != nil {
		return err
	}
	ok, err := matchesState(metaPath, p.BeforeMeta, targetMeta)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("pending commit conflicts with meta.json")
	}
	if err = atomicWrite(chunksPath, p.TargetChunks); err != nil {
		return fmt.Errorf("recover chunks: %w", err)
	}
	if err = atomicWrite(compiledPath, p.TargetCompiled); err != nil {
		return fmt.Errorf("recover compiled: %w", err)
	}
	if err = atomicWrite(metaPath, targetMeta); err != nil {
		return fmt.Errorf("recover meta: %w", err)
	}
	if p.NeedSectionSync {
		if err = NewSyncManager(base, filepath.Base(base)).SyncFromCompiled(); err != nil {
			return fmt.Errorf("recover sections: %w", err)
		}
	}
	chunksState, _, err := stateOf(chunksPath)
	if err != nil {
		return err
	}
	compiledState, _, err := stateOf(compiledPath)
	if err != nil {
		return err
	}
	metaState, _, err := stateOf(metaPath)
	if err != nil {
		return err
	}
	if chunksState.Hash != hashDocContent(string(p.TargetChunks)) || compiledState.Hash != p.TargetMeta.ETag || metaState.Hash != hashDocContent(string(targetMeta)) {
		return errors.New("pending commit verification failed")
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	dir, err := os.Open(base)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}
