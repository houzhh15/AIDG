package taskdocs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPendingCommitRecovery(t *testing.T) {
	base := filepath.Join(t.TempDir(), "requirements")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	before := initDocMeta("requirements")
	oldMeta, _ := json.MarshalIndent(before, "", "  ")
	if err := os.WriteFile(docMetaPathFromBase(base), oldMeta, 0644); err != nil {
		t.Fatal(err)
	}
	chunk := DocChunk{Sequence: 1, Timestamp: time.Now(), Op: "replace_full", Content: "target", Hash: hashDocContent("target"), Active: true}
	encoded, _ := json.Marshal(chunk)
	targetChunks := append(encoded, '\n')
	target := before
	target.Version = 1
	target.LastSequence = 1
	target.ChunkCount = 1
	target.HashWindow = []string{chunk.Hash}
	target.ETag = chunk.Hash
	target.UpdatedAt = time.Now()
	chunksState, _, _ := stateOf(docChunksPathFromBase(base))
	compiledState, _, _ := stateOf(docCompiledPathFromBase(base))
	metaState, _, _ := stateOf(docMetaPathFromBase(base))
	p := pendingCommit{Protocol: 1, OperationID: "test", BeforeVersion: 0, TargetVersion: 1, BeforeChunks: chunksState, BeforeCompiled: compiledState, BeforeMeta: metaState, TargetChunks: targetChunks, TargetCompiled: []byte("target"), TargetMeta: target, CreatedAt: time.Now()}
	if err := persistPending(base, p); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash after only the chunk reached its target state.
	if err := atomicWrite(docChunksPathFromBase(base), targetChunks); err != nil {
		t.Fatal(err)
	}
	content, meta, err := ExportWithPath(base)
	if err != nil {
		t.Fatal(err)
	}
	if content != "target" || meta.Version != 1 || meta.LastSequence != 1 {
		t.Fatalf("%q %+v", content, meta)
	}
	chunks, _, err := NewDocService().ListWithPath(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0].Sequence != 1 {
		t.Fatalf("%+v", chunks)
	}
	if _, err = os.Stat(pendingPath(base)); !os.IsNotExist(err) {
		t.Fatalf("pending remains: %v", err)
	}
	// Repeated reads do not replay the operation.
	if _, _, err = ExportWithPath(base); err != nil {
		t.Fatal(err)
	}
	chunks, _, _ = NewDocService().ListWithPath(base)
	if len(chunks) != 1 {
		t.Fatal("duplicate chunk")
	}
}

func TestPendingCommitConflictFailsClosed(t *testing.T) {
	base := filepath.Join(t.TempDir(), "requirements")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	meta := initDocMeta("requirements")
	target := meta
	target.Version = 1
	p := pendingCommit{Protocol: 1, OperationID: "conflict", BeforeVersion: 0, TargetVersion: 1, TargetCompiled: []byte("target"), TargetMeta: target, CreatedAt: time.Now()}
	if err := persistPending(base, p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docCompiledPathFromBase(base), []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ExportWithPath(base); err == nil {
		t.Fatal("conflict accepted")
	}
	data, err := os.ReadFile(docCompiledPathFromBase(base))
	if err != nil || string(data) != "external" {
		t.Fatal("external edit overwritten")
	}
	if _, err = os.Stat(pendingPath(base)); err != nil {
		t.Fatal("pending evidence removed")
	}
}

func TestPendingCommitRejectsForgedTarget(t *testing.T) {
	base := filepath.Join(t.TempDir(), "requirements")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	chunk := DocChunk{Sequence: 1, Content: "actual", Hash: hashDocContent("forged"), Active: true}
	raw, _ := json.Marshal(chunk)
	meta := initDocMeta("requirements")
	meta.Version = 1
	meta.LastSequence = 1
	meta.ChunkCount = 1
	meta.HashWindow = []string{chunk.Hash}
	meta.ETag = hashDocContent("target")
	p := pendingCommit{Protocol: 1, OperationID: "forged", BeforeVersion: 0, TargetVersion: 1, TargetChunks: append(raw, '\n'), TargetCompiled: []byte("target"), TargetMeta: meta, CreatedAt: time.Now()}
	if err := persistPending(base, p); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ExportWithPath(base); err == nil {
		t.Fatal("forged intent recovered")
	}
	if _, err := os.Stat(pendingPath(base)); err != nil {
		t.Fatal("forged evidence removed")
	}
}

func TestPendingRecoveryRejectsChunksThatDoNotCompileToTarget(t *testing.T) {
	base := t.TempDir()
	chunk := DocChunk{Sequence: 1, Op: "add_full", Content: "chunk", Hash: hashDocContent("chunk"), Active: true}
	line, err := json.Marshal(chunk)
	if err != nil {
		t.Fatal(err)
	}
	meta := initDocMeta("requirements")
	meta.Version = 1
	meta.LastSequence = 1
	meta.ChunkCount = 1
	meta.HashWindow = []string{chunk.Hash}
	meta.ETag = hashDocContent("different")
	p := pendingCommit{Protocol: 1, OperationID: "bad-compiled", BeforeVersion: 0, TargetVersion: 1, TargetChunks: append(line, '\n'), TargetCompiled: []byte("different"), TargetMeta: meta}
	if err = persistPending(base, p); err != nil {
		t.Fatal(err)
	}
	if err = recoverPendingWithPath(base); err == nil || !strings.Contains(err.Error(), "do not compile") {
		t.Fatalf("expected semantic mismatch, got %v", err)
	}
	if _, err = os.Stat(pendingPath(base)); err != nil {
		t.Fatalf("pending evidence removed: %v", err)
	}
	if _, err = os.Stat(docCompiledPathFromBase(base)); !os.IsNotExist(err) {
		t.Fatalf("compiled changed on invalid pending: %v", err)
	}
}

func TestSquashWithPathUsesRecoverableCommit(t *testing.T) {
	base := t.TempDir()
	svc := NewDocService()
	if _, _, _, err := svc.AppendWithPath(base, "first", "add_full", "test", "test", nil); err != nil {
		t.Fatal(err)
	}
	version := 1
	if _, _, _, err := svc.AppendWithPath(base, "second", "add_full", "test", "test", &version); err != nil {
		t.Fatal(err)
	}
	squashVersion := 2
	meta, err := svc.SquashWithPath(base, "test", "test", &squashVersion)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Version != 3 || meta.ChunkCount != 1 || meta.ETag != hashDocContent("first\nsecond") {
		t.Fatalf("unexpected squash meta: %+v", meta)
	}
	chunks, listed, err := svc.ListWithPath(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0].Content != "first\nsecond" || listed.Version != 3 {
		t.Fatalf("unexpected squash state: %+v %+v", chunks, listed)
	}
	if _, err = os.Stat(pendingPath(base)); !os.IsNotExist(err) {
		t.Fatalf("pending not cleaned: %v", err)
	}
}

func TestPendingCommitAllowsDeletedSequenceGaps(t *testing.T) {
	base := filepath.Join(t.TempDir(), "requirements")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	chunk := DocChunk{Sequence: 2, Content: "target", Hash: hashDocContent("target"), Active: true}
	raw, _ := json.Marshal(chunk)
	meta := initDocMeta("requirements")
	meta.Version = 1
	meta.LastSequence = 2
	meta.ChunkCount = 1
	meta.DeletedCount = 1
	meta.HashWindow = []string{chunk.Hash}
	meta.ETag = chunk.Hash
	p := pendingCommit{Protocol: 1, OperationID: "gap", BeforeVersion: 0, TargetVersion: 1, TargetChunks: append(raw, '\n'), TargetCompiled: []byte("target"), TargetMeta: meta, CreatedAt: time.Now()}
	if err := persistPending(base, p); err != nil {
		t.Fatal(err)
	}
	content, observed, err := ExportWithPath(base)
	if err != nil {
		t.Fatal(err)
	}
	if content != "target" || observed.LastSequence != 2 || observed.ChunkCount != 1 {
		t.Fatalf("%q %+v", content, observed)
	}
}
