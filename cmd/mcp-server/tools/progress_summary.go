package tools

import (
	"encoding/json"
	"fmt"
	"github.com/houzhh15-hub/AIDG/cmd/mcp-server/shared"
	"time"
)

// ProgressSummaryTool 项目进展总结工具（统一处理）
type ProgressSummaryTool struct{}

func (t *ProgressSummaryTool) Name() string {
	return "progress_summary"
}

func (t *ProgressSummaryTool) Description() string {
	return "统一处理项目进展查询和更新操作（年度树、周进展、当前周）。支持的action：get_year（获取年度进展树）、get_week（获取周进展含上下文）、get_current_week（获取当前周编号）、update（更新周进展）"
}

func (t *ProgressSummaryTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "操作类型",
				"enum":        []string{"get_year", "get_week", "get_current_week", "update"},
			},
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID",
			},
			"year": map[string]interface{}{
				"type":        "integer",
				"description": "年份（action=get_year时必填，如：2025）",
			},
			"week_number": map[string]interface{}{
				"type":        "string",
				"description": "周编号（action=get_week或update时必填，格式：YYYY-WW，如：2025-05）",
			},
			"date": map[string]interface{}{
				"type":        "string",
				"description": "日期（action=get_current_week时可选，格式：YYYY-MM-DD，默认今天）",
			},
			"quarter_summary": map[string]interface{}{
				"type":        "string",
				"description": "季度总结（action=update时可选，Markdown格式）",
			},
			"month_summary": map[string]interface{}{
				"type":        "string",
				"description": "月度总结（action=update时可选，Markdown格式）",
			},
			"week_summary": map[string]interface{}{
				"type":        "string",
				"description": "周总结（action=update时可选，Markdown格式）",
			},
		},
		"required": []string{"action", "project_id"},
	}
}

func (t *ProgressSummaryTool) Execute(
	args map[string]interface{},
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 1. 提取action参数
	action, err := shared.SafeGetString(args, "action")
	if err != nil {
		return "", fmt.Errorf("progress_summary: %w", err)
	}

	// 2. 提取project_id
	projectID, err := shared.SafeGetString(args, "project_id")
	if err != nil {
		return "", fmt.Errorf("progress_summary: %w", err)
	}

	// 3. 根据action分发到不同处理逻辑
	switch action {
	case "get_year":
		return t.getYearProgress(args, projectID, clientToken, apiClient)
	case "get_week":
		return t.getWeekProgress(args, projectID, clientToken, apiClient)
	case "get_current_week":
		return t.getCurrentWeek(args)
	case "update":
		return t.updateWeekProgress(args, projectID, clientToken, apiClient)
	default:
		return "", fmt.Errorf("progress_summary: 无效的action '%s'，支持的值：get_year, get_week, get_current_week, update", action)
	}
}

// getYearProgress 获取年度进展树
func (t *ProgressSummaryTool) getYearProgress(
	args map[string]interface{},
	projectID string,
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 提取year参数
	yearFloat, ok := args["year"].(float64)
	if !ok {
		return "", fmt.Errorf("progress_summary[get_year]: 缺少必填参数 'year'")
	}
	year := int(yearFloat)

	// 构造API路径
	path := fmt.Sprintf("/api/v1/projects/%s/progress/year/%d", projectID, year)

	// 调用API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// getWeekProgress 获取周进展（含季度/月/周上下文）
func (t *ProgressSummaryTool) getWeekProgress(
	args map[string]interface{},
	projectID string,
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 提取week_number参数
	weekNumber, err := shared.SafeGetString(args, "week_number")
	if err != nil {
		return "", fmt.Errorf("progress_summary[get_week]: 缺少必填参数 'week_number'")
	}

	// 构造API路径
	path := fmt.Sprintf("/api/v1/projects/%s/progress/week/%s", projectID, weekNumber)

	// 调用API
	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}

// getCurrentWeek 获取当前周编号（本地计算）
func (t *ProgressSummaryTool) getCurrentWeek(args map[string]interface{}) (string, error) {
	// 提取date参数（可选）
	dateStr, _ := args["date"].(string)

	var targetDate time.Time
	var err error

	if dateStr == "" {
		// 使用今天
		targetDate = time.Now()
	} else {
		// 解析指定日期
		targetDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			return "", fmt.Errorf("progress_summary[get_current_week]: 无效的日期格式 '%s'，应为 YYYY-MM-DD", dateStr)
		}
	}

	// 计算ISO 8601周编号
	year, week := targetDate.ISOWeek()
	weekNumber := fmt.Sprintf("%d-%02d", year, week)

	// 计算周范围
	weekRange := getWeekRange(year, week)

	// 构造返回结果
	result := map[string]interface{}{
		"week_number": weekNumber,
		"year":        year,
		"week":        week,
		"week_range":  weekRange,
		"date":        targetDate.Format("2006-01-02"),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("progress_summary[get_current_week]: 序列化结果失败: %w", err)
	}

	return string(resultJSON), nil
}

// updateWeekProgress 更新周进展
func (t *ProgressSummaryTool) updateWeekProgress(
	args map[string]interface{},
	projectID string,
	clientToken string,
	apiClient *shared.APIClient,
) (string, error) {
	// 提取week_number参数
	weekNumber, err := shared.SafeGetString(args, "week_number")
	if err != nil {
		return "", fmt.Errorf("progress_summary[update]: 缺少必填参数 'week_number'")
	}

	// 构造更新数据
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

	// 至少需要提供一个总结
	if len(updateData) == 0 {
		return "", fmt.Errorf("progress_summary[update]: 至少需要提供 quarter_summary、month_summary 或 week_summary 之一")
	}

	// 构造API路径
	path := fmt.Sprintf("/api/v1/projects/%s/progress/week/%s", projectID, weekNumber)

	// 调用API
	return shared.CallAPI(apiClient, "PUT", path, updateData, clientToken)
}

// getWeekRange 计算ISO周的日期范围（格式：MM/DD-MM/DD）
func getWeekRange(year, week int) string {
	// 根据ISO 8601标准计算周的第一天（周一）
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)

	// 找到第一个周一
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := jan4.AddDate(0, 0, -(weekday - 1))

	// 计算目标周的周一
	start := monday.AddDate(0, 0, (week-1)*7)
	end := start.AddDate(0, 0, 6)

	return fmt.Sprintf("%02d/%02d-%02d/%02d",
		start.Month(), start.Day(),
		end.Month(), end.Day())
}
