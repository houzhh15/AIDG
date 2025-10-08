package api

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/projects"
)

// HandleGetProjectFeatureListHistory GET /api/v1/projects/:id/feature-list/history
// 获取项目特性列表历史版本
// Required Scopes: users.ScopeFeatureRead
func HandleGetProjectFeatureListHistory(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		projDir := filepath.Join(projectsRoot(), p.ID)
		history, err := getContentHistory(projDir, "docs/feature_list.md")
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to get history")
			return
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// HandleDeleteProjectFeatureListHistory DELETE /api/v1/projects/:id/feature-list/history/:version
// 删除项目特性列表历史版本
// Required Scopes: users.ScopeFeatureWrite
func HandleDeleteProjectFeatureListHistory(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		versionStr := c.Param("version")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			badRequestResponse(c, "invalid version number")
			return
		}

		projDir := filepath.Join(projectsRoot(), p.ID)
		if err := deleteContentHistory(projDir, "docs/feature_list.md", version); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to delete history")
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "history version deleted",
		})
	}
}

// HandleGetProjectArchitectureHistory GET /api/v1/projects/:id/architecture-design/history
// 获取项目架构设计历史版本
// Required Scopes: users.ScopeArchRead
func HandleGetProjectArchitectureHistory(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		projDir := filepath.Join(projectsRoot(), p.ID)
		history, err := getContentHistory(projDir, "docs/architecture_design.md")
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to get history")
			return
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// HandleDeleteProjectArchitectureHistory DELETE /api/v1/projects/:id/architecture-design/history/:version
// 删除项目架构设计历史版本
// Required Scopes: users.ScopeArchWrite
func HandleDeleteProjectArchitectureHistory(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		versionStr := c.Param("version")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			badRequestResponse(c, "invalid version number")
			return
		}

		projDir := filepath.Join(projectsRoot(), p.ID)
		if err := deleteContentHistory(projDir, "docs/architecture_design.md", version); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to delete history")
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "history version deleted",
		})
	}
}

// HandleGetProjectTechDesignHistory GET /api/v1/projects/:id/tech-design/history
// 获取项目技术设计历史版本
// Required Scopes: users.ScopeTechRead
func HandleGetProjectTechDesignHistory(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		projDir := filepath.Join(projectsRoot(), p.ID)
		history, err := getContentHistory(projDir, "docs/tech_design.md")
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to get history")
			return
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// HandleDeleteProjectTechDesignHistory DELETE /api/v1/projects/:id/tech-design/history/:version
// 删除项目技术设计历史版本
// Required Scopes: users.ScopeTechWrite
func HandleDeleteProjectTechDesignHistory(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		versionStr := c.Param("version")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			badRequestResponse(c, "invalid version number")
			return
		}

		projDir := filepath.Join(projectsRoot(), p.ID)
		if err := deleteContentHistory(projDir, "docs/tech_design.md", version); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to delete history")
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "history version deleted",
		})
	}
}
