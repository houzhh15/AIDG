package executionplan

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/houzhh15/AIDG/cmd/server/internal/models"
	"github.com/houzhh15/AIDG/cmd/server/internal/services"

	"github.com/gin-gonic/gin"
)

// Handler 负责注册并实现 ExecutionPlan 内部 API。
type Handler struct {
	projectsRoot string
}

// NewHandler 创建一个新的执行计划 API 处理器。
func NewHandler(projectsRoot string) *Handler {
	root := strings.TrimSpace(projectsRoot)
	if root == "" {
		root = defaultProjectsRoot
	}
	return &Handler{projectsRoot: root}
}

// RegisterRoutes 将内部 API 注册到给定的 Gin 路由器上。
func (h *Handler) RegisterRoutes(r gin.IRouter) {
	// Internal API (面向 mcp-server) - 使用 POST 保持向后兼容
	r.GET("/internal/api/v1/projects/:id/tasks/:task_id/execution-plan", h.GetExecutionPlan)
	r.POST("/internal/api/v1/projects/:id/tasks/:task_id/execution-plan", h.UpdateExecutionPlan)
	r.PUT("/internal/api/v1/projects/:id/tasks/:task_id/execution-plan", h.UpdateExecutionPlan)
	r.POST("/internal/api/v1/projects/:id/tasks/:task_id/execution-plan/steps/:step_id/status", h.UpdatePlanStepStatus)
	r.PUT("/internal/api/v1/projects/:id/tasks/:task_id/execution-plan/steps/:step_id/status", h.UpdatePlanStepStatus)
	r.GET("/internal/api/v1/projects/:id/tasks/:task_id/execution-plan/next-step", h.GetNextExecutableStep)

	// Web API (面向前端) - 使用标准 RESTful 方法
	r.GET("/api/v1/projects/:id/tasks/:task_id/execution-plan", h.GetExecutionPlan)
	r.PUT("/api/v1/projects/:id/tasks/:task_id/execution-plan", h.UpdateExecutionPlanContent)
	r.POST("/api/v1/projects/:id/tasks/:task_id/execution-plan/submit", h.SubmitExecutionPlan)
	r.POST("/api/v1/projects/:id/tasks/:task_id/execution-plan/approve", h.ApproveExecutionPlan)
	r.POST("/api/v1/projects/:id/tasks/:task_id/execution-plan/reject", h.RejectExecutionPlan)
	r.POST("/api/v1/projects/:id/tasks/:task_id/execution-plan/restore-approval", h.RestoreApproval)
	r.POST("/api/v1/projects/:id/tasks/:task_id/execution-plan/reset", h.ResetExecutionPlan)
}

func (h *Handler) UpdateExecutionPlan(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid content"})
		return
	}

	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	plan, err := svc.UpdatePlan(c.Request.Context(), req.Content)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	// 自动审批通过：创建/更新执行计划时直接设置为 Approved 状态
	if plan.Status == models.PlanStatusDraft || plan.Status == models.PlanStatusPendingApproval {
		plan, err = svc.UpdatePlanStatus(c.Request.Context(), models.PlanStatusApproved)
		if err != nil {
			// 自动审批失败不阻塞响应，记录日志
			fmt.Printf("[WARN] Auto-approve execution plan failed for project=%s task=%s: %v\n", projectID, taskID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"plan_id":    plan.PlanID,
		"task_id":    plan.TaskID,
		"status":     plan.Status,
		"updated_at": plan.UpdatedAt,
	})
}

func (h *Handler) UpdatePlanStepStatus(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")
	stepID := strings.TrimSpace(c.Param("step_id"))
	if stepID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid step_id"})
		return
	}

	var req struct {
		Status string `json:"status"`
		Output string `json:"output"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	status, ok := parseStepStatus(req.Status)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        "invalid status value",
			"message":      fmt.Sprintf("状态值 '%s' 无效。有效的状态值为：pending（待开始）、in-progress（进行中）、succeeded（成功完成）、failed（失败）、cancelled（已取消）。提示：如果要标记为已完成，请使用 'succeeded' 而不是 'completed'。", req.Status),
			"valid_values": []string{"pending", "in-progress", "succeeded", "failed", "cancelled"},
			"received":     req.Status,
		})
		return
	}

	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	if _, err := svc.Load(c.Request.Context()); err != nil {
		h.writeServiceError(c, err)
		return
	}

	plan, err := svc.UpdateStepStatus(c.Request.Context(), stepID, status, req.Output)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	step := findStepByID(plan.Steps, stepID)
	stepResp := toStepResponse(step)
	if stepResp == nil {
		stepResp = gin.H{}
	}
	if _, exists := stepResp["output"]; !exists || stepResp["output"] == "" {
		stepResp["output"] = req.Output
	}
	c.JSON(http.StatusOK, gin.H{
		"plan_id": plan.PlanID,
		"status":  plan.Status,
		"step":    stepResp,
	})
}

func (h *Handler) GetNextExecutableStep(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	if _, err := svc.Load(c.Request.Context()); err != nil {
		h.writeServiceError(c, err)
		return
	}

	step, err := svc.GetNextExecutableStep(c.Request.Context())
	if err != nil {
		if errors.Is(err, services.ErrPlanNotReady) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		h.writeServiceError(c, err)
		return
	}

	if step == nil {
		c.JSON(http.StatusOK, gin.H{"step_id": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"step_id":     step.ID,
		"priority":    step.Priority,
		"description": step.Description,
		"status":      step.Status,
	})
}

func (h *Handler) GetExecutionPlan(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	plan, err := svc.Load(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	// 获取原始内容用于前端渲染
	repo, _ := NewFileRepository(h.projectsRoot, projectID, taskID)
	rawContent, _ := repo.Read(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"plan_id":      plan.PlanID,
			"task_id":      plan.TaskID,
			"status":       plan.Status,
			"created_at":   plan.CreatedAt,
			"updated_at":   plan.UpdatedAt,
			"dependencies": plan.Dependencies,
			"steps":        toStepsResponse(plan.Steps),
			"content":      rawContent,
		},
	})
}

func (h *Handler) SubmitExecutionPlan(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	var req struct {
		Comment string `json:"comment"`
	}
	c.ShouldBindJSON(&req)

	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	plan, err := svc.Load(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	// 验证计划状态
	if plan.Status != models.PlanStatusDraft && plan.Status != models.PlanStatusRejected {
		c.JSON(http.StatusConflict, gin.H{
			"error": "plan is not in draft or rejected status",
		})
		return
	}

	// 更新计划状态为 Pending Approval
	plan, err = svc.UpdatePlanStatus(c.Request.Context(), models.PlanStatusPendingApproval)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "execution plan submitted successfully",
		"plan": gin.H{
			"plan_id":      plan.PlanID,
			"task_id":      plan.TaskID,
			"status":       plan.Status,
			"updated_at":   plan.UpdatedAt,
			"dependencies": plan.Dependencies,
			"steps":        toStepsResponse(plan.Steps),
		},
	})
}

func (h *Handler) ApproveExecutionPlan(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	var req struct {
		Comment string `json:"comment"`
	}
	c.ShouldBindJSON(&req)

	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	plan, err := svc.Load(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	// 验证计划状态
	if plan.Status != models.PlanStatusPendingApproval {
		c.JSON(http.StatusConflict, gin.H{
			"error": "plan is not in pending approval status",
		})
		return
	}

	// 更新计划状态为 Approved
	plan, err = svc.UpdatePlanStatus(c.Request.Context(), models.PlanStatusApproved)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"plan_id":    plan.PlanID,
			"status":     plan.Status,
			"updated_at": plan.UpdatedAt,
		},
		"message": "plan approved successfully",
	})
}

func (h *Handler) RejectExecutionPlan(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	var req struct {
		Comment string `json:"comment"`
		Reason  string `json:"reason"`
	}
	c.ShouldBindJSON(&req)

	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	plan, err := svc.Load(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	// 验证计划状态
	if plan.Status != models.PlanStatusPendingApproval {
		c.JSON(http.StatusConflict, gin.H{
			"error": "plan is not in pending approval status",
		})
		return
	}

	// 更新计划状态为 Rejected
	plan, err = svc.UpdatePlanStatus(c.Request.Context(), models.PlanStatusRejected)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"plan_id":    plan.PlanID,
			"status":     plan.Status,
			"updated_at": plan.UpdatedAt,
			"reason":     req.Reason,
		},
		"message": "plan rejected successfully",
	})
}

// RestoreApproval 恢复执行计划到待审批状态 (Web API)
// POST /api/v1/projects/:id/tasks/:task_id/execution-plan/restore-approval
func (h *Handler) RestoreApproval(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	var req struct {
		Comment string `json:"comment"`
	}
	c.ShouldBindJSON(&req)

	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	plan, err := svc.Load(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	// 验证计划状态 - 只允许从 Approved、Executing、Completed 状态恢复
	if plan.Status != models.PlanStatusApproved &&
		plan.Status != models.PlanStatusExecuting &&
		plan.Status != models.PlanStatusCompleted {
		c.JSON(http.StatusConflict, gin.H{
			"error": "plan status does not allow restoration to pending approval",
		})
		return
	}

	// 更新计划状态为 Pending Approval
	plan, err = svc.UpdatePlanStatus(c.Request.Context(), models.PlanStatusPendingApproval)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"plan_id":    plan.PlanID,
			"status":     plan.Status,
			"updated_at": plan.UpdatedAt,
		},
		"message": "plan restored to pending approval successfully",
	})
}

// UpdateExecutionPlanContent 更新执行计划内容 (Web API)
// PUT /api/v1/projects/:id/tasks/:task_id/execution-plan
func (h *Handler) UpdateExecutionPlanContent(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	// 解析请求体
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "content field is required",
		})
		return
	}

	// 从 Gin context 获取当前用户和权限
	userInterface, exists := c.Get("user")
	if !exists {
		// 添加详细日志以帮助调试
		fmt.Printf("[ExecutionPlan] ERROR: user not found in context - ProjectID: %s, TaskID: %s, Path: %s, Method: %s\n",
			projectID, taskID, c.Request.URL.Path, c.Request.Method)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}
	username, ok := userInterface.(string)
	if !ok || username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid user context",
		})
		return
	}

	// 获取用户权限范围
	scopesInterface, _ := c.Get("scopes")
	scopes, _ := scopesInterface.([]string)

	// 读取任务信息获取 assignee
	task, err := h.loadTaskInfo(projectID, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("failed to load task info: %v", err),
		})
		return
	}

	// 创建服务
	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	// 调用 UpdatePlanContent 执行完整的编辑流程
	plan, err := services.UpdatePlanContent(svc, c.Request.Context(), req.Content, username, task.Assignee, scopes)
	if err != nil {
		// UpdatePlanContent 返回的错误已经包含详细信息，直接返回给客户端
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"plan_id":    plan.PlanID,
			"status":     plan.Status,
			"updated_at": plan.UpdatedAt,
		},
		"message": "plan updated successfully",
	})
}

// ResetExecutionPlan 重置或生成执行计划模板 (Web API)
// POST /api/v1/projects/:id/tasks/:task_id/execution-plan/reset
func (h *Handler) ResetExecutionPlan(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")

	// 解析请求体
	var req struct {
		Force bool `json:"force"`
	}
	c.ShouldBindJSON(&req)

	// 从 Gin context 获取当前用户和权限
	userInterface, exists := c.Get("user")
	if !exists {
		fmt.Printf("[ExecutionPlan] ERROR: user not found in context - ProjectID: %s, TaskID: %s, Path: %s, Method: %s\n",
			projectID, taskID, c.Request.URL.Path, c.Request.Method)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}
	username, ok := userInterface.(string)
	if !ok || username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid user context",
		})
		return
	}

	// 获取用户权限范围
	scopesInterface, _ := c.Get("scopes")
	scopes, _ := scopesInterface.([]string)

	// 读取任务信息获取 assignee
	task, err := h.loadTaskInfo(projectID, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("failed to load task info: %v", err),
		})
		return
	}

	// 权限校验：复用 UpdatePlanContent 的逻辑
	if !services.HasEditPermission(username, task.Assignee, scopes) {
		fmt.Printf("[ExecutionPlan] Permission denied - User: %s, Assignee: %s, Scopes: %v\n",
			username, task.Assignee, scopes)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "permission denied: only task assignee or users with execution_plan.edit/task.write scope can reset execution plan",
		})
		return
	}

	// 创建仓库
	repo, err := NewFileRepository(h.projectsRoot, projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	// 使用 TemplateGenerator 生成或覆盖模板
	generator := NewTemplateGenerator()
	opts := TemplateOptions{Force: req.Force}
	if err := generator.Ensure(c.Request.Context(), repo, taskID, opts); err != nil {
		if errors.Is(err, ErrPlanExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "execution plan already exists, use force=true to overwrite",
			})
		} else {
			fmt.Printf("[ExecutionPlan] Failed to generate template - ProjectID: %s, TaskID: %s, Error: %v\n",
				projectID, taskID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to generate template: %v", err),
			})
		}
		return
	}

	// 加载最新计划
	svc, err := h.newService(projectID, taskID)
	if err != nil {
		h.writeRepositoryError(c, err)
		return
	}

	plan, err := svc.Load(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	// 记录 INFO 日志
	fmt.Printf("[ExecutionPlan] Template reset successfully - ProjectID: %s, TaskID: %s, PlanID: %s, Force: %t, User: %s\n",
		projectID, taskID, plan.PlanID, req.Force, username)

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"plan_id":    plan.PlanID,
			"status":     plan.Status,
			"updated_at": plan.UpdatedAt,
		},
		"message": "execution plan reset successfully",
	})
}

// TaskInfo 表示 tasks.json 中的任务条目
type TaskInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Assignee    string `json:"assignee"`
	Status      string `json:"status"`
	Module      string `json:"module"`
	Description string `json:"description"`
}

// loadTaskInfo 从 projects/{projectID}/tasks.json 加载任务信息
func (h *Handler) loadTaskInfo(projectID, taskID string) (*TaskInfo, error) {
	tasksFile := filepath.Join(h.projectsRoot, projectID, "tasks.json")
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks.json: %w", err)
	}

	var tasks []TaskInfo
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse tasks.json: %w", err)
	}

	for _, task := range tasks {
		if task.ID == taskID {
			return &task, nil
		}
	}

	return nil, fmt.Errorf("task %s not found in tasks.json", taskID)
}

func (h *Handler) newService(projectID, taskID string) (services.ExecutionPlanService, error) {
	repo, err := NewFileRepository(h.projectsRoot, projectID, taskID)
	if err != nil {
		return nil, err
	}
	return services.NewExecutionPlanService(repo), nil
}

func (h *Handler) writeRepositoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidIdentifier):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrProjectNotFound), errors.Is(err, ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *Handler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPlanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrInvalidPlanFormat):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrStepNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrPlanNotLoaded):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func parseStepStatus(raw string) (models.StepStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(models.StepStatusPending):
		return models.StepStatusPending, true
	case string(models.StepStatusInProgress):
		return models.StepStatusInProgress, true
	case string(models.StepStatusSucceeded):
		return models.StepStatusSucceeded, true
	case string(models.StepStatusFailed):
		return models.StepStatusFailed, true
	case string(models.StepStatusCancelled):
		return models.StepStatusCancelled, true
	default:
		return "", false
	}
}

func findStepByID(steps []*models.Step, target string) *models.Step {
	for _, step := range steps {
		if step == nil {
			continue
		}
		if step.ID == target {
			return step
		}
		if res := findStepByID(step.SubSteps, target); res != nil {
			return res
		}
	}
	return nil
}

func toStepResponse(step *models.Step) gin.H {
	if step == nil {
		return nil
	}
	resp := gin.H{
		"id":          step.ID,
		"status":      step.Status,
		"priority":    step.Priority,
		"description": step.Description,
		"output":      step.Output,
		"updated_at":  step.UpdatedAt,
	}
	if step.StartedAt != nil {
		resp["started_at"] = step.StartedAt
	}
	if step.CompletedAt != nil {
		resp["completed_at"] = step.CompletedAt
	}
	return resp
}

// toStepsResponse 将步骤数组转换为响应格式
func toStepsResponse(steps []*models.Step) []gin.H {
	if steps == nil {
		return nil
	}
	result := make([]gin.H, 0, len(steps))
	for _, step := range steps {
		if step == nil {
			continue
		}
		result = append(result, toStepResponse(step))
	}
	return result
}
