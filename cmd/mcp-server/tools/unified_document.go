// Package tools provides MCP tools for document operations.
// This file implements unified document tools that work with all document scopes:
// - Project documents: feature_list, architecture_design
// - Meeting documents: polish, summary, topic
// - Task documents: requirements, design, test
//
// These tools use the unified API endpoints:
// - /api/v1/projects/:id/docs/:slot/...
// - /api/v1/meetings/:id/docs/:slot/...
// - /api/v1/projects/:id/tasks/:tid/docs/:slot/... (for task documents)
package tools

import (
	"fmt"
	"strings"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// DocumentScope 文档作用域
type DocumentScope string

const (
	ScopeProject DocumentScope = "project"
	ScopeMeeting DocumentScope = "meeting"
	ScopeTask    DocumentScope = "task"
)

// UnifiedDocExportTool 统一文档导出工具
type UnifiedDocExportTool struct {
	Scope DocumentScope
}

func (t *UnifiedDocExportTool) Name() string {
	switch t.Scope {
	case ScopeProject:
		return "export_project_document"
	case ScopeMeeting:
		return "export_meeting_document"
	case ScopeTask:
		return "export_task_document"
	default:
		return "export_unified_document"
	}
}

func (t *UnifiedDocExportTool) Description() string {
	switch t.Scope {
	case ScopeProject:
		return "导出项目文档内容。支持的槽位：feature_list（特性列表）、architecture_design（架构设计）。" +
			"返回完整的文档内容和版本信息。"
	case ScopeMeeting:
		return "导出会议文档内容。支持的槽位：polish（润色记录）、summary（会议总结）、topic（话题）。" +
			"返回完整的文档内容和版本信息。"
	case ScopeTask:
		return "导出任务文档内容。支持的槽位：requirements（需求文档）、design（设计文档）、test（测试文档）。" +
			"返回完整的文档内容和版本信息。"
	default:
		return "导出文档内容。"
	}
}

func (t *UnifiedDocExportTool) InputSchema() map[string]interface{} {
	var slots []string
	var idDesc, slotDesc string

	switch t.Scope {
	case ScopeProject:
		slots = []string{"feature_list", "architecture_design"}
		idDesc = "项目ID"
		slotDesc = "文档槽位键名"
	case ScopeMeeting:
		slots = []string{"polish", "summary", "topic"}
		idDesc = "会议任务ID"
		slotDesc = "文档槽位键名"
	case ScopeTask:
		slots = []string{"requirements", "design", "test"}
		idDesc = "项目ID"
		slotDesc = "文档槽位键名"
	}

	props := map[string]interface{}{
		"scope_id": map[string]interface{}{
			"type":        "string",
			"description": idDesc,
		},
		"slot_key": map[string]interface{}{
			"type":        "string",
			"description": slotDesc,
			"enum":        slots,
		},
	}

	required := []string{"scope_id", "slot_key"}

	// Task scope needs additional project_id
	if t.Scope == ScopeTask {
		props["project_id"] = map[string]interface{}{
			"type":        "string",
			"description": "项目ID",
		}
		props["task_id"] = map[string]interface{}{
			"type":        "string",
			"description": "任务ID",
		}
		delete(props, "scope_id")
		required = []string{"project_id", "task_id", "slot_key"}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func (t *UnifiedDocExportTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("export: %w", err)
	}

	var path string
	switch t.Scope {
	case ScopeProject:
		scopeID, err := shared.SafeGetString(args, "scope_id")
		if err != nil {
			return "", fmt.Errorf("export: %w", err)
		}
		path = fmt.Sprintf("/api/v1/projects/%s/docs/%s/export", scopeID, slotKey)

	case ScopeMeeting:
		scopeID, err := shared.SafeGetString(args, "scope_id")
		if err != nil {
			return "", fmt.Errorf("export: %w", err)
		}
		path = fmt.Sprintf("/api/v1/meetings/%s/docs/%s/export", scopeID, slotKey)

	case ScopeTask:
		projectID, err := shared.SafeGetString(args, "project_id")
		if err != nil {
			return "", fmt.Errorf("export: %w", err)
		}
		taskID, err := shared.SafeGetString(args, "task_id")
		if err != nil {
			return "", fmt.Errorf("export: %w", err)
		}
		path = fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s", projectID, taskID, slotKey)
	}

	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// UnifiedDocAppendTool 统一文档追加工具
type UnifiedDocAppendTool struct {
	Scope DocumentScope
}

func (t *UnifiedDocAppendTool) Name() string {
	switch t.Scope {
	case ScopeProject:
		return "append_project_document"
	case ScopeMeeting:
		return "append_meeting_document"
	case ScopeTask:
		return "append_task_document"
	default:
		return "append_unified_document"
	}
}

func (t *UnifiedDocAppendTool) Description() string {
	switch t.Scope {
	case ScopeProject:
		return "向项目文档追加内容（推荐：不覆盖已有历史）。支持的槽位：feature_list（特性列表）、architecture_design（架构设计）。"
	case ScopeMeeting:
		return "向会议文档追加内容（推荐：不覆盖已有历史）。支持的槽位：polish（润色记录）、summary（会议总结）、topic（话题）。"
	case ScopeTask:
		return "向任务文档追加内容（推荐：不覆盖已有历史）。支持的槽位：requirements（需求文档）、design（设计文档）、test（测试文档）。"
	default:
		return "向文档追加内容。"
	}
}

func (t *UnifiedDocAppendTool) InputSchema() map[string]interface{} {
	var slots []string
	var idDesc string

	switch t.Scope {
	case ScopeProject:
		slots = []string{"feature_list", "architecture_design"}
		idDesc = "项目ID"
	case ScopeMeeting:
		slots = []string{"polish", "summary", "topic"}
		idDesc = "会议任务ID"
	case ScopeTask:
		slots = []string{"requirements", "design", "test"}
		idDesc = "项目ID"
	}

	props := map[string]interface{}{
		"scope_id": map[string]interface{}{
			"type":        "string",
			"description": idDesc,
		},
		"slot_key": map[string]interface{}{
			"type":        "string",
			"description": "文档槽位键名",
			"enum":        slots,
		},
		"content": map[string]interface{}{
			"type":        "string",
			"description": "追加内容",
		},
		"expected_version": map[string]interface{}{
			"type":        "number",
			"description": "期望版本防并发（可选）",
		},
	}

	required := []string{"scope_id", "slot_key", "content"}

	if t.Scope == ScopeTask {
		props["project_id"] = map[string]interface{}{
			"type":        "string",
			"description": "项目ID",
		}
		props["task_id"] = map[string]interface{}{
			"type":        "string",
			"description": "任务ID",
		}
		delete(props, "scope_id")
		required = []string{"project_id", "task_id", "slot_key", "content"}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func (t *UnifiedDocAppendTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("append: %w", err)
	}
	content, err := shared.SafeGetString(args, "content")
	if err != nil {
		return "", fmt.Errorf("append: %w", err)
	}

	body := map[string]interface{}{
		"content": content,
	}

	// 可选的版本参数
	if v, ok := args["expected_version"]; ok {
		body["expected_version"] = v
	}

	var path string
	switch t.Scope {
	case ScopeProject:
		scopeID, err := shared.SafeGetString(args, "scope_id")
		if err != nil {
			return "", fmt.Errorf("append: %w", err)
		}
		path = fmt.Sprintf("/api/v1/projects/%s/docs/%s/append", scopeID, slotKey)

	case ScopeMeeting:
		scopeID, err := shared.SafeGetString(args, "scope_id")
		if err != nil {
			return "", fmt.Errorf("append: %w", err)
		}
		path = fmt.Sprintf("/api/v1/meetings/%s/docs/%s/append", scopeID, slotKey)

	case ScopeTask:
		projectID, err := shared.SafeGetString(args, "project_id")
		if err != nil {
			return "", fmt.Errorf("append: %w", err)
		}
		taskID, err := shared.SafeGetString(args, "task_id")
		if err != nil {
			return "", fmt.Errorf("append: %w", err)
		}
		// Task 使用旧的追加 API（保持兼容）
		path = fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s", projectID, taskID, slotKey)
		// PUT 方法用于追加
		return shared.CallAPI(apiClient, "PUT", path, body, clientToken)
	}

	return shared.CallAPI(apiClient, "POST", path, body, clientToken)
}

// UnifiedDocSectionsTool 统一文档章节工具
type UnifiedDocSectionsTool struct {
	Scope     DocumentScope
	Operation string // "get_list", "get_one", "update", "insert", "delete"
}

func (t *UnifiedDocSectionsTool) Name() string {
	prefix := "project"
	switch t.Scope {
	case ScopeMeeting:
		prefix = "meeting"
	case ScopeTask:
		prefix = "task"
	}

	switch t.Operation {
	case "get_list":
		return fmt.Sprintf("get_%s_doc_sections", prefix)
	case "get_one":
		return fmt.Sprintf("get_%s_doc_section", prefix)
	case "update":
		return fmt.Sprintf("update_%s_doc_section", prefix)
	case "insert":
		return fmt.Sprintf("insert_%s_doc_section", prefix)
	case "delete":
		return fmt.Sprintf("delete_%s_doc_section", prefix)
	default:
		return fmt.Sprintf("%s_doc_sections", prefix)
	}
}

func (t *UnifiedDocSectionsTool) Description() string {
	var scopeDesc string
	switch t.Scope {
	case ScopeProject:
		scopeDesc = "项目"
	case ScopeMeeting:
		scopeDesc = "会议"
	case ScopeTask:
		scopeDesc = "任务"
	}

	switch t.Operation {
	case "get_list":
		return fmt.Sprintf("获取%s文档的章节列表（返回章节元数据/版本/树结构）。任何章节级新增、修改或删除操作前【必须先调用】本工具以获取最新结构。", scopeDesc)
	case "get_one":
		return fmt.Sprintf("获取%s文档单个章节当前基线（标题+Markdown 正文，可选子章节）。用于在 update 前读取基线并构造最小差异，避免全文覆盖。", scopeDesc)
	case "update":
		return fmt.Sprintf("局部章节正文更新（标题保持不变），支持 expected_version 并发防护。优先用于细粒度修改，代替全文覆盖。", scopeDesc)
	case "insert":
		return fmt.Sprintf("插入新章节（同级）。默认追加到末尾；若需精确位置请先 get_sections 并提供 after_section_id。自动同步 compiled.md。", scopeDesc)
	case "delete":
		return fmt.Sprintf("删除章节（可级联子章节 cascade=true）。操作前建议重新获取章节列表确认 ID，删除后同步 compiled.md，谨慎使用。", scopeDesc)
	default:
		return fmt.Sprintf("%s文档章节操作", scopeDesc)
	}
}

func (t *UnifiedDocSectionsTool) InputSchema() map[string]interface{} {
	var slots []string
	switch t.Scope {
	case ScopeProject:
		slots = []string{"feature_list", "architecture_design"}
	case ScopeMeeting:
		slots = []string{"polish", "summary", "topic"}
	case ScopeTask:
		slots = []string{"requirements", "design", "test"}
	}

	props := map[string]interface{}{
		"slot_key": map[string]interface{}{
			"type":        "string",
			"description": "文档槽位键名",
			"enum":        slots,
		},
	}

	required := []string{"slot_key"}

	// Add scope-specific ID fields
	switch t.Scope {
	case ScopeProject:
		props["project_id"] = map[string]interface{}{
			"type":        "string",
			"description": "项目ID",
		}
		required = append(required, "project_id")
	case ScopeMeeting:
		props["meeting_id"] = map[string]interface{}{
			"type":        "string",
			"description": "会议任务ID",
		}
		required = append(required, "meeting_id")
	case ScopeTask:
		props["project_id"] = map[string]interface{}{
			"type":        "string",
			"description": "项目ID",
		}
		props["task_id"] = map[string]interface{}{
			"type":        "string",
			"description": "任务ID",
		}
		required = append(required, "project_id", "task_id")
	}

	// Add operation-specific fields
	switch t.Operation {
	case "get_one", "update", "delete":
		props["section_id"] = map[string]interface{}{
			"type":        "string",
			"description": "章节ID",
		}
		required = append(required, "section_id")
	}

	if t.Operation == "get_one" {
		props["include_children"] = map[string]interface{}{
			"type":        "boolean",
			"description": "是否包含子章节内容（默认false）",
		}
	}

	if t.Operation == "update" {
		props["content"] = map[string]interface{}{
			"type":        "string",
			"description": "章节内容（Markdown格式）",
		}
		props["expected_version"] = map[string]interface{}{
			"type":        "number",
			"description": "期望版本号（用于版本冲突检测，可选）",
		}
		required = append(required, "content")
	}

	if t.Operation == "insert" {
		props["title"] = map[string]interface{}{
			"type":        "string",
			"description": "章节标题（包含 Markdown # 标记，如 '## 新章节'）",
		}
		props["content"] = map[string]interface{}{
			"type":        "string",
			"description": "章节内容（Markdown格式）",
		}
		props["after_section_id"] = map[string]interface{}{
			"type":        "string",
			"description": "在哪个章节后插入（可选，不提供则插入到末尾）",
		}
		props["expected_version"] = map[string]interface{}{
			"type":        "number",
			"description": "期望版本号（可选）",
		}
		required = append(required, "title", "content")
	}

	if t.Operation == "delete" {
		props["cascade"] = map[string]interface{}{
			"type":        "boolean",
			"description": "是否级联删除子章节（默认false）",
		}
		props["expected_version"] = map[string]interface{}{
			"type":        "number",
			"description": "期望版本号（可选）",
		}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func (t *UnifiedDocSectionsTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return "", fmt.Errorf("sections: %w", err)
	}

	// Build base path
	var basePath string
	switch t.Scope {
	case ScopeProject:
		projectID, err := shared.SafeGetString(args, "project_id")
		if err != nil {
			return "", fmt.Errorf("sections: %w", err)
		}
		basePath = fmt.Sprintf("/api/v1/projects/%s/docs/%s/sections", projectID, slotKey)

	case ScopeMeeting:
		meetingID, err := shared.SafeGetString(args, "meeting_id")
		if err != nil {
			return "", fmt.Errorf("sections: %w", err)
		}
		basePath = fmt.Sprintf("/api/v1/meetings/%s/docs/%s/sections", meetingID, slotKey)

	case ScopeTask:
		projectID, err := shared.SafeGetString(args, "project_id")
		if err != nil {
			return "", fmt.Errorf("sections: %w", err)
		}
		taskID, err := shared.SafeGetString(args, "task_id")
		if err != nil {
			return "", fmt.Errorf("sections: %w", err)
		}
		// Task uses old API path pattern
		basePath = fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections", projectID, taskID, slotKey)
	}

	// Execute operation
	switch t.Operation {
	case "get_list":
		return shared.CallAPI(apiClient, "GET", basePath, nil, clientToken)

	case "get_one":
		sectionID, err := shared.SafeGetString(args, "section_id")
		if err != nil {
			return "", fmt.Errorf("get_section: %w", err)
		}
		path := fmt.Sprintf("%s/%s", basePath, sectionID)
		if include, ok := args["include_children"].(bool); ok && include {
			path += "?include_children=true"
		}
		return shared.CallAPI(apiClient, "GET", path, nil, clientToken)

	case "update":
		sectionID, err := shared.SafeGetString(args, "section_id")
		if err != nil {
			return "", fmt.Errorf("update_section: %w", err)
		}
		content, err := shared.SafeGetString(args, "content")
		if err != nil {
			return "", fmt.Errorf("update_section: %w", err)
		}
		body := map[string]interface{}{"content": content}
		if v, ok := args["expected_version"]; ok {
			body["expected_version"] = v
		}
		path := fmt.Sprintf("%s/%s", basePath, sectionID)
		return shared.CallAPI(apiClient, "PUT", path, body, clientToken)

	case "insert":
		title, err := shared.SafeGetString(args, "title")
		if err != nil {
			return "", fmt.Errorf("insert_section: %w", err)
		}
		content, err := shared.SafeGetString(args, "content")
		if err != nil {
			return "", fmt.Errorf("insert_section: %w", err)
		}
		body := map[string]interface{}{
			"title":   title,
			"content": content,
		}
		if v, ok := args["after_section_id"]; ok && v != nil && v != "" {
			body["after_section_id"] = v
		}
		if v, ok := args["expected_version"]; ok {
			body["expected_version"] = v
		}
		return shared.CallAPI(apiClient, "POST", basePath, body, clientToken)

	case "delete":
		sectionID, err := shared.SafeGetString(args, "section_id")
		if err != nil {
			return "", fmt.Errorf("delete_section: %w", err)
		}
		path := fmt.Sprintf("%s/%s", basePath, sectionID)
		// Build query params
		params := []string{}
		if cascade, ok := args["cascade"].(bool); ok && cascade {
			params = append(params, "cascade=true")
		}
		if v, ok := args["expected_version"]; ok {
			params = append(params, fmt.Sprintf("expected_version=%v", v))
		}
		if len(params) > 0 {
			path += "?" + strings.Join(params, "&")
		}
		return shared.CallAPI(apiClient, "DELETE", path, nil, clientToken)
	}

	return "", fmt.Errorf("unknown operation: %s", t.Operation)
}

// RegisterUnifiedDocTools 注册所有统一文档工具
func RegisterUnifiedDocTools(registry interface{ Register(shared.Tool) }) {
	// Project document tools
	registry.Register(&UnifiedDocExportTool{Scope: ScopeProject})
	registry.Register(&UnifiedDocAppendTool{Scope: ScopeProject})

	// Meeting document tools
	registry.Register(&UnifiedDocExportTool{Scope: ScopeMeeting})
	registry.Register(&UnifiedDocAppendTool{Scope: ScopeMeeting})

	// Project sections tools
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeProject, Operation: "get_list"})
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeProject, Operation: "get_one"})
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeProject, Operation: "update"})
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeProject, Operation: "insert"})
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeProject, Operation: "delete"})

	// Meeting sections tools
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeMeeting, Operation: "get_list"})
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeMeeting, Operation: "get_one"})
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeMeeting, Operation: "update"})
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeMeeting, Operation: "insert"})
	registry.Register(&UnifiedDocSectionsTool{Scope: ScopeMeeting, Operation: "delete"})
}
