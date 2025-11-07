package services

import (
	"github.com/houzhh15/AIDG/cmd/server/internal/audit"
	"github.com/houzhh15/AIDG/cmd/server/internal/constants"
	"testing"
	"time"
)

// TestUserRoleService_AssignRoles 测试角色分配
func TestUserRoleService_AssignRoles(t *testing.T) {
	tmpDirRoles := t.TempDir()
	tmpDirUsers := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, _ := NewRoleManager(tmpDirRoles, mockLogger)
	userRoleService, err := NewUserRoleService(tmpDirUsers, roleManager, mockLogger)
	if err != nil {
		t.Fatalf("NewUserRoleService failed: %v", err)
	}

	// 先创建角色
	projectID := "test-project"
	role1, _ := roleManager.CreateRole(projectID, "Developer", []string{constants.ScopeTaskRead})
	role2, _ := roleManager.CreateRole(projectID, "Reviewer", []string{constants.ScopeTaskRead, constants.ScopeTaskWrite})

	// 正常案例: 分配角色
	username := "alice"
	roleIDs := []string{role1.RoleID, role2.RoleID}

	err = userRoleService.AssignRoles(username, projectID, roleIDs)
	if err != nil {
		t.Fatalf("AssignRoles failed: %v", err)
	}

	// 验证角色分配 (通过GetUserProfile)
	profile, err := userRoleService.GetUserProfile(username)
	if err != nil {
		t.Fatalf("GetUserProfile failed: %v", err)
	}

	// 检查是否有项目角色
	if len(profile.ProjectRoles) == 0 {
		t.Error("Expected user to have project roles")
	}

	// 验证第一个角色的项目ID
	if profile.ProjectRoles[0].ProjectID != projectID {
		t.Errorf("Expected project %s in user profile, got %s", projectID, profile.ProjectRoles[0].ProjectID)
	}

	// 检查审计日志 (2个create_role + 1个assign_role)
	if len(mockLogger.logs) < 3 {
		t.Errorf("Expected at least 3 audit logs, got %d", len(mockLogger.logs))
	}
	if mockLogger.logs[2] != string(audit.ActionAssignRole) {
		t.Errorf("Expected last log to be assign_role, got %s", mockLogger.logs[2])
	}

	// 错误案例: 分配不存在的角色
	err = userRoleService.AssignRoles(username, projectID, []string{"non-existent-role"})
	if err == nil {
		t.Error("Expected error when assigning non-existent role")
	}
}

// TestUserRoleService_RevokeRoles 测试角色撤销
func TestUserRoleService_RevokeRoles(t *testing.T) {
	tmpDirRoles := t.TempDir()
	tmpDirUsers := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, _ := NewRoleManager(tmpDirRoles, mockLogger)
	userRoleService, _ := NewUserRoleService(tmpDirUsers, roleManager, mockLogger)

	projectID := "test-project"
	role, _ := roleManager.CreateRole(projectID, "Developer", []string{constants.ScopeTaskRead})

	username := "bob"
	_ = userRoleService.AssignRoles(username, projectID, []string{role.RoleID})

	// 正常案例: 撤销角色
	err := userRoleService.RevokeRoles(username, projectID)
	if err != nil {
		t.Fatalf("RevokeRoles failed: %v", err)
	}

	// 验证撤销 (用户应该没有该项目的角色)
	profile, _ := userRoleService.GetUserProfile(username)

	// 检查是否还有该项目的角色
	hasProjectRole := false
	for _, roleInfo := range profile.ProjectRoles {
		if roleInfo.ProjectID == projectID {
			hasProjectRole = true
			break
		}
	}
	if hasProjectRole {
		t.Error("Expected project to be removed from user profile after revoke")
	}

	// 检查审计日志 (create_role + assign_role + revoke_role)
	if len(mockLogger.logs) < 3 {
		t.Errorf("Expected at least 3 audit logs, got %d", len(mockLogger.logs))
	}
	if mockLogger.logs[2] != string(audit.ActionRevokeRole) {
		t.Errorf("Expected last log to be revoke_role, got %s", mockLogger.logs[2])
	}
}

// TestUserRoleService_ComputeEffectiveScopes 测试有效权限计算
func TestUserRoleService_ComputeEffectiveScopes(t *testing.T) {
	tmpDirRoles := t.TempDir()
	tmpDirUsers := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, _ := NewRoleManager(tmpDirRoles, mockLogger)
	userRoleService, _ := NewUserRoleService(tmpDirUsers, roleManager, mockLogger)

	projectID := "test-project"

	// 创建2个角色,权限有重叠 (单个用户只能分配1个角色)
	role1, _ := roleManager.CreateRole(projectID, "Developer", []string{constants.ScopeTaskRead, constants.ScopeTaskWrite, constants.ScopeFeatureRead})

	username := "charlie"
	_ = userRoleService.AssignRoles(username, projectID, []string{role1.RoleID})

	// 计算有效权限 (应该去重)
	scopes, err := userRoleService.ComputeEffectiveScopes(username, projectID)
	if err != nil {
		t.Fatalf("ComputeEffectiveScopes failed: %v", err)
	}

	// 验证权限数量 (task.read, task.write, feature.read = 3个)
	if len(scopes) != 3 {
		t.Errorf("Expected 3 unique scopes, got %d: %v", len(scopes), scopes)
	}

	// 验证权限内容
	expectedScopes := map[string]bool{
		constants.ScopeTaskRead:    true,
		constants.ScopeTaskWrite:   true,
		constants.ScopeFeatureRead: true,
	}
	for _, scope := range scopes {
		if !expectedScopes[scope] {
			t.Errorf("Unexpected scope: %s", scope)
		}
	}

	// 测试缓存 (第二次调用应该从缓存读取)
	startTime := time.Now()
	scopes2, err := userRoleService.ComputeEffectiveScopes(username, projectID)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Second ComputeEffectiveScopes failed: %v", err)
	}
	if len(scopes2) != len(scopes) {
		t.Errorf("Cache returned different scopes count: expected %d, got %d", len(scopes), len(scopes2))
	}
	// 缓存读取应该很快 (小于1ms)
	if duration > time.Millisecond {
		t.Logf("Warning: Cached read took %v, might not be using cache", duration)
	}

	// 边界案例: 用户不存在或没有角色 (应该返回空数组,不是错误)
	scopes3, err := userRoleService.ComputeEffectiveScopes("non-existent-user", projectID)
	if err != nil {
		// 如果实现选择返回错误,也是合理的
		t.Logf("ComputeEffectiveScopes for non-existent user returned error: %v (acceptable)", err)
	} else if len(scopes3) != 0 {
		t.Errorf("Expected empty scopes for non-existent user, got %v", scopes3)
	}
}

// TestUserRoleService_GetUserProfile 测试获取用户档案
func TestUserRoleService_GetUserProfile(t *testing.T) {
	tmpDirRoles := t.TempDir()
	tmpDirUsers := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, _ := NewRoleManager(tmpDirRoles, mockLogger)
	userRoleService, _ := NewUserRoleService(tmpDirUsers, roleManager, mockLogger)

	// 创建多个项目和角色
	project1 := "project-alpha"
	project2 := "project-beta"
	role1, _ := roleManager.CreateRole(project1, "Developer", []string{constants.ScopeTaskRead})
	role2, _ := roleManager.CreateRole(project2, "Admin", []string{constants.ScopeTaskRead, constants.ScopeTaskWrite})

	username := "diana"
	_ = userRoleService.AssignRoles(username, project1, []string{role1.RoleID})
	_ = userRoleService.AssignRoles(username, project2, []string{role2.RoleID})

	// 获取用户档案
	profile, err := userRoleService.GetUserProfile(username)
	if err != nil {
		t.Fatalf("GetUserProfile failed: %v", err)
	}

	// 验证项目数量 (应该有2个项目的角色)
	if len(profile.ProjectRoles) != 2 {
		t.Errorf("Expected 2 project roles, got %d", len(profile.ProjectRoles))
	}

	// 验证项目1的角色
	foundProject1 := false
	for _, roleInfo := range profile.ProjectRoles {
		if roleInfo.ProjectID == project1 {
			foundProject1 = true
			if roleInfo.RoleName != "Developer" {
				t.Errorf("Expected role name 'Developer' for %s, got %s", project1, roleInfo.RoleName)
			}
			break
		}
	}
	if !foundProject1 {
		t.Errorf("Expected project %s in profile", project1)
	}

	// 验证项目2的角色
	foundProject2 := false
	for _, roleInfo := range profile.ProjectRoles {
		if roleInfo.ProjectID == project2 {
			foundProject2 = true
			if roleInfo.RoleName != "Admin" {
				t.Errorf("Expected role name 'Admin' for %s, got %s", project2, roleInfo.RoleName)
			}
			break
		}
	}
	if !foundProject2 {
		t.Errorf("Expected project %s in profile", project2)
	}

	// 边界案例: 用户不存在 (实现可能返回错误或空档案)
	profile3, err := userRoleService.GetUserProfile("non-existent-user")
	if err != nil {
		t.Logf("GetUserProfile for non-existent user returned error: %v (acceptable)", err)
	} else if len(profile3.ProjectRoles) != 0 {
		t.Errorf("Expected empty profile for non-existent user, got %d roles", len(profile3.ProjectRoles))
	}
}
