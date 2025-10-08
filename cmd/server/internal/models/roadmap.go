package models

import "time"

// Roadmap 项目路线图
type Roadmap struct {
	Version   int            `json:"version"`
	UpdatedAt time.Time      `json:"updated_at"`
	Nodes     []*RoadmapNode `json:"nodes"`
}

// RoadmapNode 路线图节点
type RoadmapNode struct {
	ID          string    `json:"id"`
	Date        string    `json:"date"` // YYYY-MM-DD format
	Goal        string    `json:"goal"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // completed, in-progress, todo
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RoadmapNodeCreate 创建路线图节点的请求
type RoadmapNodeCreate struct {
	Date        string `json:"date" binding:"required"`
	Goal        string `json:"goal" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status" binding:"required"`
}

// RoadmapNodeUpdate 更新路线图节点的请求
type RoadmapNodeUpdate struct {
	Date        *string `json:"date"`
	Goal        *string `json:"goal"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}
