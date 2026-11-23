package taskdocs

import (
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"os"
	"path/filepath"
	"testing"
)

func TestPersistenceFailuresAreReturned(t *testing.T) {
	root := t.TempDir()
	previous := projects.ProjectsRoot
	projects.ProjectsRoot = root
	t.Cleanup(func() { projects.ProjectsRoot = previous })
	for _, pathAPI := range []bool{false, true} {
		name := "legacy"
		if pathAPI {
			name = "path"
		}
		t.Run(name, func(t *testing.T) {
			base := filepath.Join(root, "P", "tasks", name, "docs", "requirements")
			if err := os.MkdirAll(base, 0755); err != nil {
				t.Fatal(err)
			}
			svc := NewDocService()
			appendDoc := func(op string) error {
				if pathAPI {
					_, _, _, err := svc.AppendWithPath(base, "text", op, "test", "test", nil)
					return err
				}
				_, _, _, err := svc.Append("P", name, "requirements", "text", "test", nil, op, "test")
				return err
			}
			metaPath := filepath.Join(base, "meta.json")
			if err := os.WriteFile(metaPath, []byte("{broken"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := appendDoc("replace_full"); err == nil {
				t.Fatal("corrupt meta accepted")
			}
			var err error
			if pathAPI {
				_, _, err = ExportWithPath(base)
			} else {
				_, err = LoadOrInitMeta("P", name, "requirements")
			}
			if err == nil {
				t.Fatal("corrupt meta export succeeded")
			}
			if pathAPI {
				_, _, err = svc.ListWithPath(base)
			} else {
				_, _, err = svc.List("P", name, "requirements")
			}
			if err == nil {
				t.Fatal("corrupt meta list succeeded")
			}
			raw, err := os.ReadFile(metaPath)
			if err != nil || string(raw) != "{broken" {
				t.Fatal("corrupt metadata overwritten")
			}
			if err := os.Remove(metaPath); err != nil {
				t.Fatal(err)
			}
			if err := appendDoc("replace_full"); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(base, "compiled.md")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(base, "compiled.md"), 0755); err != nil {
				t.Fatal(err)
			}
			for _, op := range []string{"replace_full", "section_update_no_parse", "append"} {
				if err := appendDoc(op); err == nil {
					t.Fatalf("compiled failure swallowed: %s", op)
				}
			}
		})
	}
}

func TestMixedAppendInstancesCAS(t *testing.T) {
	root := t.TempDir()
	previous := projects.ProjectsRoot
	projects.ProjectsRoot = root
	t.Cleanup(func() { projects.ProjectsRoot = previous })
	base := filepath.Join(root, "P", "tasks", "T", "docs", "requirements")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	first, second := NewDocService(), NewDocService()
	version := 0
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, _, _, err := first.Append("P", "T", "requirements", "legacy", "test", &version, "replace_full", "test")
		results <- err
	}()
	go func() {
		<-start
		_, _, _, err := second.AppendWithPath(base, "path", "replace_full", "test", "test", &version)
		results <- err
	}()
	close(start)
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if err.Error() == "version_mismatch" {
			conflict++
		} else {
			t.Error(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	chunks, meta, err := second.ListWithPath(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || meta.Version != 1 || meta.LastSequence != 1 {
		t.Fatalf("chunks=%d meta=%+v", len(chunks), meta)
	}
	content, observed, err := ExportWithPath(base)
	if err != nil {
		t.Fatal(err)
	}
	if content != chunks[0].Content || observed.ETag != hashDocContent(content) {
		t.Fatalf("inconsistent content: %q", content)
	}
	appendPathLocks.Lock()
	remaining := len(appendPathLocks.entries)
	appendPathLocks.Unlock()
	if remaining != 0 {
		t.Fatalf("lock entries leaked: %d", remaining)
	}
}
