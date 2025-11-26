package docslot

import (
	"testing"
)

func TestPathResolver_ResolvePath(t *testing.T) {
	// basePath 是项目根目录（如 data/projects）
	resolver := NewPathResolver("data/projects")

	tests := []struct {
		name     string
		scope    DocumentScope
		scopeID  string
		slotKey  string
		wantPath string
		wantErr  error
	}{
		{
			name:     "project feature_list",
			scope:    ScopeProject,
			scopeID:  "proj-123",
			slotKey:  "feature_list",
			wantPath: "data/projects/proj-123/docs/feature_list",
		},
		{
			name:     "project architecture_design",
			scope:    ScopeProject,
			scopeID:  "proj-456",
			slotKey:  "architecture_design",
			wantPath: "data/projects/proj-456/docs/architecture_design",
		},
		{
			name:     "meeting polish",
			scope:    ScopeMeeting,
			scopeID:  "meet-789",
			slotKey:  "polish",
			wantPath: "data/meetings/meet-789/docs/polish",
		},
		{
			name:     "meeting summary",
			scope:    ScopeMeeting,
			scopeID:  "meet-001",
			slotKey:  "summary",
			wantPath: "data/meetings/meet-001/docs/summary",
		},
		{
			name:     "meeting topic",
			scope:    ScopeMeeting,
			scopeID:  "meet-002",
			slotKey:  "topic",
			wantPath: "data/meetings/meet-002/docs/topic",
		},
		{
			name:    "task scope should error",
			scope:   ScopeTask,
			scopeID: "task-123",
			slotKey: "requirements",
			wantErr: ErrUseTaskPath,
		},
		{
			name:    "invalid scope",
			scope:   DocumentScope("invalid"),
			scopeID: "id-123",
			slotKey: "design",
			wantErr: ErrInvalidScope,
		},
		{
			name:    "invalid slot for project",
			scope:   ScopeProject,
			scopeID: "proj-123",
			slotKey: "requirements",
			wantErr: ErrInvalidSlot,
		},
		{
			name:    "invalid slot for meeting",
			scope:   ScopeMeeting,
			scopeID: "meet-123",
			slotKey: "design",
			wantErr: ErrInvalidSlot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, err := resolver.ResolvePath(tt.scope, tt.scopeID, tt.slotKey)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errorContains(err, tt.wantErr) {
					t.Errorf("expected error containing %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if gotPath != tt.wantPath {
				t.Errorf("got path %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}

func TestPathResolver_ResolveTaskPath(t *testing.T) {
	// basePath 是项目根目录（如 data/projects）
	resolver := NewPathResolver("data/projects")

	tests := []struct {
		name      string
		projectID string
		taskID    string
		slotKey   string
		wantPath  string
		wantErr   error
	}{
		{
			name:      "task requirements",
			projectID: "proj-123",
			taskID:    "task-456",
			slotKey:   "requirements",
			wantPath:  "data/projects/proj-123/tasks/task-456/docs/requirements",
		},
		{
			name:      "task design",
			projectID: "proj-123",
			taskID:    "task-789",
			slotKey:   "design",
			wantPath:  "data/projects/proj-123/tasks/task-789/docs/design",
		},
		{
			name:      "task test",
			projectID: "proj-456",
			taskID:    "task-001",
			slotKey:   "test",
			wantPath:  "data/projects/proj-456/tasks/task-001/docs/test",
		},
		{
			name:      "invalid slot for task",
			projectID: "proj-123",
			taskID:    "task-456",
			slotKey:   "feature_list",
			wantErr:   ErrInvalidSlot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, err := resolver.ResolveTaskPath(tt.projectID, tt.taskID, tt.slotKey)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errorContains(err, tt.wantErr) {
					t.Errorf("expected error containing %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if gotPath != tt.wantPath {
				t.Errorf("got path %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}

func TestPathResolver_ValidateScope(t *testing.T) {
	resolver := NewPathResolver("data")

	tests := []struct {
		name    string
		scope   DocumentScope
		slotKey string
		wantErr bool
	}{
		{"valid task requirements", ScopeTask, "requirements", false},
		{"valid task design", ScopeTask, "design", false},
		{"valid task test", ScopeTask, "test", false},
		{"valid project feature_list", ScopeProject, "feature_list", false},
		{"valid project architecture_design", ScopeProject, "architecture_design", false},
		{"valid meeting polish", ScopeMeeting, "polish", false},
		{"valid meeting summary", ScopeMeeting, "summary", false},
		{"valid meeting topic", ScopeMeeting, "topic", false},
		{"invalid scope", DocumentScope("invalid"), "design", true},
		{"task with project slot", ScopeTask, "feature_list", true},
		{"project with task slot", ScopeProject, "design", true},
		{"meeting with task slot", ScopeMeeting, "test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.ValidateScope(tt.scope, tt.slotKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateScope() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveLegacyPath(t *testing.T) {
	tests := []struct {
		name     string
		scope    DocumentScope
		scopeID  string
		slotKey  string
		wantPath string
	}{
		{
			name:     "legacy project feature_list",
			scope:    ScopeProject,
			scopeID:  "proj-123",
			slotKey:  "feature_list",
			wantPath: "data/projects/proj-123/feature_list.md",
		},
		{
			name:     "legacy meeting polish",
			scope:    ScopeMeeting,
			scopeID:  "meet-456",
			slotKey:  "polish",
			wantPath: "data/meetings/meet-456/polish.md",
		},
		{
			name:     "task scope returns empty",
			scope:    ScopeTask,
			scopeID:  "task-123",
			slotKey:  "design",
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath := ResolveLegacyPath("data", tt.scope, tt.scopeID, tt.slotKey)
			if gotPath != tt.wantPath {
				t.Errorf("got %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}

// errorContains 检查 err 是否包含 target 错误
func errorContains(err, target error) bool {
	if err == nil {
		return target == nil
	}
	// 检查是否为 DocSlotError
	if dse, ok := err.(*DocSlotError); ok {
		return dse.Err == target
	}
	return err == target
}
