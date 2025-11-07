package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

// HandleGetMyToken GET /api/v1/me/token
// 为当前已认证用户生成新的 JWT token
// Required Scopes: users.ScopeMeetingRead
func HandleGetMyToken(userManager *users.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user context"})
			return
		}
		usernameStr, ok := username.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
			return
		}

		// Generate a new token for the current user
		token, err := userManager.GenerateToken(usernameStr)
		if err != nil {
			log.Printf("[ERROR] HandleGetMyToken: failed to generate token for user %s: %v", usernameStr, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
			return
		}

		// Get user info to include scopes
		user, exists := userManager.GetUser(usernameStr)
		if !exists || user == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token":    token,
			"username": user.Username,
			"scopes":   user.Scopes,
			"message":  "新token已生成",
		})
	}
}

// HandleListUsers GET /api/v1/users
// 列出所有用户
// Required Scopes: users.ScopeUserManage
func HandleListUsers(userManager *users.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		userList := userManager.ListUsers()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    userList,
		})
	}
}

// HandleGetUser GET /api/v1/users/:username
// 获取单个用户详情（过滤敏感字段）
// Required Scopes: users.ScopeUserManage
func HandleGetUser(userManager *users.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")
		user, exists := userManager.GetUser(username)
		if !exists {
			notFoundResponse(c, "user")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    user,
		})
	}
}

// HandleCreateUser POST /api/v1/users
// 创建新用户
// Required Scopes: users.ScopeUserManage
func HandleCreateUser(userManager *users.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string   `json:"username"`
			Password string   `json:"password"`
			Scopes   []string `json:"scopes"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// 验证用户名
		if req.Username == "" {
			badRequestResponse(c, "username is required")
			return
		}

		// 验证密码
		if req.Password == "" {
			badRequestResponse(c, "password is required")
			return
		}

		// 创建用户
		user, err := userManager.CreateUser(req.Username, req.Password, req.Scopes)
		if err != nil {
			if err.Error() == "user already exists" {
				errorResponse(c, http.StatusConflict, "user already exists")
			} else {
				errorResponse(c, http.StatusInternalServerError, "failed to create user")
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    user,
		})
	}
}

// HandleUpdateUser PATCH /api/v1/users/:username
// 更新用户信息（权限等）
// Required Scopes: users.ScopeUserManage
func HandleUpdateUser(userManager *users.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")

		var req struct {
			Scopes []string `json:"scopes"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// 更新用户
		user, err := userManager.UpdateUser(username, req.Scopes)
		if err != nil {
			if err.Error() == "not found" {
				notFoundResponse(c, "user")
			} else {
				errorResponse(c, http.StatusInternalServerError, "failed to update user")
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    user,
		})
	}
}

// HandleDeleteUser DELETE /api/v1/users/:username
// 删除用户
// Required Scopes: users.ScopeUserManage
func HandleDeleteUser(userManager *users.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")

		// 安全检查：防止删除当前用户
		currentUsername := currentUser(c)
		if username == currentUsername {
			forbiddenResponse(c, "cannot delete current user")
			return
		}

		// 删除用户
		if err := userManager.DeleteUser(username); err != nil {
			if err.Error() == "not found" {
				notFoundResponse(c, "user")
			} else {
				errorResponse(c, http.StatusInternalServerError, "failed to delete user")
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "user deleted",
		})
	}
}

// HandleChangePassword POST /api/v1/users/:username/password
// 修改用户密码
// Required Scopes: users.ScopeUserManage
func HandleChangePassword(userManager *users.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")

		var req struct {
			OldPassword string `json:"old_password"`
			NewPassword string `json:"new_password"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// 修改密码
		if err := userManager.ChangePassword(username, req.OldPassword, req.NewPassword); err != nil {
			if err.Error() == "not found" {
				notFoundResponse(c, "user")
			} else if err.Error() == "invalid password" {
				errorResponse(c, http.StatusUnauthorized, "old password is incorrect")
			} else {
				errorResponse(c, http.StatusInternalServerError, "failed to update password")
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "password changed",
		})
	}
}
