package documents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// getFileConverterURL 获取文件转换服务URL，支持环境变量配置
func getFileConverterURL() string {
	if url := os.Getenv("FILE_CONVERTER_URL"); url != "" {
		return url
	}
	return "http://localhost:5002"
}

// Handler 文档API处理器
type Handler struct {
	mu          sync.RWMutex                    // 保护以下三个map的并发访问
	baseDir     string                          // 项目的根目录
	managers    map[string]*DocumentTreeManager // projectID -> manager
	relEngines  map[string]*RelationshipEngine  // projectID -> engine
	refManagers map[string]*ReferenceManager    // projectID -> manager
}

// NewHandler 创建API处理器
func NewHandler(baseDir string) *Handler {
	if baseDir == "" {
		baseDir = "./projects"
	}
	return &Handler{
		baseDir:     baseDir,
		managers:    make(map[string]*DocumentTreeManager),
		relEngines:  make(map[string]*RelationshipEngine),
		refManagers: make(map[string]*ReferenceManager),
	}
}

// getOrCreateManager 获取或创建项目的文档管理器
func (h *Handler) getOrCreateManager(projectID string) (*DocumentTreeManager, error) {
	// 先用读锁检查是否已存在
	h.mu.RLock()
	if manager, exists := h.managers[projectID]; exists {
		h.mu.RUnlock()
		return manager, nil
	}
	h.mu.RUnlock()

	// 使用写锁创建新管理器
	h.mu.Lock()
	defer h.mu.Unlock()

	// Double-check: 可能在获取写锁期间其他goroutine已经创建
	if manager, exists := h.managers[projectID]; exists {
		return manager, nil
	}

	// 创建新的管理器
	projectDir := filepath.Join(h.baseDir, projectID, "documents")
	manager := NewDocumentTreeManager(projectDir)

	if err := manager.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize document manager: %w", err)
	}

	h.managers[projectID] = manager

	// 创建关系引擎和引用管理器
	h.relEngines[projectID] = NewRelationshipEngine(manager.index)
	h.refManagers[projectID] = NewReferenceManager(manager.index)

	return manager, nil
}

// 支持的文件扩展名
var supportedExtensions = map[string]bool{
	"pdf": true, "ppt": true, "pptx": true,
	"doc": true, "docx": true,
	"xls": true, "xlsx": true,
	"svg": true,
}

// 最大文件大小：20MB
const maxFileSize = 20 * 1024 * 1024

// ImportFile 文件导入处理器 POST /api/v1/projects/:id/documents/import
func (h *Handler) ImportFile(c *gin.Context) {
	// 解析 multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "文件上传失败: " + err.Error(),
			"code":  "UPLOAD_ERROR",
		})
		return
	}
	defer file.Close()

	filename := header.Filename
	fileSize := header.Size

	// 校验文件大小
	if fileSize > maxFileSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("文件大小超过限制（最大 %dMB）", maxFileSize/(1024*1024)),
			"code":  "FILE_TOO_LARGE",
		})
		return
	}

	// 获取文件扩展名
	ext := ""
	if idx := strings.LastIndex(filename, "."); idx != -1 {
		ext = strings.ToLower(filename[idx+1:])
	}

	// 校验文件格式
	if !supportedExtensions[ext] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "不支持的文件格式。支持: pdf, ppt, pptx, doc, docx, xls, xlsx, svg",
			"code":  "UNSUPPORTED_FORMAT",
		})
		return
	}

	// SVG 文件直接读取内容返回
	if ext == "svg" {
		content, err := readFileContent(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "读取 SVG 文件失败: " + err.Error(),
				"code":  "READ_ERROR",
			})
			return
		}

		c.JSON(http.StatusOK, ImportFileResponse{
			Success:          true,
			Content:          content,
			OriginalFilename: filename,
			FileSize:         fileSize,
			ContentType:      "svg",
			Warnings:         []string{},
		})
		return
	}

	// 其他格式调用 Python 转换服务
	result, err := callConversionService(file, filename, ext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "文件转换失败: " + err.Error(),
			"code":  "CONVERSION_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, ImportFileResponse{
		Success:          true,
		Content:          result.Content,
		OriginalFilename: filename,
		FileSize:         fileSize,
		ContentType:      "markdown",
		Warnings:         result.Warnings,
	})
}

// CreateNode 创建文档节点 POST /api/v1/projects/:id/documents/nodes
func (h *Handler) CreateNode(c *gin.Context) {
	projectID := c.Param("id")

	var req CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("[DEBUG] CreateNode bind error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	fmt.Printf("[DEBUG] CreateNode request: projectID=%s, title=%s, type=%s, parentID=%v\n",
		projectID, req.Title, req.Type, req.ParentID)

	// 如果parentID是"virtual_root"，将其设置为nil（创建根文档）
	if req.ParentID != nil && *req.ParentID == "virtual_root" {
		req.ParentID = nil
		fmt.Printf("[DEBUG] Converted virtual_root to nil for root document creation\n")
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Manager initialization failed: " + err.Error()})
		return
	}

	node, err := manager.CreateNode(req)
	if err != nil {
		switch err {
		case ErrHierarchyOverflow:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "HIERARCHY_OVERFLOW"})
		case ErrChildrenLimitReached:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "CHILDREN_LIMIT_REACHED"})
		case ErrNodeNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "PARENT_NODE_NOT_FOUND"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create node: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"node": node})
}

// GetTree 获取文档树 GET /api/v1/projects/:id/documents/tree
func (h *Handler) GetTree(c *gin.Context) {
	projectID := c.Param("id")

	// 解析查询参数
	nodeIDParam := c.Query("node_id")
	depthParam := c.DefaultQuery("depth", "5")

	depth, err := strconv.Atoi(depthParam)
	if err != nil || depth < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid depth parameter"})
		return
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Manager initialization failed: " + err.Error()})
		return
	}

	var nodeID *string
	if nodeIDParam != "" {
		nodeID = &nodeIDParam
	}

	tree, err := manager.GetTree(nodeID, depth)
	if err != nil {
		if err == ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NODE_NOT_FOUND"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get tree: " + err.Error()})
		}
		return
	}

	// 如果是虚拟根节点且没有子节点，返回空数组
	if tree.Node.ID == "virtual_root" {
		if len(tree.Children) == 0 {
			fmt.Printf("[DEBUG] GetTree returning empty array for virtual root with no children\n")
			c.JSON(http.StatusOK, gin.H{"tree": []interface{}{}})
		} else {
			fmt.Printf("[DEBUG] GetTree returning %d children for virtual root\n", len(tree.Children))
			c.JSON(http.StatusOK, gin.H{"tree": tree.Children})
		}
	} else {
		fmt.Printf("[DEBUG] GetTree returning single node: %s\n", tree.Node.ID)
		c.JSON(http.StatusOK, gin.H{"tree": tree})
	}
}

// MoveNode 移动节点 PUT /api/v1/projects/:id/documents/nodes/:node_id/move
func (h *Handler) MoveNode(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("node_id")

	var req MoveNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Manager initialization failed: " + err.Error()})
		return
	}

	err = manager.MoveNode(nodeID, req)
	if err != nil {
		switch err {
		case ErrNodeNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NODE_NOT_FOUND"})
		case ErrHierarchyOverflow:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "HIERARCHY_OVERFLOW"})
		case ErrCircularDependency:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "CIRCULAR_DEPENDENCY"})
		case ErrChildrenLimitReached:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "CHILDREN_LIMIT_REACHED"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to move node: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Node moved successfully"})
}

// UpdateNode 更新文档节点 PATCH /api/v1/projects/:id/documents/nodes/:node_id
func (h *Handler) UpdateNode(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("node_id")

	var req UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if req.Title == nil && req.Type == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Manager initialization failed: " + err.Error()})
		return
	}

	node, err := manager.UpdateNode(nodeID, req)
	if err != nil {
		switch err {
		case ErrNodeNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NODE_NOT_FOUND"})
		default:
			if strings.Contains(err.Error(), "failed to flush index") {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"node": node})
}

// DeleteNode 删除节点 DELETE /api/v1/projects/:id/documents/nodes/:node_id
func (h *Handler) DeleteNode(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("node_id")

	cascadeParam := c.DefaultQuery("cascade", "false")
	cascade := cascadeParam == "true"

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Manager initialization failed: " + err.Error()})
		return
	}

	// 在删除节点之前，先清理相关的关系和引用
	if err := h.cleanupNodeRelationsAndReferences(projectID, nodeID, cascade); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup node relations: " + err.Error()})
		return
	}

	err = manager.DeleteNode(nodeID, cascade)
	if err != nil {
		if err == ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NODE_NOT_FOUND"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete node: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Node deleted successfully"})
}

// CreateRelationship 创建文档关系 POST /api/v1/projects/:id/documents/relationships
// 根据设计文档要求，只允许用户手动创建 reference 类型的关系
// parent_child 和 sibling 关系由系统自动维护
func (h *Handler) CreateRelationship(c *gin.Context) {
	projectID := c.Param("id")

	var req CreateRelationshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// 验证关系类型：只允许创建 reference 类型的关系
	if req.Type != RelationReference {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Only reference relationships can be created manually. Parent-child and sibling relationships are automatically maintained by the system.",
			"code":  "INVALID_RELATION_TYPE",
		})
		return
	}

	// reference 类型的关系必须指定依赖类型
	if req.DependencyType == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Reference relationships must specify a dependency type (data, interface, or config)",
			"code":  "MISSING_DEPENDENCY_TYPE",
		})
		return
	}

	h.mu.RLock()
	relEngine := h.relEngines[projectID]
	h.mu.RUnlock()

	if relEngine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Relationship engine not initialized"})
		return
	}

	// 创建引用关系（隐式依赖关系）
	rel, err := relEngine.AddImplicitRelation(req.FromID, req.ToID, *req.DependencyType)

	if err != nil {
		switch err {
		case ErrNodeNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NODE_NOT_FOUND"})
		case ErrCircularDependency:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "CIRCULAR_DEPENDENCY"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create relationship: " + err.Error()})
		}
		return
	}

	// 更新描述
	if req.Description != "" {
		relEngine.UpdateRelationDescription(rel.ID, req.Description)
	}

	c.JSON(http.StatusCreated, gin.H{"relationship": rel})
}

// GetRelationships 获取所有关系 GET /api/v1/projects/:id/documents/relationships
func (h *Handler) GetRelationships(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Query("node_id") // 从查询参数获取（可选）

	if _, err := h.getOrCreateManager(projectID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Manager initialization failed: " + err.Error()})
		return
	}

	h.mu.RLock()
	relEngine := h.relEngines[projectID]
	h.mu.RUnlock()

	if relEngine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Relationship engine not initialized"})
		return
	}

	if nodeID != "" {
		// 获取特定节点的关系
		relations, err := relEngine.GetRelated(nodeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get relationships: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"relationships": relations})
	} else {
		// 获取所有关系
		allRelations := relEngine.GetAllRelations()
		c.JSON(http.StatusOK, gin.H{"relationships": allRelations})
	}
}

// RemoveRelationship 删除关系 DELETE /api/v1/projects/:id/documents/relationships/:from_id/:to_id
func (h *Handler) RemoveRelationship(c *gin.Context) {
	projectID := c.Param("id")
	fromID := c.Param("from_id")
	toID := c.Param("to_id")

	h.mu.RLock()
	relEngine := h.relEngines[projectID]
	h.mu.RUnlock()

	if relEngine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Relationship engine not initialized"})
		return
	}

	err := relEngine.RemoveRelation(fromID, toID)
	if err != nil {
		if err == ErrRelationNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "RELATION_NOT_FOUND"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove relationship: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteRelationship 删除关系 DELETE /api/v1/projects/:id/documents/relationships/:rel_id
func (h *Handler) DeleteRelationship(c *gin.Context) {
	projectID := c.Param("id")
	relID := c.Param("rel_id")

	h.mu.RLock()
	relEngine := h.relEngines[projectID]
	h.mu.RUnlock()

	if relEngine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Relationship engine not initialized"})
		return
	}

	err := relEngine.DeleteRelation(relID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete relationship: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Relationship deleted successfully"})
}

// CreateReference 创建任务引用 POST /api/v1/projects/:id/documents/references
func (h *Handler) CreateReference(c *gin.Context) {
	projectID := c.Param("id")

	var req CreateReferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	h.mu.RLock()
	refManager := h.refManagers[projectID]
	h.mu.RUnlock()

	if refManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reference manager not initialized"})
		return
	}

	ref, err := refManager.CreateReference(req)
	if err != nil {
		if err == ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "DOCUMENT_NOT_FOUND"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create reference: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"reference": ref})
}

// GetDocumentReferences 获取文档引用 GET /api/v1/projects/:id/documents/:doc_id/references
func (h *Handler) GetDocumentReferences(c *gin.Context) {
	projectID := c.Param("id")
	docID := c.Param("doc_id")

	h.mu.RLock()
	refManager := h.refManagers[projectID]
	h.mu.RUnlock()

	if refManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reference manager not initialized"})
		return
	}

	refs, err := refManager.GetReferencesByDoc(docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get references: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"references": refs})
}

// GetTaskReferences 获取任务引用 GET /api/v1/tasks/:task_id/references
func (h *Handler) GetTaskReferences(c *gin.Context) {
	// 从任务ID推断项目ID（简化处理，实际应该从任务管理系统获取）
	taskID := c.Param("task_id")

	// 暂时遍历所有项目查找任务引用
	var allRefs []*Reference

	h.mu.RLock()
	refManagersCopy := make([]*ReferenceManager, 0, len(h.refManagers))
	for _, refManager := range h.refManagers {
		refManagersCopy = append(refManagersCopy, refManager)
	}
	projectCount := len(h.refManagers)
	h.mu.RUnlock()

	for _, refManager := range refManagersCopy {
		refs, err := refManager.GetReferencesByTask(taskID)
		if err != nil {
			continue
		}

		// 添加项目信息到引用中
		// 可以扩展Reference结构体包含project_id
		allRefs = append(allRefs, refs...)
	}

	c.JSON(http.StatusOK, gin.H{"references": allRefs, "project_count": projectCount})
}

// GetReferenceStats 获取引用统计 GET /api/v1/projects/:id/documents/references/stats
func (h *Handler) GetReferenceStats(c *gin.Context) {
	projectID := c.Param("id")

	h.mu.RLock()
	refManager := h.refManagers[projectID]
	h.mu.RUnlock()

	if refManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reference manager not initialized"})
		return
	}

	stats := refManager.GetReferenceStats()
	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// UpdateReferenceStatus 更新引用状态 PUT /api/v1/projects/:id/references/:id/status
func (h *Handler) UpdateReferenceStatus(c *gin.Context) {
	projectID := c.Param("id")
	refID := c.Param("id") // 引用ID参数

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	h.mu.RLock()
	refManager := h.refManagers[projectID]
	h.mu.RUnlock()

	if refManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reference manager not initialized"})
		return
	}

	err := refManager.UpdateReferenceStatus(refID, ReferenceStatus(req.Status))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reference status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteReferenceHandler 删除引用 DELETE /api/v1/projects/:id/documents/references/:ref_id
func (h *Handler) DeleteReferenceHandler(c *gin.Context) {
	projectID := c.Param("id")
	refID := c.Param("ref_id")

	h.mu.RLock()
	refManager := h.refManagers[projectID]
	h.mu.RUnlock()

	if refManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reference manager not initialized"})
		return
	}

	err := refManager.DeleteReference(refID)
	if err != nil {
		if err.Error() == fmt.Sprintf("reference not found: %s", refID) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Reference not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete reference: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UpdateDocumentContent 更新文档内容 PUT /api/v1/projects/:id/documents/:doc_id/content
func (h *Handler) UpdateDocumentContent(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("doc_id")

	var req struct {
		Content string `json:"content" binding:"required"`
		Version int    `json:"version" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document manager: " + err.Error()})
		return
	}

	newVersion, err := manager.UpdateContent(nodeID, req.Content, req.Version)
	if err != nil {
		switch err {
		case ErrNodeNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NODE_NOT_FOUND"})
		case ErrVersionMismatch:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "VERSION_MISMATCH"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update document: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"version": newVersion, "success": true})
}

// GetDocumentContent 获取文档内容 GET /api/v1/projects/:id/documents/:doc_id/content
func (h *Handler) GetDocumentContent(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("doc_id")

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document manager: " + err.Error()})
		return
	}

	content, meta, err := manager.GetContent(nodeID)
	if err != nil {
		if err == ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NODE_NOT_FOUND"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"meta": meta, "content": content})
}

// GetDocumentVersions 获取文档版本历史 GET /api/v1/projects/:id/documents/:doc_id/versions
func (h *Handler) GetDocumentVersions(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("doc_id")

	// 获取查询参数
	limitStr := c.DefaultQuery("limit", "10")
	limit := 10
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document manager: " + err.Error()})
		return
	}

	versions, err := manager.GetVersionHistory(nodeID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get version history: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"versions": versions, "total": len(versions)})
}

// GetDocumentVersion 获取特定版本内容 GET /api/v1/projects/:id/documents/:doc_id/versions/:version
func (h *Handler) GetDocumentVersion(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("doc_id")
	versionStr := c.Param("version")

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid version number"})
		return
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document manager: " + err.Error()})
		return
	}

	content, err := manager.GetVersionContent(nodeID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Version not found: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"version": version, "content": content})
}

// CompareDocumentVersions 比较文档版本差异 GET /api/v1/projects/:id/documents/:doc_id/diff
func (h *Handler) CompareDocumentVersions(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("doc_id")

	// 获取查询参数
	fromStr := c.DefaultQuery("from", "0")
	toStr := c.DefaultQuery("to", "0")

	fromVersion, err := strconv.Atoi(fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid from version"})
		return
	}

	toVersion, err := strconv.Atoi(toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid to version"})
		return
	}

	if fromVersion == toVersion {
		c.JSON(http.StatusBadRequest, gin.H{"error": "From and to versions cannot be the same"})
		return
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document manager: " + err.Error()})
		return
	}

	diff, err := manager.CompareVersions(nodeID, fromVersion, toVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compare versions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"diff": diff})
}

// AnalyzeDocumentImpact 分析文档影响范围 GET /api/v1/projects/:id/documents/:doc_id/impact
func (h *Handler) AnalyzeDocumentImpact(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("doc_id")

	// 获取查询参数
	modesParam := c.DefaultQuery("modes", "all")
	var modes []AnalysisMode

	if modesParam == "all" {
		modes = []AnalysisMode{ModeAll}
	} else {
		// 解析逗号分隔的模式
		modeStrings := strings.Split(modesParam, ",")
		for _, modeStr := range modeStrings {
			modes = append(modes, AnalysisMode(strings.TrimSpace(modeStr)))
		}
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document manager: " + err.Error()})
		return
	}

	impact, err := manager.AnalyzeImpact(nodeID, modes)
	if err != nil {
		if err == ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NODE_NOT_FOUND"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze impact: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"impact": impact})
}

// SearchDocuments 搜索文档
func (h *Handler) SearchDocuments(c *gin.Context) {
	projectID := c.Param("id")

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document manager: " + err.Error()})
		return
	}

	// 创建搜索管理器
	searchMgr := NewSearchManager(manager.index, manager.projectDir)

	// 解析搜索选项
	var options SearchOptions
	if err := c.ShouldBindJSON(&options); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid search options: " + err.Error()})
		return
	}

	// 验证搜索查询
	if strings.TrimSpace(options.Query) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query cannot be empty", "code": "EMPTY_QUERY"})
		return
	}

	// 执行搜索
	results, err := searchMgr.SearchDocuments(options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
		"query":   options.Query,
	})
}

// GetSearchSuggestions 获取搜索建议
func (h *Handler) GetSearchSuggestions(c *gin.Context) {
	projectID := c.Param("id")
	query := c.Query("q")

	if len(query) < 2 {
		c.JSON(http.StatusOK, gin.H{"suggestions": []string{}})
		return
	}

	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document manager: " + err.Error()})
		return
	}

	// 创建搜索管理器
	searchMgr := NewSearchManager(manager.index, manager.projectDir)

	// 获取建议数量限制
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	// 获取搜索建议
	suggestions, err := searchMgr.GetSearchSuggestions(query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get suggestions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

// cleanupNodeRelationsAndReferences 清理节点相关的关系和引用
// 在删除节点之前调用，确保数据一致性
func (h *Handler) cleanupNodeRelationsAndReferences(projectID, nodeID string, cascade bool) error {
	// 获取需要清理的节点列表
	nodesToClean := []string{nodeID}

	if cascade {
		// 如果是级联删除，需要获取所有子节点
		manager, err := h.getOrCreateManager(projectID)
		if err != nil {
			return fmt.Errorf("failed to get manager: %w", err)
		}

		children, err := h.getAllChildNodes(manager, nodeID)
		if err != nil {
			return fmt.Errorf("failed to get child nodes: %w", err)
		}
		nodesToClean = append(nodesToClean, children...)
	}

	// 清理关系
	if err := h.cleanupRelationsForNodes(projectID, nodesToClean); err != nil {
		return fmt.Errorf("failed to cleanup relations: %w", err)
	}

	// 清理引用
	if err := h.cleanupReferencesForNodes(projectID, nodesToClean); err != nil {
		return fmt.Errorf("failed to cleanup references: %w", err)
	}

	return nil
}

// getAllChildNodes 递归获取所有子节点ID
func (h *Handler) getAllChildNodes(manager *DocumentTreeManager, nodeID string) ([]string, error) {
	var allChildren []string

	children := manager.index.GetChildren(nodeID)
	for _, childID := range children {
		allChildren = append(allChildren, childID)

		// 递归获取子节点的子节点
		grandChildren, err := h.getAllChildNodes(manager, childID)
		if err != nil {
			return nil, err
		}
		allChildren = append(allChildren, grandChildren...)
	}

	return allChildren, nil
}

// cleanupRelationsForNodes 清理节点列表的所有关系
func (h *Handler) cleanupRelationsForNodes(projectID string, nodeIDs []string) error {
	h.mu.RLock()
	relEngine := h.relEngines[projectID]
	h.mu.RUnlock()
	if relEngine == nil {
		// 如果关系引擎不存在，说明没有关系需要清理
		return nil
	}

	// 获取所有关系
	allRelations := relEngine.GetAllRelations()

	// 找到涉及这些节点的关系并删除
	for _, relation := range allRelations {
		shouldDelete := false

		// 检查关系的两端是否包含要删除的节点
		for _, nodeID := range nodeIDs {
			if relation.FromID == nodeID || relation.ToID == nodeID {
				shouldDelete = true
				break
			}
		}

		if shouldDelete {
			if err := relEngine.RemoveRelation(relation.FromID, relation.ToID); err != nil {
				// 记录错误但继续清理其他关系
				fmt.Printf("Warning: failed to remove relation %s->%s: %v\n",
					relation.FromID, relation.ToID, err)
			}
		}
	}

	return nil
}

// cleanupReferencesForNodes 清理节点列表的所有引用
func (h *Handler) cleanupReferencesForNodes(projectID string, nodeIDs []string) error {
	h.mu.RLock()
	refManager := h.refManagers[projectID]
	h.mu.RUnlock()

	if refManager == nil {
		// 如果引用管理器不存在，说明没有引用需要清理
		return nil
	}

	// 为每个节点清理引用
	for _, nodeID := range nodeIDs {
		// 清理以该节点为文档的引用
		if err := h.cleanupDocumentReferences(refManager, nodeID); err != nil {
			fmt.Printf("Warning: failed to cleanup references for document %s: %v\n", nodeID, err)
		}

		// 清理以该节点为任务的引用（如果该节点是任务类型）
		if err := h.cleanupTaskReferences(refManager, nodeID); err != nil {
			fmt.Printf("Warning: failed to cleanup task references for %s: %v\n", nodeID, err)
		}
	}

	return nil
}

// cleanupDocumentReferences 清理指定文档的所有引用
func (h *Handler) cleanupDocumentReferences(refManager *ReferenceManager, documentID string) error {
	// 获取该文档的所有引用
	refs, err := refManager.GetReferencesByDoc(documentID)
	if err != nil {
		return fmt.Errorf("failed to get references for document %s: %w", documentID, err)
	}

	// 删除每个引用
	for _, ref := range refs {
		if err := refManager.DeleteReference(ref.ID); err != nil {
			// 记录错误但继续清理其他引用
			fmt.Printf("Warning: failed to delete reference %s for document %s: %v\n",
				ref.ID, documentID, err)
		}
	}

	return nil
}

// cleanupTaskReferences 清理指定任务的所有引用
func (h *Handler) cleanupTaskReferences(refManager *ReferenceManager, taskID string) error {
	// 获取该任务的所有引用
	refs, err := refManager.GetReferencesByTask(taskID)
	if err != nil {
		return fmt.Errorf("failed to get references for task %s: %w", taskID, err)
	}

	// 删除每个引用
	for _, ref := range refs {
		if err := refManager.DeleteReference(ref.ID); err != nil {
			// 记录错误但继续清理其他引用
			fmt.Printf("Warning: failed to delete reference %s for task %s: %v\n",
				ref.ID, taskID, err)
		}
	}

	return nil
}

// GetReferencesByTaskInternal 内部方法:获取任务的引用列表(用于资源管理)
func (h *Handler) GetReferencesByTaskInternal(projectID, taskID string) ([]map[string]interface{}, error) {
	h.mu.RLock()
	refManager := h.refManagers[projectID]
	h.mu.RUnlock()

	if refManager == nil {
		return nil, fmt.Errorf("reference manager not initialized for project: %s", projectID)
	}

	refs, err := refManager.GetReferencesByTask(taskID)
	if err != nil {
		return nil, err
	}

	// 转换为 map[string]interface{} 格式
	result := make([]map[string]interface{}, 0, len(refs))
	for _, ref := range refs {
		result = append(result, map[string]interface{}{
			"id":          ref.ID,
			"task_id":     ref.TaskID,
			"document_id": ref.DocumentID,
			"anchor":      ref.Anchor,
			"context":     ref.Context,
			"status":      ref.Status,
		})
	}

	return result, nil
}

// GetDocumentContentInternal 内部方法:获取文档内容(用于资源管理)
func (h *Handler) GetDocumentContentInternal(projectID, docID string) (map[string]interface{}, error) {
	// 使用 getOrCreateManager 确保管理器已初始化
	manager, err := h.getOrCreateManager(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize document manager: %w", err)
	}

	// 从索引获取节点元数据
	meta, err := manager.index.GetNode(docID)
	if err != nil {
		return nil, err
	}

	// 读取文档内容
	contentFile := filepath.Join(manager.projectDir, fmt.Sprintf("%s.md", docID))
	contentBytes, err := os.ReadFile(contentFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read document content: %w", err)
	}

	return map[string]interface{}{
		"content": string(contentBytes),
		"meta": map[string]interface{}{
			"title":   meta.Title,
			"type":    meta.Type,
			"version": meta.Version,
		},
	}, nil
}

// ================== 文件导入辅助函数 ==================

// readFileContent 读取文件内容为字符串
func readFileContent(file interface{ Read([]byte) (int, error) }) (string, error) {
	content := make([]byte, 0, 1024*1024) // 预分配 1MB
	buf := make([]byte, 32*1024)          // 32KB 缓冲区

	for {
		n, err := file.Read(buf)
		if n > 0 {
			content = append(content, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return "", err
		}
	}

	return string(content), nil
}

// conversionResult Python 转换服务返回结果
type conversionResult struct {
	Content  string   `json:"content"`
	Warnings []string `json:"warnings"`
}

// callConversionService 调用 Python 文件转换服务
func callConversionService(file interface{ Read([]byte) (int, error) }, filename, ext string) (*conversionResult, error) {
	// 读取文件内容
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file.(io.Reader)); err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 创建 multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("创建 form 失败: %w", err)
	}

	if _, err := io.Copy(part, &buf); err != nil {
		return nil, fmt.Errorf("写入文件数据失败: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 writer 失败: %w", err)
	}

	// 调用转换服务（支持环境变量配置）
	baseURL := getFileConverterURL()
	conversionURL := fmt.Sprintf("%s/convert/%s", baseURL, ext)
	req, err := http.NewRequest("POST", conversionURL, &body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 使用带超时的 HTTP 客户端
	client := &http.Client{
		Timeout: 60 * time.Second, // 文件转换可能需要较长时间
	}
	resp, err := client.Do(req)
	if err != nil {
		// 提供更友好的错误信息
		if strings.Contains(err.Error(), "connection refused") {
			return nil, fmt.Errorf("文件转换服务未启动。请确保已运行 'make dev' 或手动启动转换服务 (cd file_converter && uvicorn main:app --port 5002)")
		}
		return nil, fmt.Errorf("调用转换服务失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("转换服务返回错误 (%d): %s", resp.StatusCode, string(respBody))
	}

	var result conversionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析转换结果失败: %w", err)
	}

	return &result, nil
}
