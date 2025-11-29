package shared

import (
	"fmt"
	"strings"
)

// SlotConfig 槽位配置
type SlotConfig struct {
	Key           string   // 槽位键名，如 "requirements"
	DisplayName   string   // 显示名称，如 "需求文档"
	Description   string   // 描述信息
	PathPattern   string   // API 路径模板，如 "/api/v1/projects/{project_id}/tasks/{task_id}/requirements"
	SupportedOps  []string // 支持的操作：GET, PUT, POST
	ContentType   string   // 内容类型：markdown, json
	UseUnifiedAPI bool     // 是否使用统一文档API（新文档模式）
}

// SlotRegistry 槽位注册表
type SlotRegistry struct {
	taskSlots    map[string]*SlotConfig
	meetingSlots map[string]*SlotConfig
	projectSlots map[string]*SlotConfig
}

// NewSlotRegistry 创建槽位注册表并初始化所有槽位
func NewSlotRegistry() *SlotRegistry {
	r := &SlotRegistry{
		taskSlots:    make(map[string]*SlotConfig),
		meetingSlots: make(map[string]*SlotConfig),
		projectSlots: make(map[string]*SlotConfig),
	}
	r.initTaskSlots()
	r.initMeetingSlots()
	r.initProjectSlots()
	return r
}

// initTaskSlots 定义 requirements, design, test 三个任务文档槽位配置
func (r *SlotRegistry) initTaskSlots() {
	r.taskSlots = map[string]*SlotConfig{
		"requirements": {
			Key:          "requirements",
			DisplayName:  "需求文档",
			Description:  "项目任务需求的详细描述",
			PathPattern:  "/api/v1/projects/{project_id}/tasks/{task_id}/requirements",
			SupportedOps: []string{"GET", "PUT"},
			ContentType:  "markdown",
		},
		"design": {
			Key:          "design",
			DisplayName:  "设计文档",
			Description:  "项目任务的技术设计方案",
			PathPattern:  "/api/v1/projects/{project_id}/tasks/{task_id}/design",
			SupportedOps: []string{"GET", "PUT"},
			ContentType:  "markdown",
		},
		"test": {
			Key:          "test",
			DisplayName:  "测试文档",
			Description:  "项目任务的测试计划和用例",
			PathPattern:  "/api/v1/projects/{project_id}/tasks/{task_id}/test",
			SupportedOps: []string{"GET", "PUT"},
			ContentType:  "markdown",
		},
	}
}

// initMeetingSlots 定义 meeting_info, polish, context, summary, topic, merged_all, polish_all, feature_list, architecture_design 九个会议文档槽位配置
func (r *SlotRegistry) initMeetingSlots() {
	r.meetingSlots = map[string]*SlotConfig{
		"meeting_info": {
			Key:          "meeting_info",
			DisplayName:  "会议基本信息",
			Description:  "会议的元数据和基本信息",
			PathPattern:  "/api/v1/tasks/{meeting_id}",
			SupportedOps: []string{"GET"},
			ContentType:  "json",
		},
		"polish": {
			Key:          "polish",
			DisplayName:  "会议详细记录",
			Description:  "会议的详细润色记录",
			PathPattern:  "/api/v1/tasks/{meeting_id}/polish",
			SupportedOps: []string{"GET"},
			ContentType:  "markdown",
		},
		"context": {
			Key:          "context",
			DisplayName:  "会议背景",
			Description:  "会议的背景信息和上下文",
			PathPattern:  "/api/v1/tasks/{meeting_id}/meeting-context",
			SupportedOps: []string{"GET"},
			ContentType:  "markdown",
		},
		"summary": {
			Key:           "summary",
			DisplayName:   "会议总结",
			Description:   "会议的结构化总结",
			PathPattern:   "/api/v1/meetings/{meeting_id}/docs/summary",
			SupportedOps:  []string{"GET", "PUT"},
			ContentType:   "markdown",
			UseUnifiedAPI: true,
		},
		"topic": {
			Key:           "topic",
			DisplayName:   "讨论话题",
			Description:   "会议讨论的主要话题",
			PathPattern:   "/api/v1/meetings/{meeting_id}/docs/topic",
			SupportedOps:  []string{"GET", "PUT"},
			ContentType:   "markdown",
			UseUnifiedAPI: true,
		},
		"merged_all": {
			Key:          "merged_all",
			DisplayName:  "原始转录合并",
			Description:  "会议的原始转录文本合并",
			PathPattern:  "/api/v1/tasks/{meeting_id}/merged_all",
			SupportedOps: []string{"GET"},
			ContentType:  "markdown",
		},
		"polish_all": {
			Key:           "polish_all",
			DisplayName:   "润色合成记录",
			Description:   "会议的完整润色合成记录",
			PathPattern:   "/api/v1/meetings/{meeting_id}/docs/polish",
			SupportedOps:  []string{"GET", "PUT"},
			ContentType:   "markdown",
			UseUnifiedAPI: true,
		},
		"feature_list": {
			Key:          "feature_list",
			DisplayName:  "特性列表",
			Description:  "会议讨论的特性列表",
			PathPattern:  "/api/v1/tasks/{meeting_id}/feature-list",
			SupportedOps: []string{"GET", "PUT"},
			ContentType:  "markdown",
		},
		"architecture_design": {
			Key:          "architecture_design",
			DisplayName:  "架构设计",
			Description:  "会议产出的架构设计文档",
			PathPattern:  "/api/v1/tasks/{meeting_id}/architecture-design",
			SupportedOps: []string{"GET", "PUT"},
			ContentType:  "markdown",
		},
	}
}

// initProjectSlots 定义 feature_list, architecture_design 两个项目文档槽位配置
func (r *SlotRegistry) initProjectSlots() {
	r.projectSlots = map[string]*SlotConfig{
		"feature_list": {
			Key:          "feature_list",
			DisplayName:  "项目特性列表",
			Description:  "项目的特性列表（支持 json 和 markdown 格式）",
			PathPattern:  "/api/v1/projects/{project_id}/feature-list",
			SupportedOps: []string{"GET", "PUT"},
			ContentType:  "markdown", // 默认，可通过 format 参数覆盖
		},
		"architecture_design": {
			Key:          "architecture_design",
			DisplayName:  "项目架构设计",
			Description:  "项目的架构设计文档",
			PathPattern:  "/api/v1/projects/{project_id}/architecture-design",
			SupportedOps: []string{"GET", "PUT"},
			ContentType:  "markdown",
		},
	}
}

// ValidateTaskSlot 验证任务文档槽位，无效时返回包含有效槽位列表的友好错误
func (r *SlotRegistry) ValidateTaskSlot(slotKey string) error {
	if _, exists := r.taskSlots[slotKey]; !exists {
		validKeys := make([]string, 0, len(r.taskSlots))
		for key := range r.taskSlots {
			validKeys = append(validKeys, key)
		}
		return fmt.Errorf("无效的任务文档槽位 '%s'。有效的槽位包括：%v", slotKey, validKeys)
	}
	return nil
}

// ValidateMeetingSlot 验证会议文档槽位有效性
func (r *SlotRegistry) ValidateMeetingSlot(slotKey string) error {
	if _, exists := r.meetingSlots[slotKey]; !exists {
		validKeys := make([]string, 0, len(r.meetingSlots))
		for key := range r.meetingSlots {
			validKeys = append(validKeys, key)
		}
		return fmt.Errorf("无效的会议文档槽位 '%s'。有效的槽位包括：%v", slotKey, validKeys)
	}
	return nil
}

// ValidateProjectSlot 验证项目文档槽位和格式参数有效性（feature_list 支持 json/markdown，architecture_design 仅支持 markdown）
func (r *SlotRegistry) ValidateProjectSlot(slotKey, format string) error {
	_, exists := r.projectSlots[slotKey]
	if !exists {
		validKeys := make([]string, 0, len(r.projectSlots))
		for key := range r.projectSlots {
			validKeys = append(validKeys, key)
		}
		return fmt.Errorf("无效的项目文档槽位 '%s'。有效的槽位包括：%v", slotKey, validKeys)
	}

	// 验证格式参数
	if format == "" {
		format = "markdown" // 默认格式
	}

	if format != "json" && format != "markdown" {
		return fmt.Errorf("无效的格式参数 '%s'。有效的格式包括：[json, markdown]", format)
	}

	// 特殊规则：architecture_design 仅支持 markdown
	if slotKey == "architecture_design" && format == "json" {
		return fmt.Errorf("槽位 '%s' 仅支持 markdown 格式", slotKey)
	}

	return nil
}

// GetTaskAPIPath 根据槽位和操作类型返回 API 路径，替换路径模板中的占位符
func (r *SlotRegistry) GetTaskAPIPath(slotKey, operation, projectID, taskID string) (string, error) {
	slot, exists := r.taskSlots[slotKey]
	if !exists {
		return "", fmt.Errorf("槽位 '%s' 不存在", slotKey)
	}

	// 验证操作是否支持
	if !r.isSupportedOp(slot.SupportedOps, operation) {
		return "", fmt.Errorf("槽位 '%s' 不支持操作 '%s'", slotKey, operation)
	}

	// 替换路径模板中的占位符
	path := strings.ReplaceAll(slot.PathPattern, "{project_id}", projectID)
	path = strings.ReplaceAll(path, "{task_id}", taskID)

	return path, nil
}

// GetMeetingSlotConfig 返回会议文档槽位配置
func (r *SlotRegistry) GetMeetingSlotConfig(slotKey string) (*SlotConfig, error) {
	slot, exists := r.meetingSlots[slotKey]
	if !exists {
		return nil, fmt.Errorf("槽位 '%s' 不存在", slotKey)
	}
	return slot, nil
}

// GetMeetingAPIPath 返回会议文档的 API 路径
// 对于使用统一文档API的槽位，会根据操作类型添加相应后缀（/export 或 /append）
func (r *SlotRegistry) GetMeetingAPIPath(slotKey, operation, meetingID string) (string, error) {
	slot, exists := r.meetingSlots[slotKey]
	if !exists {
		return "", fmt.Errorf("槽位 '%s' 不存在", slotKey)
	}

	// 验证操作是否支持
	if !r.isSupportedOp(slot.SupportedOps, operation) {
		return "", fmt.Errorf("槽位 '%s' 不支持操作 '%s'", slotKey, operation)
	}

	// 替换路径模板中的占位符
	path := strings.ReplaceAll(slot.PathPattern, "{meeting_id}", meetingID)

	// 如果是统一文档API，根据操作类型添加后缀
	if slot.UseUnifiedAPI {
		switch operation {
		case "GET":
			path += "/export"
		case "PUT", "POST":
			path += "/append"
		}
	}

	return path, nil
}

// GetProjectAPIPath 返回项目文档的 API 路径，根据 format 参数决定路径后缀（.json 或无后缀）
func (r *SlotRegistry) GetProjectAPIPath(slotKey, operation, projectID, format string) (string, error) {
	slot, exists := r.projectSlots[slotKey]
	if !exists {
		return "", fmt.Errorf("槽位 '%s' 不存在", slotKey)
	}

	// 验证操作是否支持
	if !r.isSupportedOp(slot.SupportedOps, operation) {
		return "", fmt.Errorf("槽位 '%s' 不支持操作 '%s'", slotKey, operation)
	}

	// 替换路径模板中的占位符
	path := strings.ReplaceAll(slot.PathPattern, "{project_id}", projectID)

	// 根据 format 参数决定路径后缀
	if format == "json" && slotKey == "feature_list" {
		path += ".json"
	}

	return path, nil
}

// isSupportedOp 检查操作是否在支持的操作列表中
func (r *SlotRegistry) isSupportedOp(supportedOps []string, operation string) bool {
	for _, op := range supportedOps {
		if op == operation {
			return true
		}
	}
	return false
}
