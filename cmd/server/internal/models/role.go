package models

import (
	"time"
)

// Role 角色模型 - 项目级别的角色抽象
type Role struct {
	RoleID    string    `json:"role_id"`    // 角色唯一标识
	ProjectID string    `json:"project_id"` // 所属项目 ID
	Name      string    `json:"name"`       // 角色名称 (在项目内唯一)
	Scopes    []string  `json:"scopes"`     // 权限范围列表 (如: ["task.read", "task.write"])
	CreatedAt time.Time `json:"created_at"` // 创建时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// RoleSummary 角色摘要 - 用于列表展示
type RoleSummary struct {
	RoleID    string   `json:"role_id"`    // 角色 ID
	ProjectID string   `json:"project_id"` // 项目 ID
	Name      string   `json:"name"`       // 角色名称
	Scopes    []string `json:"scopes"`     // 权限范围
}

// ToSummary 转换为摘要格式
func (r *Role) ToSummary() RoleSummary {
	return RoleSummary{
		RoleID:    r.RoleID,
		ProjectID: r.ProjectID,
		Name:      r.Name,
		Scopes:    r.Scopes,
	}
}
