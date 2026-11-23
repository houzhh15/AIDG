package docslot

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
)

func TestPathResolverRejectsTraversalAndAbsoluteIDs(t *testing.T) {
	resolver := NewPathResolver(t.TempDir())
	for _, id := range []string{"../escape", "/absolute", `..\\escape`, ""} {
		if _, err := resolver.ResolvePath(ScopeProject, id, "feature_list"); !errors.Is(err, ErrPathTraversal) && id != "" {
			t.Errorf("ResolvePath(%q) error=%v", id, err)
		}
	}
	if _, err := resolver.ResolveTaskPath("project", "../task", "requirements"); !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("task traversal error=%v", err)
	}
}

func TestPathResolverRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "project")); err != nil {
		t.Fatal(err)
	}
	resolver := NewPathResolver(root)
	if _, err := resolver.ResolvePath(ScopeProject, "project", "feature_list"); !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("symlink escape error=%v", err)
	}
}

func TestReviewSlotIsReadableButNotGenericallyWritable(t *testing.T) {
	root := t.TempDir()
	service := NewUnifiedDocService(root)
	if _, err := service.AppendTask("project", "task", "review", "unsafe", "user", nil, "replace_full", "api"); !errors.Is(err, ErrReadOnlySlot) {
		t.Fatalf("generic review write error=%v", err)
	}
	result, err := service.ExportTask("project", "task", "review")
	if err != nil {
		t.Fatal(err)
	}
	if result.Exists || result.Content != "" {
		t.Fatalf("unexpected review document: %#v", result)
	}
}

func TestUnifiedChunkReadsDoNotRecoverPendingCommit(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "project", "tasks", "task", "docs", "requirements")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	pendingPath := filepath.Join(base, ".pending-commit.json")
	before := []byte(`{"protocol":1}`)
	if err := os.WriteFile(pendingPath, before, 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewUnifiedDocService(root)
	if _, _, err := service.ListTaskChunks("project", "task", "requirements"); !errors.Is(err, taskdocs.ErrPendingCommit) {
		t.Fatalf("pending chunk read error=%v", err)
	}
	after, err := os.ReadFile(pendingPath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("chunk read changed pending transaction: %v", err)
	}
}
