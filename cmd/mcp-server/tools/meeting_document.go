package tools

import (
	"fmt"
	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// GetMeetingDocumentTool 获取会议文档通用工具
type GetMeetingDocumentTool struct {
	Registry *shared.SlotRegistry
}

func (t *GetMeetingDocumentTool) Name() string {
	return "get_meeting_document"
}

func (t *GetMeetingDocumentTool) Description() string {
	return "获取会议的指定槽位文档内容。支持的槽位：meeting_info（会议信息）, polish（详细记录）, context（背景）, summary（总结）, topic（话题）, merged_all（原始转录）, polish_all（润色记录）, feature_list（特性列表）, architecture_design（架构设计）"
}

func (t *GetMeetingDocumentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"meeting_id": map[string]interface{}{
				"type":        "string",
				"description": "会议任务ID",
			},
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名",
				"enum": []string{
					"meeting_info", "polish", "context", "summary",
					"topic", "merged_all", "polish_all",
					"feature_list", "architecture_design",
				},
			},
		},
		"required": []string{"meeting_id", "slot_key"},
	}
}

func (t *GetMeetingDocumentTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取参数
	meetingID, err := shared.SafeGetString(args, "meeting_id")
	if err != nil {
		return "", fmt.Errorf("get_meeting_document: %w", err)
	}
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("get_meeting_document: %w", err)
	}

	// 2. 验证槽位
	if err := t.Registry.ValidateMeetingSlot(slotKey); err != nil {
		return "", fmt.Errorf("get_meeting_document: 无效的槽位 '%s': %w", slotKey, err)
	}

	// 3. 获取 API 路径
	path, err := t.Registry.GetMeetingAPIPath(slotKey, "GET", meetingID)
	if err != nil {
		return "", fmt.Errorf("get_meeting_document: %w", err)
	}

	// 4. 调用 API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// UpdateMeetingDocumentTool 更新会议文档通用工具
type UpdateMeetingDocumentTool struct {
	Registry *shared.SlotRegistry
}

func (t *UpdateMeetingDocumentTool) Name() string {
	return "update_meeting_document"
}

func (t *UpdateMeetingDocumentTool) Description() string {
	return "更新会议的指定槽位文档内容。支持的槽位：summary（总结）, topic（话题）, polish_all（润色记录）, feature_list（特性列表）, architecture_design（架构设计）"
}

func (t *UpdateMeetingDocumentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"meeting_id": map[string]interface{}{
				"type":        "string",
				"description": "会议任务ID",
			},
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名",
				"enum": []string{
					"summary", "topic", "polish_all",
					"feature_list", "architecture_design",
				},
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "文档内容",
			},
		},
		"required": []string{"meeting_id", "slot_key", "content"},
	}
}

func (t *UpdateMeetingDocumentTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取参数
	meetingID, err := shared.SafeGetString(args, "meeting_id")
	if err != nil {
		return "", fmt.Errorf("update_meeting_document: %w", err)
	}
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("update_meeting_document: %w", err)
	}
	content, err := shared.SafeGetString(args, "content")
	if err != nil {
		return "", fmt.Errorf("update_meeting_document: %w", err)
	}

	// 2. 验证槽位
	if err := t.Registry.ValidateMeetingSlot(slotKey); err != nil {
		return "", fmt.Errorf("update_meeting_document: 无效的槽位 '%s': %w", slotKey, err)
	}

	// 3. 获取 API 路径
	path, err := t.Registry.GetMeetingAPIPath(slotKey, "PUT", meetingID)
	if err != nil {
		return "", fmt.Errorf("update_meeting_document: %w", err)
	}

	// 4. 构造请求体并调用 API
	body := map[string]string{"content": content}
	return shared.CallAPI(apiClient, "PUT", path, body, clientToken)
}
