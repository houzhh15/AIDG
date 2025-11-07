package tools

import (
	"fmt"
	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// GetUserCurrentTaskTool 实现获取当前用户任务信息的工具
// 对应后端 API: GET /api/v1/user/current-task
type GetUserCurrentTaskTool struct{}

// Name 返回工具名称
func (t *GetUserCurrentTaskTool) Name() string {
	return "get_user_current_task"
}

// Description 返回工具描述
func (t *GetUserCurrentTaskTool) Description() string {
	return "获取当前登录用户所选择的当前任务信息（包含project_id, task_id等）"
}

// InputSchema 返回输入参数的JSON Schema
// get_user_current_task 不需要输入参数
func (t *GetUserCurrentTaskTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// Execute 执行工具，获取当前用户的任务信息
//
// 参数:
//   - args: 工具参数（此工具无参数）
//   - clientToken: 客户端认证 token
//   - apiClient: API 客户端实例
//
// 返回:
//   - string: JSON 格式的任务信息（包含 project_id, task_id 等）
//   - error: 错误信息（API 请求失败时返回友好提示）
func (t *GetUserCurrentTaskTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 调用后端 API
	resp, err := shared.CallAPI(apiClient, "GET", "/api/v1/user/current-task", nil, clientToken)
	if err != nil {
		return fmt.Sprintf("⚠️  API服务器不可用\n\n后端API服务器 (%s) 当前不可用。\n错误详情: %v\n\n请确保会议记录API服务器正在运行。",
			apiClient.BaseURL, err), nil
	}

	return string(resp), nil
}

// SetUserCurrentTaskTool 实现设置当前用户任务绑定的工具
// 对应后端 API: PUT /api/v1/user/current-task (设置) + GET /api/v1/user/current-task (验证)
type SetUserCurrentTaskTool struct{}

// Name 返回工具名称
func (t *SetUserCurrentTaskTool) Name() string {
	return "set_user_current_task"
}

// Description 返回工具描述
func (t *SetUserCurrentTaskTool) Description() string {
	return "设置并返回当前用户的任务绑定 (PUT /api/v1/user/current-task 然后 GET 验证)"
}

// InputSchema 返回输入参数的JSON Schema
// set_user_current_task 需要 project_id 和 task_id 两个参数
func (t *SetUserCurrentTaskTool) InputSchema() map[string]interface{} {
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
		},
		"required": []string{"project_id", "task_id"},
	}
}

// Execute 执行工具，设置当前用户的任务绑定
//
// 参数:
//   - args: 工具参数，必须包含 "project_id" 和 "task_id"
//   - clientToken: 客户端认证 token
//   - apiClient: API 客户端实例
//
// 返回:
//   - string: JSON 格式的任务信息（设置后通过 GET 验证的结果）
//   - error: 错误信息（参数缺失或 API 请求失败时）
func (t *SetUserCurrentTaskTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 提取并验证 project_id 参数
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil {
		return "", fmt.Errorf("set_user_current_task: %w", err)
	}

	// 提取并验证 task_id 参数
	taskID, err := shared.SafeGetString(args, "task_id")
	if err != nil {
		return "", fmt.Errorf("set_user_current_task: %w", err)
	}

	// 构造请求 body
	body := map[string]string{
		"project_id": projectID,
		"task_id":    taskID,
	}

	// 第一步：调用 PUT 请求设置任务绑定
	// 注意：直接传递 map，CallAPI 会自动进行 JSON 序列化
	_, err = shared.CallAPI(apiClient, "PUT", "/api/v1/user/current-task", body, clientToken)
	if err != nil {
		return fmt.Sprintf("⚠️  API服务器不可用\n\n后端API服务器 (%s) 当前不可用。\n错误详情: %v\n\n请确保会议记录API服务器正在运行。",
			apiClient.BaseURL, err), nil
	}

	// 第二步：调用 GET 请求验证并返回最新的任务绑定
	resp, err := shared.CallAPI(apiClient, "GET", "/api/v1/user/current-task", nil, clientToken)
	if err != nil {
		return fmt.Sprintf("⚠️  API服务器不可用\n\n后端API服务器 (%s) 当前不可用。\n错误详情: %v\n\n请确保会议记录API服务器正在运行。",
			apiClient.BaseURL, err), nil
	}

	return string(resp), nil
}
