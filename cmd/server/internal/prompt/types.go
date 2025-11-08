package prompt

import "time"

// Prompt represents a custom prompt template with three-tier architecture support
type Prompt struct {
	PromptID    string           `json:"prompt_id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Content     string           `json:"content"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Scope       string           `json:"scope"`      // global, project, personal
	Visibility  string           `json:"visibility"` // public, private
	Owner       string           `json:"owner"`
	ProjectID   string           `json:"project_id,omitempty"`
	Version     int              `json:"version"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// PromptArgument represents a parameter definition in a prompt template
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

// Scope constants
const (
	ScopeGlobal   = "global"
	ScopeProject  = "project"
	ScopePersonal = "personal"
)

// Visibility constants
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)
