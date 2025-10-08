package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/services"
)

// HandleUpdateProjectMetadata PATCH /api/v1/projects/:project_id/metadata
// 更新项目元数据
func HandleUpdateProjectMetadata(reg *projects.ProjectRegistry, overviewService services.ProjectOverviewService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")

		// 验证项目存在
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 解析请求体
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body: "+err.Error())
			return
		}

		// 更新元数据
		ctx := context.Background()
		if err := overviewService.UpdateProjectMetadata(ctx, projectID, req); err != nil {
			internalErrorResponse(c, fmt.Errorf("update metadata: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "metadata updated successfully",
		})
	}
}
