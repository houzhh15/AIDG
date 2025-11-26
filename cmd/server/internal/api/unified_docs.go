package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/docslot"
)

// RegisterUnifiedDocRoutes 注册统一文档 API 路由
func RegisterUnifiedDocRoutes(router *gin.RouterGroup, svc docslot.UnifiedDocService) {
	// 项目文档路由
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

	// 会议文档路由 - 使用 :meeting_id 以匹配现有路由命名
	meetingDocs := router.Group("/meetings/:meeting_id/docs/:slot")
	{
		meetingDocs.GET("/export", handleMeetingExport(svc))
		meetingDocs.POST("/append", handleMeetingAppend(svc))
		meetingDocs.GET("/chunks", handleMeetingListChunks(svc))
		meetingDocs.POST("/squash", handleMeetingSquash(svc))
		meetingDocs.GET("/sections", handleMeetingGetSections(svc))
		meetingDocs.GET("/sections/:sid", handleMeetingGetSection(svc))
		meetingDocs.PUT("/sections/:sid", handleMeetingUpdateSection(svc))
		meetingDocs.POST("/sections", handleMeetingInsertSection(svc))
		meetingDocs.DELETE("/sections/:sid", handleMeetingDeleteSection(svc))
	}
}

// handleExport GET /export - 导出文档内容
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
			"content":    result.Content,
			"version":    result.Version,
			"etag":       result.ETag,
			"updated_at": result.UpdatedAt,
			"exists":     result.Exists,
		})
	}
}

// handleAppend POST /append - 追加文档内容
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

		// 验证 op 参数
		if req.Op == "" {
			req.Op = "add_full"
		}
		if req.Op != "add_full" && req.Op != "replace_full" {
			badRequestResponse(c, "invalid op, must be 'add_full' or 'replace_full'")
			return
		}

		// 非 replace_full 模式下不允许空内容
		if strings.TrimSpace(req.Content) == "" && req.Op != "replace_full" {
			badRequestResponse(c, "content is required for add_full operation")
			return
		}

		// 获取用户信息
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
			"version":   result.Version,
			"etag":      result.ETag,
			"duplicate": result.Duplicate,
			"sequence":  result.Sequence,
			"timestamp": result.Timestamp,
		})
	}
}

// handleListChunks GET /chunks - 列出 chunk 历史
func handleListChunks(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")

		chunks, meta, err := svc.ListChunks(scope, scopeID, slotKey)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"chunks": chunks,
			"meta":   meta,
		})
	}
}

// handleSquash POST /squash - 压缩 chunks
func handleSquash(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")

		// 获取用户信息
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

		c.JSON(http.StatusOK, gin.H{
			"meta": meta,
		})
	}
}

// handleGetSections GET /sections - 获取章节列表
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

// handleGetSection GET /sections/:sid - 获取单个章节
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

// handleUpdateSection PUT /sections/:sid - 更新章节
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

// handleInsertSection POST /sections - 插入章节
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

// handleDeleteSection DELETE /sections/:sid - 删除章节
func handleDeleteSection(svc docslot.UnifiedDocService, scope docslot.DocumentScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("id")
		slotKey := c.Param("slot")
		sectionID := c.Param("sid")
		cascade := c.Query("cascade") == "true"

		var expectedVersion int
		if v := c.Query("expected_version"); v != "" {
			// 忽略解析错误，默认为 0
			expectedVersion, _ = parseInt(v)
		}

		if err := svc.DeleteSection(scope, scopeID, slotKey, sectionID, cascade, expectedVersion); err != nil {
			handleDocSlotError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// handleDocSlotError 处理 docslot 错误
func handleDocSlotError(c *gin.Context, err error) {
	if dse, ok := err.(*docslot.DocSlotError); ok {
		switch dse.Code {
		case docslot.ErrCodeInvalidScope, docslot.ErrCodeInvalidSlot:
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   dse.Code,
				"message": dse.Message,
			})
		case docslot.ErrCodeDocNotFound, docslot.ErrCodeSectionNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error":   dse.Code,
				"message": dse.Message,
			})
		case docslot.ErrCodeVersionMismatch:
			c.JSON(http.StatusConflict, gin.H{
				"error":   dse.Code,
				"message": dse.Message,
			})
		case docslot.ErrCodeMigrationRequired:
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   dse.Code,
				"message": dse.Message,
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   dse.Code,
				"message": dse.Message,
			})
		}
		return
	}

	// 检查原始错误类型
	if docslot.IsVersionMismatch(err) {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "version_mismatch",
			"message": err.Error(),
		})
		return
	}

	if docslot.IsDocNotFound(err) || docslot.IsSectionNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": err.Error(),
		})
		return
	}

	// 默认内部错误
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":   "internal_error",
		"message": err.Error(),
	})
}

// parseInt 解析整数，失败返回 0
func parseInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

// ========== 会议文档专用 handlers（使用 :meeting_id 参数） ==========

func handleMeetingExport(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("meeting_id")
		slotKey := c.Param("slot")
		result, err := svc.Export(docslot.ScopeMeeting, scopeID, slotKey)
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

func handleMeetingAppend(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("meeting_id")
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
			badRequestResponse(c, "invalid op")
			return
		}
		if strings.TrimSpace(req.Content) == "" && req.Op != "replace_full" {
			badRequestResponse(c, "content is required")
			return
		}
		userVal, _ := c.Get("user")
		username, _ := userVal.(string)
		if username == "" {
			username = "anonymous"
		}
		result, err := svc.Append(docslot.ScopeMeeting, scopeID, slotKey, req.Content, username, req.ExpectedVersion, req.Op, req.Source)
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

func handleMeetingListChunks(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("meeting_id")
		slotKey := c.Param("slot")
		chunks, meta, err := svc.ListChunks(docslot.ScopeMeeting, scopeID, slotKey)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"chunks": chunks, "meta": meta})
	}
}

func handleMeetingSquash(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("meeting_id")
		slotKey := c.Param("slot")
		userVal, _ := c.Get("user")
		username, _ := userVal.(string)
		if username == "" {
			username = "anonymous"
		}
		meta, err := svc.Squash(docslot.ScopeMeeting, scopeID, slotKey, username, "api")
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"version": meta.Version, "etag": meta.ETag})
	}
}

func handleMeetingGetSections(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("meeting_id")
		slotKey := c.Param("slot")
		meta, err := svc.GetSections(docslot.ScopeMeeting, scopeID, slotKey)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, meta)
	}
}

func handleMeetingGetSection(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("meeting_id")
		slotKey := c.Param("slot")
		sectionID := c.Param("sid")
		includeChildren := c.Query("include_children") == "true"
		content, err := svc.GetSection(docslot.ScopeMeeting, scopeID, slotKey, sectionID, includeChildren)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, content)
	}
}

func handleMeetingUpdateSection(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("meeting_id")
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
		if err := svc.UpdateSection(docslot.ScopeMeeting, scopeID, slotKey, sectionID, req.Content, req.ExpectedVersion); err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleMeetingInsertSection(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("meeting_id")
		slotKey := c.Param("slot")
		var req struct {
			Title          string  `json:"title"`
			Content        string  `json:"content"`
			AfterSectionID *string `json:"after_section_id"`
			ExpectedVer    int     `json:"expected_version"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}
		section, err := svc.InsertSection(docslot.ScopeMeeting, scopeID, slotKey, req.Title, req.Content, req.AfterSectionID, req.ExpectedVer)
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusCreated, section)
	}
}

func handleMeetingDeleteSection(svc docslot.UnifiedDocService) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeID := c.Param("meeting_id")
		slotKey := c.Param("slot")
		sectionID := c.Param("sid")
		cascade := c.Query("cascade") == "true"
		expectedVer, _ := parseInt(c.Query("expected_version"))
		if err := svc.DeleteSection(docslot.ScopeMeeting, scopeID, slotKey, sectionID, cascade, expectedVer); err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}
