package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/aidg-lite/internal/domain/docslot"
)

// RegisterUnifiedDocRoutes 注册统一文档 API 路由（lite版本仅支持项目文档）
func RegisterUnifiedDocRoutes(router *gin.RouterGroup, svc docslot.UnifiedDocService) {
	projectDocs := router.Group("/projects/:id/docs/:slot")
	{
		projectDocs.GET("/export", handleExport(svc, docslot.ScopeProject))
		projectDocs.POST("/append", handleAppend(svc, docslot.ScopeProject))
		projectDocs.GET("/chunks", handleListChunks(svc, docslot.ScopeProject))
		projectDocs.POST("/squash", handleSquash(svc, docslot.ScopeProject))
		projectDocs.GET("/sections", handleGetSections(svc, docslot.ScopeProject))
		projectDocs.GET("/sections/:sid", handleGetSection(svc, docslot.ScopeProject))
		projectDocs.PUT("/sections/:sid", handleUpdateSection(svc, docslot.ScopeProject))
		projectDocs.POST("/sections", handleInsertSection(svc, docslot.ScopeProject))
		projectDocs.DELETE("/sections/:sid", handleDeleteSection(svc, docslot.ScopeProject))
	}
}

func handleExport(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")

		result, err := svc.Export(scope, scopeID, slotKey)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": result.Content, "version": result.Version,
			"etag": result.ETag, "updated_at": result.UpdatedAt, "exists": result.Exists,
		})
	}
}

func handleAppend(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")

		var req struct {
			Content         string `json:"content"`
			ExpectedVersion *int   `json:"expected_version"`
			Op              string `json:"op"`
			Source          string `json:"source"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}
		if req.Op == "" {
			req.Op = "add_full"
		}
		if req.Op != "add_full" && req.Op != "replace_full" {
			badRequestResponse(c, "invalid op, must be 'add_full' or 'replace_full'")
			return
		}
		if strings.TrimSpace(req.Content) == "" && req.Op != "replace_full" {
			badRequestResponse(c, "content is required for add_full operation")
			return
		}

		userVal, _ := c.Get("user")
		username, _ := userVal.(string)
		if username == "" {
			username = "anonymous"
		}

		result, err := svc.Append(scope, scopeID, slotKey, req.Content, username, req.ExpectedVersion, req.Op, req.Source)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"version": result.Version, "etag": result.ETag, "duplicate": result.Duplicate,
			"sequence": result.Sequence, "timestamp": result.Timestamp,
		})
	}
}

func handleListChunks(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")

		chunks, meta, err := svc.ListChunks(scope, scopeID, slotKey)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"chunks": chunks, "meta": meta})
	}
}

func handleSquash(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")

		userVal, _ := c.Get("user")
		username, _ := userVal.(string)
		if username == "" {
			username = "anonymous"
		}

		meta, err := svc.Squash(scope, scopeID, slotKey, username, "api")
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"meta": meta})
	}
}

func handleGetSections(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")

		meta, err := svc.GetSections(scope, scopeID, slotKey)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, meta)
	}
}

func handleGetSection(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")
		sectionID := c.Param("sid")
		includeChildren := c.Query("include_children") == "true"

		content, err := svc.GetSection(scope, scopeID, slotKey, sectionID, includeChildren)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, content)
	}
}

func handleUpdateSection(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")
		sectionID := c.Param("sid")

		var req struct {
			Content         string `json:"content"`
			ExpectedVersion int    `json:"expected_version"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		if err := svc.UpdateSection(scope, scopeID, slotKey, sectionID, req.Content, req.ExpectedVersion); err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleInsertSection(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")

		var req struct {
			Title           string  `json:"title"`
			Content         string  `json:"content"`
			AfterSectionID  *string `json:"after_section_id"`
			ExpectedVersion int     `json:"expected_version"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}
		if req.Title == "" {
			badRequestResponse(c, "title is required")
			return
		}

		section, err := svc.InsertSection(scope, scopeID, slotKey, req.Title, req.Content, req.AfterSectionID, req.ExpectedVersion)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusCreated, section)
	}
}

func handleDeleteSection(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")
		sectionID := c.Param("sid")
		cascade := c.Query("cascade") == "true"

		var expectedVersion int
		if v := c.Query("expected_version"); v != "" {
			expectedVersion, _ = parseIntStr(v)
		}

		if err := svc.DeleteSection(scope, scopeID, slotKey, sectionID, cascade, expectedVersion); err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleDocSlotError(c *gin.Context, err error) {
	if dse, ok := err.(*docslot.DocSlotError); ok {
		switch dse.Code {
		case docslot.ErrCodeInvalidScope, docslot.ErrCodeInvalidSlot:
			c.JSON(http.StatusBadRequest, gin.H{"error": dse.Code, "message": dse.Message})
		case docslot.ErrCodeDocNotFound, docslot.ErrCodeSectionNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": dse.Code, "message": dse.Message})
		case docslot.ErrCodeVersionMismatch:
			c.JSON(http.StatusConflict, gin.H{"error": dse.Code, "message": dse.Message})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": dse.Code, "message": dse.Message})
		}
		return
	}

	if docslot.IsVersionMismatch(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "version_mismatch", "message": err.Error()})
		return
	}
	if docslot.IsDocNotFound(err) || docslot.IsSectionNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": err.Error()})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
}

func parseIntStr(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}
