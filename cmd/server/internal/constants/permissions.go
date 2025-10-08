package constants

// Permission Scopes - 权限常量定义
const (
	// Feature Management - 特性管理
	ScopeFeatureRead  = "feature.read"
	ScopeFeatureWrite = "feature.write"

	// Task Management - 任务管理
	ScopeTaskRead  = "task.read"
	ScopeTaskWrite = "task.write"

	// Project Document - 项目文档 (新增)
	ScopeProjectDocRead  = "project.doc.read"  // 项目文档读取
	ScopeProjectDocWrite = "project.doc.write" // 项目文档编辑

	// Task Planning - 任务计划 (新增)
	ScopeTaskPlanApprove = "task.plan.approve" // 任务计划审批

	// Meeting Management - 会议管理 (新增)
	ScopeMeetingRead  = "meeting.read"  // 会议读取
	ScopeMeetingWrite = "meeting.write" // 会议编辑
)

// PermissionGroups - 权限分组配置 (供前端使用)
var PermissionGroups = map[string][]string{
	"项目文档": {
		ScopeProjectDocRead,
		ScopeProjectDocWrite,
	},
	"任务管理": {
		ScopeTaskRead,
		ScopeTaskWrite,
		ScopeTaskPlanApprove,
	},
	"会议管理": {
		ScopeMeetingRead,
		ScopeMeetingWrite,
	},
	"特性管理": {
		ScopeFeatureRead,
		ScopeFeatureWrite,
	},
}

// ValidScopes - 所有合法的 scope 集合 (用于校验)
var ValidScopes = []string{
	ScopeFeatureRead,
	ScopeFeatureWrite,
	ScopeTaskRead,
	ScopeTaskWrite,
	ScopeProjectDocRead,
	ScopeProjectDocWrite,
	ScopeTaskPlanApprove,
	ScopeMeetingRead,
	ScopeMeetingWrite,
}

// IsValidScope 检查给定的 scope 是否合法
func IsValidScope(scope string) bool {
	for _, validScope := range ValidScopes {
		if scope == validScope {
			return true
		}
	}
	return false
}

// Error Codes - 错误码定义
const (
	// Role Related Errors - 角色相关错误
	ErrCodeRoleNotFound      = "ROLE_NOT_FOUND"      // 角色不存在
	ErrCodeRoleInUse         = "ROLE_IN_USE"         // 角色正在使用中,无法删除
	ErrCodeDuplicateRoleName = "DUPLICATE_ROLE_NAME" // 角色名称重复
	ErrCodeInvalidScope      = "INVALID_SCOPE"       // 无效的权限 scope

	// User Related Errors - 用户相关错误
	ErrCodeUserNotFound = "USER_NOT_FOUND" // 用户不存在

	// Permission Related Errors - 权限相关错误
	ErrCodePermissionDenied = "PERMISSION_DENIED" // 权限不足
)

// Audit Log Actions - 审计日志动作类型
const (
	AuditActionCreateRole     = "create_role"     // 创建角色
	AuditActionUpdateRole     = "update_role"     // 更新角色
	AuditActionDeleteRole     = "delete_role"     // 删除角色
	AuditActionAssignRole     = "assign_role"     // 指派角色
	AuditActionRevokeRole     = "revoke_role"     // 撤销角色
	AuditActionChangePassword = "change_password" // 修改密码
)

// Log Prefixes - 日志前缀
const (
	LogPrefixRole     = "[ROLE]"      // 角色操作日志
	LogPrefixUserRole = "[USER_ROLE]" // 用户角色操作日志
	LogPrefixAuth     = "[AUTH]"      // 权限校验日志
	LogPrefixSecurity = "[SECURITY]"  // 安全相关日志
)
