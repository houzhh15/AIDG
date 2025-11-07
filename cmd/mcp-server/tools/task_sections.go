package tools

import (
	"fmt"
	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
	"strings"
)

// GetTaskDocSectionsTool 获取任务文档的章节列表
type GetTaskDocSectionsTool struct{}

func (t *GetTaskDocSectionsTool) Name() string {
	return "get_task_doc_sections"
}

func (t *GetTaskDocSectionsTool) Description() string {
	return "获取任务文档的章节列表（返回章节元数据/版本/树结构）。任何章节级新增、修改或删除操作前【必须先调用】本工具以获取最新结构。支持 requirements/design/test。"
}

func (t *GetTaskDocSectionsTool) InputSchema() map[string]interface{} {
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
			"doc_type": map[string]interface{}{
				"type":        "string",
				"description": "文档类型: requirements, design, test",
				"enum":        []string{"requirements", "design", "test"},
			},
		},
		"required": []string{"project_id", "task_id", "doc_type"},
	}
}

func (t *GetTaskDocSectionsTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.SafeGetString(arguments, "project_id")
	if err != nil || projectID == "" {
		return "", fmt.Errorf("参数错误：project_id 是必需的字符串参数。请提供项目ID，例如：\"AI-Dev-Gov\"")
	}

	taskID, err := shared.SafeGetString(arguments, "task_id")
	if err != nil || taskID == "" {
		return "", fmt.Errorf("参数错误：task_id 是必需的字符串参数。请提供任务ID，例如：\"task_1759401721\"")
	}

	docType, err := shared.SafeGetString(arguments, "doc_type")
	if err != nil || docType == "" {
		return "", fmt.Errorf("参数错误：doc_type 是必需的字符串参数")
	}

	// 验证 doc_type
	docType = strings.ToLower(strings.TrimSpace(docType))
	if docType != "requirements" && docType != "design" && docType != "test" {
		return "", fmt.Errorf("参数错误：doc_type 值 \"%s\" 无效。\n\n有效的文档类型为：\n  - \"requirements\"  (需求文档)\n  - \"design\"        (设计文档)\n  - \"test\"          (测试文档)\n\n正确的调用示例：\n{\n  \"project_id\": \"%s\",\n  \"task_id\": \"%s\",\n  \"doc_type\": \"design\"\n}", docType, projectID, taskID)
	}

	return shared.CallAPI(apiClient, "GET", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections", projectID, taskID, docType), nil, clientToken)
}

// GetTaskDocSectionTool 获取任务文档的单个章节内容
type GetTaskDocSectionTool struct{}

func (t *GetTaskDocSectionTool) Name() string {
	return "get_task_doc_section"
}

func (t *GetTaskDocSectionTool) Description() string {
	return "获取单个章节当前基线（标题+Markdown 正文，可选子章节）。用于在 update 前读取基线并构造最小差异，避免全文覆盖。"
}

func (t *GetTaskDocSectionTool) InputSchema() map[string]interface{} {
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
			"doc_type": map[string]interface{}{
				"type":        "string",
				"description": "文档类型: requirements, design, test",
				"enum":        []string{"requirements", "design", "test"},
			},
			"section_id": map[string]interface{}{
				"type":        "string",
				"description": "章节ID",
			},
			"include_children": map[string]interface{}{
				"type":        "boolean",
				"description": "是否包含子章节内容（默认false）",
			},
		},
		"required": []string{"project_id", "task_id", "doc_type", "section_id"},
	}
}

func (t *GetTaskDocSectionTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.SafeGetString(arguments, "project_id")
	if err != nil {
		return "", fmt.Errorf("get_task_doc_section: %w", err)
	}
	taskID, err := shared.SafeGetString(arguments, "task_id")
	if err != nil {
		return "", fmt.Errorf("get_task_doc_section: %w", err)
	}
	docType, err := shared.SafeGetString(arguments, "doc_type")
	if err != nil {
		return "", fmt.Errorf("get_task_doc_section: %w", err)
	}
	sectionID, err := shared.SafeGetString(arguments, "section_id")
	if err != nil {
		return "", fmt.Errorf("get_task_doc_section: %w", err)
	}

	// 验证 doc_type
	if docType != "requirements" && docType != "design" && docType != "test" {
		return "", fmt.Errorf("get_task_doc_section: invalid doc_type, must be one of: requirements, design, test")
	}

	// 构建查询参数
	url := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections/%s", projectID, taskID, docType, sectionID)
	if includeChildren, ok := arguments["include_children"].(bool); ok && includeChildren {
		url += "?include_children=true"
	}

	return shared.CallAPI(apiClient, "GET", url, nil, clientToken)
}

// UpdateTaskDocSectionTool 更新任务文档的单个章节内容
type UpdateTaskDocSectionTool struct{}

func (t *UpdateTaskDocSectionTool) Name() string {
	return "update_task_doc_section"
}

func (t *UpdateTaskDocSectionTool) Description() string {
	return "局部章节正文更新（标题保持不变），支持 expected_version 并发防护。优先用于细粒度修改，代替全文覆盖 update_task_document。"
}

func (t *UpdateTaskDocSectionTool) InputSchema() map[string]interface{} {
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
			"doc_type": map[string]interface{}{
				"type":        "string",
				"description": "文档类型: requirements, design, test",
				"enum":        []string{"requirements", "design", "test"},
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
		"required": []string{"project_id", "task_id", "doc_type", "section_id", "content"},
	}
}

func (t *UpdateTaskDocSectionTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.SafeGetString(arguments, "project_id")
	if err != nil {
		return "", fmt.Errorf("update_task_doc_section: %w", err)
	}
	taskID, err := shared.SafeGetString(arguments, "task_id")
	if err != nil {
		return "", fmt.Errorf("update_task_doc_section: %w", err)
	}
	docType, err := shared.SafeGetString(arguments, "doc_type")
	if err != nil {
		return "", fmt.Errorf("update_task_doc_section: %w", err)
	}
	sectionID, err := shared.SafeGetString(arguments, "section_id")
	if err != nil {
		return "", fmt.Errorf("update_task_doc_section: %w", err)
	}
	content, err := shared.SafeGetString(arguments, "content")
	if err != nil {
		return "", fmt.Errorf("update_task_doc_section: %w", err)
	}

	// 验证 doc_type
	if docType != "requirements" && docType != "design" && docType != "test" {
		return "", fmt.Errorf("update_task_doc_section: invalid doc_type, must be one of: requirements, design, test")
	}

	// 构建请求体
	body := map[string]interface{}{
		"content": content,
	}
	if expectedVersion, ok := arguments["expected_version"].(float64); ok {
		body["expected_version"] = int(expectedVersion)
	}

	return shared.CallAPI(apiClient, "PUT", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections/%s", projectID, taskID, docType, sectionID), body, clientToken)
}

// InsertTaskDocSectionTool 在任务文档中插入新章节
type InsertTaskDocSectionTool struct{}

func (t *InsertTaskDocSectionTool) Name() string {
	return "insert_task_doc_section"
}

func (t *InsertTaskDocSectionTool) Description() string {
	return "插入新章节（同级）。默认追加到末尾；若需精确位置请先 get_task_doc_sections 并提供 after_section_id。自动同步 compiled.md。"
}

func (t *InsertTaskDocSectionTool) InputSchema() map[string]interface{} {
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
			"doc_type": map[string]interface{}{
				"type":        "string",
				"description": "文档类型: requirements, design, test",
				"enum":        []string{"requirements", "design", "test"},
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "章节标题（包含 Markdown # 标记，如 '## 新章节'）",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "章节内容（Markdown格式）",
			},
			"after_section_id": map[string]interface{}{
				"type":        "string",
				"description": "在哪个章节后插入（可选，不提供则插入到末尾）",
			},
		},
		"required": []string{"project_id", "task_id", "doc_type", "title", "content"},
	}
}

func (t *InsertTaskDocSectionTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.SafeGetString(arguments, "project_id")
	if err != nil {
		return "", fmt.Errorf("insert_task_doc_section: %w", err)
	}
	taskID, err := shared.SafeGetString(arguments, "task_id")
	if err != nil {
		return "", fmt.Errorf("insert_task_doc_section: %w", err)
	}
	docType, err := shared.SafeGetString(arguments, "doc_type")
	if err != nil {
		return "", fmt.Errorf("insert_task_doc_section: %w", err)
	}
	title, err := shared.SafeGetString(arguments, "title")
	if err != nil {
		return "", fmt.Errorf("insert_task_doc_section: %w", err)
	}
	content, err := shared.SafeGetString(arguments, "content")
	if err != nil {
		return "", fmt.Errorf("insert_task_doc_section: %w", err)
	}

	// 验证 doc_type
	if docType != "requirements" && docType != "design" && docType != "test" {
		return "", fmt.Errorf("insert_task_doc_section: invalid doc_type, must be one of: requirements, design, test")
	}

	// 验证 title 不为空
	if strings.TrimSpace(title) == "" {
		return "", fmt.Errorf("insert_task_doc_section: title cannot be empty")
	}

	// 构建请求体
	body := map[string]interface{}{
		"title":   title,
		"content": content,
	}
	if afterSectionID, ok := arguments["after_section_id"].(string); ok && afterSectionID != "" {
		body["after_section_id"] = afterSectionID
	}

	return shared.CallAPI(apiClient, "POST", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections", projectID, taskID, docType), body, clientToken)
}

// DeleteTaskDocSectionTool 删除任务文档中的章节
type DeleteTaskDocSectionTool struct{}

func (t *DeleteTaskDocSectionTool) Name() string {
	return "delete_task_doc_section"
}

func (t *DeleteTaskDocSectionTool) Description() string {
	return "删除章节（可级联子章节 cascade=true）。操作前建议重新获取章节列表确认 ID，删除后同步 compiled.md，谨慎使用。"
}

func (t *DeleteTaskDocSectionTool) InputSchema() map[string]interface{} {
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
			"doc_type": map[string]interface{}{
				"type":        "string",
				"description": "文档类型: requirements, design, test",
				"enum":        []string{"requirements", "design", "test"},
			},
			"section_id": map[string]interface{}{
				"type":        "string",
				"description": "章节ID",
			},
			"cascade": map[string]interface{}{
				"type":        "boolean",
				"description": "是否级联删除子章节（默认false）",
			},
		},
		"required": []string{"project_id", "task_id", "doc_type", "section_id"},
	}
}

func (t *DeleteTaskDocSectionTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.SafeGetString(arguments, "project_id")
	if err != nil {
		return "", fmt.Errorf("delete_task_doc_section: %w", err)
	}
	taskID, err := shared.SafeGetString(arguments, "task_id")
	if err != nil {
		return "", fmt.Errorf("delete_task_doc_section: %w", err)
	}
	docType, err := shared.SafeGetString(arguments, "doc_type")
	if err != nil {
		return "", fmt.Errorf("delete_task_doc_section: %w", err)
	}
	sectionID, err := shared.SafeGetString(arguments, "section_id")
	if err != nil {
		return "", fmt.Errorf("delete_task_doc_section: %w", err)
	}

	// 验证 doc_type
	if docType != "requirements" && docType != "design" && docType != "test" {
		return "", fmt.Errorf("delete_task_doc_section: invalid doc_type, must be one of: requirements, design, test")
	}

	// 构建查询参数
	url := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections/%s", projectID, taskID, docType, sectionID)
	if cascade, ok := arguments["cascade"].(bool); ok && cascade {
		url += "?cascade=true"
	}

	return shared.CallAPI(apiClient, "DELETE", url, nil, clientToken)
}

// SyncTaskDocSectionsTool 同步任务文档的章节（与 compiled.md 双向同步）
type SyncTaskDocSectionsTool struct{}

func (t *SyncTaskDocSectionsTool) Name() string {
	return "sync_task_doc_sections"
}

func (t *SyncTaskDocSectionsTool) Description() string {
	return "章节结构与 compiled.md 之间的同步/修复工具（from_compiled 或 to_compiled）。不用于日常内容编辑。"
}

func (t *SyncTaskDocSectionsTool) InputSchema() map[string]interface{} {
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
			"doc_type": map[string]interface{}{
				"type":        "string",
				"description": "文档类型: requirements, design, test",
				"enum":        []string{"requirements", "design", "test"},
			},
			"direction": map[string]interface{}{
				"type":        "string",
				"description": "同步方向: from_compiled (从compiled.md解析) 或 to_compiled (拼接回compiled.md)",
				"enum":        []string{"from_compiled", "to_compiled"},
			},
		},
		"required": []string{"project_id", "task_id", "doc_type", "direction"},
	}
}

func (t *SyncTaskDocSectionsTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.SafeGetString(arguments, "project_id")
	if err != nil || projectID == "" {
		return "", fmt.Errorf("参数错误：project_id 是必需的字符串参数。请提供项目ID，例如：\"AI-Dev-Gov\"")
	}

	taskID, err := shared.SafeGetString(arguments, "task_id")
	if err != nil || taskID == "" {
		return "", fmt.Errorf("参数错误：task_id 是必需的字符串参数。请提供任务ID，例如：\"task_1759401721\"")
	}

	docType, err := shared.SafeGetString(arguments, "doc_type")
	if err != nil || docType == "" {
		return "", fmt.Errorf("参数错误：doc_type 是必需的字符串参数")
	}

	direction, err := shared.SafeGetString(arguments, "direction")
	if err != nil || direction == "" {
		return "", fmt.Errorf("参数错误：direction 是必需的字符串参数")
	}

	// 验证 doc_type
	docType = strings.ToLower(strings.TrimSpace(docType))
	if docType != "requirements" && docType != "design" && docType != "test" {
		return "", fmt.Errorf("参数错误：doc_type 值 \"%s\" 无效。\n\n有效的文档类型为：\n  - \"requirements\"  (需求文档)\n  - \"design\"        (设计文档)\n  - \"test\"          (测试文档)", docType)
	}

	// 验证 direction
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction != "from_compiled" && direction != "to_compiled" {
		return "", fmt.Errorf("参数错误：direction 值 \"%s\" 无效。\n\n有效的同步方向为：\n  - \"from_compiled\"  (从 compiled.md 解析章节到独立文件)\n  - \"to_compiled\"    (将独立章节文件拼接回 compiled.md)\n\n正确的调用示例：\n{\n  \"project_id\": \"%s\",\n  \"task_id\": \"%s\",\n  \"doc_type\": \"%s\",\n  \"direction\": \"from_compiled\"\n}", direction, projectID, taskID, docType)
	}

	// 构建请求体
	body := map[string]interface{}{
		"direction": direction,
	}

	return shared.CallAPI(apiClient, "POST", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections/sync", projectID, taskID, docType), body, clientToken)
}
