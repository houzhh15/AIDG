package tools

import (
	"fmt"
	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
	"strings"
)

// GetExecutionPlanTool 获取执行计划工具
type GetExecutionPlanTool struct{}

func (t *GetExecutionPlanTool) Name() string {
	return "get_execution_plan"
}

func (t *GetExecutionPlanTool) Description() string {
	return "获取执行计划"
}

func (t *GetExecutionPlanTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{"type": "string", "description": "项目ID"},
			"task_id":    map[string]interface{}{"type": "string", "description": "任务ID"},
		},
		"required": []string{"project_id", "task_id"},
	}
}

func (t *GetExecutionPlanTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, _ := shared.SafeGetString(args, "project_id")
	taskID, _ := shared.SafeGetString(args, "task_id")
	return shared.CallAPI(apiClient, "GET", fmt.Sprintf("/internal/api/v1/projects/%s/tasks/%s/execution-plan", projectID, taskID), nil, clientToken)
}

// UpdateExecutionPlanTool 更新执行计划工具
type UpdateExecutionPlanTool struct{}

func (t *UpdateExecutionPlanTool) Name() string {
	return "update_execution_plan"
}

func (t *UpdateExecutionPlanTool) Description() string {
	return "全文覆盖更新执行计划"
}

func (t *UpdateExecutionPlanTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{"type": "string", "description": "项目ID"},
			"task_id":    map[string]interface{}{"type": "string", "description": "任务ID"},
			"content":    map[string]interface{}{"type": "string", "description": "执行计划 Markdown 全文"},
		},
		"required": []string{"project_id", "task_id", "content"},
	}
}

func (t *UpdateExecutionPlanTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, _ := shared.SafeGetString(args, "project_id")
	taskID, _ := shared.SafeGetString(args, "task_id")
	content, _ := shared.SafeGetString(args, "content")
	return shared.CallAPI(apiClient, "POST", fmt.Sprintf("/internal/api/v1/projects/%s/tasks/%s/execution-plan", projectID, taskID), map[string]interface{}{"content": content}, clientToken)
}

// GetNextExecutableStepTool 获取下一个可执行步骤工具
type GetNextExecutableStepTool struct{}

func (t *GetNextExecutableStepTool) Name() string {
	return "get_next_executable_step"
}

func (t *GetNextExecutableStepTool) Description() string {
	return "获取执行计划中下一个可执行的步骤"
}

func (t *GetNextExecutableStepTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{"type": "string", "description": "项目ID"},
			"task_id":    map[string]interface{}{"type": "string", "description": "任务ID"},
		},
		"required": []string{"project_id", "task_id"},
	}
}

func (t *GetNextExecutableStepTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, _ := shared.SafeGetString(args, "project_id")
	taskID, _ := shared.SafeGetString(args, "task_id")
	return shared.CallAPI(apiClient, "GET", fmt.Sprintf("/internal/api/v1/projects/%s/tasks/%s/execution-plan/next-step", projectID, taskID), nil, clientToken)
}

// UpdatePlanStepStatusTool 更新执行计划步骤状态工具
type UpdatePlanStepStatusTool struct{}

func (t *UpdatePlanStepStatusTool) Name() string {
	return "update_plan_step_status"
}

func (t *UpdatePlanStepStatusTool) Description() string {
	return "更新执行计划中单个步骤的状态。状态值必须是以下之一：pending（待开始）、in-progress（进行中）、succeeded（成功完成）、failed（失败）、cancelled（已取消）"
}

func (t *UpdatePlanStepStatusTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "任务ID",
			},
			"step_id": map[string]interface{}{
				"type":        "string",
				"description": "步骤ID",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "步骤状态",
				"enum":        []string{"pending", "in-progress", "succeeded", "failed", "cancelled"},
			},
			"output": map[string]interface{}{
				"type":        "string",
				"description": "步骤执行输出（可选）",
			},
		},
		"required": []string{"project_id", "task_id", "step_id", "status"},
	}
}

func (t *UpdatePlanStepStatusTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil || projectID == "" {
		return "", fmt.Errorf("参数错误：project_id 是必需的字符串参数。请提供项目ID，例如：\"AI-Dev-Gov\"")
	}

	taskID, err := shared.SafeGetString(args, "task_id")
	if err != nil || taskID == "" {
		return "", fmt.Errorf("参数错误：task_id 是必需的字符串参数。请提供任务ID，例如：\"task_1759401721\"")
	}

	stepID, err := shared.SafeGetString(args, "step_id")
	if err != nil || stepID == "" {
		return "", fmt.Errorf("参数错误：step_id 是必需的字符串参数。请提供步骤ID，例如：\"step-01\"")
	}

	status, err := shared.SafeGetString(args, "status")
	if err != nil || status == "" {
		return "", fmt.Errorf("参数错误：status 是必需的字符串参数。请提供有效的状态值")
	}

	// 验证状态值
	validStatuses := []string{"pending", "in-progress", "succeeded", "failed", "cancelled"}
	isValid := false
	for _, validStatus := range validStatuses {
		if strings.ToLower(strings.TrimSpace(status)) == validStatus {
			status = validStatus // 规范化状态值
			isValid = true
			break
		}
	}

	if !isValid {
		return "", fmt.Errorf("参数错误：status 值 \"%s\" 无效。\n\n有效的状态值为：\n  - \"pending\"      (待开始)\n  - \"in-progress\"  (进行中)\n  - \"succeeded\"    (成功完成) ← 用这个表示已完成\n  - \"failed\"       (失败)\n  - \"cancelled\"    (已取消)\n\n提示：如果要标记步骤为已完成，请使用 \"succeeded\" 而不是 \"completed\"。\n\n正确的调用示例：\n{\n  \"project_id\": \"%s\",\n  \"task_id\": \"%s\",\n  \"step_id\": \"%s\",\n  \"status\": \"succeeded\",\n  \"output\": \"步骤执行成功的详细说明\"\n}", status, projectID, taskID, stepID)
	}

	output, _ := shared.SafeGetString(args, "output")

	body := map[string]interface{}{"status": status, "output": output}
	return shared.CallAPI(apiClient, "POST", fmt.Sprintf("/internal/api/v1/projects/%s/tasks/%s/execution-plan/steps/%s/status", projectID, taskID, stepID), body, clientToken)
}
