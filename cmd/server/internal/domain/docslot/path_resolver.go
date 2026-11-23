package docslot

import (
	"os"
	"path/filepath"
	"strings"
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
	return &pathResolverImpl{basePath: filepath.Clean(basePath)}
}

// BasePath 返回配置的基础路径
func (r *pathResolverImpl) BasePath() string {
	return r.basePath
}

// ResolvePath 解析项目或会议文档路径
func (r *pathResolverImpl) ResolvePath(scope DocumentScope, scopeID, slotKey string) (string, error) {
	if err := r.ValidateScope(scope, slotKey); err != nil {
		return "", err
	}
	if !safeResourceID(scopeID) {
		return "", &DocSlotError{Code: ErrCodeInvalidScope, Message: "invalid resource identifier", Scope: scope, ScopeID: scopeID, Err: ErrPathTraversal}
	}

	switch scope {
	case ScopeProject:
		return containedPath(r.basePath, filepath.Join(r.basePath, scopeID, "docs", slotKey))
	case ScopeMeeting:
		meetingsPath := filepath.Join(filepath.Dir(r.basePath), "meetings")
		return containedPath(meetingsPath, filepath.Join(meetingsPath, scopeID, "docs", slotKey))
	case ScopeTask:
		return "", ErrUseTaskPath
	default:
		return "", NewInvalidScopeError(scope)
	}
}

// ResolveTaskPath 解析任务文档路径
func (r *pathResolverImpl) ResolveTaskPath(projectID, taskID, slotKey string) (string, error) {
	if err := r.ValidateScope(ScopeTask, slotKey); err != nil {
		return "", err
	}
	if !safeResourceID(projectID) || !safeResourceID(taskID) {
		return "", &DocSlotError{Code: ErrCodeInvalidScope, Message: "invalid project or task identifier", Scope: ScopeTask, ScopeID: taskID, SlotKey: slotKey, Err: ErrPathTraversal}
	}
	return containedPath(r.basePath, filepath.Join(r.basePath, projectID, "tasks", taskID, "docs", slotKey))
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

func safeResourceID(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && len(id) <= 200 && id != "." && id != ".." && filepath.Base(id) == id && !strings.ContainsAny(id, "/\\\x00")
}

func containedPath(root, target string) (string, error) {
	originalTarget := filepath.Clean(target)
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := resolveExistingPath(root)
	if err != nil {
		return "", err
	}
	resolvedTarget, err := resolveExistingPath(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", &DocSlotError{Code: ErrCodeInvalidScope, Message: "resolved path escapes configured data root", Err: ErrPathTraversal}
	}
	return originalTarget, nil
}

func resolveExistingPath(path string) (string, error) {
	current := filepath.Clean(path)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path), nil
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
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
