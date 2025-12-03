package tools

import (
	"fmt"
	"strings"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// ReadDocumentContentTool 读取文档内容工具
type ReadDocumentContentTool struct{}

func (t *ReadDocumentContentTool) Name() string {
	return "read_document_content"
}

func (t *ReadDocumentContentTool) Description() string {
	return "读取指定项目文档树中一个文档的正文内容"
}

func (t *ReadDocumentContentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"node_id": map[string]interface{}{
				"type":        "string",
				"description": "文档节点ID",
			},
			"version": map[string]interface{}{
				"type":        "integer",
				"description": "可选的版本号，如果不提供则返回最新版本",
			},
		},
		"required": []string{"node_id"},
	}
}

func (t *ReadDocumentContentTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取参数
	projectID, err := shared.GetProjectIDWithFallback(args, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("read_document_content: %w", err)
	}
	nodeID, err := shared.SafeGetString(args, "node_id")
	if err != nil {
		return "", fmt.Errorf("read_document_content: %w", err)
	}

	// 2. 构建API路径
	path := fmt.Sprintf("/api/v1/projects/%s/documents/%s/content", projectID, nodeID)

	// 3. 检查是否有版本参数
	if version, exists := args["version"]; exists {
		if versionInt, ok := version.(float64); ok {
			path += fmt.Sprintf("?version=%d", int(versionInt))
		}
	}

	// 4. 调用API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// WriteDocumentContentTool 写入文档内容工具
type WriteDocumentContentTool struct{}

func (t *WriteDocumentContentTool) Name() string {
	return "write_document_content"
}

func (t *WriteDocumentContentTool) Description() string {
	return "写入或更新项目文档树中一个文档的正文内容"
}

func (t *WriteDocumentContentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"node_id": map[string]interface{}{
				"type":        "string",
				"description": "文档节点ID",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "文档内容",
			},
			"version": map[string]interface{}{
				"type":        "integer",
				"description": "当前版本号，用于乐观锁控制",
			},
		},
		"required": []string{"node_id", "content", "version"},
	}
}

func (t *WriteDocumentContentTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取参数
	projectID, err := shared.GetProjectIDWithFallback(args, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("write_document_content: %w", err)
	}
	nodeID, err := shared.SafeGetString(args, "node_id")
	if err != nil {
		return "", fmt.Errorf("write_document_content: %w", err)
	}
	content, err := shared.SafeGetString(args, "content")
	if err != nil {
		return "", fmt.Errorf("write_document_content: %w", err)
	}
	version, err := shared.SafeGetInt(args, "version")
	if err != nil {
		return "", fmt.Errorf("write_document_content: %w", err)
	}

	// 2. 构建请求体
	requestBody := map[string]interface{}{
		"content": content,
		"version": version,
	}

	// 3. 构建API路径
	path := fmt.Sprintf("/api/v1/projects/%s/documents/%s/content", projectID, nodeID)

	// 4. 调用API
	return shared.CallAPI(apiClient, "PUT", path, requestBody, clientToken)
}

// GetHierarchicalDocumentsTool 获取项目层级文档结构工具
type GetHierarchicalDocumentsTool struct{}

func (t *GetHierarchicalDocumentsTool) Name() string {
	return "get_hierarchical_documents"
}

func (t *GetHierarchicalDocumentsTool) Description() string {
	return "获取一个项目文档树的层级结构"
}

func (t *GetHierarchicalDocumentsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"node_id": map[string]interface{}{
				"type":        "string",
				"description": "可选的节点ID，从该节点开始获取树结构",
			},
			"depth": map[string]interface{}{
				"type":        "integer",
				"description": "可选的深度限制",
			},
		},
		"required": []string{},
	}
}

func (t *GetHierarchicalDocumentsTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取参数
	projectID, err := shared.GetProjectIDWithFallback(args, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("get_hierarchical_documents: %w", err)
	}

	// 2. 构建API路径
	path := fmt.Sprintf("/api/v1/projects/%s/documents/tree", projectID)

	// 3. 添加查询参数
	queryParams := []string{}
	if nodeID, exists := args["node_id"]; exists {
		if nodeIDStr, ok := nodeID.(string); ok && nodeIDStr != "" {
			queryParams = append(queryParams, fmt.Sprintf("node_id=%s", nodeIDStr))
		}
	}
	if depth, exists := args["depth"]; exists {
		if depthInt, ok := depth.(float64); ok {
			queryParams = append(queryParams, fmt.Sprintf("depth=%d", int(depthInt)))
		}
	}
	if len(queryParams) > 0 {
		path += "?" + strings.Join(queryParams, "&")
	}

	// 4. 调用API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// AnalyzeDocumentRelationshipsTool 分析文档关系与影响工具
type AnalyzeDocumentRelationshipsTool struct{}

func (t *AnalyzeDocumentRelationshipsTool) Name() string {
	return "analyze_document_relationships"
}

func (t *AnalyzeDocumentRelationshipsTool) Description() string {
	return "分析文档树中一个文档的关系与影响"
}

func (t *AnalyzeDocumentRelationshipsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"node_id": map[string]interface{}{
				"type":        "string",
				"description": "文档节点ID",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"description": "分析模式，可以是单个字符串或字符串数组",
				"oneOf": []map[string]interface{}{
					{"type": "string"},
					{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		"required": []string{"node_id"},
	}
}

func (t *AnalyzeDocumentRelationshipsTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取参数
	projectID, err := shared.GetProjectIDWithFallback(args, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("analyze_document_relationships: %w", err)
	}
	nodeID, err := shared.SafeGetString(args, "node_id")
	if err != nil {
		return "", fmt.Errorf("analyze_document_relationships: %w", err)
	}

	// 2. 构建API路径
	path := fmt.Sprintf("/api/v1/projects/%s/documents/%s/impact", projectID, nodeID)

	// 3. 处理mode参数（可选）
	if mode, exists := args["mode"]; exists {
		query := ""
		if modeStr, ok := mode.(string); ok {
			query = fmt.Sprintf("?mode=%s", modeStr)
		} else if modeArr, ok := mode.([]interface{}); ok {
			modes := []string{}
			for _, m := range modeArr {
				if mStr, ok := m.(string); ok {
					modes = append(modes, mStr)
				}
			}
			if len(modes) > 0 {
				query = fmt.Sprintf("?mode=%s", strings.Join(modes, ","))
			}
		}
		path += query
	}

	// 4. 调用API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// ManageDocumentReferenceTool 创建或查询任务与文档引用工具
type ManageDocumentReferenceTool struct{}

func (t *ManageDocumentReferenceTool) Name() string {
	return "manage_document_reference"
}

func (t *ManageDocumentReferenceTool) Description() string {
	return "创建或查询任务与文档关联和引用"
}

func (t *ManageDocumentReferenceTool) InputSchema() map[string]interface{} {
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
			"doc_id": map[string]interface{}{
				"type":        "string",
				"description": "可选的文档ID，用于创建关联引用时指定",
			},
			"action": map[string]interface{}{
				"type":        "string",
				"description": "操作类型",
				"enum":        []string{"create", "list"},
			},
			"anchor": map[string]interface{}{
				"type":        "string",
				"description": "创建引用时的锚点（段落标识）",
			},
			"context": map[string]interface{}{
				"type":        "string",
				"description": "创建引用时的上下文信息",
			},
		},
		"required": []string{"action"},
	}
}

func (t *ManageDocumentReferenceTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取参数
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(args, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("manage_document_reference: %w", err)
	}
	action, err := shared.SafeGetString(args, "action")
	if err != nil {
		return "", fmt.Errorf("manage_document_reference: %w", err)
	}

	// 2. 根据action执行不同操作
	switch action {
	case "create":
		// 创建引用
		docID, err := shared.SafeGetString(args, "doc_id")
		if err != nil {
			return "", fmt.Errorf("manage_document_reference create: %w", err)
		}
		anchor, err := shared.SafeGetString(args, "anchor")
		if err != nil {
			return "", fmt.Errorf("manage_document_reference create: %w", err)
		}
		context, err := shared.SafeGetString(args, "context")
		if err != nil {
			return "", fmt.Errorf("manage_document_reference create: %w", err)
		}

		requestBody := map[string]interface{}{
			"task_id":     taskID,
			"document_id": docID,
			"anchor":      anchor,
			"context":     context,
		}

		path := fmt.Sprintf("/api/v1/projects/%s/documents/references", projectID)
		return shared.CallAPI(apiClient, "POST", path, requestBody, clientToken)

	case "list":
		// 查询引用 - 按项目和任务查询
		path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/references", projectID, taskID)
		return shared.CallAPI(apiClient, "GET", path, nil, clientToken)

	default:
		return "", fmt.Errorf("manage_document_reference: unsupported action '%s'", action)
	}
}
