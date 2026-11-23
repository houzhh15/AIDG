package shared

import "testing"

func TestFallbackHelpersRequireExplicitIdentifiers(t *testing.T) {
	if _, err := GetProjectIDWithFallback(map[string]interface{}{}, nil, ""); err == nil {
		t.Fatal("project helper accepted implicit current-task fallback")
	}
	if _, err := GetTaskIDWithFallback(map[string]interface{}{}, nil, ""); err == nil {
		t.Fatal("task helper accepted implicit current-task fallback")
	}
	if _, _, err := GetProjectAndTaskIDWithFallback(map[string]interface{}{"project_id": "p"}, nil, ""); err == nil {
		t.Fatal("combined helper accepted missing task_id")
	}
	projectID, taskID, err := GetProjectAndTaskIDWithFallback(map[string]interface{}{"project_id": "p", "task_id": "t"}, nil, "")
	if err != nil || projectID != "p" || taskID != "t" {
		t.Fatalf("explicit identifiers rejected: %q %q %v", projectID, taskID, err)
	}
}
