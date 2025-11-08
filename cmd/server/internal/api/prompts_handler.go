package api

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/prompt"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

// Error codes
const (
	ErrPromptNotFound      = "PROMPT_NOT_FOUND"
	ErrPermissionDenied    = "PERMISSION_DENIED"
	ErrInvalidInput        = "INVALID_INPUT"
	ErrPromptAlreadyExists = "PROMPT_ALREADY_EXISTS"
	ErrStorageFailure      = "STORAGE_FAILURE"
)

// PromptsHandler handles prompts CRUD operations
type PromptsHandler struct {
	storage           *prompt.PromptStorage
	permissionChecker *prompt.PromptPermissionChecker
	userManager       *users.Manager
	notifyTriggerPath string // 触发MCP Server重新加载的文件路径
}

// NewPromptsHandler creates a new handler instance
func NewPromptsHandler(storage *prompt.PromptStorage, permChecker *prompt.PromptPermissionChecker, userMgr *users.Manager, triggerPath string) *PromptsHandler {
	return &PromptsHandler{
		storage:           storage,
		permissionChecker: permChecker,
		userManager:       userMgr,
		notifyTriggerPath: triggerPath,
	}
}

// notifyMCPServerPromptsChanged 通知MCP Server Prompts已变更
// 通过创建触发文件的方式通知独立进程的MCP Server
func (h *PromptsHandler) notifyMCPServerPromptsChanged() {
	if h.notifyTriggerPath == "" {
		return
	}

	// 创建或更新触发文件
	if err := os.WriteFile(h.notifyTriggerPath, []byte(time.Now().Format(time.RFC3339)), 0644); err != nil {
		log.Printf("⚠️  [PROMPTS] 触发MCP通知失败: %v", err)
		return
	}

	log.Printf("📢 [PROMPTS] 已触发MCP Server重新加载: %s", h.notifyTriggerPath)
}

// CreatePrompt handles POST /api/v1/prompts
func (h *PromptsHandler) CreatePrompt(c *gin.Context) {
	username := currentUser(c)

	var req struct {
		Name        string                  `json:"name"`
		Description string                  `json:"description"`
		Content     string                  `json:"content"`
		Arguments   []prompt.PromptArgument `json:"arguments"`
		Scope       string                  `json:"scope"`
		Visibility  string                  `json:"visibility"`
		ProjectID   string                  `json:"project_id,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, 400, "Invalid request body", ErrInvalidInput)
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Name) == "" {
		h.errorResponse(c, 400, "参数校验失败：name 不能为空", ErrInvalidInput)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		h.errorResponse(c, 400, "参数校验失败：content 不能为空", ErrInvalidInput)
		return
	}

	// Validate scope
	if req.Scope != prompt.ScopeGlobal && req.Scope != prompt.ScopeProject && req.Scope != prompt.ScopePersonal {
		h.errorResponse(c, 400, fmt.Sprintf("无效的 scope: %s", req.Scope), ErrInvalidInput)
		return
	}

	// Validate visibility
	if req.Visibility != prompt.VisibilityPublic && req.Visibility != prompt.VisibilityPrivate {
		h.errorResponse(c, 400, fmt.Sprintf("无效的 visibility: %s", req.Visibility), ErrInvalidInput)
		return
	}

	// Validate project_id for project scope
	if req.Scope == prompt.ScopeProject && req.ProjectID == "" {
		h.errorResponse(c, 400, "project scope 需要提供 project_id", ErrInvalidInput)
		return
	}

	// Validate arguments format
	if err := h.validateArguments(req.Arguments); err != nil {
		h.errorResponse(c, 400, err.Error(), ErrInvalidInput)
		return
	}

	// Generate prompt_id
	promptID := fmt.Sprintf("prompt_%d", time.Now().Unix())

	// Create prompt object
	now := time.Now()
	p := &prompt.Prompt{
		PromptID:    promptID,
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Arguments:   req.Arguments,
		Scope:       req.Scope,
		Visibility:  req.Visibility,
		Owner:       username,
		ProjectID:   req.ProjectID,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Save to storage
	if err := h.storage.Save(p); err != nil {
		log.Printf("[ERROR] CreatePrompt - Failed to save: %v", err)
		h.errorResponse(c, 500, fmt.Sprintf("存储操作失败: %v", err), ErrStorageFailure)
		return
	}

	log.Printf("[INFO] CreatePrompt - user=%s, scope=%s, prompt_id=%s", username, req.Scope, promptID)

	// Trigger MCP Server to reload prompts (step-06)
	h.notifyMCPServerPromptsChanged()

	c.JSON(200, gin.H{
		"success": true,
		"data": gin.H{
			"prompt_id":  promptID,
			"created_at": now.Format(time.RFC3339),
			"version":    1,
		},
	})
}

// ListPrompts handles GET /api/v1/prompts
func (h *PromptsHandler) ListPrompts(c *gin.Context) {
	username := currentUser(c)

	// Get query parameters
	scope := c.Query("scope")
	visibility := c.Query("visibility")
	projectID := c.Query("project_id")

	// 如果路径中有项目ID（/api/v1/projects/:id/prompts），优先使用路径参数
	if pathProjectID := c.Param("id"); pathProjectID != "" {
		projectID = pathProjectID
		// 项目级别的 Prompts 自动设置 scope 为 "project"
		if scope == "" {
			scope = "project"
		}
	}

	// Build filter
	filter := make(map[string]string)
	if visibility != "" {
		filter["visibility"] = visibility
	}
	if projectID != "" {
		filter["project_id"] = projectID
	}
	// Only filter by owner for personal scope
	if scope == "personal" && username != "" {
		filter["owner"] = username
	}

	// Load prompts from storage
	prompts, err := h.storage.LoadAll(scope, filter)
	if err != nil {
		log.Printf("[ERROR] ListPrompts - Failed to load: %v", err)
		h.errorResponse(c, 500, fmt.Sprintf("加载失败: %v", err), ErrStorageFailure)
		return
	}

	// Get user object for permission check
	user, found := h.userManager.GetUser(username)
	if !found {
		log.Printf("[WARN] ListPrompts - User not found: %s", username)
		user = &users.User{Username: username, Scopes: []string{}}
	}

	// Filter by permission
	var result []gin.H
	for _, p := range prompts {
		if h.permissionChecker.CanView(user, p) {
			// Exclude content field in list response
			result = append(result, gin.H{
				"prompt_id":   p.PromptID,
				"name":        p.Name,
				"description": p.Description,
				"arguments":   p.Arguments,
				"scope":       p.Scope,
				"visibility":  p.Visibility,
				"owner":       p.Owner,
				"project_id":  p.ProjectID,
				"version":     p.Version,
				"created_at":  p.CreatedAt.Format(time.RFC3339),
				"updated_at":  p.UpdatedAt.Format(time.RFC3339),
			})
		}
	}

	c.JSON(200, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetPrompt handles GET /api/v1/prompts/:prompt_id
func (h *PromptsHandler) GetPrompt(c *gin.Context) {
	username := currentUser(c)
	promptID := c.Param("prompt_id")

	// Load prompt from storage
	p, err := h.storage.Load(promptID)
	if err != nil {
		log.Printf("[WARN] GetPrompt - Prompt not found: %s", promptID)
		h.errorResponse(c, 404, "Prompt 不存在", ErrPromptNotFound)
		return
	}

	// Get user object for permission check
	user, found := h.userManager.GetUser(username)
	if !found {
		log.Printf("[WARN] GetPrompt - User not found: %s", username)
		user = &users.User{Username: username, Scopes: []string{}}
	}

	// Check permission
	if !h.permissionChecker.CanView(user, p) {
		log.Printf("[WARN] PermissionDenied - user=%s, action=view, prompt_id=%s, owner=%s", username, promptID, p.Owner)
		h.errorResponse(c, 403, "权限不足：无法查看此 Prompt", ErrPermissionDenied)
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"data": gin.H{
			"prompt_id":   p.PromptID,
			"name":        p.Name,
			"description": p.Description,
			"content":     p.Content,
			"arguments":   p.Arguments,
			"scope":       p.Scope,
			"visibility":  p.Visibility,
			"owner":       p.Owner,
			"project_id":  p.ProjectID,
			"version":     p.Version,
			"created_at":  p.CreatedAt.Format(time.RFC3339),
			"updated_at":  p.UpdatedAt.Format(time.RFC3339),
		},
	})
}

// UpdatePrompt handles PUT /api/v1/prompts/:prompt_id
func (h *PromptsHandler) UpdatePrompt(c *gin.Context) {
	username := currentUser(c)
	promptID := c.Param("prompt_id")

	// Load existing prompt
	p, err := h.storage.Load(promptID)
	if err != nil {
		log.Printf("[WARN] UpdatePrompt - Prompt not found: %s", promptID)
		h.errorResponse(c, 404, "Prompt 不存在", ErrPromptNotFound)
		return
	}

	// Get user object for permission check
	user, found := h.userManager.GetUser(username)
	if !found {
		log.Printf("[WARN] UpdatePrompt - User not found: %s", username)
		user = &users.User{Username: username, Scopes: []string{}}
	}

	// Check permission
	if !h.permissionChecker.CanEdit(user, p) {
		log.Printf("[WARN] PermissionDenied - user=%s, action=edit, prompt_id=%s, owner=%s", username, promptID, p.Owner)
		h.errorResponse(c, 403, "权限不足：您不是该 Prompts 的创建者", ErrPermissionDenied)
		return
	}

	// Parse request body
	var req struct {
		Name        *string                  `json:"name"`
		Description *string                  `json:"description"`
		Content     *string                  `json:"content"`
		Arguments   *[]prompt.PromptArgument `json:"arguments"`
		Visibility  *string                  `json:"visibility"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, 400, "Invalid request body", ErrInvalidInput)
		return
	}

	// Update fields
	updated := false
	if req.Name != nil && *req.Name != p.Name {
		p.Name = *req.Name
		updated = true
	}
	if req.Description != nil && *req.Description != p.Description {
		p.Description = *req.Description
		updated = true
	}
	if req.Content != nil && *req.Content != p.Content {
		p.Content = *req.Content
		updated = true
	}
	if req.Arguments != nil {
		if err := h.validateArguments(*req.Arguments); err != nil {
			h.errorResponse(c, 400, err.Error(), ErrInvalidInput)
			return
		}
		p.Arguments = *req.Arguments
		updated = true
	}
	if req.Visibility != nil && (*req.Visibility == prompt.VisibilityPublic || *req.Visibility == prompt.VisibilityPrivate) {
		if *req.Visibility != p.Visibility {
			p.Visibility = *req.Visibility
			updated = true
		}
	}

	if !updated {
		c.JSON(200, gin.H{
			"success": true,
			"message": "No changes detected",
			"data": gin.H{
				"prompt_id":  p.PromptID,
				"version":    p.Version,
				"updated_at": p.UpdatedAt.Format(time.RFC3339),
			},
		})
		return
	}

	// Increment version and update timestamp
	p.Version++
	p.UpdatedAt = time.Now()

	// Save to storage
	if err := h.storage.Save(p); err != nil {
		log.Printf("[ERROR] UpdatePrompt - Failed to save: %v", err)
		h.errorResponse(c, 500, fmt.Sprintf("存储操作失败: %v", err), ErrStorageFailure)
		return
	}

	log.Printf("[INFO] UpdatePrompt - user=%s, prompt_id=%s, version=%d", username, promptID, p.Version)

	// Trigger MCP Server to reload prompts (step-06)
	h.notifyMCPServerPromptsChanged()

	c.JSON(200, gin.H{
		"success": true,
		"data": gin.H{
			"prompt_id":  p.PromptID,
			"version":    p.Version,
			"updated_at": p.UpdatedAt.Format(time.RFC3339),
		},
	})
}

// DeletePrompt handles DELETE /api/v1/prompts/:prompt_id
func (h *PromptsHandler) DeletePrompt(c *gin.Context) {
	username := currentUser(c)
	promptID := c.Param("prompt_id")

	// Load existing prompt
	p, err := h.storage.Load(promptID)
	if err != nil {
		log.Printf("[WARN] DeletePrompt - Prompt not found: %s", promptID)
		h.errorResponse(c, 404, "Prompt 不存在", ErrPromptNotFound)
		return
	}

	// Get user object for permission check
	user, found := h.userManager.GetUser(username)
	if !found {
		log.Printf("[WARN] DeletePrompt - User not found: %s", username)
		user = &users.User{Username: username, Scopes: []string{}}
	}

	// Check permission
	if !h.permissionChecker.CanDelete(user, p) {
		log.Printf("[WARN] PermissionDenied - user=%s, action=delete, prompt_id=%s, owner=%s", username, promptID, p.Owner)
		h.errorResponse(c, 403, "权限不足：无法删除此 Prompt", ErrPermissionDenied)
		return
	}

	// Delete from storage
	if err := h.storage.Delete(promptID); err != nil {
		log.Printf("[ERROR] DeletePrompt - Failed to delete: %v", err)
		h.errorResponse(c, 500, fmt.Sprintf("删除失败: %v", err), ErrStorageFailure)
		return
	}

	log.Printf("[INFO] DeletePrompt - user=%s, prompt_id=%s", username, promptID)

	// Trigger MCP Server to reload prompts (step-06)
	h.notifyMCPServerPromptsChanged()

	c.JSON(200, gin.H{
		"success": true,
		"message": "Prompt 删除成功",
	})
}

// Helper methods

func (h *PromptsHandler) validateArguments(args []prompt.PromptArgument) error {
	argNamePattern := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	for i, arg := range args {
		if arg.Name == "" {
			return fmt.Errorf("参数 %d 的 name 不能为空", i+1)
		}
		if !argNamePattern.MatchString(arg.Name) {
			return fmt.Errorf("参数 %d 的 name 格式不正确（仅允许字母、数字、下划线）", i+1)
		}
	}
	return nil
}

func (h *PromptsHandler) errorResponse(c *gin.Context, code int, message string, errorCode string) {
	c.JSON(code, gin.H{
		"success":    false,
		"error":      message,
		"error_code": errorCode,
	})
}
