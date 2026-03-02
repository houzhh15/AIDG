package tools

import (
	"encoding/json"
	"fmt"
	"strings"

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
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "任务ID（可选，缺失时从当前任务获取）",
			},
		},
		"required": []string{},
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
	// 提取并验证 project_id 和 task_id 参数（使用回退机制）
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(args, apiClient, clientToken)
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

// GetUserProjectsTool 实现获取当前用户项目列表的工具
// 对应后端 API: GET /api/v1/user/projects
type GetUserProjectsTool struct{}

// Name 返回工具名称
func (t *GetUserProjectsTool) Name() string {
	return "get_user_projects"
}

// Description 返回工具描述
func (t *GetUserProjectsTool) Description() string {
	return "获取当前登录用户的项目列表，包含每个项目的可见性设置。返回所有项目及其 visible 状态。"
}

// InputSchema 返回输入参数的JSON Schema
func (t *GetUserProjectsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"visible_only": map[string]interface{}{
				"type":        "boolean",
				"description": "是否只返回可见的项目（默认 false，返回所有项目）",
			},
		},
	}
}

// Execute 执行工具，获取当前用户的项目列表
func (t *GetUserProjectsTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 调用后端 API
	resp, err := shared.CallAPI(apiClient, "GET", "/api/v1/user/projects", nil, clientToken)
	if err != nil {
		return fmt.Sprintf("⚠️  API服务器不可用\n\n后端API服务器 (%s) 当前不可用。\n错误详情: %v\n\n请确保API服务器正在运行。",
			apiClient.BaseURL, err), nil
	}

	// 检查是否需要只返回可见项目
	visibleOnly, _ := shared.SafeGetBool(args, "visible_only")
	if visibleOnly {
		var result struct {
			Success bool `json:"success"`
			Data    []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				ProductLine string `json:"product_line"`
				Visible     bool   `json:"visible"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(resp), &result); err == nil {
			var visibleProjects []map[string]interface{}
			for _, p := range result.Data {
				if p.Visible {
					visibleProjects = append(visibleProjects, map[string]interface{}{
						"id":           p.ID,
						"name":         p.Name,
						"product_line": p.ProductLine,
						"visible":      true,
					})
				}
			}
			// 构建简化的输出
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("可见项目列表 (%d 个):\n\n", len(visibleProjects)))
			for i, p := range visibleProjects {
				sb.WriteString(fmt.Sprintf("%d. %s (ID: %s)", i+1, p["name"], p["id"]))
				if pl, ok := p["product_line"].(string); ok && pl != "" {
					sb.WriteString(fmt.Sprintf(" [%s]", pl))
				}
				sb.WriteString("\n")
			}
			return sb.String(), nil
		}
	}

	return resp, nil
}
