package tools

import (
	"fmt"
	"github.com/houzhh15-hub/AIDG/cmd/mcp-server/shared"
)

// ListAllMeetingsTool 获取所有会议列表工具
// 对应后端 API: GET /api/v1/tasks
type ListAllMeetingsTool struct{}

// Name 返回工具名称
func (t *ListAllMeetingsTool) Name() string {
	return "list_all_meetings"
}

// Description 返回工具描述
func (t *ListAllMeetingsTool) Description() string {
	return "获取所有会议列表"
}

// InputSchema 返回输入参数的JSON Schema
func (t *ListAllMeetingsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// Execute 执行工具，获取所有会议列表
func (t *ListAllMeetingsTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	resp, err := shared.CallAPI(apiClient, "GET", "/api/v1/tasks", nil, clientToken)
	if err != nil {
		return fmt.Sprintf("⚠️  API服务器不可用\n\n后端API服务器 (%s) 当前不可用。\n错误详情: %v\n\n请确保会议记录API服务器正在运行。",
			apiClient.BaseURL, err), nil
	}

	return string(resp), nil
}
