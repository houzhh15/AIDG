package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/houzhh15/AIDG/cmd/server/internal/audit"
	"github.com/houzhh15/AIDG/cmd/server/internal/models"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// UserRoleService 用户角色服务接口
type UserRoleService interface {
	// 为用户分配角色
	AssignRoles(username, projectID string, roleIDs []string) error
	// 撤销用户在项目中的所有角色
	RevokeRoles(username, projectID string) error
	// 移除用户的单个角色
	RemoveRole(username, projectID, roleID string) error
	// 获取用户在项目中的角色列表
	GetUserRoles(username, projectID string) ([]models.UserRoleInfo, error)
	// 获取项目的所有用户角色映射
	GetProjectUserRoles(projectID string) ([]models.UserRoleInfo, error)
	// 计算用户在项目中的有效权限 scopes
	ComputeEffectiveScopes(username, projectID string) ([]string, error)
	// 获取用户的所有项目角色信息
	GetUserProfile(username string) (*models.UserProfileData, error)
}

// userRoleService 用户角色服务实现
type userRoleService struct {
	basePath    string
	roleManager RoleManager
	auditLogger audit.AuditLogger
	mu          sync.RWMutex
	// 权限缓存 (username + projectID -> scopes)
	permissionCache map[string]*permissionCacheEntry
	cacheMu         sync.RWMutex
}

// permissionCacheEntry 权限缓存条目
type permissionCacheEntry struct {
	scopes    []string
	expiresAt time.Time
}

const (
	cacheTTL = 5 * time.Minute // 缓存 5 分钟
)

// NewUserRoleService 创建用户角色服务实例
func NewUserRoleService(basePath string, roleManager RoleManager, auditLogger audit.AuditLogger) (UserRoleService, error) {
	// 确保基础目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}
	return &userRoleService{
		basePath:        basePath,
		roleManager:     roleManager,
		auditLogger:     auditLogger,
		permissionCache: make(map[string]*permissionCacheEntry),
	}, nil
}

// AssignRoles 为用户分配角色
func (urs *userRoleService) AssignRoles(username, projectID string, roleIDs []string) error {
	urs.mu.Lock()
	defer urs.mu.Unlock()

	// 1. 验证角色是否存在
	for _, roleID := range roleIDs {
		if !urs.roleManager.RoleExists(projectID, roleID) {
			return fmt.Errorf("role %s not found in project %s", roleID, projectID)
		}
	}

	// 2. 读取或创建用户角色映射
	mapping, err := urs.getUserRolesMappingUnsafe(username)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to get user roles mapping: %w", err)
	}
	if mapping == nil {
		mapping = &models.UserRolesMapping{
			Username: username,
			Projects: make(map[string]models.ProjectRoleInfo),
		}
	}

	// 3. 更新角色信息 (取第一个 roleID,设计文档中暂时只支持单角色)
	if len(roleIDs) > 0 {
		roleID := roleIDs[0]
		role, err := urs.roleManager.GetRole(projectID, roleID)
		if err != nil {
			return fmt.Errorf("failed to get role: %w", err)
		}

		mapping.Projects[projectID] = models.ProjectRoleInfo{
			RoleID:     roleID,
			RoleName:   role.Name,
			AssignedAt: time.Now(),
		}
		mapping.UpdatedAt = time.Now()
	}

	// 4. 保存映射
	if err := urs.saveUserRolesMappingUnsafe(mapping); err != nil {
		return fmt.Errorf("failed to save user roles mapping: %w", err)
	}

	// 5. 清除缓存
	urs.invalidateCache(username, projectID)

	// 6. 记录审计日志
	if urs.auditLogger != nil {
		_ = urs.auditLogger.LogActionSimple("system", audit.ActionAssignRole, username,
			fmt.Sprintf("Assigned %d role(s) to user '%s' in project '%s'", len(roleIDs), username, projectID))
	}

	return nil
}

// RevokeRoles 撤销用户在项目中的所有角色
func (urs *userRoleService) RevokeRoles(username, projectID string) error {
	urs.mu.Lock()
	defer urs.mu.Unlock()

	// 1. 读取用户角色映射
	mapping, err := urs.getUserRolesMappingUnsafe(username)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // 用户没有角色,直接返回
		}
		return fmt.Errorf("failed to get user roles mapping: %w", err)
	}

	// 2. 删除项目的角色信息
	delete(mapping.Projects, projectID)
	mapping.UpdatedAt = time.Now()

	// 3. 保存映射
	if err := urs.saveUserRolesMappingUnsafe(mapping); err != nil {
		return fmt.Errorf("failed to save user roles mapping: %w", err)
	}

	// 4. 清除缓存
	urs.invalidateCache(username, projectID)

	// 5. 记录审计日志
	if urs.auditLogger != nil {
		_ = urs.auditLogger.LogActionSimple("system", audit.ActionRevokeRole, username,
			fmt.Sprintf("Revoked all roles from user '%s' in project '%s'", username, projectID))
	}

	return nil
}

// ComputeEffectiveScopes 计算用户在项目中的有效权限 scopes
func (urs *userRoleService) ComputeEffectiveScopes(username, projectID string) ([]string, error) {
	// 1. 检查缓存
	cacheKey := username + ":" + projectID
	urs.cacheMu.RLock()
	if entry, exists := urs.permissionCache[cacheKey]; exists {
		if time.Now().Before(entry.expiresAt) {
			scopes := entry.scopes
			urs.cacheMu.RUnlock()
			return scopes, nil
		}
	}
	urs.cacheMu.RUnlock()

	// 2. 计算权限
	urs.mu.RLock()
	mapping, err := urs.getUserRolesMappingUnsafe(username)
	urs.mu.RUnlock()

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to get user roles mapping: %w", err)
	}

	projectInfo, exists := mapping.Projects[projectID]
	if !exists {
		return []string{}, nil
	}

	// 3. 获取角色的 scopes
	role, err := urs.roleManager.GetRole(projectID, projectInfo.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	scopes := role.Scopes

	// 4. 更新缓存
	urs.cacheMu.Lock()
	urs.permissionCache[cacheKey] = &permissionCacheEntry{
		scopes:    scopes,
		expiresAt: time.Now().Add(cacheTTL),
	}
	urs.cacheMu.Unlock()

	return scopes, nil
}

// GetUserProfile 获取用户的所有项目角色信息
func (urs *userRoleService) GetUserProfile(username string) (*models.UserProfileData, error) {
	urs.mu.RLock()
	defer urs.mu.RUnlock()

	// 1. 读取用户角色映射
	mapping, err := urs.getUserRolesMappingUnsafe(username)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &models.UserProfileData{
				Username:     username,
				ProjectRoles: []models.UserRoleInfo{},
			}, nil
		}
		return nil, fmt.Errorf("failed to get user roles mapping: %w", err)
	}

	// 2. 构建用户资料
	var projectRoles []models.UserRoleInfo
	for projectID, projectInfo := range mapping.Projects {
		role, err := urs.roleManager.GetRole(projectID, projectInfo.RoleID)
		if err != nil {
			// 如果角色不存在,跳过
			continue
		}

		projectRoles = append(projectRoles, models.UserRoleInfo{
			Username:   username,
			ProjectID:  projectID,
			RoleID:     projectInfo.RoleID,
			RoleName:   projectInfo.RoleName,
			Scopes:     role.Scopes,
			AssignedAt: projectInfo.AssignedAt,
		})
	}

	return &models.UserProfileData{
		Username:     username,
		ProjectRoles: projectRoles,
	}, nil
}

// ========== 内部辅助方法 ==========

// getUserRolesMappingUnsafe 不加锁地获取用户角色映射 (内部使用)
func (urs *userRoleService) getUserRolesMappingUnsafe(username string) (*models.UserRolesMapping, error) {
	userRolePath := filepath.Join(urs.basePath, username+".json")
	data, err := os.ReadFile(userRolePath)
	if err != nil {
		return nil, err
	}

	var mapping models.UserRolesMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user roles mapping: %w", err)
	}

	return &mapping, nil
}

// saveUserRolesMappingUnsafe 不加锁地保存用户角色映射 (内部使用)
func (urs *userRoleService) saveUserRolesMappingUnsafe(mapping *models.UserRolesMapping) error {
	userRolePath := filepath.Join(urs.basePath, mapping.Username+".json")
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user roles mapping: %w", err)
	}

	if err := os.WriteFile(userRolePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write user roles mapping file: %w", err)
	}

	return nil
}

// invalidateCache 清除缓存
func (urs *userRoleService) invalidateCache(username, projectID string) {
	cacheKey := username + ":" + projectID
	urs.cacheMu.Lock()
	delete(urs.permissionCache, cacheKey)
	urs.cacheMu.Unlock()
}

// RemoveRole 移除用户的单个角色
func (urs *userRoleService) RemoveRole(username, projectID, roleID string) error {
	urs.mu.Lock()
	defer urs.mu.Unlock()

	mapping, err := urs.getUserRolesMappingUnsafe(username)
	if err != nil {
		return fmt.Errorf("failed to get user roles mapping: %w", err)
	}

	// 检查是否存在该项目的角色
	projectRole, exists := mapping.Projects[projectID]
	if !exists || projectRole.RoleID != roleID {
		return fmt.Errorf("role not found for user %s in project %s", username, projectID)
	}

	// 移除角色
	delete(mapping.Projects, projectID)
	mapping.UpdatedAt = time.Now()

	// 保存映射
	if err := urs.saveUserRolesMappingUnsafe(mapping); err != nil {
		return err
	}

	// 清除缓存
	urs.invalidateCache(username, projectID)

	// 审计日志
	details := fmt.Sprintf("Removed role %s from user %s in project %s", roleID, username, projectID)
	_ = urs.auditLogger.LogActionSimple(username, audit.ActionRevokeRole, roleID, details)

	return nil
}

// GetUserRoles 获取用户在项目中的角色列表
func (urs *userRoleService) GetUserRoles(username, projectID string) ([]models.UserRoleInfo, error) {
	urs.mu.RLock()
	defer urs.mu.RUnlock()

	mapping, err := urs.getUserRolesMappingUnsafe(username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles mapping: %w", err)
	}

	projectRole, exists := mapping.Projects[projectID]
	if !exists {
		return []models.UserRoleInfo{}, nil
	}

	// 获取角色详情
	role, err := urs.roleManager.GetRole(projectID, projectRole.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role details: %w", err)
	}

	return []models.UserRoleInfo{
		{
			Username:   username,
			ProjectID:  projectID,
			RoleID:     role.RoleID,
			RoleName:   role.Name,
			Scopes:     role.Scopes,
			AssignedAt: projectRole.AssignedAt,
		},
	}, nil
}

// GetProjectUserRoles 获取项目的所有用户角色映射
func (urs *userRoleService) GetProjectUserRoles(projectID string) ([]models.UserRoleInfo, error) {
	urs.mu.RLock()
	defer urs.mu.RUnlock()

	var result []models.UserRoleInfo

	// 遍历所有用户角色映射文件
	entries, err := os.ReadDir(urs.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("failed to read user roles directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		username := strings.TrimSuffix(entry.Name(), ".json")
		mapping, err := urs.getUserRolesMappingUnsafe(username)
		if err != nil {
			continue // 跳过无法读取的文件
		}

		projectRole, exists := mapping.Projects[projectID]
		if !exists {
			continue
		}

		// 获取角色详情
		role, err := urs.roleManager.GetRole(projectID, projectRole.RoleID)
		if err != nil {
			continue // 跳过角色不存在的情况
		}

		result = append(result, models.UserRoleInfo{
			Username:   username,
			ProjectID:  projectID,
			RoleID:     role.RoleID,
			RoleName:   role.Name,
			Scopes:     role.Scopes,
			AssignedAt: projectRole.AssignedAt,
		})
	}

	return result, nil
}
