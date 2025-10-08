package models

import "time"

// TaskSummaries 任务总结集合，包含版本管理
type TaskSummaries struct {
	Version   int            `json:"version"`
	UpdatedAt time.Time      `json:"updated_at"`
	Summaries []*TaskSummary `json:"summaries"`
}

// TaskSummary 单条任务总结记录
type TaskSummary struct {
	ID         string    `json:"id"`          // 总结ID，格式: summary_{timestamp}
	Time       time.Time `json:"time"`        // 总结时间
	WeekNumber string    `json:"week_number"` // 自动计算的周编号，格式: YYYY-WW
	Content    string    `json:"content"`     // Markdown格式的总结内容
	Creator    string    `json:"creator"`     // 创建人
	CreatedAt  time.Time `json:"created_at"`  // 创建时间
	UpdatedAt  time.Time `json:"updated_at"`  // 最后更新时间
}

// TaskSummaryUpdate 用于更新任务总结的请求结构
type TaskSummaryUpdate struct {
	Time    *time.Time `json:"time,omitempty"`
	Content *string    `json:"content,omitempty"`
}
