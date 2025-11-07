package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/houzhh15/AIDG/cmd/server/internal/audit"
	"github.com/houzhh15/AIDG/cmd/server/internal/constants"
	"github.com/houzhh15/AIDG/cmd/server/internal/models"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RoleManager 角色管理接口
type RoleManager interface {
	// 创建角色
	CreateRole(projectID, name string, scopes []string) (*models.Role, error)
	// 获取角色
	GetRole(projectID, roleID string) (*models.Role, error)
	// 列出项目的所有角色
	ListRoles(projectID string) ([]models.RoleSummary, error)
	// 更新角色
	UpdateRole(projectID, roleID, name string, scopes []string) error
	// 删除角色 (检查是否被使用)
	DeleteRole(projectID, roleID string) error
	// 检查角色是否存在
	RoleExists(projectID, roleID string) bool
}

// 错误定义
var (
	ErrRoleNotFound      = errors.New("role not found")
	ErrDuplicateRoleName = errors.New("role name already exists in project")
	ErrInvalidScope      = errors.New("invalid permission scope")
	ErrRoleInUse         = errors.New("role is currently in use and cannot be deleted")
)

// roleManager 角色管理器实现
type roleManager struct {
	basePath    string
	auditLogger audit.AuditLogger
	mu          sync.RWMutex
}

// NewRoleManager 创建角色管理器实例
func NewRoleManager(basePath string, auditLogger audit.AuditLogger) (RoleManager, error) {
	// 确保基础目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}
	return &roleManager{
		basePath:    basePath,
		auditLogger: auditLogger,
	}, nil
}

// CreateRole 创建新角色
func (rm *roleManager) CreateRole(projectID, name string, scopes []string) (*models.Role, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 1. 验证 scopes
	for _, scope := range scopes {
		if !constants.IsValidScope(scope) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidScope, scope)
		}
	}

	// 2. 检查角色名称是否重复
	projectDir := filepath.Join(rm.basePath, projectID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", err)
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read project directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		existingRolePath := filepath.Join(projectDir, entry.Name())
		data, err := os.ReadFile(existingRolePath)
		if err != nil {
			continue
		}
		var existingRole models.Role
		if err := json.Unmarshal(data, &existingRole); err != nil {
			continue
		}
		if existingRole.Name == name {
			return nil, ErrDuplicateRoleName
		}
	}

	// 3. 生成 roleID (使用纳秒精度确保唯一性)
	roleID := fmt.Sprintf("role_%d", time.Now().UnixNano())

	// 4. 创建角色对象
	role := &models.Role{
		RoleID:    roleID,
		ProjectID: projectID,
		Name:      name,
		Scopes:    scopes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 5. 保存到文件
	if err := rm.saveRoleUnsafe(role); err != nil {
		return nil, fmt.Errorf("failed to save role: %w", err)
	}

	// 6. 记录审计日志
	if rm.auditLogger != nil {
		_ = rm.auditLogger.LogAction("system", audit.ActionCreateRole, roleID, nil, role,
			fmt.Sprintf("Created role '%s' in project '%s' with %d scopes", name, projectID, len(scopes)))
	}

	return role, nil
}

// GetRole 获取指定角色
func (rm *roleManager) GetRole(projectID, roleID string) (*models.Role, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rolePath := rm.getRolePath(projectID, roleID)
	data, err := os.ReadFile(rolePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to read role file: %w", err)
	}

	var role models.Role
	if err := json.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("failed to unmarshal role: %w", err)
	}

	return &role, nil
}

// ListRoles 列出项目的所有角色
func (rm *roleManager) ListRoles(projectID string) ([]models.RoleSummary, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	projectDir := filepath.Join(rm.basePath, projectID)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.RoleSummary{}, nil
		}
		return nil, fmt.Errorf("failed to read project directory: %w", err)
	}

	var summaries []models.RoleSummary
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		rolePath := filepath.Join(projectDir, entry.Name())
		data, err := os.ReadFile(rolePath)
		if err != nil {
			continue
		}
		var role models.Role
		if err := json.Unmarshal(data, &role); err != nil {
			continue
		}
		summaries = append(summaries, role.ToSummary())
	}

	return summaries, nil
}

// UpdateRole 更新角色
func (rm *roleManager) UpdateRole(projectID, roleID, name string, scopes []string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 1. 验证 scopes
	for _, scope := range scopes {
		if !constants.IsValidScope(scope) {
			return fmt.Errorf("%w: %s", ErrInvalidScope, scope)
		}
	}

	// 2. 读取现有角色
	role, err := rm.getRoleUnsafe(projectID, roleID)
	if err != nil {
		return err
	}

	// 3. 检查名称冲突 (如果名称被修改)
	if role.Name != name {
		projectDir := filepath.Join(rm.basePath, projectID)
		entries, err := os.ReadDir(projectDir)
		if err != nil {
			return fmt.Errorf("failed to read project directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			existingRolePath := filepath.Join(projectDir, entry.Name())
			data, err := os.ReadFile(existingRolePath)
			if err != nil {
				continue
			}
			var existingRole models.Role
			if err := json.Unmarshal(data, &existingRole); err != nil {
				continue
			}
			if existingRole.RoleID != roleID && existingRole.Name == name {
				return ErrDuplicateRoleName
			}
		}
	}

	// 4. 记录更新前状态 (用于审计)
	beforeState := *role

	// 5. 更新角色
	role.Name = name
	role.Scopes = scopes
	role.UpdatedAt = time.Now()

	// 6. 保存
	if err := rm.saveRoleUnsafe(role); err != nil {
		return fmt.Errorf("failed to save role: %w", err)
	}

	// 7. 记录审计日志
	if rm.auditLogger != nil {
		_ = rm.auditLogger.LogAction("system", audit.ActionUpdateRole, roleID, beforeState, role,
			fmt.Sprintf("Updated role '%s' in project '%s': name='%s', scopes=%d", roleID, projectID, name, len(scopes)))
	}

	return nil
}

// DeleteRole 删除角色 (检查是否被使用)
func (rm *roleManager) DeleteRole(projectID, roleID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 1. 检查角色是否存在
	role, err := rm.getRoleUnsafe(projectID, roleID)
	if err != nil {
		return err
	}

	// 2. 检查角色是否被使用
	inUse, err := rm.checkRoleUsageUnsafe(projectID, roleID)
	if err != nil {
		return fmt.Errorf("failed to check role usage: %w", err)
	}
	if inUse {
		return ErrRoleInUse
	}

	// 3. 删除角色文件
	rolePath := rm.getRolePath(projectID, roleID)
	if err := os.Remove(rolePath); err != nil {
		return fmt.Errorf("failed to remove role file: %w", err)
	}

	// 4. 记录审计日志
	if rm.auditLogger != nil {
		_ = rm.auditLogger.LogAction("system", audit.ActionDeleteRole, roleID, role, nil,
			fmt.Sprintf("Deleted role '%s' (name='%s') from project '%s'", roleID, role.Name, projectID))
	}

	return nil
}

// RoleExists 检查角色是否存在
func (rm *roleManager) RoleExists(projectID, roleID string) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rolePath := rm.getRolePath(projectID, roleID)
	_, err := os.Stat(rolePath)
	return err == nil
}

// ========== 内部辅助方法 ==========

// getRoleUnsafe 不加锁地获取角色 (内部使用)
func (rm *roleManager) getRoleUnsafe(projectID, roleID string) (*models.Role, error) {
	rolePath := rm.getRolePath(projectID, roleID)
	data, err := os.ReadFile(rolePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to read role file: %w", err)
	}

	var role models.Role
	if err := json.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("failed to unmarshal role: %w", err)
	}

	return &role, nil
}

// saveRoleUnsafe 不加锁地保存角色 (内部使用)
func (rm *roleManager) saveRoleUnsafe(role *models.Role) error {
	projectDir := filepath.Join(rm.basePath, role.ProjectID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	rolePath := rm.getRolePath(role.ProjectID, role.RoleID)
	data, err := json.MarshalIndent(role, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal role: %w", err)
	}

	if err := os.WriteFile(rolePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write role file: %w", err)
	}

	return nil
}

// getRolePath 获取角色文件路径
func (rm *roleManager) getRolePath(projectID, roleID string) string {
	return filepath.Join(rm.basePath, projectID, roleID+".json")
}

// checkRoleUsageUnsafe 检查角色是否被使用 (不加锁,内部使用)
func (rm *roleManager) checkRoleUsageUnsafe(projectID, roleID string) (bool, error) {
	// 扫描 user_roles 目录,检查是否有用户使用该角色
	userRolesDir := filepath.Join(rm.basePath, "../user_roles")
	entries, err := os.ReadDir(userRolesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read user_roles directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		userRolePath := filepath.Join(userRolesDir, entry.Name())
		data, err := os.ReadFile(userRolePath)
		if err != nil {
			continue
		}
		var mapping models.UserRolesMapping
		if err := json.Unmarshal(data, &mapping); err != nil {
			continue
		}
		if projectInfo, exists := mapping.Projects[projectID]; exists {
			if projectInfo.RoleID == roleID {
				return true, nil
			}
		}
	}

	return false, nil
}
