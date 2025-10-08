package executionplan

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHandleUpdatePlan_Success(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	if err := os.MkdirAll(filepath.Join(projectsRoot, projectID, "tasks", taskID), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	NewHandler(projectsRoot).RegisterRoutes(router)

	planContent := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Pending Approval"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
`

	body, _ := json.Marshal(map[string]string{"content": planContent})
	req, _ := http.NewRequest(http.MethodPost, "/internal/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["status"] != "Pending Approval" {
		t.Fatalf("unexpected status: %v", payload["status"])
	}

	planPath := filepath.Join(projectsRoot, projectID, "tasks", taskID, executionPlanFile)
	stored, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read stored plan: %v", err)
	}
	if string(stored) != planContent {
		t.Fatalf("plan content mismatch\nexpected:\n%s\nactual:\n%s", planContent, string(stored))
	}
}

func TestHandleUpdateStepStatus_Success(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	initialPlan := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Pending Approval"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
`
	if err := os.WriteFile(filepath.Join(taskDir, executionPlanFile), []byte(initialPlan), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	NewHandler(projectsRoot).RegisterRoutes(router)

	body, _ := json.Marshal(map[string]string{
		"status": "in-progress",
		"output": "working",
	})
	req, _ := http.NewRequest(http.MethodPost, "/internal/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan/steps/step-01/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	step, ok := payload["step"].(map[string]any)
	if !ok {
		t.Fatalf("missing step payload: %v", payload)
	}
	if step["status"] != "in-progress" {
		t.Fatalf("unexpected step status: %v", step["status"])
	}
	if step["output"] != "working" {
		t.Fatalf("unexpected output: %v", step["output"])
	}
}

func TestHandleGetNextStep_ReturnsCandidate(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	plan := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Approved"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies:
  - { source: 'step-02', target: 'step-01' }
---
- [x] step-01: 完成初始化 priority:medium
- [ ] step-02: 待执行 priority:high
`
	if err := os.WriteFile(filepath.Join(taskDir, executionPlanFile), []byte(plan), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	NewHandler(projectsRoot).RegisterRoutes(router)

	req, _ := http.NewRequest(http.MethodGet, "/internal/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan/next-step", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload["step_id"] != "step-02" {
		t.Fatalf("unexpected step id: %v", payload["step_id"])
	}
}

func TestHandleGetPlan_Success(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	planContent := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Pending Approval"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
- [ ] step-02: 测试 priority:medium
`
	if err := os.WriteFile(filepath.Join(taskDir, executionPlanFile), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	NewHandler(projectsRoot).RegisterRoutes(router)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// 响应格式为 {success: true, data: {...}}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data field in response")
	}

	if data["plan_id"] != "plan-123" {
		t.Fatalf("unexpected plan_id: %v", data["plan_id"])
	}
	if data["status"] != "Pending Approval" {
		t.Fatalf("unexpected status: %v", data["status"])
	}
	if data["content"] == nil {
		t.Fatalf("missing content field")
	}
}

func TestHandleGetExecutionPlan_InternalAPI(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	planContent := `---
plan_id: "plan-456"
task_id: "task_1759127546"
status: "Approved"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
- [x] step-02: 测试 priority:medium
`
	if err := os.WriteFile(filepath.Join(taskDir, executionPlanFile), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	NewHandler(projectsRoot).RegisterRoutes(router)

	// 测试 Internal API 路径
	req, _ := http.NewRequest(http.MethodGet, "/internal/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// 验证响应结构
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing or invalid data field")
	}

	if data["plan_id"] != "plan-456" {
		t.Fatalf("unexpected plan_id: %v", data["plan_id"])
	}
	if data["status"] != "Approved" {
		t.Fatalf("unexpected status: %v", data["status"])
	}
	if data["content"] == nil {
		t.Fatalf("missing content field")
	}
}

func TestHandleApprovePlan_Success(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	planContent := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Pending Approval"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
`
	if err := os.WriteFile(filepath.Join(taskDir, executionPlanFile), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	NewHandler(projectsRoot).RegisterRoutes(router)

	body, _ := json.Marshal(map[string]string{"comment": "looks good"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// 响应格式为 {success: true, data: {...}, message: "..."}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data field in response")
	}

	if data["status"] != "Approved" {
		t.Fatalf("unexpected status: %v", data["status"])
	}
	if payload["message"] != "plan approved successfully" {
		t.Fatalf("unexpected message: %v", payload["message"])
	}
}

func TestHandleApprovePlan_InvalidStatus(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 计划已经是 Approved 状态，不应该再次批准
	planContent := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Approved"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
`
	if err := os.WriteFile(filepath.Join(taskDir, executionPlanFile), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	NewHandler(projectsRoot).RegisterRoutes(router)

	body, _ := json.Marshal(map[string]string{"comment": "approve again"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got: %d", resp.Code)
	}
}

func TestHandleRejectPlan_Success(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	planContent := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Pending Approval"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
`
	if err := os.WriteFile(filepath.Join(taskDir, executionPlanFile), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	NewHandler(projectsRoot).RegisterRoutes(router)

	body, _ := json.Marshal(map[string]string{
		"comment": "needs more work",
		"reason":  "missing test cases",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan/reject", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// 响应格式为 {success: true, data: {...}, message: "..."}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data field in response")
	}

	if data["status"] != "Rejected" {
		t.Fatalf("unexpected status: %v", data["status"])
	}
	if data["reason"] != "missing test cases" {
		t.Fatalf("unexpected reason: %v", data["reason"])
	}
	if payload["message"] != "plan rejected successfully" {
		t.Fatalf("unexpected message: %v", payload["message"])
	}
}

// TestUpdateExecutionPlanContent_Success 测试编辑执行计划内容成功的场景
func TestUpdateExecutionPlanContent_Success(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 创建 tasks.json 文件
	tasksFile := filepath.Join(projectsRoot, projectID, "tasks.json")
	tasksData := `[
		{
			"id": "task-1",
			"name": "测试任务",
			"assignee": "testuser",
			"status": "in-progress"
		}
	]`
	if err := os.WriteFile(tasksFile, []byte(tasksData), 0o644); err != nil {
		t.Fatalf("write tasks.json: %v", err)
	}

	// 创建初始执行计划（状态为 Rejected，可编辑）
	initialPlan := `---
plan_id: "plan-123"
task_id: "task-1"
status: "Rejected"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
`
	planPath := filepath.Join(taskDir, executionPlanFile)
	if err := os.WriteFile(planPath, []byte(initialPlan), 0o644); err != nil {
		t.Fatalf("write initial plan: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	// 模拟认证中间件
	router.Use(func(c *gin.Context) {
		c.Set("user", "testuser")
		c.Set("scopes", []string{"task:write"})
		c.Next()
	})
	NewHandler(projectsRoot).RegisterRoutes(router)

	// 更新计划内容
	updatedContent := `---
plan_id: "plan-123"
task_id: "task-1"
status: "Rejected"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
- [ ] step-02: 添加新步骤 priority:medium
`

	body, _ := json.Marshal(map[string]string{"content": updatedContent})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !payload["success"].(bool) {
		t.Fatalf("expected success=true")
	}
	if payload["message"] != "plan updated successfully" {
		t.Fatalf("unexpected message: %v", payload["message"])
	}
}

// TestUpdateExecutionPlanContent_Unauthorized 测试未认证用户无法编辑
func TestUpdateExecutionPlanContent_Unauthorized(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	router := gin.New()
	router.Use(gin.Recovery())
	// 不设置用户信息，模拟未认证
	NewHandler(projectsRoot).RegisterRoutes(router)

	body, _ := json.Marshal(map[string]string{"content": "test content"})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.Code)
	}
}

// TestUpdateExecutionPlanContent_PermissionDenied 测试非任务负责人无法编辑
func TestUpdateExecutionPlanContent_PermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()
	projectsRoot := filepath.Join(tmpDir, "projects")
	projectID := "proj-1"
	taskID := "task-1"

	taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 创建 tasks.json，assignee 为 otheruser
	tasksFile := filepath.Join(projectsRoot, projectID, "tasks.json")
	tasksData := `[
		{
			"id": "task-1",
			"name": "测试任务",
			"assignee": "otheruser",
			"status": "in-progress"
		}
	]`
	if err := os.WriteFile(tasksFile, []byte(tasksData), 0o644); err != nil {
		t.Fatalf("write tasks.json: %v", err)
	}

	// 创建初始计划
	initialPlan := `---
plan_id: "plan-123"
task_id: "task-1"
status: "Rejected"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:00:00Z"
dependencies: []
---
- [ ] step-01: 初始化 priority:high
`
	planPath := filepath.Join(taskDir, executionPlanFile)
	if err := os.WriteFile(planPath, []byte(initialPlan), 0o644); err != nil {
		t.Fatalf("write initial plan: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	// 当前用户为 testuser，但任务 assignee 为 otheruser
	router.Use(func(c *gin.Context) {
		c.Set("user", "testuser")
		c.Set("scopes", []string{"task:write"})
		c.Next()
	})
	NewHandler(projectsRoot).RegisterRoutes(router)

	body, _ := json.Marshal(map[string]string{"content": "new content"})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID+"/tasks/"+taskID+"/execution-plan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// 应该返回 400 BadRequest（因为权限检查失败返回的错误会被处理为 BadRequest）
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// 验证错误信息包含 permission
	errorMsg, ok := payload["error"].(string)
	if !ok {
		t.Fatalf("expected error field in response")
	}
	if errorMsg == "" || !contains(errorMsg, "permission") {
		t.Fatalf("expected permission error, got: %s", errorMsg)
	}
}

// contains 是一个辅助函数，检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || bytes.Contains([]byte(s), []byte(substr))))
}
