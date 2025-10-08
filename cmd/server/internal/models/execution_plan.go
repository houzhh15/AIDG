package models

import "time"

// PlanStatus 表示执行计划的整体状态。
type PlanStatus string

const (
	PlanStatusDraft           PlanStatus = "Draft"
	PlanStatusPendingApproval PlanStatus = "Pending Approval"
	PlanStatusApproved        PlanStatus = "Approved"
	PlanStatusRejected        PlanStatus = "Rejected"
	PlanStatusExecuting       PlanStatus = "Executing"
	PlanStatusCompleted       PlanStatus = "Completed"
	PlanStatusFailed          PlanStatus = "Failed"
)

// StepStatus 表示单个步骤的执行状态。
type StepStatus string

const (
	StepStatusPending    StepStatus = "pending"
	StepStatusInProgress StepStatus = "in-progress"
	StepStatusSucceeded  StepStatus = "succeeded"
	StepStatusFailed     StepStatus = "failed"
	StepStatusCancelled  StepStatus = "cancelled"
)

// StepPriority 表示步骤的优先级。
type StepPriority string

const (
	StepPriorityHigh   StepPriority = "high"
	StepPriorityMedium StepPriority = "medium"
	StepPriorityLow    StepPriority = "low"
)

// Dependency 表示执行计划中的依赖关系边。
type Dependency struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Step 表示执行计划中的单个步骤。
type Step struct {
	ID          string         `json:"id"`
	Status      StepStatus     `json:"status"`
	Priority    StepPriority   `json:"priority,omitempty"`
	Description string         `json:"description"`
	Output      string         `json:"output,omitempty"`
	SubSteps    []*Step        `json:"sub_steps,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ExecutionPlan 表示一个任务的完整执行计划。
type ExecutionPlan struct {
	PlanID       string         `json:"plan_id"`
	TaskID       string         `json:"task_id"`
	Status       PlanStatus     `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	ApprovedBy   *string        `json:"approved_by,omitempty"`
	Dependencies []Dependency   `json:"dependencies,omitempty"`
	Steps        []*Step        `json:"steps,omitempty"`
	RawContent   string         `json:"raw_content,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}
