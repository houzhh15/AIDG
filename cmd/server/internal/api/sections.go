package api

import (
	"net/http"
	"strings"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/taskdocs"

	"github.com/gin-gonic/gin"
)

// SectionHandlers 章节相关的 HTTP 处理器
type SectionHandlers struct {
	service taskdocs.SectionService
}

// NewSectionHandlers 创建处理器实例
func NewSectionHandlers(service taskdocs.SectionService) *SectionHandlers {
	return &SectionHandlers{service: service}
}

// HandleGetSections 获取章节列表
func (h *SectionHandlers) HandleGetSections(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	// 从路径中提取 docType (requirements/design/test)
	docType := extractDocTypeFromPath(c.Request.URL.Path)
	if docType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc type in path"})
		return
	}

	// 验证 docType
	if !isValidDocType(docType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc type"})
		return
	}

	// 获取章节
	meta, err := h.service.GetSections(projectID, taskID, docType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, meta)
}

// HandleGetSection 获取单个章节
func (h *SectionHandlers) HandleGetSection(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	// 从路径中提取 docType
	docType := extractDocTypeFromPath(c.Request.URL.Path)
	if docType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc type in path"})
		return
	}
	sectionID := c.Param("section_id")

	// 解析查询参数
	includeChildren := c.Query("include_children") == "true"

	// 获取章节内容
	section, err := h.service.GetSection(projectID, taskID, docType, sectionID, includeChildren)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, section)
}

// HandleUpdateSection 更新章节
func (h *SectionHandlers) HandleUpdateSection(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	// 从路径中提取 docType
	docType := extractDocTypeFromPath(c.Request.URL.Path)
	if docType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc type in path"})
		return
	}
	sectionID := c.Param("section_id")

	// 解析请求体
	var req struct {
		Content         string `json:"content" binding:"required"`
		ExpectedVersion int    `json:"expected_version"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新章节
	err := h.service.UpdateSection(projectID, taskID, docType, sectionID, req.Content, req.ExpectedVersion)
	if err != nil {
		if err.Error() == "version conflict" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleInsertSection 插入新章节
func (h *SectionHandlers) HandleInsertSection(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	// 从路径中提取 docType
	docType := extractDocTypeFromPath(c.Request.URL.Path)
	if docType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc type in path"})
		return
	}

	// 解析请求体
	var req struct {
		Title           string  `json:"title" binding:"required"`
		Content         string  `json:"content" binding:"required"`
		AfterSectionID  *string `json:"after_section_id"`
		ExpectedVersion int     `json:"expected_version"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 插入章节
	section, err := h.service.InsertSection(
		projectID, taskID, docType,
		req.Title, req.Content, req.AfterSectionID,
		req.ExpectedVersion,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, section)
}

// HandleDeleteSection 删除章节
func (h *SectionHandlers) HandleDeleteSection(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	// 从路径中提取 docType
	docType := extractDocTypeFromPath(c.Request.URL.Path)
	if docType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc type in path"})
		return
	}
	sectionID := c.Param("section_id")

	// 解析查询参数
	cascade := c.Query("cascade") == "true"

	// 解析请求体（用于传递 expected_version）
	var req struct {
		ExpectedVersion int `json:"expected_version"`
	}
	c.ShouldBindJSON(&req)

	// 删除章节
	err := h.service.DeleteSection(projectID, taskID, docType, sectionID, cascade, req.ExpectedVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleReorderSection 调整章节顺序
func (h *SectionHandlers) HandleReorderSection(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	// 从路径中提取 docType
	docType := extractDocTypeFromPath(c.Request.URL.Path)
	if docType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc type in path"})
		return
	}

	// 解析请求体
	var req struct {
		SectionID       string  `json:"section_id" binding:"required"`
		AfterSectionID  *string `json:"after_section_id"`
		ExpectedVersion int     `json:"expected_version"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 调整顺序
	err := h.service.ReorderSection(
		projectID, taskID, docType,
		req.SectionID, req.AfterSectionID,
		req.ExpectedVersion,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleUpdateSectionFull 更新父章节的全文内容
func (h *SectionHandlers) HandleUpdateSectionFull(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")
	sectionID := c.Param("section_id")
	docType := extractDocTypeFromPath(c.Request.URL.Path)

	if docType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc type in path"})
		return
	}

	var req struct {
		Content         string `json:"content" binding:"required"`
		ExpectedVersion int    `json:"expected_version"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.UpdateSectionFull(projectID, taskID, docType, sectionID, req.Content, req.ExpectedVersion)
	if err != nil {
		if strings.Contains(err.Error(), "version conflict") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleSyncSections 手动同步章节
func (h *SectionHandlers) HandleSyncSections(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	// 从路径中提取 docType
	docType := extractDocTypeFromPath(c.Request.URL.Path)
	if docType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc type in path"})
		return
	}

	// 解析请求体
	var req struct {
		Direction string `json:"direction" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证 direction
	if req.Direction != "from_compiled" && req.Direction != "to_compiled" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid direction"})
		return
	}

	// 执行同步
	err := h.service.SyncSections(projectID, taskID, docType, req.Direction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// isValidDocType 验证文档类型
func isValidDocType(docType string) bool {
	return docType == "requirements" || docType == "design" || docType == "test"
}

// extractDocTypeFromPath 从路径中提取文档类型
// 例如: /api/v1/projects/xxx/tasks/yyy/requirements/sections -> "requirements"
func extractDocTypeFromPath(path string) string {
	// 查找 docType (requirements/design/test)
	if strings.Contains(path, "/requirements/") {
		return "requirements"
	}
	if strings.Contains(path, "/design/") {
		return "design"
	}
	if strings.Contains(path, "/test/") {
		return "test"
	}
	return ""
}

// RegisterSectionRoutes 注册章节管理路由
// docType 由路由组决定 (requirements/design/test)
func RegisterSectionRoutes(router *gin.RouterGroup, service taskdocs.SectionService) {
	handlers := NewSectionHandlers(service)

	// 章节管理路由
	router.GET("/sections", handlers.HandleGetSections)
	router.GET("/sections/:section_id", handlers.HandleGetSection)
	router.PUT("/sections/:section_id", handlers.HandleUpdateSection)
	router.PUT("/sections/:section_id/full", handlers.HandleUpdateSectionFull) // 新增：全文编辑
	router.POST("/sections", handlers.HandleInsertSection)
	router.DELETE("/sections/:section_id", handlers.HandleDeleteSection)
	router.PATCH("/sections/reorder", handlers.HandleReorderSection)
	router.POST("/sections/sync", handlers.HandleSyncSections)
}
