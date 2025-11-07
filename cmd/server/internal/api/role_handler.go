package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/houzhh15/AIDG/cmd/server/internal/constants"
	"github.com/houzhh15/AIDG/cmd/server/internal/services"

	"github.com/gin-gonic/gin"
)

// HandleCreateRole POST /api/v1/roles
// 创建项目角色
func HandleCreateRole(roleManager services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			ProjectID   string   `json:"project_id"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Scopes      []string `json:"scopes"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			badRequestResponse(c, "invalid request")
			return
		}

		// 参数校验
		if strings.TrimSpace(body.ProjectID) == "" || strings.TrimSpace(body.Name) == "" {
			badRequestResponse(c, "project_id and name are required")
			return
		}

		if len(body.Scopes) == 0 {
			badRequestResponse(c, "at least one scope is required")
			return
		}

		// 获取当前用户
		username, exists := c.Get("user")
		if !exists {
			unauthorizedResponse(c, "authentication required")
			return
		}

		// 创建角色
		role, err := roleManager.CreateRole(
			body.ProjectID,
			body.Name,
			body.Scopes,
		)

		if err != nil {
			if errors.Is(err, services.ErrDuplicateRoleName) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeDuplicateRoleName,
						"message": err.Error(),
					},
				})
				return
			}
			if errors.Is(err, services.ErrInvalidScope) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeInvalidScope,
						"message": err.Error(),
					},
				})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		// 记录日志
		log.Printf("%s Created role %s in project %s by %s",
			constants.LogPrefixRole, role.RoleID, role.ProjectID, username)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    role,
		})
	}
}

// HandleListRoles GET /api/v1/roles?project_id=xxx
// 列出项目角色
func HandleListRoles(roleManager services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Query("project_id")
		if projectID == "" {
			badRequestResponse(c, "project_id is required")
			return
		}

		roles, err := roleManager.ListRoles(projectID)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    roles,
		})
	}
}

// HandleGetRole GET /api/v1/roles/:role_id?project_id=xxx
// 获取角色详情
func HandleGetRole(roleManager services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleID := c.Param("role_id")
		projectID := c.Query("project_id")

		if projectID == "" {
			badRequestResponse(c, "project_id is required")
			return
		}

		role, err := roleManager.GetRole(projectID, roleID)
		if err != nil {
			if errors.Is(err, services.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeRoleNotFound,
						"message": err.Error(),
					},
				})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    role,
		})
	}
}

// HandleUpdateRole PUT /api/v1/roles/:role_id
// 更新角色权限配置
func HandleUpdateRole(roleManager services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleID := c.Param("role_id")

		var body struct {
			ProjectID string   `json:"project_id"`
			Scopes    []string `json:"scopes"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			badRequestResponse(c, "invalid request")
			return
		}

		if body.ProjectID == "" {
			badRequestResponse(c, "project_id is required")
			return
		}

		if len(body.Scopes) == 0 {
			badRequestResponse(c, "at least one scope is required")
			return
		}

		// 获取当前用户
		username, exists := c.Get("user")
		if !exists {
			unauthorizedResponse(c, "authentication required")
			return
		}

		// 更新角色 (这里需要获取原角色名称或者从请求中获取)
		// 注意: 新接口需要 name 参数,这里暂时使用空字符串,建议在请求体中添加 name 字段
		role, err := roleManager.GetRole(body.ProjectID, roleID)
		if err != nil {
			if errors.Is(err, services.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeRoleNotFound,
						"message": err.Error(),
					},
				})
				return
			}
			internalErrorResponse(c, err)
			return
		}
		err = roleManager.UpdateRole(body.ProjectID, roleID, role.Name, body.Scopes)
		if err != nil {
			if errors.Is(err, services.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeRoleNotFound,
						"message": err.Error(),
					},
				})
				return
			}
			if errors.Is(err, services.ErrInvalidScope) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeInvalidScope,
						"message": err.Error(),
					},
				})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		// 记录日志
		log.Printf("%s Updated role %s in project %s by %s",
			constants.LogPrefixRole, roleID, body.ProjectID, username)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "角色已更新",
		})
	}
}

// HandleDeleteRole DELETE /api/v1/roles/:role_id?project_id=xxx
// 删除角色
func HandleDeleteRole(roleManager services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleID := c.Param("role_id")
		projectID := c.Query("project_id")

		if projectID == "" {
			badRequestResponse(c, "project_id is required")
			return
		}

		// 获取当前用户
		username, exists := c.Get("user")
		if !exists {
			unauthorizedResponse(c, "authentication required")
			return
		}

		// 删除角色
		err := roleManager.DeleteRole(projectID, roleID)
		if err != nil {
			if errors.Is(err, services.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeRoleNotFound,
						"message": err.Error(),
					},
				})
				return
			}
			if errors.Is(err, services.ErrRoleInUse) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeRoleInUse,
						"message": err.Error(),
					},
				})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		// 记录日志
		log.Printf("%s Deleted role %s from project %s by %s",
			constants.LogPrefixRole, roleID, projectID, username)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "角色已删除",
		})
	}
}

// ========== RESTful 风格路由适配器 ==========

// HandleListProjectRoles GET /api/v1/projects/:id/roles
// 列出项目角色 (RESTful 风格)
func HandleListProjectRoles(svc services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
			return
		}

		roles, err := svc.ListRoles(projectID)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    roles,
		})
	}
}

// HandleGetProjectRole GET /api/v1/projects/:id/roles/:role_id
// 获取项目角色详情 (RESTful 风格)
func HandleGetProjectRole(svc services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		roleID := c.Param("role_id")

		if projectID == "" || roleID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project_id and role_id are required"})
			return
		}

		role, err := svc.GetRole(projectID, roleID)
		if err != nil {
			if errors.Is(err, services.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})
			} else {
				internalErrorResponse(c, err)
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    role,
		})
	}
}

// HandleCreateProjectRole POST /api/v1/projects/:id/roles
// 创建项目角色 (RESTful 风格)
func HandleCreateProjectRole(svc services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
			return
		}

		var body struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Scopes      []string `json:"scopes"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			badRequestResponse(c, "invalid request")
			return
		}

		// 参数校验
		if strings.TrimSpace(body.Name) == "" {
			badRequestResponse(c, "name is required")
			return
		}

		if len(body.Scopes) == 0 {
			badRequestResponse(c, "at least one scope is required")
			return
		}

		// 获取当前用户
		username, exists := c.Get("user")
		if !exists {
			unauthorizedResponse(c, "authentication required")
			return
		}

		// 创建角色
		role, err := svc.CreateRole(
			projectID,
			body.Name,
			body.Scopes,
		)

		if err != nil {
			if errors.Is(err, services.ErrDuplicateRoleName) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeDuplicateRoleName,
						"message": err.Error(),
					},
				})
				return
			}
			if errors.Is(err, services.ErrInvalidScope) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeInvalidScope,
						"message": err.Error(),
					},
				})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		// 记录日志
		log.Printf("%s Created role %s in project %s by %s",
			constants.LogPrefixRole, role.RoleID, role.ProjectID, username)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    role,
		})
	}
}

// HandleUpdateProjectRole PUT /api/v1/projects/:id/roles/:role_id
// 更新项目角色 (RESTful 风格)
func HandleUpdateProjectRole(svc services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		roleID := c.Param("role_id")

		var body struct {
			Name   *string  `json:"name"`
			Scopes []string `json:"scopes"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			badRequestResponse(c, "invalid request")
			return
		}

		// 获取当前用户
		username, exists := c.Get("user")
		if !exists {
			unauthorizedResponse(c, "authentication required")
			return
		}

		// 获取原角色信息
		role, err := svc.GetRole(projectID, roleID)
		if err != nil {
			if errors.Is(err, services.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeRoleNotFound,
						"message": err.Error(),
					},
				})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		// 使用新值或保留原值
		name := role.Name
		if body.Name != nil && *body.Name != "" {
			name = *body.Name
		}

		scopes := role.Scopes
		if len(body.Scopes) > 0 {
			scopes = body.Scopes
		}

		// 验证至少有一个权限
		if len(scopes) == 0 {
			badRequestResponse(c, "at least one scope is required")
			return
		}

		// 更新角色
		err = svc.UpdateRole(projectID, roleID, name, scopes)
		if err != nil {
			if errors.Is(err, services.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeRoleNotFound,
						"message": err.Error(),
					},
				})
				return
			}
			if errors.Is(err, services.ErrInvalidScope) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeInvalidScope,
						"message": err.Error(),
					},
				})
				return
			}
			if errors.Is(err, services.ErrDuplicateRoleName) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeDuplicateRoleName,
						"message": err.Error(),
					},
				})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		// 记录日志
		log.Printf("%s Updated role %s in project %s by %s",
			constants.LogPrefixRole, roleID, projectID, username)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "角色已更新",
		})
	}
}

// HandleDeleteProjectRole DELETE /api/v1/projects/:id/roles/:role_id
// 删除项目角色 (RESTful 风格)
func HandleDeleteProjectRole(svc services.RoleManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		roleID := c.Param("role_id")

		// 获取当前用户
		username, exists := c.Get("user")
		if !exists {
			unauthorizedResponse(c, "authentication required")
			return
		}

		// 删除角色
		err := svc.DeleteRole(projectID, roleID)
		if err != nil {
			if errors.Is(err, services.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeRoleNotFound,
						"message": err.Error(),
					},
				})
				return
			}
			if errors.Is(err, services.ErrRoleInUse) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    constants.ErrCodeRoleInUse,
						"message": err.Error(),
					},
				})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		// 记录日志
		log.Printf("%s Deleted role %s from project %s by %s",
			constants.LogPrefixRole, roleID, projectID, username)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "角色已删除",
		})
	}
}
