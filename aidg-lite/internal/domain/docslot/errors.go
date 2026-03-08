package docslot

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidScope    = errors.New("invalid document scope")
	ErrInvalidSlot     = errors.New("invalid document slot")
	ErrDocNotFound     = errors.New("document not found")
	ErrSectionNotFound = errors.New("section not found")
	ErrVersionMismatch = errors.New("version mismatch")
	ErrUseTaskPath     = errors.New("task scope requires ResolveTaskPath method")
)

type DocSlotError struct {
	Code    string
	Message string
	Scope   DocumentScope
	SlotKey string
	ScopeID string
	Err     error
}

func (e *DocSlotError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] scope=%s slot=%s id=%s: %s (%v)",
			e.Code, e.Scope, e.SlotKey, e.ScopeID, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] scope=%s slot=%s id=%s: %s",
		e.Code, e.Scope, e.SlotKey, e.ScopeID, e.Message)
}

func (e *DocSlotError) Unwrap() error { return e.Err }

const (
	ErrCodeInvalidScope    = "INVALID_SCOPE"
	ErrCodeInvalidSlot     = "INVALID_SLOT"
	ErrCodeDocNotFound     = "DOC_NOT_FOUND"
	ErrCodeSectionNotFound = "SECTION_NOT_FOUND"
	ErrCodeVersionMismatch = "VERSION_MISMATCH"
	ErrCodeInternalError   = "INTERNAL_ERROR"
)

func NewInvalidScopeError(scope DocumentScope) *DocSlotError {
	return &DocSlotError{Code: ErrCodeInvalidScope, Message: fmt.Sprintf("scope '%s' is not valid", scope), Scope: scope, Err: ErrInvalidScope}
}

func NewInvalidSlotError(scope DocumentScope, slotKey string) *DocSlotError {
	return &DocSlotError{Code: ErrCodeInvalidSlot, Message: fmt.Sprintf("slot '%s' is not valid for scope '%s'", slotKey, scope), Scope: scope, SlotKey: slotKey, Err: ErrInvalidSlot}
}

func WrapError(scope DocumentScope, scopeID, slotKey string, err error) *DocSlotError {
	return &DocSlotError{Code: ErrCodeInternalError, Message: err.Error(), Scope: scope, SlotKey: slotKey, ScopeID: scopeID, Err: err}
}

func IsVersionMismatch(err error) bool { return errors.Is(err, ErrVersionMismatch) }
func IsDocNotFound(err error) bool     { return errors.Is(err, ErrDocNotFound) }
func IsSectionNotFound(err error) bool { return errors.Is(err, ErrSectionNotFound) }
