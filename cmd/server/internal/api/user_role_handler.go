package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/constants"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/services"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/users"

	"github.com/gin-gonic/gin"
)

// HandleAssignRoles POST /api/v1/users/roles
// 为用户分配项目角色
func HandleAssignRoles(userRoleService services.UserRoleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Username  string   `json:"username"`
			ProjectID string   `json:"project_id"`
			RoleIDs   []string `json:"role_ids"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			badRequestResponse(c, "invalid request")
			return
		}

		// 参数校验
		if body.Username == "" || body.ProjectID == "" || len(body.RoleIDs) == 0 {
			badRequestResponse(c, "username, project_id and role_ids are required")
			return
		}

		// 获取当前用户 (操作者)
		operator, exists := c.Get("user")
		if !exists {
			unauthorizedResponse(c, "authentication required")
			return
		}

		// 分配角色
		err := userRoleService.AssignRoles(body.Username, body.ProjectID, body.RoleIDs)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		// 记录日志
		log.Printf("%s Assigned roles %v to user %s in project %s by %s",
			constants.LogPrefixUserRole, body.RoleIDs, body.Username, body.ProjectID, operator)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "角色已分配",
		})
	}
}

// HandleRevokeRoles DELETE /api/v1/users/roles
// 撤销用户在项目中的所有角色
func HandleRevokeRoles(userRoleService services.UserRoleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Username  string `json:"username"`
			ProjectID string `json:"project_id"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			badRequestResponse(c, "invalid request")
			return
		}

		// 参数校验
		if body.Username == "" || body.ProjectID == "" {
			badRequestResponse(c, "username and project_id are required")
			return
		}

		// 获取当前用户 (操作者)
		operator, exists := c.Get("user")
		if !exists {
			unauthorizedResponse(c, "authentication required")
			return
		}

		// 撤销角色
		err := userRoleService.RevokeRoles(body.Username, body.ProjectID)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		// 记录日志
		log.Printf("%s Revoked roles from user %s in project %s by %s",
			constants.LogPrefixUserRole, body.Username, body.ProjectID, operator)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "角色已撤销",
		})
	}
}

// HandleGetUserPermissions GET /api/v1/users/:username/permissions?project_id=xxx
// 获取用户在项目中的有效权限
func HandleGetUserPermissions(userRoleService services.UserRoleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")
		projectID := c.Query("project_id")

		if projectID == "" {
			badRequestResponse(c, "project_id is required")
			return
		}

		// 计算有效权限
		scopes, err := userRoleService.ComputeEffectiveScopes(username, projectID)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"username":   username,
				"project_id": projectID,
				"scopes":     scopes,
			},
		})
	}
}

// HandleGetCurrentUserProfile GET /api/v1/user/profile
// 获取当前登录用户的所有项目角色信息和基础权限
func HandleGetCurrentUserProfile(userRoleService services.UserRoleService, userManager *users.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "用户未登录",
				},
			})
			return
		}

		// 1. 获取项目角色
		profile, err := userRoleService.GetUserProfile(username.(string))
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

		// 2. 获取用户基础权限（来自 users.json）
		user, exists := userManager.GetUser(username.(string))
		if !exists || user == nil {
			// 如果用户不存在，返回空权限
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"username":            profile.Username,
					"roles":               profile.ProjectRoles,
					"default_permissions": []gin.H{},
				},
			})
			return
		}

		// 3. 将用户scopes转换为default_permissions格式
		var defaultPermissions []gin.H
		if len(user.Scopes) > 0 {
			defaultPermissions = append(defaultPermissions, gin.H{
				"source": "user_scopes",
				"scopes": user.Scopes,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"username":            profile.Username,
				"roles":               profile.ProjectRoles,
				"default_permissions": defaultPermissions,
			},
		})
	}
}

// HandleGetUserProfile GET /api/v1/users/:username/profile
// 获取用户的所有项目角色信息
func HandleGetUserProfile(userRoleService services.UserRoleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")

		profile, err := userRoleService.GetUserProfile(username)
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
			"data":    profile,
		})
	}
}

// ========== RESTful 风格路由适配器 ==========

// HandleGetProjectUserRoles GET /api/v1/projects/:project_id/users/:username/roles
// 获取用户在项目中的角色列表 (RESTful 风格)
func HandleGetProjectUserRoles(userRoleService services.UserRoleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id") // 修复: 路由使用:id而不是:project_id
		username := c.Param("username")

		roles, err := userRoleService.GetUserRoles(username, projectID)
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

// HandleAssignProjectUserRole POST /api/v1/projects/:project_id/users/:username/roles
// 为用户分配项目角色 (RESTful 风格)
func HandleAssignProjectUserRole(userRoleService services.UserRoleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id") // 修复: 路由使用:id而不是:project_id
		username := c.Param("username")

		var body struct {
			RoleIDs []string `json:"role_ids"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			log.Printf("[DEBUG] Failed to bind JSON: %v", err)
			badRequestResponse(c, "invalid request")
			return
		}

		log.Printf("[DEBUG] Received request - projectID: %s, username: %s, roleIDs: %v", projectID, username, body.RoleIDs)

		// 参数校验
		if len(body.RoleIDs) == 0 {
			log.Printf("[DEBUG] role_ids is empty")
			badRequestResponse(c, "role_ids are required")
			return
		}

		// 获取当前用户 (操作者)
		operator, exists := c.Get("user")
		if !exists {
			log.Printf("[DEBUG] operator not found in context")
			unauthorizedResponse(c, "authentication required")
			return
		}

		log.Printf("[DEBUG] Calling AssignRoles service - username: %s, projectID: %s, roleIDs: %v", username, projectID, body.RoleIDs)
		// 分配角色
		err := userRoleService.AssignRoles(username, projectID, body.RoleIDs)
		if err != nil {
			log.Printf("[DEBUG] AssignRoles failed: %v", err)
			internalErrorResponse(c, err)
			return
		} // 记录日志
		log.Printf("%s Assigned roles %v to user %s in project %s by %s",
			constants.LogPrefixUserRole, body.RoleIDs, username, projectID, operator)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "角色已分配",
		})
	}
}

// HandleRemoveProjectUserRole DELETE /api/v1/projects/:id/users/:username/roles/:role_id
// 移除用户的项目角色 (RESTful 风格)
func HandleRemoveProjectUserRole(userRoleService services.UserRoleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id") // 修复: 路由使用:id而不是:project_id
		username := c.Param("username")
		roleID := c.Param("role_id")

		// 获取当前用户 (操作者)
		operator, exists := c.Get("user")
		if !exists {
			unauthorizedResponse(c, "authentication required")
			return
		}

		// 移除角色
		err := userRoleService.RemoveRole(username, projectID, roleID)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		// 记录日志
		log.Printf("%s Removed role %s from user %s in project %s by %s",
			constants.LogPrefixUserRole, roleID, username, projectID, operator)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "角色已移除",
		})
	}
}

// HandleGetProjectUserRolesList GET /api/v1/projects/:project_id/user-roles
// 获取项目的所有用户角色映射 (RESTful 风格)
func HandleGetProjectUserRolesList(userRoleService services.UserRoleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("project_id")

		mappings, err := userRoleService.GetProjectUserRoles(projectID)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    mappings,
		})
	}
}
