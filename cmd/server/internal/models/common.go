package models

// UserCurrentTask represents a user's currently selected task
type UserCurrentTask struct {
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
	SetAt     string `json:"set_at"` // RFC3339 format
}
