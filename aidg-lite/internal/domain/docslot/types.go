package docslot

import "time"

// DocumentScope 定义文档作用域类型
type DocumentScope string

const (
	ScopeTask    DocumentScope = "task"
	ScopeProject DocumentScope = "project"
)

var ValidScopes = []DocumentScope{ScopeTask, ScopeProject}

func IsValidScope(scope DocumentScope) bool {
	for _, s := range ValidScopes {
		if s == scope {
			return true
		}
	}
	return false
}

var SlotConfig = map[DocumentScope][]string{
	ScopeTask:    {"requirements", "design", "test"},
	ScopeProject: {"feature_list", "architecture_design"},
}

func IsValidSlot(scope DocumentScope, slotKey string) bool {
	slots, ok := SlotConfig[scope]
	if !ok {
		return false
	}
	for _, s := range slots {
		if s == slotKey {
			return true
		}
	}
	return false
}

// AppendResult 追加操作结果
type AppendResult struct {
	Version   int       `json:"version"`
	ETag      string    `json:"etag"`
	Duplicate bool      `json:"duplicate"`
	Sequence  int       `json:"sequence"`
	Timestamp time.Time `json:"timestamp"`
}

// ExportResult 导出文档结果
type ExportResult struct {
	Content   string    `json:"content"`
	Version   int       `json:"version"`
	ETag      string    `json:"etag"`
	UpdatedAt time.Time `json:"updated_at"`
	Exists    bool      `json:"exists"`
}
