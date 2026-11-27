package tasklifecycle

import (
	"errors"
	"time"
)

const (
	StatusTodo       = "todo"
	StatusInProgress = "in-progress"
	StatusReview     = "review"
	StatusCompleted  = "completed"

	ReviewPassed    = "通过"
	ReviewNotPassed = "不通过"
)

var (
	ErrInvalidID         = errors.New("invalid identifier")
	ErrInvalidInput      = errors.New("invalid input")
	ErrInvalidStatus     = errors.New("invalid task status")
	ErrInvalidTransition = errors.New("invalid task status transition")
	ErrReviewRequired    = errors.New("passed review required")
	ErrReviewNotFound    = errors.New("review not found")
	ErrVersionConflict   = errors.New("resource revision conflict")
	ErrNotFound          = errors.New("task not found")
	ErrUnauthorized      = errors.New("authentication required")
)

type Readiness struct {
	ReadyCount   int  `json:"ready_count"`
	Requirements bool `json:"requirements"`
	Design       bool `json:"design"`
	Test         bool `json:"test"`
}

type ReviewRequest struct {
	TargetType       string `json:"target_type"`
	TargetPath       string `json:"target_path"`
	Criteria         string `json:"criteria"`
	ExpectedRevision *int   `json:"expected_revision,omitempty"`
}

type ReviewResultRequest struct {
	Issues           string `json:"issues,omitempty"`
	Result           string `json:"result"`
	ChangeSummary    string `json:"change_summary,omitempty"`
	Verification     string `json:"verification,omitempty"`
	ExpectedRevision *int   `json:"expected_revision,omitempty"`
}

type ReviewDocument struct {
	Content   string    `json:"content"`
	Version   int       `json:"version"`
	ETag      string    `json:"etag"`
	UpdatedAt time.Time `json:"updated_at"`
	Exists    bool      `json:"exists"`
}

// LifecycleImport is a privileged synchronization operation. ImportedBy is
// intentionally absent: the authenticated AIDG actor is supplied by the server.
type LifecycleImport struct {
	Status       string          `json:"status"`
	Review       LifecycleReview `json:"review"`
	Source       LifecycleSource `json:"source"`
	EvidenceHash string          `json:"evidence_hash"`
}

type LifecycleReview struct {
	Content       string `json:"content"`
	Verdict       string `json:"verdict"`
	ChangeSummary string `json:"change_summary"`
	Verification  string `json:"verification"`
	Issues        string `json:"issues,omitempty"`
}

type LifecycleSource struct {
	System    string `json:"system"`
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
	Actor     string `json:"actor"`
}

func ValidStatus(status string) bool {
	switch status {
	case StatusTodo, StatusInProgress, StatusReview, StatusCompleted:
		return true
	default:
		return false
	}
}

func CanTransition(from, to string) bool {
	switch {
	case from == StatusTodo && to == StatusInProgress:
		return true
	case from == StatusInProgress && to == StatusReview:
		return true
	case from == StatusReview && to == StatusInProgress:
		return true
	case from == StatusReview && to == StatusCompleted:
		return true
	default:
		return false
	}
}
