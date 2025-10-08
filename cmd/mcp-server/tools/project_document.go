package tools

import (
	"fmt"
	"github.com/houzhh15-hub/AIDG/cmd/mcp-server/shared"
)

// GetProjectDocumentTool 获取项目文档通用工具
type GetProjectDocumentTool struct {
	Registry *shared.SlotRegistry
}

func (t *GetProjectDocumentTool) Name() string {
	return "get_project_document"
}

func (t *GetProjectDocumentTool) Description() string {
	return "获取项目的指定槽位文档内容。支持的槽位：feature_list（特性列表，支持json/markdown格式）、architecture_design（架构设计，仅markdown格式）"
}

func (t *GetProjectDocumentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID",
			},
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名",
				"enum":        []string{"feature_list", "architecture_design"},
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "格式类型（可选：json | markdown，默认 markdown，仅 feature_list 支持 json）",
				"enum":        []string{"json", "markdown"},
				"default":     "markdown",
			},
		},
		"required": []string{"project_id", "slot_key"},
	}
}

func (t *GetProjectDocumentTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取参数
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil {
		return "", fmt.Errorf("get_project_document: %w", err)
	}
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("get_project_document: %w", err)
	}

	// 获取 format 参数（默认为 markdown）
	format := "markdown"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	// 2. 验证槽位和格式
	if err := t.Registry.ValidateProjectSlot(slotKey, format); err != nil {
		return "", fmt.Errorf("get_project_document: %w", err)
	}

	// 3. 获取 API 路径
	path, err := t.Registry.GetProjectAPIPath(slotKey, "GET", projectID, format)
	if err != nil {
		return "", fmt.Errorf("get_project_document: %w", err)
	}

	// 4. 调用 API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// UpdateProjectDocumentTool 更新项目文档通用工具
type UpdateProjectDocumentTool struct {
	Registry *shared.SlotRegistry
}

func (t *UpdateProjectDocumentTool) Name() string {
	return "update_project_document"
}

func (t *UpdateProjectDocumentTool) Description() string {
	return "更新项目的指定槽位文档内容。支持的槽位：feature_list（特性列表，支持json/markdown格式）、architecture_design（架构设计，仅markdown格式）"
}

func (t *UpdateProjectDocumentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID",
			},
			"slot_key": map[string]interface{}{
				"type":        "string",
				"description": "文档槽位键名",
				"enum":        []string{"feature_list", "architecture_design"},
			},
			"content": map[string]interface{}{
				"description": "文档内容（根据 format 类型为 string 或 object）",
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "格式类型（可选：json | markdown，默认 markdown）",
				"enum":        []string{"json", "markdown"},
				"default":     "markdown",
			},
		},
		"required": []string{"project_id", "slot_key", "content"},
	}
}

func (t *UpdateProjectDocumentTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取参数
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil {
		return "", fmt.Errorf("update_project_document: %w", err)
	}
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("update_project_document: %w", err)
	}

	// content 可能是 string 或 object
	content, hasContent := args["content"]
	if !hasContent {
		return "", fmt.Errorf("update_project_document: 缺少 content 参数")
	}

	// 获取 format 参数（默认为 markdown）
	format := "markdown"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	// 2. 验证槽位和格式
	if err := t.Registry.ValidateProjectSlot(slotKey, format); err != nil {
		return "", fmt.Errorf("update_project_document: %w", err)
	}

	// 3. 获取 API 路径
	path, err := t.Registry.GetProjectAPIPath(slotKey, "PUT", projectID, format)
	if err != nil {
		return "", fmt.Errorf("update_project_document: %w", err)
	}

	// 4. 构造请求体（根据 format 处理 content 类型）
	var body map[string]interface{}
	if format == "json" {
		// 对于 json 格式，content 应该是对象
		body = map[string]interface{}{"content": content}
	} else {
		// 对于 markdown 格式，content 应该是字符串
		contentStr, ok := content.(string)
		if !ok {
			return "", fmt.Errorf("update_project_document: markdown 格式的 content 必须是字符串")
		}
		body = map[string]interface{}{"content": contentStr}
	}

	// 5. 调用 API
	return shared.CallAPI(apiClient, "PUT", path, body, clientToken)
}
