package services

import (
	"testing"
)

func TestParseExecutionPlanWithStatusValidation(t *testing.T) {
	// 模拟LLM生成的包含不标准状态的执行计划
	content := `---
project_id: AI-Dev-Gov
task_id: task_1758707040
name: 测试LLM生成不稳定状态的容错
description: 这是一个包含不标准状态的测试用例
assignee: coder
status: in-progress
created_at: '2025-10-01T10:00:00Z'
updated_at: '2025-10-01T16:35:00Z'
version: 1.0
---

# 测试执行计划

这是一个用于测试状态容错机制的执行计划文件。

## 测试步骤

#### step-01: 测试步骤 ✅
- **状态**: succeeded
- **描述**: 这是一个测试步骤`

	plan, err := parseExecutionPlan(content)
	if err != nil {
		t.Fatalf("parseExecutionPlan failed: %v", err)
	}

	// 验证状态被正确修正为标准状态
	if plan.Status != "Executing" {
		t.Errorf("Expected status to be 'Executing' (converted from 'in-progress'), got %q", plan.Status)
	}

	// 验证其他字段保持正确
	if plan.TaskID != "task_1758707040" {
		t.Errorf("Expected task_id to be 'task_1758707040', got %q", plan.TaskID)
	}

	// 验证步骤解析正常
	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	if plan.Steps[0].ID != "step-01" {
		t.Errorf("Expected step ID to be 'step-01', got %q", plan.Steps[0].ID)
	}
}

func TestStatusValidationEdgeCases(t *testing.T) {
	testCases := []struct {
		name           string
		inputStatus    string
		expectedStatus string
	}{
		{"LLM Common: in-progress", "in-progress", "Executing"},
		{"LLM Common: pending", "pending", "Pending Approval"},
		{"LLM Common: done", "done", "Completed"},
		{"LLM Variation: RUNNING", "RUNNING", "Executing"},
		{"LLM Variation: In Progress", "In Progress", "Pending Approval"}, // 不匹配精确模式，回到默认
		{"LLM Random: processing", "processing", "Pending Approval"},
		{"LLM Empty", "", "Pending Approval"},
		{"LLM Typo: aproved", "aproved", "Pending Approval"},
		{"LLM Chinese: 进行中", "进行中", "Pending Approval"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := `---
project_id: test
task_id: test_task
status: ` + tc.inputStatus + `
created_at: '2025-10-01T10:00:00Z'
updated_at: '2025-10-01T16:35:00Z'
---

# Test Plan

Test content.`

			plan, err := parseExecutionPlan(content)
			if err != nil {
				t.Fatalf("parseExecutionPlan failed: %v", err)
			}

			if string(plan.Status) != tc.expectedStatus {
				t.Errorf("For input status %q, expected %q, got %q", tc.inputStatus, tc.expectedStatus, plan.Status)
			}
		})
	}
}
