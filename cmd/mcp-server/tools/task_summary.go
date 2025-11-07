package tools

import (
	"fmt"
	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
	"net/url"
)

// TaskSummaryTool 任务总结工具（统一处理）
type TaskSummaryTool struct{}

func (t *TaskSummaryTool) Name() string {
	return "task_summary"
}

func (t *TaskSummaryTool) Description() string {
	return "统一处理任务总结的CRUD操作和查询。支持的action：list（获取总结列表）、add（添加总结）、update（更新总结）、delete（删除总结）、query_by_week（跨任务周范围查询）"
}

func (t *TaskSummaryTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "操作类型",
				"enum":        []string{"list", "add", "update", "delete", "query_by_week"},
			},
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "任务ID（action=list/add/update/delete时必填，query_by_week时可选）",
			},
			"summary_id": map[string]interface{}{
				"type":        "string",
				"description": "总结ID（action=update/delete时必填）",
			},
			"time": map[string]interface{}{
				"type":        "string",
				"description": "总结时间（action=add时必填，update时可选，ISO 8601格式）",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "总结内容（action=add时必填，update时可选，Markdown格式）",
			},
			"start_week": map[string]interface{}{
				"type":        "string",
				"description": "开始周（action=list/query_by_week时可选，格式：YYYY-WW）",
			},
			"end_week": map[string]interface{}{
				"type":        "string",
				"description": "结束周（action=list/query_by_week时可选，格式：YYYY-WW）",
			},
		},
		"required": []string{"action", "project_id"},
	}
}

func (t *TaskSummaryTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取action参数
	action, err := shared.SafeGetString(args, "action")
	if err != nil {
		return "", fmt.Errorf("task_summary: %w", err)
	}

	// 2. 提取project_id
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil {
		return "", fmt.Errorf("task_summary: %w", err)
	}

	// 3. 根据action分发到不同处理逻辑
	switch action {
	case "list":
		return t.listTaskSummaries(args, projectID, clientToken, apiClient)
	case "add":
		return t.addTaskSummary(args, projectID, clientToken, apiClient)
	case "update":
		return t.updateTaskSummary(args, projectID, clientToken, apiClient)
	case "delete":
		return t.deleteTaskSummary(args, projectID, clientToken, apiClient)
	case "query_by_week":
		return t.queryByWeek(args, projectID, clientToken, apiClient)
	default:
		return "", fmt.Errorf("task_summary: 无效的action '%s'，支持的值：list, add, update, delete, query_by_week", action)
	}
}

// listTaskSummaries 获取任务总结列表
func (t *TaskSummaryTool) listTaskSummaries(
	args map[string]interface{},
	projectID string,
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 提取task_id参数
	taskID, err := shared.SafeGetString(args, "task_id")
	if err != nil {
		return "", fmt.Errorf("task_summary[list]: 缺少必填参数 'task_id'")
	}

	// 构造API路径（带查询参数）
	path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/summaries", projectID, taskID)

	// 添加可选的周范围过滤参数
	queryParams := url.Values{}
	if startWeek, ok := args["start_week"].(string); ok && startWeek != "" {
		queryParams.Add("start_week", startWeek)
	}
	if endWeek, ok := args["end_week"].(string); ok && endWeek != "" {
		queryParams.Add("end_week", endWeek)
	}

	if len(queryParams) > 0 {
		path = path + "?" + queryParams.Encode()
	}

	// 调用API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// addTaskSummary 添加任务总结
func (t *TaskSummaryTool) addTaskSummary(
	args map[string]interface{},
	projectID string,
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 提取task_id参数
	taskID, err := shared.SafeGetString(args, "task_id")
	if err != nil {
		return "", fmt.Errorf("task_summary[add]: 缺少必填参数 'task_id'")
	}

	// 提取time参数
	timeStr, err := shared.SafeGetString(args, "time")
	if err != nil {
		return "", fmt.Errorf("task_summary[add]: 缺少必填参数 'time'")
	}

	// 提取content参数
	content, err := shared.SafeGetString(args, "content")
	if err != nil {
		return "", fmt.Errorf("task_summary[add]: 缺少必填参数 'content'")
	}

	// 构造请求数据
	data := map[string]interface{}{
		"time":    timeStr,
		"content": content,
	}

	// 构造API路径
	path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/summaries", projectID, taskID)

	// 调用API
	return shared.CallAPI(apiClient, "POST", path, data, clientToken)
}

// updateTaskSummary 更新任务总结
func (t *TaskSummaryTool) updateTaskSummary(
	args map[string]interface{},
	projectID string,
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 提取task_id参数
	taskID, err := shared.SafeGetString(args, "task_id")
	if err != nil {
		return "", fmt.Errorf("task_summary[update]: 缺少必填参数 'task_id'")
	}

	// 提取summary_id参数
	summaryID, err := shared.SafeGetString(args, "summary_id")
	if err != nil {
		return "", fmt.Errorf("task_summary[update]: 缺少必填参数 'summary_id'")
	}

	// 构造更新数据（至少需要一个字段）
	updateData := make(map[string]interface{})

	if timeStr, ok := args["time"].(string); ok && timeStr != "" {
		updateData["time"] = timeStr
	}

	if content, ok := args["content"].(string); ok && content != "" {
		updateData["content"] = content
	}

	// 至少需要提供一个字段
	if len(updateData) == 0 {
		return "", fmt.Errorf("task_summary[update]: 至少需要提供 time 或 content 之一")
	}

	// 构造API路径
	path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/summaries/%s", projectID, taskID, summaryID)

	// 调用API
	return shared.CallAPI(apiClient, "PUT", path, updateData, clientToken)
}

// deleteTaskSummary 删除任务总结
func (t *TaskSummaryTool) deleteTaskSummary(
	args map[string]interface{},
	projectID string,
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 提取task_id参数
	taskID, err := shared.SafeGetString(args, "task_id")
	if err != nil {
		return "", fmt.Errorf("task_summary[delete]: 缺少必填参数 'task_id'")
	}

	// 提取summary_id参数
	summaryID, err := shared.SafeGetString(args, "summary_id")
	if err != nil {
		return "", fmt.Errorf("task_summary[delete]: 缺少必填参数 'summary_id'")
	}

	// 构造API路径
	path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/summaries/%s", projectID, taskID, summaryID)

	// 调用API
	return shared.CallAPI(apiClient, "DELETE", path, nil, clientToken)
}

// queryByWeek 跨任务按周范围查询
func (t *TaskSummaryTool) queryByWeek(
	args map[string]interface{},
	projectID string,
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 构造API路径
	path := fmt.Sprintf("/api/v1/projects/%s/summaries/by-week", projectID)

	// 添加查询参数
	queryParams := url.Values{}

	// start_week（必填）
	if startWeek, ok := args["start_week"].(string); ok && startWeek != "" {
		queryParams.Add("start_week", startWeek)
	} else {
		return "", fmt.Errorf("task_summary[query_by_week]: 缺少必填参数 'start_week'")
	}

	// end_week（可选）
	if endWeek, ok := args["end_week"].(string); ok && endWeek != "" {
		queryParams.Add("end_week", endWeek)
	}

	// task_id（可选）
	if taskID, ok := args["task_id"].(string); ok && taskID != "" {
		queryParams.Add("task_id", taskID)
	}

	path = path + "?" + queryParams.Encode()

	// 调用API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}
