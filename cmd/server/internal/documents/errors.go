package documents

import (
	"errors"
)

// 错误定义
var (
	ErrNodeNotFound         = errors.New("DOC_NODE_NOT_FOUND")
	ErrCircularDependency   = errors.New("CIRCULAR_DEPENDENCY")
	ErrInvalidHierarchy     = errors.New("INVALID_HIERARCHY")
	ErrVersionMismatch      = errors.New("VERSION_MISMATCH")
	ErrHierarchyOverflow    = errors.New("HIERARCHY_OVERFLOW")
	ErrDuplicateRelation    = errors.New("relationship already exists")
	ErrRelationNotFound     = errors.New("relationship not found")
	ErrChildrenLimitReached = errors.New("CHILDREN_LIMIT_REACHED")
)
