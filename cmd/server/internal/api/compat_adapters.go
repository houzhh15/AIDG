// Package api provides HTTP handlers for the AIDG server.
// This file contains backward compatibility adapters for legacy document APIs.
// These adapters internally use the unified docslot service while preserving
// the original API contracts.
//
// DEPRECATION NOTICE:
// The following legacy endpoints are deprecated and will be removed in a future version:
//   - PUT /api/v1/projects/:id/feature-list → use POST /api/v1/projects/:id/docs/feature_list/append
//   - PUT /api/v1/projects/:id/architecture-design → use POST /api/v1/projects/:id/docs/architecture_design/append
//   - PUT /api/v1/tasks/:id/meeting-summary → use POST /api/v1/meetings/:id/docs/summary/append
//   - PUT /api/v1/tasks/:id/topic → use POST /api/v1/meetings/:id/docs/topic/append
//   - PUT /api/v1/tasks/:id/polish → use POST /api/v1/meetings/:id/docs/polish/append
//
// New clients should use the unified API at:
//   - /api/v1/projects/:id/docs/:slot/...
//   - /api/v1/meetings/:id/docs/:slot/...
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/docslot"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
)

// DeprecationMiddleware 添加 Deprecation 头，提示客户端迁移到新 API
func DeprecationMiddleware(newEndpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Deprecation", "true")
		c.Header("Sunset", "2025-12-31")
		c.Header("Link", newEndpoint+"; rel=\"successor-version\"")
		c.Next()
	}
}

// ========== Project Document Adapters ==========

// HandlePutProjectFeatureListCompat PUT /api/v1/projects/:id/feature-list
// @deprecated Use POST /api/v1/projects/:id/docs/feature_list/append instead
func HandlePutProjectFeatureListCompat(reg *projects.ProjectRegistry, svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		// 验证项目存在
		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		var req struct {
			Content string `json:"content"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// 使用统一服务追加内容（全文替换模式）
		user := c.GetString("username")
		if user == "" {
			user = "legacy_api"
		}

		_, err := svc.Append(docslot.ScopeProject, id, "feature_list", req.Content, user, nil, "replace", "legacy_api")
		if err != nil {
			handleCompatError(c, err)
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "feature list updated",
		})
	}
}

// HandlePutProjectArchitectureDesignCompat PUT /api/v1/projects/:id/architecture-design
// @deprecated Use POST /api/v1/projects/:id/docs/architecture_design/append instead
func HandlePutProjectArchitectureDesignCompat(reg *projects.ProjectRegistry, svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		var req struct {
			Content string `json:"content"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		user := c.GetString("username")
		if user == "" {
			user = "legacy_api"
		}

		_, err := svc.Append(docslot.ScopeProject, id, "architecture_design", req.Content, user, nil, "replace", "legacy_api")
		if err != nil {
			handleCompatError(c, err)
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "architecture design updated",
		})
	}
}

// HandleGetProjectFeatureListCompat GET /api/v1/projects/:id/feature-list
// @deprecated Use GET /api/v1/projects/:id/docs/feature_list/export instead
func HandleGetProjectFeatureListCompat(reg *projects.ProjectRegistry, svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		result, err := svc.Export(docslot.ScopeProject, id, "feature_list")
		if err != nil {
			if docslot.IsDocNotFound(err) {
				// 返回空内容而非错误，保持向后兼容
				c.JSON(http.StatusOK, gin.H{
					"content": "",
					"exists":  false,
				})
				return
			}
			handleCompatError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": result.Content,
			"exists":  true,
		})
	}
}

// HandleGetProjectArchitectureDesignCompat GET /api/v1/projects/:id/architecture-design
// @deprecated Use GET /api/v1/projects/:id/docs/architecture_design/export instead
func HandleGetProjectArchitectureDesignCompat(reg *projects.ProjectRegistry, svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		result, err := svc.Export(docslot.ScopeProject, id, "architecture_design")
		if err != nil {
			if docslot.IsDocNotFound(err) {
				c.JSON(http.StatusOK, gin.H{
					"content": "",
					"exists":  false,
				})
				return
			}
			handleCompatError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": result.Content,
			"exists":  true,
		})
	}
}

// ========== Meeting Document Adapters ==========

// HandleGetMeetingSummaryCompat GET /api/v1/tasks/:id/meeting-summary
// @deprecated Use GET /api/v1/meetings/:id/docs/summary/export instead
func HandleGetMeetingSummaryCompat(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "task_id is required"})
			return
		}

		result, err := svc.Export(docslot.ScopeMeeting, taskID, "summary")
		if err != nil {
			if docslot.IsDocNotFound(err) {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "meeting-summary document not found",
				})
				return
			}
			handleCompatError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": result.Content,
			"exists":  true,
		})
	}
}

// HandleUpdateMeetingSummaryCompat PUT /api/v1/tasks/:id/meeting-summary
// @deprecated Use POST /api/v1/meetings/:id/docs/summary/append instead
func HandleUpdateMeetingSummaryCompat(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "task_id is required"})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content field is required"})
			return
		}

		user := c.GetString("username")
		if user == "" {
			user = "legacy_api"
		}

		_, err := svc.Append(docslot.ScopeMeeting, taskID, "summary", req.Content, user, nil, "replace", "legacy_api")
		if err != nil {
			handleCompatError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "meeting-summary document updated successfully",
		})
	}
}

// HandleGetTopicCompat GET /api/v1/tasks/:id/topic
// @deprecated Use GET /api/v1/meetings/:id/docs/topic/export instead
func HandleGetTopicCompat(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "task_id is required"})
			return
		}

		result, err := svc.Export(docslot.ScopeMeeting, taskID, "topic")
		if err != nil {
			if docslot.IsDocNotFound(err) {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "topic document not found",
				})
				return
			}
			handleCompatError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": result.Content,
			"exists":  true,
		})
	}
}

// HandleUpdateTopicCompat PUT /api/v1/tasks/:id/topic
// @deprecated Use POST /api/v1/meetings/:id/docs/topic/append instead
func HandleUpdateTopicCompat(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "task_id is required"})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content field is required"})
			return
		}

		user := c.GetString("username")
		if user == "" {
			user = "legacy_api"
		}

		_, err := svc.Append(docslot.ScopeMeeting, taskID, "topic", req.Content, user, nil, "replace", "legacy_api")
		if err != nil {
			handleCompatError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "topic document updated successfully",
		})
	}
}

// HandleGetPolishCompat GET /api/v1/tasks/:id/polish
// @deprecated Use GET /api/v1/meetings/:id/docs/polish/export instead
func HandleGetPolishCompat(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "task_id is required"})
			return
		}

		result, err := svc.Export(docslot.ScopeMeeting, taskID, "polish")
		if err != nil {
			if docslot.IsDocNotFound(err) {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "polish document not found",
				})
				return
			}
			handleCompatError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": result.Content,
		})
	}
}

// HandleUpdatePolishCompat PUT /api/v1/tasks/:id/polish
// @deprecated Use POST /api/v1/meetings/:id/docs/polish/append instead
func HandleUpdatePolishCompat(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "task_id is required"})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content field is required"})
			return
		}

		user := c.GetString("username")
		if user == "" {
			user = "legacy_api"
		}

		_, err := svc.Append(docslot.ScopeMeeting, taskID, "polish", req.Content, user, nil, "replace", "legacy_api")
		if err != nil {
			handleCompatError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	}
}

// ========== Helper Functions ==========

// handleCompatError 处理兼容层错误
func handleCompatError(c *gin.Context, err error) {
	// 检查是否为 docslot 服务错误
	if dse, ok := err.(*docslot.DocSlotError); ok {
		switch dse.Code {
		case docslot.ErrCodeInvalidScope, docslot.ErrCodeInvalidSlot:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": dse.Message,
			})
		case docslot.ErrCodeDocNotFound, docslot.ErrCodeSectionNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error": dse.Message,
			})
		case docslot.ErrCodeVersionMismatch:
			c.JSON(http.StatusConflict, gin.H{
				"error": dse.Message,
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": dse.Message,
			})
		}
		return
	}

	// 默认内部错误
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": err.Error(),
	})
}

// RegisterCompatRoutes 注册所有兼容性路由
// 这些路由使用 DeprecationMiddleware 添加废弃头
func RegisterCompatRoutes(
	router *gin.RouterGroup,
	projectsReg *projects.ProjectRegistry,
	svc docslot.UnifiedDocService,
) {
	// 项目文档兼容路由
	router.GET("/projects/:id/feature-list",
		DeprecationMiddleware("/api/v1/projects/:id/docs/feature_list/export"),
		HandleGetProjectFeatureListCompat(projectsReg, svc))
	router.PUT("/projects/:id/feature-list",
		DeprecationMiddleware("/api/v1/projects/:id/docs/feature_list/append"),
		HandlePutProjectFeatureListCompat(projectsReg, svc))
	router.GET("/projects/:id/architecture-design",
		DeprecationMiddleware("/api/v1/projects/:id/docs/architecture_design/export"),
		HandleGetProjectArchitectureDesignCompat(projectsReg, svc))
	router.PUT("/projects/:id/architecture-design",
		DeprecationMiddleware("/api/v1/projects/:id/docs/architecture_design/append"),
		HandlePutProjectArchitectureDesignCompat(projectsReg, svc))

	// 会议文档兼容路由
	router.GET("/tasks/:id/meeting-summary",
		DeprecationMiddleware("/api/v1/meetings/:id/docs/summary/export"),
		HandleGetMeetingSummaryCompat(svc))
	router.PUT("/tasks/:id/meeting-summary",
		DeprecationMiddleware("/api/v1/meetings/:id/docs/summary/append"),
		HandleUpdateMeetingSummaryCompat(svc))
	router.GET("/tasks/:id/topic",
		DeprecationMiddleware("/api/v1/meetings/:id/docs/topic/export"),
		HandleGetTopicCompat(svc))
	router.PUT("/tasks/:id/topic",
		DeprecationMiddleware("/api/v1/meetings/:id/docs/topic/append"),
		HandleUpdateTopicCompat(svc))
	router.GET("/tasks/:id/polish-all",
		DeprecationMiddleware("/api/v1/meetings/:id/docs/polish/export"),
		HandleGetPolishCompat(svc))
	router.PUT("/tasks/:id/polish-all",
		DeprecationMiddleware("/api/v1/meetings/:id/docs/polish/append"),
		HandleUpdatePolishCompat(svc))
}
