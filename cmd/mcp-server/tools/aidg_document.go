package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

var aidgLineNumberPrefixRe = regexp.MustCompile(`(?m)^\s*\d+\t`)

type aidgDocExportResponse struct {
	Content   string `json:"content"`
	Version   int    `json:"version"`
	ETag      string `json:"etag"`
	UpdatedAt string `json:"updated_at"`
	Exists    bool   `json:"exists"`
}

type aidgDocumentTarget struct {
	ProjectID string
	TaskID    string
	SlotKey   string
	TaskScope bool
}

type AIDGReadDocumentTool struct{}

type AIDGEditDocumentTool struct{}

type AIDGWriteDocumentTool struct{}

func (t *AIDGReadDocumentTool) Name() string { return "aidg_read_document" }

func (t *AIDGReadDocumentTool) Description() string {
	return "读取 AIDG 文档内容，对应 read_file 语义。通过 slot_key 自动区分任务文档（requirements/design/test，需要 task_id）和项目文档（feature_list/architecture_design）。支持 start_line/end_line 部分读取，返回带行号的内容。"
}

func (t *AIDGReadDocumentTool) InputSchema() map[string]interface{} {
	return aidgDocumentBaseSchema(false, false)
}

func (t *AIDGReadDocumentTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	target, err := resolveAIDGDocumentTarget(args, apiClient, clientToken)
	if err != nil {
		return "", err
	}
	exported, err := exportAIDGDocument(target, clientToken, apiClient)
	if err != nil {
		return "", err
	}
	startLine, _ := optionalInt(args, "start_line")
	endLine, _ := optionalInt(args, "end_line")
	content, start, end, total, truncated := numberAIDGLines(exported.Content, startLine, endLine)
	resp := map[string]interface{}{
		"content":     content,
		"version":     exported.Version,
		"etag":        exported.ETag,
		"updated_at":  exported.UpdatedAt,
		"exists":      exported.Exists,
		"start_line":  start,
		"end_line":    end,
		"total_lines": total,
		"truncated":   truncated,
	}
	b, _ := json.Marshal(resp)
	return string(b), nil
}

func (t *AIDGWriteDocumentTool) Name() string { return "aidg_write_document" }

func (t *AIDGWriteDocumentTool) Description() string {
	return "创建、覆盖或追加 AIDG 文档内容，对应 write_file 语义。append=false 覆盖写入，append=true 追加写入；长文分块时首次覆盖，后续使用 append=true。为支持截断恢复，调用时应先给出 project_id/task_id/slot_key/append，再给出 content。"
}

func (t *AIDGWriteDocumentTool) InputSchema() map[string]interface{} {
	return aidgDocumentBaseSchema(true, false)
}

func (t *AIDGWriteDocumentTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	target, err := resolveAIDGDocumentTarget(args, apiClient, clientToken)
	if err != nil {
		return "", err
	}
	content, err := shared.SafeGetString(args, "content")
	if err != nil {
		return "", fmt.Errorf("aidg_write_document: %w", err)
	}
	return writeAIDGDocument(target, content, optionalBool(args, "append"), args["expected_version"], clientToken, apiClient)
}

func (t *AIDGEditDocumentTool) Name() string { return "aidg_edit_document" }

func (t *AIDGEditDocumentTool) Description() string {
	return "精确编辑 AIDG 文档内容，对应 edit_file 语义。编辑时传 old_text/new_text；old_text 必须精确匹配原始内容。若只传 start_line/end_line，将返回 old_text_hint 供下一次调用。默认要求唯一匹配，replace_all=true 时替换所有匹配。"
}

func (t *AIDGEditDocumentTool) InputSchema() map[string]interface{} {
	return aidgDocumentBaseSchema(false, true)
}

func (t *AIDGEditDocumentTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	target, err := resolveAIDGDocumentTarget(args, apiClient, clientToken)
	if err != nil {
		return "", err
	}
	exported, err := exportAIDGDocument(target, clientToken, apiClient)
	if err != nil {
		return "", err
	}
	oldText, oldOK := args["old_text"].(string)
	newText, newOK := args["new_text"].(string)
	if !oldOK || oldText == "" {
		startLine, _ := optionalInt(args, "start_line")
		endLine, _ := optionalInt(args, "end_line")
		hint, start, end, total, _ := rawAIDGLineRange(exported.Content, startLine, endLine)
		b, _ := json.Marshal(map[string]interface{}{
			"error":         "old_text_required",
			"message":       "edit_document does not edit by line number; call again with this exact old_text",
			"old_text_hint": hint,
			"start_line":    start,
			"end_line":      end,
			"total_lines":   total,
		})
		return string(b), nil
	}
	if !newOK {
		return "", fmt.Errorf("aidg_edit_document: new_text is required when old_text is provided")
	}
	oldText = stripAIDGReadLinePrefixes(oldText)
	newText = stripAIDGReadLinePrefixes(newText)
	matches := strings.Count(exported.Content, oldText)
	if matches == 0 {
		return "", fmt.Errorf("old_text not found in document; make sure whitespace matches exactly")
	}
	replaceAll := optionalBool(args, "replace_all")
	if matches > 1 && !replaceAll {
		return "", fmt.Errorf("old_text matches %d locations; provide more context or set replace_all=true", matches)
	}
	next := strings.Replace(exported.Content, oldText, newText, 1)
	if replaceAll {
		next = strings.ReplaceAll(exported.Content, oldText, newText)
	}
	return writeAIDGDocument(target, next, false, exported.Version, clientToken, apiClient)
}

func aidgDocumentBaseSchema(includeContent, edit bool) map[string]interface{} {
	props := map[string]interface{}{
		"project_id": map[string]interface{}{"type": "string", "description": "项目ID（可选，缺失时从当前任务获取）"},
		"task_id":    map[string]interface{}{"type": "string", "description": "任务ID（任务文档可选，缺失时从当前任务获取；项目文档忽略）"},
		"slot_key": map[string]interface{}{
			"type":        "string",
			"description": "文档槽位。任务文档：requirements/design/test；项目文档：feature_list/architecture_design",
			"enum":        []string{"requirements", "design", "test", "feature_list", "architecture_design"},
		},
	}
	required := []string{"slot_key"}
	if includeContent {
		props["append"] = map[string]interface{}{"type": "boolean", "description": "是否追加到现有文档。默认 false 表示覆盖"}
		props["expected_version"] = map[string]interface{}{"type": "number", "description": "期望版本号（可选），用于乐观锁"}
		props["content"] = map[string]interface{}{"type": "string", "description": "要写入的 Markdown 文档内容"}
		required = append(required, "content")
	}
	if edit {
		props["old_text"] = map[string]interface{}{"type": "string", "description": "要替换的原始文本，必须精确匹配"}
		props["new_text"] = map[string]interface{}{"type": "string", "description": "替换后的文本，可为空字符串用于删除"}
		props["replace_all"] = map[string]interface{}{"type": "boolean", "description": "是否替换所有匹配。默认 false"}
	}
	props["start_line"] = map[string]interface{}{"type": "number", "description": "开始行号（1-based，可选；edit 缺少 old_text 时用于回显 old_text_hint）"}
	props["end_line"] = map[string]interface{}{"type": "number", "description": "结束行号（1-based，可选；edit 缺少 old_text 时用于回显 old_text_hint）"}
	return map[string]interface{}{"type": "object", "properties": props, "required": required}
}

func resolveAIDGDocumentTarget(args map[string]interface{}, apiClient *shared.APIClient, clientToken string) (aidgDocumentTarget, error) {
	slotKey, err := shared.SafeGetString(args, "slot_key")
	if err != nil {
		return aidgDocumentTarget{}, err
	}
	if isAIDGTaskSlot(slotKey) {
		if _, projectProvided := args["project_id"]; projectProvided {
			if taskID, err := shared.SafeGetString(args, "task_id"); err != nil || taskID == "" {
				return aidgDocumentTarget{}, fmt.Errorf("slot_key=%s is a task document slot; task_id is required when project_id is provided", slotKey)
			}
		}
		projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(args, apiClient, clientToken)
		if err != nil {
			return aidgDocumentTarget{}, err
		}
		return aidgDocumentTarget{ProjectID: projectID, TaskID: taskID, SlotKey: slotKey, TaskScope: true}, nil
	}
	if isAIDGProjectSlot(slotKey) {
		if _, taskProvided := args["task_id"]; taskProvided {
			return aidgDocumentTarget{}, fmt.Errorf("slot_key=%s is a project document slot; task_id must not be provided", slotKey)
		}
		projectID, err := shared.GetProjectIDWithFallback(args, apiClient, clientToken)
		if err != nil {
			return aidgDocumentTarget{}, err
		}
		return aidgDocumentTarget{ProjectID: projectID, SlotKey: slotKey}, nil
	}
	return aidgDocumentTarget{}, fmt.Errorf("invalid slot_key: %s", slotKey)
}

func exportAIDGDocument(target aidgDocumentTarget, clientToken string, apiClient *shared.APIClient) (aidgDocExportResponse, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/docs/%s/export", target.ProjectID, target.SlotKey)
	if target.TaskScope {
		path = fmt.Sprintf("/api/v1/projects/%s/tasks/%s/docs/%s/export", target.ProjectID, target.TaskID, target.SlotKey)
	}
	resp, err := shared.CallAPI(apiClient, "GET", path, nil, clientToken)
	if err != nil {
		return aidgDocExportResponse{}, err
	}
	var out aidgDocExportResponse
	if err := json.Unmarshal([]byte(resp), &out); err != nil {
		return aidgDocExportResponse{}, fmt.Errorf("parse document export response: %w", err)
	}
	return out, nil
}

func writeAIDGDocument(target aidgDocumentTarget, content string, appendMode bool, expectedVersion interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	body := map[string]interface{}{"content": content, "source": "mcp_tool"}
	if appendMode {
		body["op"] = "add_full"
	} else {
		body["op"] = "replace_full"
	}
	if expectedVersion != nil {
		body["expected_version"] = expectedVersion
	}
	path := fmt.Sprintf("/api/v1/projects/%s/docs/%s/append", target.ProjectID, target.SlotKey)
	if target.TaskScope {
		path = fmt.Sprintf("/api/v1/projects/%s/tasks/%s/docs/%s/append", target.ProjectID, target.TaskID, target.SlotKey)
	}
	return shared.CallAPI(apiClient, "POST", path, body, clientToken)
}

func isAIDGTaskSlot(slotKey string) bool {
	switch slotKey {
	case "requirements", "design", "test":
		return true
	default:
		return false
	}
}

func isAIDGProjectSlot(slotKey string) bool {
	switch slotKey {
	case "feature_list", "architecture_design":
		return true
	default:
		return false
	}
}

func optionalBool(args map[string]interface{}, key string) bool {
	v, ok := args[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func optionalInt(args map[string]interface{}, key string) (int, bool) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := strconv.Atoi(n.String())
		return i, err == nil
	default:
		return 0, false
	}
}

func stripAIDGReadLinePrefixes(s string) string {
	return aidgLineNumberPrefixRe.ReplaceAllString(s, "")
}

func splitAIDGLinesPreserve(content string) []string {
	if content == "" {
		return []string{}
	}
	parts := strings.SplitAfter(content, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func numberAIDGLines(content string, startLine, endLine int) (string, int, int, int, bool) {
	lines := splitAIDGLinesPreserve(content)
	total := len(lines)
	if total == 0 {
		return "", 0, 0, 0, false
	}
	start, end, truncated := normalizeAIDGRange(total, startLine, endLine)
	var b strings.Builder
	for i := start; i <= end; i++ {
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('\t')
		b.WriteString(lines[i-1])
		if !strings.HasSuffix(lines[i-1], "\n") && i < end {
			b.WriteByte('\n')
		}
	}
	return b.String(), start, end, total, truncated
}

func rawAIDGLineRange(content string, startLine, endLine int) (string, int, int, int, bool) {
	lines := splitAIDGLinesPreserve(content)
	total := len(lines)
	if total == 0 {
		return "", 0, 0, 0, false
	}
	start, end, truncated := normalizeAIDGRange(total, startLine, endLine)
	return strings.Join(lines[start-1:end], ""), start, end, total, truncated
}

func normalizeAIDGRange(total, startLine, endLine int) (int, int, bool) {
	start := startLine
	if start <= 0 {
		start = 1
	}
	if start > total {
		start = total
	}
	end := endLine
	truncated := false
	if end <= 0 {
		end = start + 399
		if end < total {
			truncated = true
		}
	}
	if end > total {
		end = total
	}
	if end < start {
		end = start
	}
	if end < total {
		truncated = true
	}
	return start, end, truncated
}
