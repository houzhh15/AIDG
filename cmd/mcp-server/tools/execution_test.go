package tools

import (
	"strings"
	"testing"
)

func TestExecutionPlanWriteToolsAreReadOnly(t *testing.T) {
	args := map[string]interface{}{"project_id": "p", "task_id": "t", "content": "mutate", "step_id": "s", "status": "succeeded"}
	for name, execute := range map[string]func() error{
		"update_plan": func() error {
			_, err := (&UpdateExecutionPlanTool{}).Execute(args, "token", nil)
			return err
		},
		"update_step": func() error {
			_, err := (&UpdatePlanStepStatusTool{}).Execute(args, "token", nil)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := execute()
			if err == nil || !strings.Contains(err.Error(), "historical_resource_read_only") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestReviewToolsRequireExplicitIDs(t *testing.T) {
	if _, _, err := reviewIDs(map[string]interface{}{"project_id": "p"}); err == nil {
		t.Fatal("review tool accepted missing task_id")
	}
	if _, _, err := reviewIDs(map[string]interface{}{"task_id": "t"}); err == nil {
		t.Fatal("review tool accepted missing project_id")
	}
}

func TestMutationToolsRejectNonNumericRevisionBeforeHTTP(t *testing.T) {
	updateArgs := map[string]interface{}{"project_id": "p", "task_id": "t", "expected_revision": "fake", "name": "changed"}
	if _, err := (&UpdateProjectTaskTool{}).Execute(updateArgs, "token", nil); err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("task update accepted fake revision: %v", err)
	}
	reviewArgs := map[string]interface{}{"project_id": "p", "task_id": "t", "expected_revision": "fake", "result": "通过", "change_summary": "x", "verification": "y"}
	if _, err := (&UpdateTaskReviewResultTool{}).Execute(reviewArgs, "token", nil); err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("review result accepted fake revision: %v", err)
	}
}
