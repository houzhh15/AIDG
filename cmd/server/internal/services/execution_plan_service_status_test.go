package services

import (
	"testing"
)

func TestValidateAndFixPlanStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// 标准状态应该保持不变
		{"Standard Draft", "Draft", "Draft"},
		{"Standard Pending Approval", "Pending Approval", "Pending Approval"},
		{"Standard Approved", "Approved", "Approved"},
		{"Standard Rejected", "Rejected", "Rejected"},
		{"Standard Executing", "Executing", "Executing"},
		{"Standard Completed", "Completed", "Completed"},
		{"Standard Failed", "Failed", "Failed"},

		// 常见的非标准状态映射
		{"In Progress", "in-progress", "Executing"},
		{"In Progress Alt", "in_progress", "Executing"},
		{"In Progress No Space", "inprogress", "Executing"},
		{"Running", "running", "Executing"},
		{"Done", "done", "Completed"},
		{"Finished", "finished", "Completed"},
		{"Complete", "complete", "Completed"},
		{"Pending", "pending", "Pending Approval"},
		{"Waiting", "waiting", "Pending Approval"},
		{"Review", "review", "Pending Approval"},
		{"New", "new", "Draft"},
		{"OK", "ok", "Approved"},
		{"Ready", "ready", "Approved"},
		{"Denied", "denied", "Rejected"},
		{"Cancelled", "cancelled", "Rejected"},
		{"Error", "error", "Failed"},
		{"Failure", "failure", "Failed"},

		// 大小写测试
		{"Mixed Case", "IN-PROGRESS", "Executing"},
		{"Upper Case", "PENDING", "Pending Approval"},

		// 边界情况
		{"Empty String", "", "Pending Approval"},
		{"Whitespace", "  ", "Pending Approval"},
		{"Random String", "some-random-status", "Pending Approval"},
		{"Numbers", "123", "Pending Approval"},
		{"Special Chars", "!@#$%", "Pending Approval"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateAndFixPlanStatus(tt.input)
			if result != tt.expected {
				t.Errorf("validateAndFixPlanStatus(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseFrontMatterWithInvalidStatus(t *testing.T) {
	// 测试包含无效状态的 front matter
	yamlContent := `plan_id: test
task_id: test_task
status: in-progress
created_at: '2025-10-01T10:00:00Z'
updated_at: '2025-10-01T10:00:00Z'`

	meta, err := parseFrontMatter(yamlContent)
	if err != nil {
		t.Fatalf("parseFrontMatter failed: %v", err)
	}

	// 验证状态被正确修正
	if meta.Status != "Executing" {
		t.Errorf("Expected status to be 'Executing', got %q", meta.Status)
	}
}

func TestParseFrontMatterWithEmptyStatus(t *testing.T) {
	// 测试包含空状态的 front matter
	yamlContent := `plan_id: test
task_id: test_task
status: ""
created_at: '2025-10-01T10:00:00Z'
updated_at: '2025-10-01T10:00:00Z'`

	meta, err := parseFrontMatter(yamlContent)
	if err != nil {
		t.Fatalf("parseFrontMatter failed: %v", err)
	}

	// 验证空状态被设为默认值
	if meta.Status != "Pending Approval" {
		t.Errorf("Expected status to be 'Pending Approval', got %q", meta.Status)
	}
}
