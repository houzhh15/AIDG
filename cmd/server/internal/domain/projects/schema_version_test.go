package projects

import (
	"os"
	"testing"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/config"
)

func useProjectsRoot(t *testing.T, root string) {
	t.Helper()
	previous := config.GlobalConfig
	config.GlobalConfig = &config.Config{Data: config.DataConfig{ProjectsDir: root}}
	t.Cleanup(func() { config.GlobalConfig = previous })
	InitPaths()
}

func TestLoadLegacyProjectKeepsExplicitZeroWithoutWriting(t *testing.T) {
	root := t.TempDir()
	useProjectsRoot(t, root)
	legacy := []byte(`{"projects":[{"id":"old","name":"old","product_line":"","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}]}`)
	if err := os.WriteFile(ProjectsStatePath, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(ProjectsStatePath)
	if err != nil {
		t.Fatal(err)
	}
	registry := NewProjectRegistry()
	if err := LoadProjects(registry); err != nil {
		t.Fatal(err)
	}
	if project := registry.Get("old"); project == nil || project.SchemaVersion != 0 {
		t.Fatalf("legacy schema version = %+v", project)
	}
	after, err := os.Stat(ProjectsStatePath)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(ProjectsStatePath)
	if string(data) != string(legacy) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("legacy read modified project registry")
	}
}

func TestSaveProjectsPersistsSchemaVersionDeterministically(t *testing.T) {
	root := t.TempDir()
	useProjectsRoot(t, root)
	registry := NewProjectRegistry()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	registry.Set(&Project{ID: "z", Name: "z", SchemaVersion: CurrentSchemaVersion, CreatedAt: now, UpdatedAt: now})
	registry.Set(&Project{ID: "a", Name: "a", SchemaVersion: CurrentSchemaVersion, CreatedAt: now, UpdatedAt: now})
	if err := SaveProjects(registry); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(ProjectsStatePath)
	if err := SaveProjects(registry); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(ProjectsStatePath)
	if string(first) != string(second) {
		t.Fatal("project serialization is nondeterministic")
	}
	loaded := NewProjectRegistry()
	if err := LoadProjects(loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Get("a").SchemaVersion != CurrentSchemaVersion {
		t.Fatal("schema version was not persisted")
	}
}
