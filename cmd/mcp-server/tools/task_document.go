package tools

import (
	"fmt"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// GetTaskDocumentTool 获取任务文档通用工具
type GetTaskDocumentTool struct {
	Registry *shared.SlotRegistry
}

func (t *GetTaskDocumentTool) Name() string {
	return "get_task_document"
}

func (t *GetTaskDocumentTool) Description() string {
	return "获取任务的指定槽位文档内容。支持的槽位：requirements（需求文档）、design（设计文档）、test（测试文档）。" +
		"可选参数 include_recommendations=true 将返回基于语义相似度的历史任务推荐（Top-5）。"
}

func (t *GetTaskDocumentTool) InputSchema() map[string]interface{} {
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
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名",
				"enum":        []string{"requirements", "design", "test"},
			},
			"include_recommendations": map[string]interface{}{
				"type":        "boolean",
				"description": "是否包含语义相似推荐（可选，默认false）",
				"default":     false,
			},
		},
		"required": []string{"slot_key"},
	}
}

func (t *GetTaskDocumentTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 使用带回退的方式获取 project_id 和 task_id
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(args, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("get_task_document: %w", err)
	}
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("get_task_document: %w", err)
	}

	// 2. 验证槽位
	if err := t.Registry.ValidateTaskSlot(slotKey); err != nil {
		return "", fmt.Errorf("get_task_document: 无效的槽位 '%s': %w", slotKey, err)
	}

	// 3. 获取 API 路径
	path, err := t.Registry.GetTaskAPIPath(slotKey, "GET", projectID, taskID)
	if err != nil {
		return "", fmt.Errorf("get_task_document: %w", err)
	}

	// 4. 可选：添加推荐参数
	includeRec, _ := args["include_recommendations"].(bool)
	if includeRec {
		path += "?include_recommendations=true"
	}

	// 5. 调用 API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// UpdateTaskDocumentTool 更新任务文档通用工具
type UpdateTaskDocumentTool struct {
	Registry *shared.SlotRegistry
}

func (t *UpdateTaskDocumentTool) Name() string {
	return "update_task_document"
}

func (t *UpdateTaskDocumentTool) Description() string {
	return "【高风险全文覆盖】替换整个槽位文档内容。局部或单章节修改请使用：get_task_doc_sections -> get_task_doc_section -> update_task_doc_section。仅在明确需要大规模重写且已确认(FULL_OVERRIDE_CONFIRM)时使用。支持槽位：requirements、design、test。"
}

func (t *UpdateTaskDocumentTool) InputSchema() map[string]interface{} {
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
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名",
				"enum":        []string{"requirements", "design", "test"},
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "文档内容（Markdown 格式）",
			},
		},
		"required": []string{"slot_key", "content"},
	}
}

func (t *UpdateTaskDocumentTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 使用带回退的方式获取 project_id 和 task_id
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(args, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("update_task_document: %w", err)
	}
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("update_task_document: %w", err)
	}
	content, err := shared.SafeGetString(args, "content")
	if err != nil {
		return "", fmt.Errorf("update_task_document: %w", err)
	}

	// 2. 验证槽位
	if err := t.Registry.ValidateTaskSlot(slotKey); err != nil {
		return "", fmt.Errorf("update_task_document: 无效的槽位 '%s': %w", slotKey, err)
	}

	// 3. 获取 API 路径
	path, err := t.Registry.GetTaskAPIPath(slotKey, "PUT", projectID, taskID)
	if err != nil {
		return "", fmt.Errorf("update_task_document: %w", err)
	}

	// 4. 构造请求体并调用 API
	body := map[string]string{"content": content}
	return shared.CallAPI(apiClient, "PUT", path, body, clientToken)
}

// AppendTaskDocumentTool 追加任务文档通用工具
type AppendTaskDocumentTool struct {
	Registry *shared.SlotRegistry
}

func (t *AppendTaskDocumentTool) Name() string {
	return "append_task_document"
}

func (t *AppendTaskDocumentTool) Description() string {
	return "向任务的指定槽位文档追加内容（推荐：不覆盖已有历史）。支持的槽位：requirements、design、test"
}

func (t *AppendTaskDocumentTool) InputSchema() map[string]interface{} {
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
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名",
				"enum":        []string{"requirements", "design", "test"},
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "追加内容",
			},
			"expected_version": map[string]interface{}{
				"type":        "number",
				"description": "期望版本防并发（可选）",
			},
		},
		"required": []string{"slot_key", "content"},
	}
}

func (t *AppendTaskDocumentTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 使用带回退的方式获取 project_id 和 task_id
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(args, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("append_task_document: %w", err)
	}
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("append_task_document: %w", err)
	}
	content, err := shared.SafeGetString(args, "content")
	if err != nil {
		return "", fmt.Errorf("append_task_document: %w", err)
	}

	// 2. 验证槽位
	if err := t.Registry.ValidateTaskSlot(slotKey); err != nil {
		return "", fmt.Errorf("append_task_document: 无效的槽位 '%s': %w", slotKey, err)
	}

	// 3. 构造特殊的 Append 路径
	path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/append", projectID, taskID, slotKey)

	// 4. 构造请求体
	body := map[string]interface{}{
		"content": content,
		"op":      "add_full",
	}

	// 可选的版本参数
	if expectedVersion, ok := args["expected_version"].(float64); ok {
		body["expected_version"] = int(expectedVersion)
	}

	// 5. 使用 POST 方法调用 API
	return shared.CallAPI(apiClient, "POST", path, body, clientToken)
}
