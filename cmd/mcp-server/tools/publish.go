package tools

import (
	"fmt"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

type PublishManifestTool struct{}

func (t *PublishManifestTool) Name() string { return "publish_manifest" }
func (t *PublishManifestTool) Description() string {
	return "原子发布单项目 manifest v1；仅更新已存在资源，不抓取 URL。"
}
func (t *PublishManifestTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{"type": "string", "description": "项目ID（必填，须与 manifest.project_id 一致）"},
			"manifest": map[string]interface{}{
				"type":        "object",
				"description": "publish manifest v1，包含 manifest_version/project_id/idempotency_key/source/resources",
			},
		},
		"required": []string{"project_id", "manifest"},
	}
}
func (t *PublishManifestTool) Execute(args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil {
		return "", fmt.Errorf("explicit project_id is required")
	}
	manifest, ok := args["manifest"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("manifest object is required")
	}
	if value, ok := manifest["project_id"].(string); !ok || value != projectID {
		return "", fmt.Errorf("manifest project_id must match project_id")
	}
	return shared.CallAPI(apiClient, "POST", fmt.Sprintf("/api/v1/projects/%s/publish", projectID), manifest, clientToken)
}
