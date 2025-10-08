package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/services"
)

// HandleGetTaskStatistics GET /api/v1/projects/:id/tasks/statistics
// 获取项目的任务状态统计
func HandleGetTaskStatistics(reg *projects.ProjectRegistry, statsService services.StatisticsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		ctx := context.Background()
		distribution, err := statsService.GetTaskStatusDistribution(ctx, projectID)
		if err != nil {
			internalErrorResponse(c, fmt.Errorf("get task statistics: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    distribution,
		})
	}
}

// HandleGetProjectOverview GET /api/v1/projects/:project_id/overview
// 获取项目基本信息（不包含统计指标，统计指标请使用 /tasks/statistics API）
func HandleGetProjectOverview(reg *projects.ProjectRegistry, statsService services.StatisticsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")

		// 获取项目信息
		project := reg.Get(projectID)
		if project == nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 组装响应 - 仅返回基本信息
		response := gin.H{
			"basic_info": gin.H{
				"id":           project.ID,
				"name":         project.Name,
				"product_line": project.ProductLine,
				"created_at":   project.CreatedAt,
				"updated_at":   project.UpdatedAt,
			},
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    response,
		})
	}
}
