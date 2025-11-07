package api

import (
	"github.com/houzhh15/AIDG/cmd/server/internal/models"
	"github.com/houzhh15/AIDG/cmd/server/internal/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RoadmapHandler Roadmap API处理器
type RoadmapHandler struct {
	service *services.RoadmapService
}

// NewRoadmapHandler 创建RoadmapHandler实例
func NewRoadmapHandler(service *services.RoadmapService) *RoadmapHandler {
	return &RoadmapHandler{
		service: service,
	}
}

// HandleGetRoadmap 获取项目Roadmap
// GET /api/v1/projects/:project_id/roadmap
func (h *RoadmapHandler) HandleGetRoadmap(c *gin.Context) {
	projectID := c.Param("id")

	roadmap, err := h.service.GetRoadmap(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "ROADMAP_READ_FAILED",
				"message": "读取Roadmap失败",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    roadmap,
	})
}

// HandleAddNode 添加Roadmap节点
// POST /api/v1/projects/:project_id/roadmap/nodes
func (h *RoadmapHandler) HandleAddNode(c *gin.Context) {
	projectID := c.Param("id")

	var req models.RoadmapNodeCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_INPUT",
				"message": "请求参数错误",
				"details": err.Error(),
			},
		})
		return
	}

	node, err := h.service.AddNode(projectID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "ADD_NODE_FAILED",
				"message": "添加节点失败",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    node,
	})
}

// HandleUpdateNode 更新Roadmap节点
// PUT /api/v1/projects/:project_id/roadmap/nodes/:node_id
func (h *RoadmapHandler) HandleUpdateNode(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("node_id")

	// 获取期望版本号（乐观锁）
	expectedVersionStr := c.Query("expected_version")
	if expectedVersionStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_VERSION",
				"message": "缺少expected_version参数",
			},
		})
		return
	}

	expectedVersion, err := strconv.Atoi(expectedVersionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_VERSION",
				"message": "版本号格式错误",
			},
		})
		return
	}

	var req models.RoadmapNodeUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_INPUT",
				"message": "请求参数错误",
				"details": err.Error(),
			},
		})
		return
	}

	err = h.service.UpdateNode(projectID, nodeID, &req, expectedVersion)
	if err != nil {
		// 检查是否是版本冲突
		if err.Error() == "版本冲突" {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VERSION_CONFLICT",
					"message": "版本冲突，请刷新后重试",
					"details": err.Error(),
				},
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UPDATE_NODE_FAILED",
				"message": "更新节点失败",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
	})
}

// HandleDeleteNode 删除Roadmap节点
// DELETE /api/v1/projects/:project_id/roadmap/nodes/:node_id
func (h *RoadmapHandler) HandleDeleteNode(c *gin.Context) {
	projectID := c.Param("id")
	nodeID := c.Param("node_id")

	err := h.service.DeleteNode(projectID, nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "DELETE_NODE_FAILED",
				"message": "删除节点失败",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}
