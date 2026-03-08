package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/aidg-lite/internal/domain/projects"
)

// UserHiddenProjects 用户隐藏的项目列表
type UserHiddenProjects struct {
	HiddenProjectIDs []string `json:"hidden_project_ids"`
}

// UserProjectItem 用户项目可见性信息
type UserProjectItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProductLine string `json:"product_line"`
	Visible     bool   `json:"visible"`
}

// getUserHiddenProjects 读取用户隐藏的项目列表
func getUserHiddenProjects(username string) (*UserHiddenProjects, error) {
	usersDir := os.Getenv("USERS_DIR")
	if usersDir == "" {
		usersDir = "./data/users"
	}
	filePath := filepath.Join(usersDir, username, "hidden_projects.json")
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return &UserHiddenProjects{HiddenProjectIDs: []string{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read hidden projects: %w", err)
	}

	var hidden UserHiddenProjects
	if err := json.Unmarshal(data, &hidden); err != nil {
		return nil, fmt.Errorf("unmarshal hidden projects: %w", err)
	}

	return &hidden, nil
}

// setUserHiddenProjects 保存用户隐藏的项目列表
func setUserHiddenProjects(username string, hidden *UserHiddenProjects) error {
	usersDir := os.Getenv("USERS_DIR")
	if usersDir == "" {
		usersDir = "./data/users"
	}
	userDir := filepath.Join(usersDir, username)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("create user dir: %w", err)
	}

	data, err := json.MarshalIndent(hidden, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hidden projects: %w", err)
	}

	filePath := filepath.Join(userDir, "hidden_projects.json")
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write hidden projects: %w", err)
	}

	return nil
}

// IsProjectVisibleToUser 检查项目对用户是否可见
func IsProjectVisibleToUser(username, projectID string) bool {
	hidden, err := getUserHiddenProjects(username)
	if err != nil {
		return true // 出错时默认可见
	}
	for _, id := range hidden.HiddenProjectIDs {
		if id == projectID {
			return false
		}
	}
	return true
}

// HandleGetUserProjects GET /api/v1/user/projects
// 获取当前用户的项目列表（包含可见性信息）
// 返回所有项目（包括用户不可见的），并标记可见性状态
func HandleGetUserProjects(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := currentUser(c)

		list := reg.List()
		sort.Slice(list, func(i, j int) bool {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		})

		hidden, err := getUserHiddenProjects(username)
		if err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to get hidden projects: %w", err))
			return
		}

		// 创建隐藏项目ID的集合以便快速查找
		hiddenSet := make(map[string]bool)
		for _, id := range hidden.HiddenProjectIDs {
			hiddenSet[id] = true
		}

		items := make([]UserProjectItem, 0, len(list))
		for _, p := range list {
			items = append(items, UserProjectItem{
				ID:          p.ID,
				Name:        p.Name,
				ProductLine: p.ProductLine,
				Visible:     !hiddenSet[p.ID],
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    items,
		})
	}
}

// HandleUpdateUserProjects PUT /api/v1/user/projects
// 更新当前用户的项目可见性设置
// 请求体: { "hidden_project_ids": ["project-id-1", "project-id-2"] }
func HandleUpdateUserProjects(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := currentUser(c)

		var req struct {
			HiddenProjectIDs []string `json:"hidden_project_ids"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// 验证项目ID是否存在
		for _, id := range req.HiddenProjectIDs {
			if reg.Get(id) == nil {
				badRequestResponse(c, fmt.Sprintf("project not found: %s", id))
				return
			}
		}

		hidden := &UserHiddenProjects{
			HiddenProjectIDs: req.HiddenProjectIDs,
		}

		if err := setUserHiddenProjects(username, hidden); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to save hidden projects: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "project visibility updated",
		})
	}
}
