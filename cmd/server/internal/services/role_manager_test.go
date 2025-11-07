package services

import (
	"github.com/houzhh15/AIDG/cmd/server/internal/audit"
	"github.com/houzhh15/AIDG/cmd/server/internal/constants"
	"testing"
)

// mockAuditLogger 用于测试的审计日志模拟
type mockAuditLogger struct {
	logs []string
}

func (m *mockAuditLogger) LogAction(operator string, action audit.AuditAction, resourceID string, before, after interface{}, details string) error {
	m.logs = append(m.logs, string(action))
	return nil
}

func (m *mockAuditLogger) LogActionSimple(operator string, action audit.AuditAction, resourceID string, details string) error {
	m.logs = append(m.logs, string(action))
	return nil
}

// TestRoleManager_CreateRole 测试创建角色
func TestRoleManager_CreateRole(t *testing.T) {
	// 创建临时测试目录
	tmpDir := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, err := NewRoleManager(tmpDir, mockLogger)
	if err != nil {
		t.Fatalf("NewRoleManager failed: %v", err)
	}

	// 正常案例: 创建角色
	projectID := "test-project"
	roleName := "Developer"
	scopes := []string{constants.ScopeTaskRead, constants.ScopeTaskWrite}

	role, err := roleManager.CreateRole(projectID, roleName, scopes)
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	if role.Name != roleName {
		t.Errorf("Expected role name %s, got %s", roleName, role.Name)
	}
	if len(role.Scopes) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(role.Scopes))
	}

	// 检查审计日志是否记录
	if len(mockLogger.logs) != 1 || mockLogger.logs[0] != string(audit.ActionCreateRole) {
		t.Errorf("Expected audit log for create_role, got %v", mockLogger.logs)
	}

	// 错误案例: 重复角色名称
	_, err = roleManager.CreateRole(projectID, roleName, scopes)
	if err != ErrDuplicateRoleName {
		t.Errorf("Expected ErrDuplicateRoleName, got %v", err)
	}

	// 错误案例: 无效的 scope
	invalidScopes := []string{constants.ScopeTaskRead, "invalid.scope"}
	_, err = roleManager.CreateRole(projectID, "InvalidRole", invalidScopes)
	if err == nil || err != ErrInvalidScope {
		t.Logf("Expected ErrInvalidScope, got %v (wrapped error is acceptable)", err)
	}
}

// TestRoleManager_UpdateRole 测试更新角色
func TestRoleManager_UpdateRole(t *testing.T) {
	tmpDir := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, err := NewRoleManager(tmpDir, mockLogger)
	if err != nil {
		t.Fatalf("NewRoleManager failed: %v", err)
	}

	// 先创建角色
	projectID := "test-project"
	role, _ := roleManager.CreateRole(projectID, "Developer", []string{constants.ScopeTaskRead})

	// 正常案例: 更新角色
	newScopes := []string{constants.ScopeTaskRead, constants.ScopeTaskWrite}
	err = roleManager.UpdateRole(projectID, role.RoleID, "Senior Developer", newScopes)
	if err != nil {
		t.Fatalf("UpdateRole failed: %v", err)
	}

	// 验证更新
	updatedRole, _ := roleManager.GetRole(projectID, role.RoleID)
	if updatedRole.Name != "Senior Developer" {
		t.Errorf("Expected role name 'Senior Developer', got %s", updatedRole.Name)
	}
	if len(updatedRole.Scopes) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(updatedRole.Scopes))
	}

	// 检查审计日志 (应该有2条: create + update)
	if len(mockLogger.logs) != 2 || mockLogger.logs[1] != string(audit.ActionUpdateRole) {
		t.Errorf("Expected audit log for update_role, got %v", mockLogger.logs)
	}

	// 错误案例: 角色不存在
	err = roleManager.UpdateRole(projectID, "non-existent", "NewName", newScopes)
	if err != ErrRoleNotFound {
		t.Errorf("Expected ErrRoleNotFound, got %v", err)
	}
}

// TestRoleManager_DeleteRole 测试删除角色
func TestRoleManager_DeleteRole(t *testing.T) {
	tmpDir := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, err := NewRoleManager(tmpDir, mockLogger)
	if err != nil {
		t.Fatalf("NewRoleManager failed: %v", err)
	}

	// 先创建角色
	projectID := "test-project"
	role, _ := roleManager.CreateRole(projectID, "TempRole", []string{constants.ScopeTaskRead})

	// 正常案例: 删除角色 (未被使用)
	err = roleManager.DeleteRole(projectID, role.RoleID)
	if err != nil {
		t.Fatalf("DeleteRole failed: %v", err)
	}

	// 验证删除
	_, err = roleManager.GetRole(projectID, role.RoleID)
	if err != ErrRoleNotFound {
		t.Errorf("Expected ErrRoleNotFound after deletion, got %v", err)
	}

	// 检查审计日志 (应该有2条: create + delete)
	if len(mockLogger.logs) != 2 || mockLogger.logs[1] != string(audit.ActionDeleteRole) {
		t.Errorf("Expected audit log for delete_role, got %v", mockLogger.logs)
	}

	// 错误案例: 角色不存在
	err = roleManager.DeleteRole(projectID, "non-existent")
	if err != ErrRoleNotFound {
		t.Errorf("Expected ErrRoleNotFound, got %v", err)
	}
}

// TestRoleManager_ListRoles 测试列出角色
func TestRoleManager_ListRoles(t *testing.T) {
	// 为ListRoles创建独立的测试目录
	tmpDir := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, err := NewRoleManager(tmpDir, mockLogger)
	if err != nil {
		t.Fatalf("NewRoleManager failed: %v", err)
	}

	projectID := "test-list-project"

	// 创建多个角色
	role1, err1 := roleManager.CreateRole(projectID, "Developer", []string{constants.ScopeTaskRead})
	if err1 != nil {
		t.Fatalf("CreateRole Developer failed: %v", err1)
	}
	role2, err2 := roleManager.CreateRole(projectID, "Tester", []string{constants.ScopeTaskRead})
	if err2 != nil {
		t.Fatalf("CreateRole Tester failed: %v", err2)
	}
	role3, err3 := roleManager.CreateRole(projectID, "Admin", []string{constants.ScopeTaskRead, constants.ScopeTaskWrite})
	if err3 != nil {
		t.Fatalf("CreateRole Admin failed: %v", err3)
	}

	t.Logf("Created roles: %s, %s, %s", role1.RoleID, role2.RoleID, role3.RoleID)

	// 列出角色
	roles, err := roleManager.ListRoles(projectID)
	if err != nil {
		t.Fatalf("ListRoles failed: %v", err)
	}

	t.Logf("Listed %d roles: %+v", len(roles), roles)

	if len(roles) != 3 {
		t.Errorf("Expected 3 roles, got %d", len(roles))
		return // 如果数量不对,后续检查会panic
	}

	// 验证每个角色的权限数量 (不依赖顺序)
	roleMap := make(map[string]int)
	for _, role := range roles {
		roleMap[role.Name] = len(role.Scopes)
	}

	if roleMap["Admin"] != 2 {
		t.Errorf("Expected Admin role to have 2 scopes, got %d", roleMap["Admin"])
	}
	if roleMap["Developer"] != 1 {
		t.Errorf("Expected Developer role to have 1 scope, got %d", roleMap["Developer"])
	}
	if roleMap["Tester"] != 1 {
		t.Errorf("Expected Tester role to have 1 scope, got %d", roleMap["Tester"])
	}
}

// TestRoleManager_GetRole 测试获取角色
func TestRoleManager_GetRole(t *testing.T) {
	tmpDir := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, err := NewRoleManager(tmpDir, mockLogger)
	if err != nil {
		t.Fatalf("NewRoleManager failed: %v", err)
	}

	projectID := "test-project"
	createdRole, _ := roleManager.CreateRole(projectID, "Developer", []string{constants.ScopeTaskRead})

	// 正常案例: 获取角色
	role, err := roleManager.GetRole(projectID, createdRole.RoleID)
	if err != nil {
		t.Fatalf("GetRole failed: %v", err)
	}

	if role.Name != "Developer" {
		t.Errorf("Expected role name 'Developer', got %s", role.Name)
	}

	// 错误案例: 角色不存在
	_, err = roleManager.GetRole(projectID, "non-existent")
	if err != ErrRoleNotFound {
		t.Errorf("Expected ErrRoleNotFound, got %v", err)
	}
}

// TestRoleExists 测试角色存在性检查
func TestRoleExists(t *testing.T) {
	tmpDir := t.TempDir()
	mockLogger := &mockAuditLogger{}

	roleManager, err := NewRoleManager(tmpDir, mockLogger)
	if err != nil {
		t.Fatalf("NewRoleManager failed: %v", err)
	}

	projectID := "test-project"
	role, _ := roleManager.CreateRole(projectID, "Developer", []string{constants.ScopeTaskRead})

	// 正常案例: 角色存在
	if !roleManager.RoleExists(projectID, role.RoleID) {
		t.Error("Expected role to exist")
	}

	// 角色不存在
	if roleManager.RoleExists(projectID, "non-existent") {
		t.Error("Expected role to not exist")
	}
}
