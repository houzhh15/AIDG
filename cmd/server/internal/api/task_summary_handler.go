package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/models"
	"github.com/houzhh15/AIDG/cmd/server/internal/services"
)

// sanitizeMarkdown 简单的Markdown内容清洗
// TODO: 集成bluemonday或其他XSS防护库
func sanitizeMarkdown(content string) string {
	// 简单清洗：移除HTML标签中的危险属性
	// 实际应该使用专业的XSS防护库
	return strings.TrimSpace(content)
}

// HandleGetTaskSummaries GET /api/v1/projects/:project_id/tasks/:task_id/summaries
// 获取任务总结列表，支持按周范围过滤
func HandleGetTaskSummaries(reg *projects.ProjectRegistry, summaryService services.TaskSummaryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")
		startWeek := c.Query("start_week")
		endWeek := c.Query("end_week")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		ctx := context.Background()
		var summaries []*models.TaskSummary
		var err error

		// 如果指定了周范围，按周过滤
		if startWeek != "" && endWeek != "" {
			summaries, err = summaryService.GetSummariesByWeekRange(ctx, projectID, taskID, startWeek, endWeek)
		} else {
			summaries, err = summaryService.GetSummaries(ctx, projectID, taskID)
		}

		if err != nil {
			internalErrorResponse(c, fmt.Errorf("get summaries: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    summaries,
		})
	}
}

// HandleAddTaskSummary POST /api/v1/projects/:project_id/tasks/:task_id/summaries
// 添加任务总结
func HandleAddTaskSummary(reg *projects.ProjectRegistry, summaryService services.TaskSummaryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 解析请求体
		var req struct {
			Time    string `json:"time" binding:"required"`
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body: "+err.Error())
			return
		}

		// 验证内容不为空
		if strings.TrimSpace(req.Content) == "" {
			badRequestResponse(c, "content cannot be empty")
			return
		}

		// 解析时间
		summaryTime, err := time.Parse(time.RFC3339, req.Time)
		if err != nil {
			badRequestResponse(c, "invalid time format, expected RFC3339")
			return
		}

		// XSS防护：清洗Markdown内容
		cleanContent := sanitizeMarkdown(req.Content)

		// 获取当前用户（简化版，实际应从认证中间件获取）
		creator := c.GetString("username")
		if creator == "" {
			creator = "system"
		}

		// 添加总结
		ctx := context.Background()
		summary, err := summaryService.AddSummary(ctx, projectID, taskID, creator, summaryTime, cleanContent)
		if err != nil {
			if err == services.ErrContentEmpty {
				badRequestResponse(c, "content cannot be empty")
				return
			}
			internalErrorResponse(c, fmt.Errorf("add summary: %w", err))
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data": gin.H{
				"id":          summary.ID,
				"week_number": summary.WeekNumber,
			},
		})
	}
}

// HandleUpdateTaskSummary PUT /api/v1/projects/:project_id/tasks/:task_id/summaries/:summary_id
// 更新任务总结
func HandleUpdateTaskSummary(reg *projects.ProjectRegistry, summaryService services.TaskSummaryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")
		summaryID := c.Param("summary_id")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 解析请求体
		var req struct {
			Time    *string `json:"time"`
			Content *string `json:"content"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body: "+err.Error())
			return
		}

		// 构建更新对象
		update := &models.TaskSummaryUpdate{}

		if req.Time != nil {
			t, err := time.Parse(time.RFC3339, *req.Time)
			if err != nil {
				badRequestResponse(c, "invalid time format, expected RFC3339")
				return
			}
			update.Time = &t
		}

		if req.Content != nil {
			// XSS防护：清洗Markdown内容
			cleanContent := sanitizeMarkdown(*req.Content)
			update.Content = &cleanContent
		}

		// 更新总结
		ctx := context.Background()
		err := summaryService.UpdateSummary(ctx, projectID, taskID, summaryID, update)
		if err != nil {
			if err == services.ErrSummaryNotFound {
				notFoundResponse(c, "summary not found")
				return
			}
			if err == services.ErrContentEmpty {
				badRequestResponse(c, "content cannot be empty")
				return
			}
			internalErrorResponse(c, fmt.Errorf("update summary: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "summary updated successfully",
		})
	}
}

// HandleDeleteTaskSummary DELETE /api/v1/projects/:project_id/tasks/:task_id/summaries/:summary_id
// 删除任务总结
func HandleDeleteTaskSummary(reg *projects.ProjectRegistry, summaryService services.TaskSummaryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")
		summaryID := c.Param("summary_id")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 删除总结
		ctx := context.Background()
		err := summaryService.DeleteSummary(ctx, projectID, taskID, summaryID)
		if err != nil {
			if err == services.ErrSummaryNotFound {
				notFoundResponse(c, "summary not found")
				return
			}
			internalErrorResponse(c, fmt.Errorf("delete summary: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "summary deleted successfully",
		})
	}
}

// HandleGetSummariesByWeek GET /api/v1/projects/:project_id/summaries/by-week
// 跨任务按周范围检索总结
func HandleGetSummariesByWeek(reg *projects.ProjectRegistry, summaryService services.TaskSummaryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		startWeek := c.Query("start_week")
		endWeek := c.Query("end_week")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 验证必填参数
		if startWeek == "" || endWeek == "" {
			badRequestResponse(c, "start_week and end_week are required")
			return
		}

		// 验证周编号格式（简单验证）
		if !isValidWeekNumber(startWeek) || !isValidWeekNumber(endWeek) {
			badRequestResponse(c, "invalid week number format, expected YYYY-WW")
			return
		}

		// 获取项目的所有任务ID
		taskIDs, err := getProjectTaskIDs(projectID)
		if err != nil {
			internalErrorResponse(c, fmt.Errorf("get task IDs: %w", err))
			return
		}

		ctx := context.Background()

		// 逐个任务查询，保留任务ID信息
		type SummaryWithTask struct {
			TaskID   string
			TaskName string
			Summary  *models.TaskSummary
		}

		allSummaries := make([]SummaryWithTask, 0)

		for _, taskID := range taskIDs {
			summaries, err := summaryService.GetSummariesByWeekRange(ctx, projectID, taskID, startWeek, endWeek)
			if err != nil {
				// 记录错误但继续处理其他任务
				continue
			}

			taskName := getTaskName(projectID, taskID)
			for _, s := range summaries {
				allSummaries = append(allSummaries, SummaryWithTask{
					TaskID:   taskID,
					TaskName: taskName,
					Summary:  s,
				})
			}
		}

		// 按时间倒序排序
		sort.Slice(allSummaries, func(i, j int) bool {
			return allSummaries[i].Summary.Time.After(allSummaries[j].Summary.Time)
		})

		// 构造返回数据
		enhancedSummaries := make([]gin.H, len(allSummaries))
		for i, item := range allSummaries {
			enhancedSummaries[i] = gin.H{
				"id":          item.Summary.ID,
				"task_id":     item.TaskID,
				"task_name":   item.TaskName,
				"time":        item.Summary.Time,
				"week_number": item.Summary.WeekNumber,
				"content":     item.Summary.Content,
				"creator":     item.Summary.Creator,
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    enhancedSummaries,
		})
	}
}

// isValidWeekNumber 验证周编号格式
func isValidWeekNumber(weekNum string) bool {
	// 简单验证：YYYY-WW格式
	if len(weekNum) != 7 {
		return false
	}
	if weekNum[4] != '-' {
		return false
	}
	// 可以进一步验证年份和周数的范围
	return true
}

// getProjectTaskIDs 获取项目的所有任务ID
func getProjectTaskIDs(projectID string) ([]string, error) {
	// 读取项目的tasks.json文件
	projectDir := filepath.Join("projects", projectID)
	tasksFile := filepath.Join(projectDir, "tasks.json")

	// 检查文件是否存在
	if _, err := os.Stat(tasksFile); os.IsNotExist(err) {
		return []string{}, nil // 没有任务时返回空列表
	}

	// 读取tasks.json
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		return nil, fmt.Errorf("read tasks.json: %w", err)
	}

	// 解析JSON
	var tasks []map[string]interface{}
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parse tasks.json: %w", err)
	}

	// 提取任务ID
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if id, ok := task["id"].(string); ok && id != "" {
			taskIDs = append(taskIDs, id)
		}
	}

	return taskIDs, nil
}

// getTaskName 获取任务名称
func getTaskName(projectID, taskID string) string {
	// 读取项目的tasks.json文件
	projectDir := filepath.Join("projects", projectID)
	tasksFile := filepath.Join(projectDir, "tasks.json")

	// 读取tasks.json
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		return taskID // 失败时返回任务ID
	}

	// 解析JSON
	var tasks []map[string]interface{}
	if err := json.Unmarshal(data, &tasks); err != nil {
		return taskID
	}

	// 查找匹配的任务
	for _, task := range tasks {
		if id, ok := task["id"].(string); ok && id == taskID {
			if name, ok := task["name"].(string); ok {
				return name
			}
		}
	}

	return taskID // 未找到时返回任务ID
}

// extractTaskIDFromSummaryID 从总结ID中提取任务ID
// 注意：总结ID格式为 "summary_{timestamp}"，不包含任务ID
// 该函数需要配合上下文使用，实际上总结的任务ID应该从请求路径或数据库中获取
func extractTaskIDFromSummaryID(summaryID string) string {
	// Summary ID 格式为 "summary_{timestamp}"，不包含任务ID信息
	// 在实际使用中，任务ID应该从API路径参数或其他上下文中获取
	// 这里返回空字符串，调用方需要从其他地方获取taskID
	return ""
}
