package models

import (
	"time"
)

// UserRolesMapping 用户角色映射 - 存储在 user_roles/{username}.json
type UserRolesMapping struct {
	Username  string                     `json:"username"`   // 用户名
	Projects  map[string]ProjectRoleInfo `json:"projects"`   // 项目 ID -> 角色信息
	UpdatedAt time.Time                  `json:"updated_at"` // 最后更新时间
}

// ProjectRoleInfo 项目角色信息
type ProjectRoleInfo struct {
	RoleID     string    `json:"role_id"`     // 角色 ID
	RoleName   string    `json:"role_name"`   // 角色名称 (冗余,便于查询)
	AssignedAt time.Time `json:"assigned_at"` // 指派时间
}

// RoleAssignment 角色指派请求 - 用于 API 接口
type RoleAssignment struct {
	Username  string   `json:"username"`   // 用户名
	ProjectID string   `json:"project_id"` // 项目 ID
	RoleIDs   []string `json:"role_ids"`   // 角色 ID 列表 (支持批量指派)
}

// UserRoleInfo 用户角色信息 - 用于 API 响应
type UserRoleInfo struct {
	Username   string    `json:"username"`    // 用户名
	ProjectID  string    `json:"project_id"`  // 项目 ID
	RoleID     string    `json:"role_id"`     // 角色 ID
	RoleName   string    `json:"role_name"`   // 角色名称
	Scopes     []string  `json:"scopes"`      // 权限范围
	AssignedAt time.Time `json:"assigned_at"` // 指派时间
}

// DefaultPermission 默认权限配置 - 用于项目创建时自动授予创建者的权限
type DefaultPermission struct {
	Scope       string `json:"scope"`       // 权限 scope
	Description string `json:"description"` // 权限描述
}

// UserProfileData 用户资料聚合 - 用于获取用户所有项目的角色信息
type UserProfileData struct {
	Username     string         `json:"username"`      // 用户名
	ProjectRoles []UserRoleInfo `json:"project_roles"` // 用户在各项目中的角色
}
