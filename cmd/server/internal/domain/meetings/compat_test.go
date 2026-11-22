package meetings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/houzhh15/AIDG/cmd/server/internal/config"
)

func TestCompatServerTasksFixturesLoad(t *testing.T) {
	fixtureRoot := filepath.Clean(filepath.Join("..", "..", "..", "..", "..", "test", "compat", "server_tasks"))
	fixtures, err := filepath.Glob(filepath.Join(fixtureRoot, "*.json"))
	if err != nil || len(fixtures) != 3 {
		t.Fatalf("fixtures=%v err=%v", fixtures, err)
	}
	previousConfig := config.GlobalConfig
	defer func() { config.GlobalConfig = previousConfig; InitPaths() }()
	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "server_tasks.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			config.GlobalConfig = &config.Config{}
			config.GlobalConfig.Data.MeetingsDir = dir
			InitPaths()
			registry := NewRegistry()
			if err := LoadTasks(registry); err != nil {
				t.Fatalf("fixture is not loadable by current compatibility loader: %v", err)
			}
			if len(registry.List()) != 1 {
				t.Fatalf("loaded tasks=%d", len(registry.List()))
			}
		})
	}
}
