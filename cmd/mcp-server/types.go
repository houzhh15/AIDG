package main

import (
	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// types.go - 类型定义
// 从 main.go 提取的所有请求和响应结构体定义

const (
	DEFAULT_SERVER_BASE_URL = shared.DEFAULT_SERVER_BASE_URL
)

// 请求结构体定义
type ProductLineRequest struct {
	ProductLine string `json:"product_line" description:"产品线名称" required:"true"`
}

type DateRangeRequest struct {
	StartDate string `json:"start_date" description:"开始日期 (YYYY-MM-DD 格式)" required:"true"`
	EndDate   string `json:"end_date" description:"结束日期 (YYYY-MM-DD 格式)" required:"true"`
}

type ProductAndDateRequest struct {
	ProductLine string `json:"product_line" description:"产品线名称" required:"true"`
	StartDate   string `json:"start_date" description:"开始日期 (YYYY-MM-DD 格式)" required:"true"`
	EndDate     string `json:"end_date" description:"结束日期 (YYYY-MM-DD 格式)" required:"true"`
}

// 项目任务相关的结构体
type TaskIDRequest struct {
	TaskID string `json:"task_id" description:"任务ID" required:"true"`
}

type UpdateContentRequest struct {
	TaskID  string `json:"task_id" description:"任务ID" required:"true"`
	Content string `json:"content" description:"内容" required:"true"`
}

// 会议相关的结构体（使用meeting_id参数名）
type MeetingIDRequest struct {
	MeetingID string `json:"meeting_id" description:"会议任务ID" required:"true"`
}

type UpdateMeetingContentRequest struct {
	MeetingID string `json:"meeting_id" description:"会议任务ID" required:"true"`
	Content   string `json:"content" description:"内容" required:"true"`
}

type ChunkPolishPlanRequest struct {
	MeetingID      string `json:"meeting_id" description:"会议任务ID" required:"true"`
	MaxChunkChars  int    `json:"max_chunk_chars" description:"单个chunk最大字符数(默认8000)"`
	IncludeContext bool   `json:"include_context" description:"每个chunk提示中包含会议背景"`
	OverlapChars   int    `json:"overlap_chars" description:"相邻chunk重叠字符数(默认0)"`
}

type ProjectIDRequest struct {
	ProjectID string `json:"project_id" description:"项目ID" required:"true"`
}

type UpdateProjectContentRequest struct {
	ProjectID string `json:"project_id" description:"项目ID" required:"true"`
	Content   string `json:"content" description:"内容" required:"true"`
}

// HTTP处理器结构
type MCPHandler struct {
	apiClient       *shared.APIClient
	registry        *ToolRegistry
	PromptManager   *PromptManager   // Prompts 模版管理器（公共访问）
	NotificationHub *NotificationHub // 通知中心（公共访问）
}

// MCP请求结构
type MCPRequest struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}
