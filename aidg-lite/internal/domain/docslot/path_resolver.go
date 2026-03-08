package docslot

import "path/filepath"

// PathResolver 路径解析器接口
type PathResolver interface {
	ResolvePath(scope DocumentScope, scopeID, slotKey string) (string, error)
	ResolveTaskPath(projectID, taskID, slotKey string) (string, error)
	ValidateScope(scope DocumentScope, slotKey string) error
}

type pathResolverImpl struct {
	basePath string // 项目根目录，如 data/projects
}

func NewPathResolver(basePath string) PathResolver {
	return &pathResolverImpl{basePath: basePath}
}

func (r *pathResolverImpl) ResolvePath(scope DocumentScope, scopeID, slotKey string) (string, error) {
	if err := r.ValidateScope(scope, slotKey); err != nil {
		return "", err
	}
	switch scope {
	case ScopeProject:
		return filepath.Join(r.basePath, scopeID, "docs", slotKey), nil
	case ScopeTask:
		return "", ErrUseTaskPath
	default:
		return "", NewInvalidScopeError(scope)
	}
}

func (r *pathResolverImpl) ResolveTaskPath(projectID, taskID, slotKey string) (string, error) {
	if err := r.ValidateScope(ScopeTask, slotKey); err != nil {
		return "", err
	}
	return filepath.Join(r.basePath, projectID, "tasks", taskID, "docs", slotKey), nil
}

func (r *pathResolverImpl) ValidateScope(scope DocumentScope, slotKey string) error {
	if !IsValidScope(scope) {
		return NewInvalidScopeError(scope)
	}
	if !IsValidSlot(scope, slotKey) {
		return NewInvalidSlotError(scope, slotKey)
	}
	return nil
}
