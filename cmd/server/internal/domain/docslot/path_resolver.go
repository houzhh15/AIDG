package docslot

import (
	"path/filepath"
)

// PathResolver 路径解析器接口
type PathResolver interface {
	// ResolvePath 解析项目或会议文档路径
	// 对于 ScopeTask 会返回错误，需使用 ResolveTaskPath
	ResolvePath(scope DocumentScope, scopeID, slotKey string) (string, error)

	// ResolveTaskPath 解析任务文档路径
	ResolveTaskPath(projectID, taskID, slotKey string) (string, error)

	// ValidateScope 验证作用域和槽位组合的有效性
	ValidateScope(scope DocumentScope, slotKey string) error

	// BasePath 返回配置的基础路径
	BasePath() string
}

// pathResolverImpl PathResolver 实现
type pathResolverImpl struct {
	basePath string // 数据根目录，如 "data"
}

// NewPathResolver 创建路径解析器
func NewPathResolver(basePath string) PathResolver {
	return &pathResolverImpl{basePath: basePath}
}

// BasePath 返回配置的基础路径
func (r *pathResolverImpl) BasePath() string {
	return r.basePath
}

// ResolvePath 解析项目或会议文档路径
func (r *pathResolverImpl) ResolvePath(scope DocumentScope, scopeID, slotKey string) (string, error) {
	// 验证作用域和槽位
	if err := r.ValidateScope(scope, slotKey); err != nil {
		return "", err
	}

	switch scope {
	case ScopeProject:
		// basePath 是项目根目录（如 data/projects），直接拼接 {projectID}/docs/{slotKey}
		return filepath.Join(r.basePath, scopeID, "docs", slotKey), nil

	case ScopeMeeting:
		// basePath 是项目根目录，会议在同级别的 meetings 目录
		// 需要回退一层：data/projects -> data/meetings
		meetingsPath := filepath.Join(filepath.Dir(r.basePath), "meetings")
		return filepath.Join(meetingsPath, scopeID, "docs", slotKey), nil

	case ScopeTask:
		// 任务作用域需要使用 ResolveTaskPath
		return "", ErrUseTaskPath

	default:
		return "", NewInvalidScopeError(scope)
	}
}

// ResolveTaskPath 解析任务文档路径
func (r *pathResolverImpl) ResolveTaskPath(projectID, taskID, slotKey string) (string, error) {
	// 验证槽位
	if err := r.ValidateScope(ScopeTask, slotKey); err != nil {
		return "", err
	}

	// basePath 是项目根目录（如 data/projects），拼接 {projectID}/tasks/{taskID}/docs/{slotKey}
	return filepath.Join(r.basePath, projectID, "tasks", taskID, "docs", slotKey), nil
}

// ValidateScope 验证作用域和槽位组合的有效性
func (r *pathResolverImpl) ValidateScope(scope DocumentScope, slotKey string) error {
	// 验证作用域
	if !IsValidScope(scope) {
		return NewInvalidScopeError(scope)
	}

	// 验证槽位
	if !IsValidSlot(scope, slotKey) {
		return NewInvalidSlotError(scope, slotKey)
	}

	return nil
}

// ResolveLegacyPath 解析旧版文档路径（用于迁移检测）
// basePath 是项目根目录（如 data/projects）
func ResolveLegacyPath(basePath string, scope DocumentScope, scopeID, slotKey string) string {
	switch scope {
	case ScopeProject:
		// 旧版项目文档路径: {basePath}/{projectID}/{slot}.md
		return filepath.Join(basePath, scopeID, slotKey+".md")

	case ScopeMeeting:
		// 旧版会议文档路径: basePath 回退一层找 meetings
		meetingsPath := filepath.Join(filepath.Dir(basePath), "meetings")
		return filepath.Join(meetingsPath, scopeID, slotKey+".md")

	default:
		return ""
	}
}

// IsNewStructure 检查是否为新结构存储
func IsNewStructure(path string) bool {
	// 新结构路径以 /docs/ 结尾或包含 /docs/ 目录
	return filepath.Base(filepath.Dir(path)) == "docs"
}
