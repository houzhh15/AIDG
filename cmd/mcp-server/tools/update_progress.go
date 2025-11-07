package tools

import (
	"fmt"
	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// UpdateProgressTool 进展更新工具（批量更新季度/月/周总结）
type UpdateProgressTool struct{}

func (t *UpdateProgressTool) Name() string {
	return "update_progress"
}

func (t *UpdateProgressTool) Description() string {
	return "批量更新项目进展总结（季度/月/周总结）。可以选择性地更新一个或多个层级的总结。"
}

func (t *UpdateProgressTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID",
			},
			"week_number": map[string]interface{}{
				"type":        "string",
				"description": "周编号（格式：YYYY-WW，如：2025-05）",
			},
			"quarter_summary": map[string]interface{}{
				"type":        "string",
				"description": "季度总结（可选，Markdown格式）",
			},
			"month_summary": map[string]interface{}{
				"type":        "string",
				"description": "月度总结（可选，Markdown格式）",
			},
			"week_summary": map[string]interface{}{
				"type":        "string",
				"description": "周总结（可选，Markdown格式）",
			},
		},
		"required": []string{"project_id", "week_number"},
	}
}

func (t *UpdateProgressTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取project_id
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil {
		return "", fmt.Errorf("update_progress: %w", err)
	}

	// 2. 提取week_number
	weekNumber, err := shared.SafeGetString(args, "week_number")
	if err != nil {
		return "", fmt.Errorf("update_progress: %w", err)
	}

	// 3. 构造更新数据
	updateData := make(map[string]interface{})

	if quarterSummary, ok := args["quarter_summary"].(string); ok && quarterSummary != "" {
		updateData["quarter_summary"] = quarterSummary
	}

	if monthSummary, ok := args["month_summary"].(string); ok && monthSummary != "" {
		updateData["month_summary"] = monthSummary
	}

	if weekSummary, ok := args["week_summary"].(string); ok && weekSummary != "" {
		updateData["week_summary"] = weekSummary
	}

	// 4. 至少需要提供一个总结
	if len(updateData) == 0 {
		return "", fmt.Errorf("update_progress: 至少需要提供 quarter_summary、month_summary 或 week_summary 之一")
	}

	// 5. 构造API路径
	path := fmt.Sprintf("/api/v1/projects/%s/progress/week/%s", projectID, weekNumber)

	// 6. 调用API
	return shared.CallAPI(apiClient, "PUT", path, updateData, clientToken)
}
