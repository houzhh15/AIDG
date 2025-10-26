package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/resource"
)

// ResourceHandler 资源处理器
type ResourceHandler struct {
	manager *resource.ResourceManager
}

// NewResourceHandler 创建资源处理器
// 参数:
//   - manager: ResourceManager 实例
//
// 返回:
//   - *ResourceHandler: 资源处理器实例
func NewResourceHandler(manager *resource.ResourceManager) *ResourceHandler {
	return &ResourceHandler{manager: manager}
}

// GetUserResources 获取用户资源列表
// GET /api/v1/users/:username/resources
// Query参数:
//   - project_id: 项目ID (可选,用于过滤)
//
// 功能:
//   - 验证 JWT token (通过中间件)
//   - 从 URL 提取 username
//   - 从查询参数提取 project_id
//   - 调用 manager.GetUserResources 获取资源列表
//   - 返回 JSON 响应
func (h *ResourceHandler) GetUserResources(c *gin.Context) {
	username := c.Param("username")

	// 获取查询参数
	projectID := c.Query("project_id")

	resources, err := h.manager.GetUserResources(username, projectID)
	if err != nil {
		log.Printf("[ERROR] GetUserResources: failed to get resources for user %s: %v", username, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch resources",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resources,
	})
}

// AddCustomResource 添加自定义资源
// POST /api/v1/users/:username/resources
// 请求体:
//   - name: 资源名称 (必填)
//   - description: 资源描述 (可选)
//   - content: 资源内容 (必填)
//   - mimeType: MIME 类型 (可选,默认 text/plain)
//   - visibility: 可见性 (可选,默认 private)
//   - projectID: 项目ID (可选)
//   - taskID: 任务ID (可选)
//
// 功能:
//   - 验证 JWT token
//   - 解析请求体
//   - 生成 resourceID 和 URI
//   - 调用 manager.AddResource 添加资源
//   - 返回 resourceID 和 createdAt
func (h *ResourceHandler) AddCustomResource(c *gin.Context) {
	username := c.Param("username")

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Content     string `json:"content" binding:"required"`
		MimeType    string `json:"mimeType"`
		Visibility  string `json:"visibility"`
		ProjectID   string `json:"projectID"`
		TaskID      string `json:"taskID"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// 设置默认值
	if req.MimeType == "" {
		req.MimeType = "text/plain"
	}
	if req.Visibility == "" {
		req.Visibility = "private"
	}

	// 生成 resource_id
	resourceID := fmt.Sprintf("res_%d", time.Now().Unix())
	uri := fmt.Sprintf("aidg://user/%s/custom/%s", username, resourceID)

	res := &resource.Resource{
		ResourceID:  resourceID,
		ProjectID:   req.ProjectID,
		TaskID:      req.TaskID,
		URI:         uri,
		Name:        req.Name,
		Description: req.Description,
		MimeType:    req.MimeType,
		Visibility:  req.Visibility,
		AutoAdded:   false,
		Content:     req.Content,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.manager.AddResource(username, res); err != nil {
		log.Printf("[ERROR] AddCustomResource: failed to add resource for user %s: %v", username, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to add resource",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"resource_id": resourceID,
			"uri":         uri,
			"created_at":  res.CreatedAt,
		},
	})
}

// DeleteResource 删除资源
// DELETE /api/v1/users/:username/resources/:resource_id
// 功能:
//   - 验证 JWT token
//   - 从 URL 提取 username 和 resource_id
//   - 校验所有权 (通过中间件或逻辑)
//   - 调用 manager.DeleteResource 删除资源
//   - 返回成功消息
func (h *ResourceHandler) DeleteResource(c *gin.Context) {
	username := c.Param("username")
	resourceID := c.Param("resource_id")

	// TODO: 校验所有权 (检查当前用户是否有权限删除该资源)

	if err := h.manager.DeleteResource(username, resourceID); err != nil {
		log.Printf("[ERROR] DeleteResource: failed to delete resource %s for user %s: %v", resourceID, username, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Resource deleted",
	})
}

// UpdateResource 更新资源
// PUT /api/v1/users/:username/resources/:resource_id
// 请求体:
//   - name: 资源名称 (可选)
//   - description: 资源描述 (可选)
//   - content: 资源内容 (可选)
//   - visibility: 可见性 (可选)
//   - projectID: 项目ID (可选)
//   - taskID: 任务ID (可选)
//
// 功能:
//   - 验证 JWT token
//   - 从 URL 提取 username 和 resource_id
//   - 解析请求体
//   - 调用 manager.UpdateResource 更新资源
//   - 返回成功消息
func (h *ResourceHandler) UpdateResource(c *gin.Context) {
	username := c.Param("username")
	resourceID := c.Param("resource_id")

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Content     string `json:"content"`
		Visibility  string `json:"visibility"`
		ProjectID   string `json:"projectID"`
		TaskID      string `json:"taskID"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// 构建更新对象
	updates := &resource.Resource{
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Visibility:  req.Visibility,
		ProjectID:   req.ProjectID,
		TaskID:      req.TaskID,
	}

	if err := h.manager.UpdateResource(username, resourceID, updates); err != nil {
		log.Printf("[ERROR] UpdateResource: failed to update resource %s for user %s: %v", resourceID, username, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Resource updated",
	})
}

// GetResourceByID 获取单个资源详情
// GET /api/v1/users/:username/resources/:resource_id
// 功能:
//   - 验证 JWT token
//   - 从 URL 提取 username 和 resource_id
//   - 调用 manager.GetUserResources 并过滤匹配的资源
//   - 返回资源详情
func (h *ResourceHandler) GetResourceByID(c *gin.Context) {
	username := c.Param("username")
	resourceID := c.Param("resource_id")

	resources, err := h.manager.GetUserResources(username, "")
	if err != nil {
		log.Printf("[ERROR] GetResourceByID: failed to get resources for user %s: %v", username, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch resource",
		})
		return
	}

	// 查找匹配的资源
	for _, res := range resources {
		if res.ResourceID == resourceID {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    res,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"error":   "Resource not found",
	})
}
