package tools

import (
	"strings"
	"testing"
)

func TestMigrationToolsRequireExplicitProjectButAllowProjectScope(t *testing.T) {
	for _, tool := range []interface{ InputSchema() map[string]interface{} }{
		&PreviewSchemaMigrationTool{},
		&ExecuteSchemaMigrationTool{},
	} {
		schema := tool.InputSchema()
		required, _ := schema["required"].([]string)
		if len(required) == 0 || required[0] != "project_id" {
			t.Fatalf("project_id not required: %#v", required)
		}
		for _, field := range required {
			if field == "task_id" {
				t.Fatalf("task_id must remain optional for project scope: %#v", required)
			}
		}
	}
}

func TestMigrationPathSelectsScopeAndEscapesIDs(t *testing.T) {
	_, projectPath, err := migrationPath(map[string]interface{}{"project_id": "project a"}, "preview")
	if err != nil || projectPath != "/api/v1/projects/project%20a/migrations/schema/preview" {
		t.Fatalf("project path=%q err=%v", projectPath, err)
	}
	_, taskPath, err := migrationPath(map[string]interface{}{"project_id": "project a", "task_id": "task one"}, "execute")
	if err != nil || taskPath != "/api/v1/projects/project%20a/tasks/task%20one/migrations/schema/execute" {
		t.Fatalf("task path=%q err=%v", taskPath, err)
	}
	_, traversalPath, err := migrationPath(map[string]interface{}{"project_id": "../secret"}, "preview")
	if err != nil || strings.Contains(traversalPath, "../") {
		t.Fatalf("unescaped path=%q err=%v", traversalPath, err)
	}
}

func TestMigrationPathRejectsMalformedTaskScope(t *testing.T) {
	for _, value := range []interface{}{123, map[string]interface{}{"id": "task"}, "", "   "} {
		_, path, err := migrationPath(map[string]interface{}{"project_id": "project-a", "task_id": value}, "execute")
		if err == nil || path != "" {
			t.Fatalf("task_id=%#v widened to path %q without error", value, path)
		}
	}
}
