package taskdocs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExportReadOnlyDoesNotInitializeStorage(t *testing.T) {
	base := filepath.Join(t.TempDir(), "missing", "docs", "requirements")
	content, meta, err := ExportReadOnlyWithPath(base)
	if err != nil {
		t.Fatal(err)
	}
	if content != "" || meta.Version != 0 {
		t.Fatalf("unexpected empty export: %q %+v", content, meta)
	}
	if _, err = os.Stat(base); !os.IsNotExist(err) {
		t.Fatalf("read initialized storage: %v", err)
	}
}

func TestExportReadOnlyDoesNotRecoverPendingCommit(t *testing.T) {
	base := t.TempDir()
	pending := pendingCommit{Protocol: 1, OperationID: "pending", BeforeVersion: 0, TargetVersion: 1}
	if err := persistPending(base, pending); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(pendingPath(base))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = ExportReadOnlyWithPath(base); !errors.Is(err, ErrPendingCommit) {
		t.Fatalf("pending commit error=%v", err)
	}
	after, err := os.ReadFile(pendingPath(base))
	if err != nil || string(after) != string(before) {
		t.Fatalf("read changed pending evidence: %v", err)
	}
}

func TestGetSectionsDoesNotImplicitlySync(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "compiled.md"), []byte("# Heading\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewSectionServiceWithPath(base)
	if _, err := service.GetSections(); err == nil {
		t.Fatal("expected explicit sync requirement")
	}
	if _, err := os.Stat(filepath.Join(base, "sections.json")); !os.IsNotExist(err) {
		t.Fatalf("section read created metadata: %v", err)
	}
}

func TestResolveDocPathRejectsTaskSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(project, "tasks", "task")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveDocPath(root, "project", "task", "requirements"); err == nil {
		t.Fatal("task symlink escape accepted")
	}
}
