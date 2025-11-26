package docslot

import (
	"errors"
	"fmt"
)

// 预定义错误类型
var (
	// ErrInvalidScope 无效的作用域
	ErrInvalidScope = errors.New("invalid document scope")

	// ErrInvalidSlot 无效的槽位
	ErrInvalidSlot = errors.New("invalid document slot")

	// ErrDocNotFound 文档不存在
	ErrDocNotFound = errors.New("document not found")

	// ErrSectionNotFound 章节不存在
	ErrSectionNotFound = errors.New("section not found")

	// ErrVersionMismatch 版本不匹配
	ErrVersionMismatch = errors.New("version mismatch")

	// ErrMigrationRequired 需要数据迁移
	ErrMigrationRequired = errors.New("migration required")

	// ErrUseTaskPath 需要使用任务路径方法
	ErrUseTaskPath = errors.New("task scope requires ResolveTaskPath method")

	// ErrEmptyContent 内容为空
	ErrEmptyContent = errors.New("content is empty")

	// ErrInvalidOp 无效的操作类型
	ErrInvalidOp = errors.New("invalid operation type")
)

// DocSlotError 文档槽位错误（包含上下文信息）
type DocSlotError struct {
	Code    string        // 错误码
	Message string        // 错误消息
	Scope   DocumentScope // 作用域
	SlotKey string        // 槽位
	ScopeID string        // 作用域 ID
	Err     error         // 原始错误
}

// Error 实现 error 接口
func (e *DocSlotError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] scope=%s slot=%s id=%s: %s (%v)",
			e.Code, e.Scope, e.SlotKey, e.ScopeID, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] scope=%s slot=%s id=%s: %s",
		e.Code, e.Scope, e.SlotKey, e.ScopeID, e.Message)
}

// Unwrap 返回原始错误
func (e *DocSlotError) Unwrap() error {
	return e.Err
}

// 错误码常量
const (
	ErrCodeInvalidScope      = "INVALID_SCOPE"
	ErrCodeInvalidSlot       = "INVALID_SLOT"
	ErrCodeDocNotFound       = "DOC_NOT_FOUND"
	ErrCodeSectionNotFound   = "SECTION_NOT_FOUND"
	ErrCodeVersionMismatch   = "VERSION_MISMATCH"
	ErrCodeMigrationRequired = "MIGRATION_REQUIRED"
	ErrCodeInternalError     = "INTERNAL_ERROR"
	ErrCodeInvalidOp         = "INVALID_OP"
)

// NewInvalidScopeError 创建无效作用域错误
func NewInvalidScopeError(scope DocumentScope) *DocSlotError {
	return &DocSlotError{
		Code:    ErrCodeInvalidScope,
		Message: fmt.Sprintf("scope '%s' is not valid", scope),
		Scope:   scope,
		Err:     ErrInvalidScope,
	}
}

// NewInvalidSlotError 创建无效槽位错误
func NewInvalidSlotError(scope DocumentScope, slotKey string) *DocSlotError {
	return &DocSlotError{
		Code:    ErrCodeInvalidSlot,
		Message: fmt.Sprintf("slot '%s' is not valid for scope '%s'", slotKey, scope),
		Scope:   scope,
		SlotKey: slotKey,
		Err:     ErrInvalidSlot,
	}
}

// NewDocNotFoundError 创建文档不存在错误
func NewDocNotFoundError(scope DocumentScope, scopeID, slotKey string) *DocSlotError {
	return &DocSlotError{
		Code:    ErrCodeDocNotFound,
		Message: "document not found",
		Scope:   scope,
		SlotKey: slotKey,
		ScopeID: scopeID,
		Err:     ErrDocNotFound,
	}
}

// NewSectionNotFoundError 创建章节不存在错误
func NewSectionNotFoundError(scope DocumentScope, scopeID, slotKey, sectionID string) *DocSlotError {
	return &DocSlotError{
		Code:    ErrCodeSectionNotFound,
		Message: fmt.Sprintf("section '%s' not found", sectionID),
		Scope:   scope,
		SlotKey: slotKey,
		ScopeID: scopeID,
		Err:     ErrSectionNotFound,
	}
}

// NewVersionMismatchError 创建版本不匹配错误
func NewVersionMismatchError(scope DocumentScope, scopeID, slotKey string, expected, actual int) *DocSlotError {
	return &DocSlotError{
		Code:    ErrCodeVersionMismatch,
		Message: fmt.Sprintf("expected version %d, but current version is %d", expected, actual),
		Scope:   scope,
		SlotKey: slotKey,
		ScopeID: scopeID,
		Err:     ErrVersionMismatch,
	}
}

// NewMigrationRequiredError 创建需要迁移错误
func NewMigrationRequiredError(scope DocumentScope, scopeID, slotKey string) *DocSlotError {
	return &DocSlotError{
		Code:    ErrCodeMigrationRequired,
		Message: "document exists in legacy format, migration required",
		Scope:   scope,
		SlotKey: slotKey,
		ScopeID: scopeID,
		Err:     ErrMigrationRequired,
	}
}

// WrapError 包装内部错误
func WrapError(scope DocumentScope, scopeID, slotKey string, err error) *DocSlotError {
	return &DocSlotError{
		Code:    ErrCodeInternalError,
		Message: err.Error(),
		Scope:   scope,
		SlotKey: slotKey,
		ScopeID: scopeID,
		Err:     err,
	}
}

// IsVersionMismatch 判断是否为版本不匹配错误
func IsVersionMismatch(err error) bool {
	return errors.Is(err, ErrVersionMismatch)
}

// IsDocNotFound 判断是否为文档不存在错误
func IsDocNotFound(err error) bool {
	return errors.Is(err, ErrDocNotFound)
}

// IsSectionNotFound 判断是否为章节不存在错误
func IsSectionNotFound(err error) bool {
	return errors.Is(err, ErrSectionNotFound)
}

// IsMigrationRequired 判断是否需要数据迁移
func IsMigrationRequired(err error) bool {
	return errors.Is(err, ErrMigrationRequired)
}
