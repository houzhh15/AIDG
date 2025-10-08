package models

import "time"

// YearProgress 年度进展结构
type YearProgress struct {
	Year     int        `json:"year"`
	Quarters []*Quarter `json:"quarters"`
}

// Quarter 季度进展结构
type Quarter struct {
	Quarter       string   `json:"quarter"`        // Q1, Q2, Q3, Q4
	QuarterNumber int      `json:"quarter_number"` // 1-4 (前端兼容字段)
	Summary       string   `json:"summary"`        // Markdown内容
	Months        []*Month `json:"months"`
}

// Month 月度进展结构
type Month struct {
	Month       string  `json:"month"`        // 01-12
	MonthNumber int     `json:"month_number"` // 1-12 (前端兼容字段)
	Name        string  `json:"name"`         // "2025年1月"
	Summary     string  `json:"summary"`      // Markdown内容
	Weeks       []*Week `json:"weeks"`
}

// Week 周进展结构
type Week struct {
	WeekNumber    string `json:"week_number"`     // YYYY-WW格式 (例如 "2025-05")
	WeekNumberInt int    `json:"week_number_int"` // 周编号整数 (前端兼容字段)
	Range         string `json:"range"`           // "01/29-02/04"格式
	Summary       string `json:"summary"`         // Markdown内容
}

// WeekProgress 单周完整进展（包含上下文）
type WeekProgress struct {
	Year       int      `json:"year"`
	WeekNumber string   `json:"week_number"`
	WeekRange  string   `json:"week_range"`
	Quarter    *Quarter `json:"quarter"`
	Month      *Month   `json:"month"`
	Week       *Week    `json:"week"`
}

// ProgressMetadata 进展元数据（用于版本管理）
type ProgressMetadata struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}
