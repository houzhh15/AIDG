// Package docslot 提供统一文档服务抽象层
//
// 该包封装了项目文档、会议文档和任务文档的统一操作接口，
// 底层复用 taskdocs 包的核心逻辑。
package docslot

import "time"

// DocumentScope 定义文档作用域类型
type DocumentScope string

const (
	// ScopeTask 任务文档作用域
	// 存储路径: data/projects/{projectID}/tasks/{taskID}/docs/{slotKey}/
	ScopeTask DocumentScope = "task"

	// ScopeProject 项目文档作用域
	// 存储路径: data/projects/{projectID}/docs/{slotKey}/
	ScopeProject DocumentScope = "project"

	// ScopeMeeting 会议文档作用域
	// 存储路径: data/meetings/{meetingID}/docs/{slotKey}/
	ScopeMeeting DocumentScope = "meeting"
)

// ValidScopes 有效的作用域列表
var ValidScopes = []DocumentScope{ScopeTask, ScopeProject, ScopeMeeting}

// IsValidScope 检查作用域是否有效
func IsValidScope(scope DocumentScope) bool {
	for _, s := range ValidScopes {
		if s == scope {
			return true
		}
	}
	return false
}

// SlotConfig 定义各作用域支持的槽位配置
var SlotConfig = map[DocumentScope][]string{
	ScopeTask:    {"requirements", "design", "test"},
	ScopeProject: {"feature_list", "architecture_design"},
	ScopeMeeting: {"polish", "summary", "topic"},
}

// ScopeConfigItem 槽位配置结构体
type ScopeConfigItem struct {
	Scope        DocumentScope `json:"scope"`         // 作用域类型
	SlotKey      string        `json:"slot_key"`      // 槽位键名
	DisplayName  string        `json:"display_name"`  // 显示名称
	BasePath     string        `json:"base_path"`     // 基础路径模板
	SupportedOps []string      `json:"supported_ops"` // 支持的操作: GET, POST, PUT, DELETE
}

// AllSlotConfigs 所有槽位配置
var AllSlotConfigs = []ScopeConfigItem{
	// 任务文档槽位
	{ScopeTask, "requirements", "需求文档", "data/projects/{projectID}/tasks/{taskID}/docs/requirements", []string{"GET", "POST", "PUT", "DELETE"}},
	{ScopeTask, "design", "设计文档", "data/projects/{projectID}/tasks/{taskID}/docs/design", []string{"GET", "POST", "PUT", "DELETE"}},
	{ScopeTask, "test", "测试文档", "data/projects/{projectID}/tasks/{taskID}/docs/test", []string{"GET", "POST", "PUT", "DELETE"}},
	// 项目文档槽位
	{ScopeProject, "feature_list", "特性列表", "data/projects/{projectID}/docs/feature_list", []string{"GET", "POST", "PUT"}},
	{ScopeProject, "architecture_design", "架构设计", "data/projects/{projectID}/docs/architecture_design", []string{"GET", "POST", "PUT"}},
	// 会议文档槽位
	{ScopeMeeting, "polish", "会议详情", "data/meetings/{meetingID}/docs/polish", []string{"GET", "POST", "PUT"}},
	{ScopeMeeting, "summary", "会议总结", "data/meetings/{meetingID}/docs/summary", []string{"GET", "POST", "PUT"}},
	{ScopeMeeting, "topic", "会议话题", "data/meetings/{meetingID}/docs/topic", []string{"GET", "POST", "PUT"}},
}

// GetScopeConfigItem 获取指定作用域和槽位的配置
func GetScopeConfigItem(scope DocumentScope, slotKey string) *ScopeConfigItem {
	for _, cfg := range AllSlotConfigs {
		if cfg.Scope == scope && cfg.SlotKey == slotKey {
			return &cfg
		}
	}
	return nil
}

// IsValidSlot 检查槽位对于指定作用域是否有效
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

// AppendRequest 追加文档请求
type AppendRequest struct {
	Content         string `json:"content"`          // 文档内容
	Op              string `json:"op"`               // 操作类型: add_full, replace_full
	ExpectedVersion *int   `json:"expected_version"` // 期望版本号（乐观锁）
	User            string `json:"user"`             // 操作用户
	Source          string `json:"source"`           // 来源: ui, mcp, api, migration
}

// AppendResult 追加操作结果
type AppendResult struct {
	Version   int       `json:"version"`   // 新版本号
	ETag      string    `json:"etag"`      // 内容哈希
	Duplicate bool      `json:"duplicate"` // 是否重复内容
	Sequence  int       `json:"sequence"`  // chunk 序列号
	Timestamp time.Time `json:"timestamp"` // 操作时间
}

// ExportResult 导出文档结果
type ExportResult struct {
	Content   string    `json:"content"`    // 完整内容
	Version   int       `json:"version"`    // 当前版本
	ETag      string    `json:"etag"`       // 内容哈希
	UpdatedAt time.Time `json:"updated_at"` // 最后更新时间
	Exists    bool      `json:"exists"`     // 文档是否存在
}

// SectionInfo 章节信息
type SectionInfo struct {
	ID        string        `json:"id"`         // 章节 ID
	Title     string        `json:"title"`      // 章节标题
	Level     int           `json:"level"`      // 标题级别 (1-6)
	ParentID  string        `json:"parent_id"`  // 父章节 ID
	LineStart int           `json:"line_start"` // 起始行号
	LineEnd   int           `json:"line_end"`   // 结束行号
	Version   int           `json:"version"`    // 版本号
	Children  []SectionInfo `json:"children"`   // 子章节
}

// SectionContent 章节内容
type SectionContent struct {
	SectionInfo
	Content  string `json:"content"`  // 章节 Markdown 内容
	Baseline string `json:"baseline"` // 基线内容（用于 diff）
}

// UpdateSectionRequest 更新章节请求
type UpdateSectionRequest struct {
	Content         string `json:"content"`          // 新内容
	ExpectedVersion int    `json:"expected_version"` // 期望版本号
}

// InsertSectionRequest 插入章节请求
type InsertSectionRequest struct {
	Title          string  `json:"title"`            // 章节标题（含 # 标记）
	Content        string  `json:"content"`          // 章节内容
	AfterSectionID *string `json:"after_section_id"` // 插入位置（在此章节后）
}

// DeleteSectionRequest 删除章节请求
type DeleteSectionRequest struct {
	Cascade         bool `json:"cascade"`          // 是否级联删除子章节
	ExpectedVersion int  `json:"expected_version"` // 期望版本号
}
