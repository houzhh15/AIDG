package prompt

import (
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

// PromptPermissionChecker handles permission checks for prompt operations
type PromptPermissionChecker struct {
	userManager *users.Manager
}

// NewPromptPermissionChecker creates a new permission checker
func NewPromptPermissionChecker(userManager *users.Manager) *PromptPermissionChecker {
	return &PromptPermissionChecker{
		userManager: userManager,
	}
}

// CanView determines if a user can view a prompt
func (c *PromptPermissionChecker) CanView(user *users.User, prompt *Prompt) bool {
	// Personal prompts: only owner can view
	if prompt.Scope == ScopePersonal {
		return user.Username == prompt.Owner
	}

	// Global/Project public prompts: all users can view
	if prompt.Visibility == VisibilityPublic {
		return true
	}

	// Global/Project private prompts: only owner can view
	if prompt.Visibility == VisibilityPrivate {
		return user.Username == prompt.Owner
	}

	// Project public prompts: project members can view
	// Note: Project membership check would require integration with project service
	// For now, assuming public prompts in project scope are viewable by all
	if prompt.Scope == ScopeProject && prompt.Visibility == VisibilityPublic {
		// TODO: Add project membership check
		// return c.isProjectMember(user.Username, prompt.ProjectID)
		return true
	}

	return false
}

// CanEdit determines if a user can edit a prompt
func (c *PromptPermissionChecker) CanEdit(user *users.User, prompt *Prompt) bool {
	// Owner can always edit their own prompts
	if user.Username == prompt.Owner {
		return true
	}

	// Admin can edit prompts owned by "admin" (preset prompts)
	if c.isAdmin(user) && prompt.Owner == "admin" {
		return true
	}

	return false
}

// CanDelete determines if a user can delete a prompt
func (c *PromptPermissionChecker) CanDelete(user *users.User, prompt *Prompt) bool {
	// Same rules as CanEdit
	return c.CanEdit(user, prompt)
}

// Helper methods

func (c *PromptPermissionChecker) isAdmin(user *users.User) bool {
	// Check if user has admin scope
	for _, scope := range user.Scopes {
		if scope == "project.admin" || scope == "user.manage" {
			return true
		}
	}
	return false
}

// isProjectMember checks if user is a member of the project
// TODO: Implement when project service integration is available
func (c *PromptPermissionChecker) isProjectMember(username, projectID string) bool {
	// Placeholder implementation
	// In production, this should query the project service
	return true
}
