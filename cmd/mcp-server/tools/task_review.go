package tools

import (
	"fmt"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

func reviewIDs(args map[string]interface{}) (string, string, error) {
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil {
		return "", "", fmt.Errorf("explicit project_id is required")
	}
	taskID, err := shared.SafeGetString(args, "task_id")
	if err != nil {
		return "", "", fmt.Errorf("explicit task_id is required")
	}
	return projectID, taskID, nil
}

func reviewIDProperties() map[string]interface{} {
	return map[string]interface{}{
		"project_id": map[string]interface{}{"type": "string", "description": "项目ID（必填）"},
		"task_id":    map[string]interface{}{"type": "string", "description": "任务ID（必填）"},
	}
}

type GetTaskReviewTool struct{}

func (t *GetTaskReviewTool) Name() string { return "get_task_review" }
func (t *GetTaskReviewTool) Description() string {
	return "读取任务审核文档及版本信息。必须显式提供 project_id 和 task_id。"
}
func (t *GetTaskReviewTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": reviewIDProperties(), "required": []string{"project_id", "task_id"}}
}
func (t *GetTaskReviewTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, taskID, err := reviewIDs(args)
	if err != nil {
		return "", err
	}
	return shared.CallAPI(apiClient, "GET", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/review", projectID, taskID), nil, clientToken)
}

type CreateTaskReviewTool struct{}

func (t *CreateTaskReviewTool) Name() string { return "create_task_review" }
func (t *CreateTaskReviewTool) Description() string {
	return "按固定 Mote AIDG Lite 格式创建审核请求并将任务推进到 review。"
}
func (t *CreateTaskReviewTool) InputSchema() map[string]interface{} {
	properties := reviewIDProperties()
	properties["target_type"] = map[string]interface{}{"type": "string", "enum": []string{"文件", "AIDG文档", "代码"}}
	properties["target_path"] = map[string]interface{}{"type": "string"}
	properties["criteria"] = map[string]interface{}{"type": "string"}
	properties["expected_revision"] = map[string]interface{}{"type": "number", "minimum": 1}
	return map[string]interface{}{"type": "object", "properties": properties, "required": []string{"project_id", "task_id", "target_type", "target_path", "criteria", "expected_revision"}}
}
func (t *CreateTaskReviewTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, taskID, err := reviewIDs(args)
	if err != nil {
		return "", err
	}
	expectedRevision, err := shared.SafeGetInt(args, "expected_revision")
	if err != nil || expectedRevision < 1 {
		return "", fmt.Errorf("expected_revision must be a positive integer")
	}
	body := map[string]interface{}{"expected_revision": expectedRevision}
	for _, key := range []string{"target_type", "target_path", "criteria"} {
		value, ok := args[key]
		if !ok {
			return "", fmt.Errorf("explicit %s is required", key)
		}
		body[key] = value
	}
	return shared.CallAPI(apiClient, "POST", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/review", projectID, taskID), body, clientToken)
}

type UpdateTaskReviewResultTool struct{}

func (t *UpdateTaskReviewResultTool) Name() string { return "update_task_review_result" }
func (t *UpdateTaskReviewResultTool) Description() string {
	return "更新审核结果；通过时必须提供变更摘要和验证证据，仅服务端认证身份可成为 reviewer。"
}
func (t *UpdateTaskReviewResultTool) InputSchema() map[string]interface{} {
	properties := reviewIDProperties()
	properties["issues"] = map[string]interface{}{"type": "string", "description": "每行以 '- ' 开头；无问题可省略"}
	properties["result"] = map[string]interface{}{"type": "string", "enum": []string{"通过", "不通过"}}
	properties["change_summary"] = map[string]interface{}{"type": "string"}
	properties["verification"] = map[string]interface{}{"type": "string"}
	properties["expected_revision"] = map[string]interface{}{"type": "number", "minimum": 1}
	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   []string{"project_id", "task_id", "result", "expected_revision"},
		"allOf": []interface{}{
			map[string]interface{}{
				"if":   map[string]interface{}{"properties": map[string]interface{}{"result": map[string]interface{}{"const": "通过"}}},
				"then": map[string]interface{}{"required": []string{"change_summary", "verification"}},
			},
		},
	}
}
func (t *UpdateTaskReviewResultTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, taskID, err := reviewIDs(args)
	if err != nil {
		return "", err
	}
	expectedRevision, err := shared.SafeGetInt(args, "expected_revision")
	if err != nil || expectedRevision < 1 {
		return "", fmt.Errorf("expected_revision must be a positive integer")
	}
	body := map[string]interface{}{"expected_revision": expectedRevision}
	for _, key := range []string{"issues", "result", "change_summary", "verification"} {
		if value, ok := args[key]; ok {
			body[key] = value
		}
	}
	result, err := shared.SafeGetString(args, "result")
	if err != nil || (result != "通过" && result != "不通过") {
		return "", fmt.Errorf("result must be 通过 or 不通过")
	}
	body["result"] = result
	if result == "通过" {
		changeSummary, summaryErr := shared.SafeGetString(args, "change_summary")
		verification, verificationErr := shared.SafeGetString(args, "verification")
		if summaryErr != nil || verificationErr != nil {
			return "", fmt.Errorf("passed review requires change_summary and verification")
		}
		body["change_summary"], body["verification"] = changeSummary, verification
	}
	return shared.CallAPI(apiClient, "PUT", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/review/result", projectID, taskID), body, clientToken)
}
