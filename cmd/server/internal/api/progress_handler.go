package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/services"
)

// HandleGetWeekProgress GET /api/v1/projects/:project_id/progress/week/:week_number
// 获取周进展（包含季度、月、周）
func HandleGetWeekProgress(reg *projects.ProjectRegistry, progressService services.ProgressService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		weekNumber := c.Param("week_number")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 获取周进展
		ctx := context.Background()
		progress, err := progressService.GetWeekProgress(ctx, projectID, weekNumber)
		if err != nil {
			if err == services.ErrProgressNotFound {
				notFoundResponse(c, "progress not found")
				return
			}
			if err == services.ErrInvalidWeekNumber {
				badRequestResponse(c, "invalid week number format")
				return
			}
			internalErrorResponse(c, fmt.Errorf("get week progress: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    progress,
		})
	}
}

// HandleUpdateWeekProgress PUT /api/v1/projects/:project_id/progress/week/:week_number
// 更新周进展
func HandleUpdateWeekProgress(reg *projects.ProjectRegistry, progressService services.ProgressService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		weekNumber := c.Param("week_number")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 解析请求体
		var req struct {
			QuarterSummary *string `json:"quarter_summary"`
			MonthSummary   *string `json:"month_summary"`
			WeekSummary    *string `json:"week_summary"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body: "+err.Error())
			return
		}

		// Markdown内容清洗
		if req.QuarterSummary != nil {
			cleaned := sanitizeMarkdown(*req.QuarterSummary)
			req.QuarterSummary = &cleaned
		}
		if req.MonthSummary != nil {
			cleaned := sanitizeMarkdown(*req.MonthSummary)
			req.MonthSummary = &cleaned
		}
		if req.WeekSummary != nil {
			cleaned := sanitizeMarkdown(*req.WeekSummary)
			req.WeekSummary = &cleaned
		}

		// 更新进展
		ctx := context.Background()
		err := progressService.UpdateWeekProgress(ctx, projectID, weekNumber, req.QuarterSummary, req.MonthSummary, req.WeekSummary)
		if err != nil {
			if err == services.ErrInvalidWeekNumber {
				badRequestResponse(c, "invalid week number format")
				return
			}
			internalErrorResponse(c, fmt.Errorf("update week progress: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "progress updated successfully",
		})
	}
}

// HandleGetYearProgress GET /api/v1/projects/:project_id/progress/year/:year
// 获取年度进展
func HandleGetYearProgress(reg *projects.ProjectRegistry, progressService services.ProgressService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		yearStr := c.Param("year")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 解析年份
		year, err := strconv.Atoi(yearStr)
		if err != nil || year < 2000 || year > 2100 {
			badRequestResponse(c, "invalid year format")
			return
		}

		// 获取年度进展
		ctx := context.Background()
		yearProgress, err := progressService.GetYearProgress(ctx, projectID, year)
		if err != nil {
			internalErrorResponse(c, fmt.Errorf("get year progress: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    yearProgress,
		})
	}
}
