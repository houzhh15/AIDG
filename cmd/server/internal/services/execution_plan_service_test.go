package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/models"
)

type stubExecutionPlanRepo struct {
	content  string
	readErr  error
	writeErr error
}

func (s *stubExecutionPlanRepo) Read(context.Context) (string, error) {
	if s.readErr != nil {
		return "", s.readErr
	}
	return s.content, nil
}

func (s *stubExecutionPlanRepo) Write(ctx context.Context, content string) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.content = content
	return nil
}

func TestExecutionPlanService_Load_Success(t *testing.T) {
	repo := &stubExecutionPlanRepo{content: sampleExecutionPlan()}
	svc := NewExecutionPlanService(repo)

	plan, err := svc.Load(context.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if plan.PlanID != "plan-123" {
		t.Fatalf("unexpected plan id: %s", plan.PlanID)
	}
	if plan.Status != models.PlanStatus("Pending Approval") {
		t.Fatalf("unexpected status: %s", plan.Status)
	}
	if len(plan.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(plan.Dependencies))
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 top-level steps, got %d", len(plan.Steps))
	}

	step1 := plan.Steps[0]
	if step1.ID != "step-01" || step1.Priority != models.StepPriorityHigh {
		t.Fatalf("step-01 parsing failed: %+v", step1)
	}

	step2 := plan.Steps[1]
	if step2.Status != models.StepStatusSucceeded {
		t.Fatalf("expected step-02 status succeeded, got %s", step2.Status)
	}
	if len(step2.SubSteps) != 1 {
		t.Fatalf("expected step-02 to have 1 sub-step, got %d", len(step2.SubSteps))
	}

	sub := step2.SubSteps[0]
	if sub.ID != "step-02-sub-01" || sub.Priority != models.StepPriorityHigh {
		t.Fatalf("sub-step parsing failed: %+v", sub)
	}
}

func TestExecutionPlanService_Load_InvalidFormat(t *testing.T) {
	repo := &stubExecutionPlanRepo{content: "invalid execution plan"}
	svc := NewExecutionPlanService(repo)

	_, err := svc.Load(context.Background())
	if err == nil {
		t.Fatalf("expected error but got nil")
	}

	if !errors.Is(err, ErrInvalidPlanFormat) {
		t.Fatalf("expected ErrInvalidPlanFormat, got %v", err)
	}
}

func sampleExecutionPlan() string {
	return "---\n" +
		"plan_id: \"plan-123\"\n" +
		"task_id: \"task_1759127546\"\n" +
		"status: \"Pending Approval\"\n" +
		"created_at: \"2025-09-29T18:00:00Z\"\n" +
		"updated_at: \"2025-09-29T18:10:00Z\"\n" +
		"dependencies:\n" +
		"  - { source: 'step-02', target: 'step-01' }\n" +
		"---\n" +
		"- [ ] step-01: 初始化服务 priority:high\n" +
		"- [x] step-02: 实现功能 priority:medium\n" +
		"    - [ ] step-02-sub-01: 编写测试 priority:high\n"
}

func TestParseTimeValue(t *testing.T) {
	timeStr := "2025-09-29T18:10:00Z"
	parsed, err := parseTimeValue(timeStr)
	if err != nil {
		t.Fatalf("parseTimeValue returned error: %v", err)
	}

	if parsed.Format(time.RFC3339) != timeStr {
		t.Fatalf("expected %s, got %s", timeStr, parsed.Format(time.RFC3339))
	}

	parsed, err = parseTimeValue("")
	if err != nil {
		t.Fatalf("parseTimeValue empty returned error: %v", err)
	}
	if !parsed.IsZero() {
		t.Fatalf("expected zero time for empty input")
	}
}

func TestGetNextExecutableStep_SelectsHighestPriority(t *testing.T) {
	repo := &stubExecutionPlanRepo{content: `---
plan_id: "plan-nav"
task_id: "task_1759127546"
status: "Approved"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies:
  - { source: 'step-02', target: 'step-01' }
  - { source: 'step-03', target: 'step-01' }
  - { source: 'step-04', target: 'step-02' }
---
- [x] step-01: 初始化 priority:high
- [ ] step-02: 高优先级候选 priority:high
- [ ] step-03: 中优先级候选 priority:medium
- [ ] step-04: 未满足依赖 priority:high
`}
	svc := NewExecutionPlanService(repo)
	if _, err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	step, err := svc.GetNextExecutableStep(context.Background())
	if err != nil {
		t.Fatalf("GetNextExecutableStep returned error: %v", err)
	}
	if step == nil {
		t.Fatalf("expected step, got nil")
	}
	if step.ID != "step-02" {
		t.Fatalf("expected step-02, got %s", step.ID)
	}
}

func TestGetNextExecutableStep_TieBreakByStepID(t *testing.T) {
	repo := &stubExecutionPlanRepo{content: `---
plan_id: "plan-nav-tie"
task_id: "task_1759127546"
status: "Approved"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies:
  - { source: 'step-02', target: 'step-01' }
  - { source: 'step-03', target: 'step-01' }
---
- [x] step-01: 完成初始化 priority:medium
- [ ] step-02: 候选一 priority:medium
- [ ] step-03: 候选二 priority:medium
`}
	svc := NewExecutionPlanService(repo)
	if _, err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	step, err := svc.GetNextExecutableStep(context.Background())
	if err != nil {
		t.Fatalf("GetNextExecutableStep returned error: %v", err)
	}
	if step == nil {
		t.Fatalf("expected step, got nil")
	}
	if step.ID != "step-02" {
		t.Fatalf("expected step-02 for tie-break, got %s", step.ID)
	}
}

func TestGetNextExecutableStep_PlanNotReady(t *testing.T) {
	repo := &stubExecutionPlanRepo{content: `---
plan_id: "plan-not-ready"
task_id: "task_1759127546"
status: "Pending Approval"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies: []
---
- [ ] step-01: 等待审批 priority:high
`}
	svc := NewExecutionPlanService(repo)
	if _, err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	step, err := svc.GetNextExecutableStep(context.Background())
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	if !errors.Is(err, ErrPlanNotReady) {
		t.Fatalf("expected ErrPlanNotReady, got %v", err)
	}
	if step != nil {
		t.Fatalf("expected nil step when plan not ready")
	}
}

func TestGetNextExecutableStep_NoEligibleStep(t *testing.T) {
	repo := &stubExecutionPlanRepo{content: `---
plan_id: "plan-no-eligible"
task_id: "task_1759127546"
status: "Approved"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies:
  - { source: 'step-03', target: 'step-02' }
---
- [x] step-01: 完成基础 priority:medium
- [!] step-02: 依赖失败 priority:high
- [ ] step-03: 等待依赖 priority:high
`}
	svc := NewExecutionPlanService(repo)
	if _, err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	step, err := svc.GetNextExecutableStep(context.Background())
	if err != nil {
		t.Fatalf("GetNextExecutableStep returned error: %v", err)
	}
	if step != nil {
		t.Fatalf("expected nil step when no executable candidate")
	}
}

func TestUpdateStepStatus_InProgress(t *testing.T) {
	repo := &stubExecutionPlanRepo{content: sampleStatusPlan()}
	svc := NewExecutionPlanService(repo)
	if _, err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	updated, err := svc.UpdateStepStatus(context.Background(), "step-01", models.StepStatusInProgress, "starting work")
	if err != nil {
		t.Fatalf("UpdateStepStatus returned error: %v", err)
	}

	if updated.Status != models.PlanStatusExecuting {
		t.Fatalf("expected plan status Executing, got %s", updated.Status)
	}

	_, index := flattenAndIndexSteps(updated.Steps)
	step := index["step-01"]
	if step == nil {
		t.Fatalf("step-01 not found in updated plan")
	}
	if step.Status != models.StepStatusInProgress {
		t.Fatalf("expected step-01 in-progress, got %s", step.Status)
	}
	if step.Output != "starting work" {
		t.Fatalf("expected output to persist, got %s", step.Output)
	}
	if step.StartedAt == nil {
		t.Fatalf("expected started_at to be set")
	}

	content := repo.content
	if !strings.Contains(content, "status: Executing") {
		t.Fatalf("repository content missing plan status update: %s", content)
	}
	if !strings.Contains(content, "- [>] step-01") {
		t.Fatalf("repository content missing in-progress marker: %s", content)
	}
	if !strings.Contains(content, "output:\"starting work\"") {
		t.Fatalf("repository content missing output attribute: %s", content)
	}
}

func TestUpdateStepStatus_CompletesPlan(t *testing.T) {
	repo := &stubExecutionPlanRepo{content: sampleStatusPlan()}
	svc := NewExecutionPlanService(repo)
	if _, err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if _, err := svc.UpdateStepStatus(context.Background(), "step-01", models.StepStatusSucceeded, "done"); err != nil {
		t.Fatalf("first UpdateStepStatus returned error: %v", err)
	}

	updated, err := svc.UpdateStepStatus(context.Background(), "step-02", models.StepStatusSucceeded, "done too")
	if err != nil {
		t.Fatalf("second UpdateStepStatus returned error: %v", err)
	}

	if updated.Status != models.PlanStatusCompleted {
		t.Fatalf("expected plan status Completed, got %s", updated.Status)
	}

	_, index := flattenAndIndexSteps(updated.Steps)
	if step := index["step-01"]; step == nil || step.Status != models.StepStatusSucceeded {
		t.Fatalf("expected step-01 succeeded, got %+v", step)
	}
	if step := index["step-02"]; step == nil || step.Status != models.StepStatusSucceeded {
		t.Fatalf("expected step-02 succeeded, got %+v", step)
	}

	content := repo.content
	if !strings.Contains(content, "status: Completed") {
		t.Fatalf("repository content missing completed status: %s", content)
	}
	if !strings.Contains(content, "- [x] step-02") {
		t.Fatalf("repository content missing success marker: %s", content)
	}
}

func TestUpdateStepStatus_StepNotFound(t *testing.T) {
	repo := &stubExecutionPlanRepo{content: sampleStatusPlan()}
	svc := NewExecutionPlanService(repo)
	if _, err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	_, err := svc.UpdateStepStatus(context.Background(), "step-99", models.StepStatusSucceeded, "oops")
	if err == nil {
		t.Fatalf("expected error for missing step but got nil")
	}
	if !errors.Is(err, ErrStepNotFound) {
		t.Fatalf("expected ErrStepNotFound, got %v", err)
	}
}

func sampleStatusPlan() string {
	return "---\n" +
		"plan_id: \"plan-status\"\n" +
		"task_id: \"task_1759127546\"\n" +
		"status: \"Approved\"\n" +
		"created_at: \"2025-09-29T18:00:00Z\"\n" +
		"updated_at: \"2025-09-29T18:00:00Z\"\n" +
		"dependencies: []\n" +
		"---\n" +
		"- [ ] step-01: 准备环境 priority:high\n" +
		"- [ ] step-02: 编码实现 priority:medium\n"
}

// TestHasEditPermission 测试权限检查逻辑
func TestHasEditPermission(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		taskAssignee string
		userScopes   []string
		want         bool
	}{
		{
			name:         "用户是任务负责人",
			userID:       "houzhh",
			taskAssignee: "houzhh",
			userScopes:   []string{},
			want:         true,
		},
		{
			name:         "用户具有 task.write 权限",
			userID:       "alice",
			taskAssignee: "bob",
			userScopes:   []string{"task.read", "task.write"},
			want:         true,
		},
		{
			name:         "用户具有 execution_plan.edit 权限",
			userID:       "alice",
			taskAssignee: "bob",
			userScopes:   []string{"execution_plan.edit"},
			want:         true,
		},
		{
			name:         "用户无权限",
			userID:       "alice",
			taskAssignee: "bob",
			userScopes:   []string{"task.read"},
			want:         false,
		},
		{
			name:         "空用户",
			userID:       "",
			taskAssignee: "bob",
			userScopes:   []string{},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasEditPermission(tt.userID, tt.taskAssignee, tt.userScopes)
			if got != tt.want {
				t.Errorf("HasEditPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsEditableState 测试状态检查逻辑
func TestIsEditableState(t *testing.T) {
	tests := []struct {
		name   string
		status models.PlanStatus
		want   bool
	}{
		{
			name:   "Pending Approval 可编辑",
			status: models.PlanStatusPendingApproval,
			want:   true,
		},
		{
			name:   "Rejected 可编辑",
			status: models.PlanStatusRejected,
			want:   true,
		},
		{
			name:   "Approved 可编辑（有条件）",
			status: models.PlanStatusApproved,
			want:   true,
		},
		{
			name:   "Executing 可编辑（有条件）",
			status: models.PlanStatusExecuting,
			want:   true,
		},
		{
			name:   "Completed 可编辑（有条件）",
			status: models.PlanStatusCompleted,
			want:   true,
		},
		{
			name:   "Failed 不可编辑",
			status: models.PlanStatusFailed,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEditableState(tt.status)
			if got != tt.want {
				t.Errorf("IsEditableState() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestProtectExecutedSteps 测试步骤保护逻辑
func TestProtectExecutedSteps(t *testing.T) {
	now := time.Now().UTC()

	t.Run("允许修改 pending 步骤", func(t *testing.T) {
		current := &models.ExecutionPlan{
			Status: models.PlanStatusExecuting,
			Steps: []*models.Step{
				{ID: "step-01", Status: models.StepStatusSucceeded, Description: "已完成"},
				{ID: "step-02", Status: models.StepStatusPending, Description: "待执行"},
			},
		}

		new := &models.ExecutionPlan{
			Status: models.PlanStatusExecuting,
			Steps: []*models.Step{
				{ID: "step-01", Status: models.StepStatusSucceeded, Description: "已完成"},
				{ID: "step-02", Status: models.StepStatusPending, Description: "修改后的描述"},
			},
		}

		err := ProtectExecutedSteps(current, new)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("禁止修改已执行步骤的描述", func(t *testing.T) {
		current := &models.ExecutionPlan{
			Status: models.PlanStatusExecuting,
			Steps: []*models.Step{
				{ID: "step-01", Status: models.StepStatusSucceeded, Description: "已完成", CompletedAt: &now},
				{ID: "step-02", Status: models.StepStatusPending, Description: "待执行"},
			},
		}

		new := &models.ExecutionPlan{
			Status: models.PlanStatusExecuting,
			Steps: []*models.Step{
				{ID: "step-01", Status: models.StepStatusSucceeded, Description: "尝试修改已完成步骤"},
				{ID: "step-02", Status: models.StepStatusPending, Description: "待执行"},
			},
		}

		err := ProtectExecutedSteps(current, new)
		if err == nil {
			t.Error("expected error when modifying executed step, got nil")
		}
		if !strings.Contains(err.Error(), "step-01") {
			t.Errorf("expected error to mention step-01, got %v", err)
		}
	})

	t.Run("禁止修改已执行步骤的优先级", func(t *testing.T) {
		current := &models.ExecutionPlan{
			Status: models.PlanStatusApproved,
			Steps: []*models.Step{
				{ID: "step-01", Status: models.StepStatusInProgress, Priority: models.StepPriorityHigh, Description: "执行中"},
			},
		}

		new := &models.ExecutionPlan{
			Status: models.PlanStatusApproved,
			Steps: []*models.Step{
				{ID: "step-01", Status: models.StepStatusInProgress, Priority: models.StepPriorityLow, Description: "执行中"},
			},
		}

		err := ProtectExecutedSteps(current, new)
		if err == nil {
			t.Error("expected error when modifying priority of in-progress step, got nil")
		}
	})

	t.Run("禁止删除已执行步骤", func(t *testing.T) {
		current := &models.ExecutionPlan{
			Status: models.PlanStatusExecuting,
			Steps: []*models.Step{
				{ID: "step-01", Status: models.StepStatusSucceeded, Description: "已完成"},
				{ID: "step-02", Status: models.StepStatusPending, Description: "待执行"},
			},
		}

		new := &models.ExecutionPlan{
			Status: models.PlanStatusExecuting,
			Steps: []*models.Step{
				{ID: "step-02", Status: models.StepStatusPending, Description: "待执行"},
			},
		}

		err := ProtectExecutedSteps(current, new)
		if err == nil {
			t.Error("expected error when deleting executed step, got nil")
		}
		if !strings.Contains(err.Error(), "step-01") {
			t.Errorf("expected error to mention step-01, got %v", err)
		}
	})

	t.Run("Pending Approval 状态下不需要保护", func(t *testing.T) {
		current := &models.ExecutionPlan{
			Status: models.PlanStatusPendingApproval,
			Steps: []*models.Step{
				{ID: "step-01", Status: models.StepStatusPending, Description: "原描述"},
			},
		}

		new := &models.ExecutionPlan{
			Status: models.PlanStatusPendingApproval,
			Steps: []*models.Step{
				{ID: "step-01", Status: models.StepStatusPending, Description: "新描述"},
			},
		}

		err := ProtectExecutedSteps(current, new)
		if err != nil {
			t.Errorf("expected no error in Pending Approval state, got %v", err)
		}
	})
}

// TestUpdatePlanContent 测试执行计划内容更新逻辑
func TestUpdatePlanContent(t *testing.T) {
	t.Run("成功更新 Pending Approval 状态的计划", func(t *testing.T) {
		repo := &stubExecutionPlanRepo{content: sampleExecutionPlan()}
		svc := NewExecutionPlanService(repo)

		// 先加载计划
		_, err := svc.Load(context.Background())
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		newContent := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Pending Approval"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies:
  - { source: 'step-02', target: 'step-01' }
---
- [ ] step-01: 初始化服务（已修改） priority:high
- [x] step-02: 实现功能 priority:medium
    - [ ] step-02-sub-01: 子步骤 priority:high
`

		updatedPlan, err := UpdatePlanContent(
			svc,
			context.Background(),
			newContent,
			"houzhh",   // userID
			"houzhh",   // taskAssignee (匹配，有权限)
			[]string{}, // userScopes
		)

		if err != nil {
			t.Fatalf("UpdatePlanContent failed: %v", err)
		}

		if updatedPlan.Steps[0].Description != "初始化服务（已修改）" {
			t.Errorf("step description not updated: %s", updatedPlan.Steps[0].Description)
		}
	})

	t.Run("权限不足时拒绝更新", func(t *testing.T) {
		repo := &stubExecutionPlanRepo{content: sampleExecutionPlan()}
		svc := NewExecutionPlanService(repo)
		_, _ = svc.Load(context.Background())

		_, err := UpdatePlanContent(
			svc,
			context.Background(),
			sampleExecutionPlan(),
			"alice",    // userID
			"bob",      // taskAssignee (不匹配)
			[]string{}, // userScopes (无权限)
		)

		if err == nil {
			t.Fatal("expected permission error, got nil")
		}
		if !strings.Contains(err.Error(), "permission") {
			t.Errorf("expected permission error, got: %v", err)
		}
	})

	t.Run("Failed 状态时拒绝更新", func(t *testing.T) {
		failedPlan := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Failed"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies: []
---
- [x] step-01: 已失败 priority:high
`
		repo := &stubExecutionPlanRepo{content: failedPlan}
		svc := NewExecutionPlanService(repo)
		_, _ = svc.Load(context.Background())

		_, err := UpdatePlanContent(
			svc,
			context.Background(),
			failedPlan,
			"houzhh",
			"houzhh",
			[]string{},
		)

		if err == nil {
			t.Fatal("expected state error, got nil")
		}
		if !strings.Contains(err.Error(), "cannot be edited") {
			t.Errorf("expected state error, got: %v", err)
		}
	})

	t.Run("格式错误时拒绝更新", func(t *testing.T) {
		repo := &stubExecutionPlanRepo{content: sampleExecutionPlan()}
		svc := NewExecutionPlanService(repo)
		_, _ = svc.Load(context.Background())

		_, err := UpdatePlanContent(
			svc,
			context.Background(),
			"invalid plan content",
			"houzhh",
			"houzhh",
			[]string{},
		)

		if err == nil {
			t.Fatal("expected format error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid plan format") {
			t.Errorf("expected format error, got: %v", err)
		}
	})

	t.Run("循环依赖时拒绝更新", func(t *testing.T) {
		repo := &stubExecutionPlanRepo{content: sampleExecutionPlan()}
		svc := NewExecutionPlanService(repo)
		_, _ = svc.Load(context.Background())

		circularPlan := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Pending Approval"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies:
  - { source: 'step-02', target: 'step-01' }
  - { source: 'step-01', target: 'step-02' }
---
- [ ] step-01: 步骤1 priority:high
- [ ] step-02: 步骤2 priority:high
`

		_, err := UpdatePlanContent(
			svc,
			context.Background(),
			circularPlan,
			"houzhh",
			"houzhh",
			[]string{},
		)

		if err == nil {
			t.Fatal("expected circular dependency error, got nil")
		}
		if !strings.Contains(err.Error(), "circular dependency") {
			t.Errorf("expected circular dependency error, got: %v", err)
		}
	})

	t.Run("Executing 状态下保护已执行步骤", func(t *testing.T) {
		executingPlan := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Executing"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies: []
---
- [x] step-01: 已完成 priority:high
- [ ] step-02: 待执行 priority:medium
`
		repo := &stubExecutionPlanRepo{content: executingPlan}
		svc := NewExecutionPlanService(repo)
		_, _ = svc.Load(context.Background())

		modifiedPlan := `---
plan_id: "plan-123"
task_id: "task_1759127546"
status: "Executing"
created_at: "2025-09-29T18:00:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies: []
---
- [x] step-01: 已完成（尝试修改） priority:high
- [ ] step-02: 待执行 priority:medium
`

		_, err := UpdatePlanContent(
			svc,
			context.Background(),
			modifiedPlan,
			"houzhh",
			"houzhh",
			[]string{},
		)

		if err == nil {
			t.Fatal("expected protection error, got nil")
		}
		if !strings.Contains(err.Error(), "cannot modify") {
			t.Errorf("expected protection error, got: %v", err)
		}
	})
}

// TestValidateDependencyGraph 测试依赖图验证逻辑
func TestValidateDependencyGraph(t *testing.T) {
	t.Run("无依赖的计划合法", func(t *testing.T) {
		plan := &models.ExecutionPlan{
			Steps:        []*models.Step{{ID: "step-01"}},
			Dependencies: []models.Dependency{},
		}
		err := validateDependencyGraph(plan)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("简单线性依赖合法", func(t *testing.T) {
		plan := &models.ExecutionPlan{
			Steps: []*models.Step{
				{ID: "step-01"},
				{ID: "step-02"},
			},
			Dependencies: []models.Dependency{
				{Source: "step-02", Target: "step-01"},
			},
		}
		err := validateDependencyGraph(plan)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("检测简单循环依赖", func(t *testing.T) {
		plan := &models.ExecutionPlan{
			Steps: []*models.Step{
				{ID: "step-01"},
				{ID: "step-02"},
			},
			Dependencies: []models.Dependency{
				{Source: "step-02", Target: "step-01"},
				{Source: "step-01", Target: "step-02"},
			},
		}
		err := validateDependencyGraph(plan)
		if err == nil {
			t.Error("expected circular dependency error, got nil")
		}
		if !strings.Contains(err.Error(), "circular dependency") {
			t.Errorf("expected circular dependency error, got: %v", err)
		}
	})

	t.Run("检测复杂循环依赖", func(t *testing.T) {
		plan := &models.ExecutionPlan{
			Steps: []*models.Step{
				{ID: "step-01"},
				{ID: "step-02"},
				{ID: "step-03"},
			},
			Dependencies: []models.Dependency{
				{Source: "step-02", Target: "step-01"},
				{Source: "step-03", Target: "step-02"},
				{Source: "step-01", Target: "step-03"},
			},
		}
		err := validateDependencyGraph(plan)
		if err == nil {
			t.Error("expected circular dependency error, got nil")
		}
	})

	t.Run("依赖引用不存在的步骤时返回错误", func(t *testing.T) {
		plan := &models.ExecutionPlan{
			Steps: []*models.Step{
				{ID: "step-01"},
			},
			Dependencies: []models.Dependency{
				{Source: "step-02", Target: "step-01"},
			},
		}
		err := validateDependencyGraph(plan)
		if err == nil {
			t.Error("expected error for missing step, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})
}
