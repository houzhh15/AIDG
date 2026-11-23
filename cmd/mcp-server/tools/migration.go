package tools

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

type PreviewSchemaMigrationTool struct{}

func (t *PreviewSchemaMigrationTool) Name() string { return "preview_schema_migration" }
func (t *PreviewSchemaMigrationTool) Description() string {
	return "只读预览项目或任务的显式 schema 迁移；不创建或修改任何资源。输出仅含资源标识、版本/hash、摘要和脱敏问题，不含文档全文或认证/会话信息。"
}
func (t *PreviewSchemaMigrationTool) InputSchema() map[string]interface{} {
	return migrationInputSchema(false)
}
func (t *PreviewSchemaMigrationTool) Execute(args map[string]interface{}, token string, client *shared.APIClient) (string, error) {
	projectID, path, err := migrationPath(args, "preview")
	if err != nil {
		return "", err
	}
	_ = projectID
	return shared.CallAPI(client, "POST", path, map[string]interface{}{}, token)
}

type ExecuteSchemaMigrationTool struct{}

func (t *ExecuteSchemaMigrationTool) Name() string { return "execute_schema_migration" }
func (t *ExecuteSchemaMigrationTool) Description() string {
	return "执行先前预览的 schema 迁移计划；计划基线变化时拒绝执行。"
}
func (t *ExecuteSchemaMigrationTool) InputSchema() map[string]interface{} {
	return migrationInputSchema(true)
}
func (t *ExecuteSchemaMigrationTool) Execute(args map[string]interface{}, token string, client *shared.APIClient) (string, error) {
	_, path, err := migrationPath(args, "execute")
	if err != nil {
		return "", err
	}
	plan, ok := args["plan"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("plan object from preview is required")
	}
	return shared.CallAPI(client, "POST", path, map[string]interface{}{"plan": plan}, token)
}

func migrationInputSchema(execute bool) map[string]interface{} {
	properties := map[string]interface{}{
		"project_id": map[string]interface{}{"type": "string", "description": "项目ID（必填）"},
		"task_id":    map[string]interface{}{"type": "string", "description": "任务范围时必填；省略表示项目范围"},
	}
	required := []string{"project_id"}
	if execute {
		properties["plan"] = map[string]interface{}{"type": "object", "description": "preview 返回的完整 plan"}
		required = append(required, "plan")
	}
	return map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func migrationPath(args map[string]interface{}, operation string) (string, string, error) {
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil {
		return "", "", fmt.Errorf("explicit project_id is required")
	}
	escapedProjectID := url.PathEscape(projectID)
	if rawTaskID, present := args["task_id"]; present {
		taskID, ok := rawTaskID.(string)
		if !ok || strings.TrimSpace(taskID) == "" {
			return "", "", fmt.Errorf("task_id must be a non-empty string when provided")
		}
		return projectID, fmt.Sprintf("/api/v1/projects/%s/tasks/%s/migrations/schema/%s", escapedProjectID, url.PathEscape(taskID), operation), nil
	}
	return projectID, fmt.Sprintf("/api/v1/projects/%s/migrations/schema/%s", escapedProjectID, operation), nil
}
