package tools

import (
	"fmt"
	"strings"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// GetMeetingDocSectionsTool 获取会议文档的章节列表
type GetMeetingDocSectionsTool struct{}

func (t *GetMeetingDocSectionsTool) Name() string {
	return "get_meeting_doc_sections"
}

func (t *GetMeetingDocSectionsTool) Description() string {
	return "获取会议文档的章节列表（返回章节元数据/版本/树结构）。任何章节级新增、修改或删除操作前【必须先调用】本工具以获取最新结构。支持的槽位：polish（润色记录）、summary（总结）、topic（话题）。"
}

func (t *GetMeetingDocSectionsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"meeting_id": map[string]interface{}{
				"type":        "string",
				"description": "会议任务ID",
			},
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名：polish（润色记录）、summary（总结）、topic（话题）",
				"enum":        []string{"polish", "summary", "topic"},
			},
		},
		"required": []string{"meeting_id", "slot_key"},
	}
}

func (t *GetMeetingDocSectionsTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	meetingID, err := shared.SafeGetString(arguments, "meeting_id")
	if err != nil {
		return "", fmt.Errorf("get_meeting_doc_sections: %w", err)
	}

	slotKey, err := shared.SafeGetString(arguments, "slot_key")
	if err != nil || slotKey == "" {
		return "", fmt.Errorf("参数错误：slot_key 是必需的字符串参数")
	}

	// 验证 slot_key
	slotKey = strings.ToLower(strings.TrimSpace(slotKey))
	validSlots := map[string]bool{"polish": true, "summary": true, "topic": true}
	if !validSlots[slotKey] {
		return "", fmt.Errorf("参数错误：slot_key 值 \"%s\" 无效。\n\n有效的槽位为：\n  - \"polish\"   (润色记录)\n  - \"summary\"  (总结)\n  - \"topic\"    (话题)", slotKey)
	}

	return shared.CallAPI(apiClient, "GET", fmt.Sprintf("/api/v1/meetings/%s/docs/%s/sections", meetingID, slotKey), nil, clientToken)
}

// UpdateMeetingDocSectionTool 更新会议文档的单个章节内容
type UpdateMeetingDocSectionTool struct{}

func (t *UpdateMeetingDocSectionTool) Name() string {
	return "update_meeting_doc_section"
}

func (t *UpdateMeetingDocSectionTool) Description() string {
	return "局部章节正文更新（标题保持不变），支持 expected_version 并发防护。优先用于细粒度修改，代替全文覆盖 update_meeting_document。"
}

func (t *UpdateMeetingDocSectionTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"meeting_id": map[string]interface{}{
				"type":        "string",
				"description": "会议任务ID",
			},
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名：polish（润色记录）、summary（总结）、topic（话题）",
				"enum":        []string{"polish", "summary", "topic"},
			},
			"section_id": map[string]interface{}{
				"type":        "string",
				"description": "章节ID",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "章节内容（Markdown格式）",
			},
			"expected_version": map[string]interface{}{
				"type":        "number",
				"description": "期望版本号（用于版本冲突检测，可选）",
			},
		},
		"required": []string{"meeting_id", "slot_key", "section_id", "content"},
	}
}

func (t *UpdateMeetingDocSectionTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	meetingID, err := shared.SafeGetString(arguments, "meeting_id")
	if err != nil {
		return "", fmt.Errorf("update_meeting_doc_section: %w", err)
	}

	slotKey, err := shared.SafeGetString(arguments, "slot_key")
	if err != nil {
		return "", fmt.Errorf("update_meeting_doc_section: %w", err)
	}

	sectionID, err := shared.SafeGetString(arguments, "section_id")
	if err != nil {
		return "", fmt.Errorf("update_meeting_doc_section: %w", err)
	}

	content, err := shared.SafeGetString(arguments, "content")
	if err != nil {
		return "", fmt.Errorf("update_meeting_doc_section: %w", err)
	}

	// 验证 slot_key
	slotKey = strings.ToLower(strings.TrimSpace(slotKey))
	validSlots := map[string]bool{"polish": true, "summary": true, "topic": true}
	if !validSlots[slotKey] {
		return "", fmt.Errorf("update_meeting_doc_section: invalid slot_key, must be one of: polish, summary, topic")
	}

	// 构建请求体
	body := map[string]interface{}{
		"content": content,
	}
	if expectedVersion, ok := arguments["expected_version"].(float64); ok {
		body["expected_version"] = int(expectedVersion)
	}

	return shared.CallAPI(apiClient, "PUT", fmt.Sprintf("/api/v1/meetings/%s/docs/%s/sections/%s", meetingID, slotKey, sectionID), body, clientToken)
}

// SyncMeetingDocSectionsTool 同步会议文档的章节（与 compiled.md 双向同步）
type SyncMeetingDocSectionsTool struct{}

func (t *SyncMeetingDocSectionsTool) Name() string {
	return "sync_meeting_doc_sections"
}

func (t *SyncMeetingDocSectionsTool) Description() string {
	return "章节结构与 compiled.md 之间的同步/修复工具（from_compiled 或 to_compiled）。不用于日常内容编辑。"
}

func (t *SyncMeetingDocSectionsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"meeting_id": map[string]interface{}{
				"type":        "string",
				"description": "会议任务ID",
			},
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名：polish（润色记录）、summary（总结）、topic（话题）",
				"enum":        []string{"polish", "summary", "topic"},
			},
			"direction": map[string]interface{}{
				"type":        "string",
				"description": "同步方向: from_compiled (从compiled.md解析) 或 to_compiled (拼接回compiled.md)",
				"enum":        []string{"from_compiled", "to_compiled"},
			},
		},
		"required": []string{"meeting_id", "slot_key", "direction"},
	}
}

func (t *SyncMeetingDocSectionsTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	meetingID, err := shared.SafeGetString(arguments, "meeting_id")
	if err != nil {
		return "", fmt.Errorf("sync_meeting_doc_sections: %w", err)
	}

	slotKey, err := shared.SafeGetString(arguments, "slot_key")
	if err != nil || slotKey == "" {
		return "", fmt.Errorf("参数错误：slot_key 是必需的字符串参数")
	}

	direction, err := shared.SafeGetString(arguments, "direction")
	if err != nil || direction == "" {
		return "", fmt.Errorf("参数错误：direction 是必需的字符串参数")
	}

	// 验证 slot_key
	slotKey = strings.ToLower(strings.TrimSpace(slotKey))
	validSlots := map[string]bool{"polish": true, "summary": true, "topic": true}
	if !validSlots[slotKey] {
		return "", fmt.Errorf("参数错误：slot_key 值 \"%s\" 无效。\n\n有效的槽位为：\n  - \"polish\"   (润色记录)\n  - \"summary\"  (总结)\n  - \"topic\"    (话题)", slotKey)
	}

	// 验证 direction
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction != "from_compiled" && direction != "to_compiled" {
		return "", fmt.Errorf("参数错误：direction 值 \"%s\" 无效。\n\n有效的同步方向为：\n  - \"from_compiled\"  (从 compiled.md 解析章节到独立文件)\n  - \"to_compiled\"    (将独立章节文件拼接回 compiled.md)", direction)
	}

	// 构建请求体
	body := map[string]interface{}{
		"direction": direction,
	}

	return shared.CallAPI(apiClient, "POST", fmt.Sprintf("/api/v1/meetings/%s/docs/%s/sections/sync", meetingID, slotKey), body, clientToken)
}
