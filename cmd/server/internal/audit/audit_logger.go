package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditAction 审计日志操作类型
type AuditAction string

const (
	ActionCreateRole     AuditAction = "create_role"
	ActionUpdateRole     AuditAction = "update_role"
	ActionDeleteRole     AuditAction = "delete_role"
	ActionAssignRole     AuditAction = "assign_role"
	ActionRevokeRole     AuditAction = "revoke_role"
	ActionChangePassword AuditAction = "change_password"
)

// AuditEntry 审计日志条目
type AuditEntry struct {
	Timestamp  time.Time   `json:"timestamp"`
	Operator   string      `json:"operator"`          // 操作者用户名
	Action     AuditAction `json:"action"`            // 操作类型
	ResourceID string      `json:"resource_id"`       // 资源标识 (role_id, username, task_id等)
	Before     interface{} `json:"before,omitempty"`  // 操作前状态 (JSON对象)
	After      interface{} `json:"after,omitempty"`   // 操作后状态 (JSON对象)
	Details    string      `json:"details,omitempty"` // 额外详情
}

// AuditLogger 审计日志记录器接口
type AuditLogger interface {
	// LogAction 记录审计日志
	LogAction(operator string, action AuditAction, resourceID string, before, after interface{}, details string) error

	// LogActionSimple 记录简单审计日志 (不包含before/after)
	LogActionSimple(operator string, action AuditAction, resourceID string, details string) error
}

// FileAuditLogger 基于文件的审计日志实现
type FileAuditLogger struct {
	baseDir string // 审计日志根目录 (例如: audit_logs/)
	mu      sync.Mutex
}

// NewFileAuditLogger 创建文件审计日志记录器
func NewFileAuditLogger(baseDir string) (*FileAuditLogger, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit logs directory: %w", err)
	}
	return &FileAuditLogger{
		baseDir: baseDir,
	}, nil
}

// LogAction 记录审计日志到 JSONL 文件 (按日期分组)
func (f *FileAuditLogger) LogAction(operator string, action AuditAction, resourceID string, before, after interface{}, details string) error {
	entry := AuditEntry{
		Timestamp:  time.Now(),
		Operator:   operator,
		Action:     action,
		ResourceID: resourceID,
		Before:     before,
		After:      after,
		Details:    details,
	}

	return f.writeEntry(entry)
}

// LogActionSimple 记录简单审计日志
func (f *FileAuditLogger) LogActionSimple(operator string, action AuditAction, resourceID string, details string) error {
	return f.LogAction(operator, action, resourceID, nil, nil, details)
}

// writeEntry 将审计条目写入文件 (按 {year}/{month}/{day}.jsonl 分组)
func (f *FileAuditLogger) writeEntry(entry AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 构造文件路径: audit_logs/{year}/{month}/{day}.jsonl
	year := entry.Timestamp.Format("2006")
	month := entry.Timestamp.Format("01")
	day := entry.Timestamp.Format("02")

	dirPath := filepath.Join(f.baseDir, year, month)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create audit log directory: %w", err)
	}

	filePath := filepath.Join(dirPath, fmt.Sprintf("%s.jsonl", day))

	// 序列化为JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	// 追加写入文件 (JSONL格式,每行一条记录)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}

	return nil
}

// GetAuditLogs 读取指定日期范围的审计日志 (可选功能,用于查询)
func (f *FileAuditLogger) GetAuditLogs(startDate, endDate time.Time) ([]AuditEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var entries []AuditEntry

	// 遍历日期范围内的所有文件
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		year := d.Format("2006")
		month := d.Format("01")
		day := d.Format("02")

		filePath := filepath.Join(f.baseDir, year, month, fmt.Sprintf("%s.jsonl", day))

		// 文件不存在则跳过
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		// 读取文件内容
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read audit log file %s: %w", filePath, err)
		}

		// 逐行解析JSONL
		lines := string(data)
		for i, line := range splitLines(lines) {
			if line == "" {
				continue
			}
			var entry AuditEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				return nil, fmt.Errorf("failed to unmarshal audit entry at %s:%d: %w", filePath, i+1, err)
			}
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// splitLines 按换行符分割字符串 (辅助函数)
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
