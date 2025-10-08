package documents

import (
	"time"
)

// DocumentType 文档类型枚举
type DocumentType string

const (
	TypeFeatureList  DocumentType = "feature_list"
	TypeArchitecture DocumentType = "architecture"
	TypeTechDesign   DocumentType = "tech_design"
	TypeBackground   DocumentType = "background"
	TypeRequirements DocumentType = "requirements"
	TypeMeeting      DocumentType = "meeting"
	TypeTask         DocumentType = "task"
)

// DocMetaEntry 文档节点元数据
type DocMetaEntry struct {
	ID        string       `json:"id"`
	ParentID  *string      `json:"parent_id"`
	Title     string       `json:"title"`
	Type      DocumentType `json:"type"`
	Level     int          `json:"level"`
	Position  int          `json:"position"`
	Version   int          `json:"version"`
	UpdatedAt time.Time    `json:"updated_at"`
	CreatedAt time.Time    `json:"created_at"`
}

// DocumentTreeDTO 文档树数据传输对象
type DocumentTreeDTO struct {
	Node     *DocMetaEntry      `json:"node"`
	Children []*DocumentTreeDTO `json:"children,omitempty"`
}

// CreateNodeRequest 创建节点请求
type CreateNodeRequest struct {
	ParentID *string      `json:"parent_id"`
	Title    string       `json:"title"`
	Type     DocumentType `json:"type"`
	Content  string       `json:"content"`
}

// MoveNodeRequest 移动节点请求
type MoveNodeRequest struct {
	NewParentID *string `json:"new_parent_id"`
	Position    int     `json:"position"`
}

// UpdateNodeRequest 更新节点请求
type UpdateNodeRequest struct {
	Title *string       `json:"title"`
	Type  *DocumentType `json:"type"`
}

// DocumentsIndex 文档索引结构
type DocumentsIndex struct {
	Documents map[string]*DocMetaEntry `json:"documents"`
	Version   int                      `json:"version"`
	UpdatedAt time.Time                `json:"updated_at"`
}

// 验证相关常量
const (
	MaxLevel           = 5
	MaxChildrenPerNode = 50
)

// ValidateLevel 验证层级深度
func (d *DocMetaEntry) ValidateLevel() error {
	if d.Level > MaxLevel {
		return ErrHierarchyOverflow
	}
	return nil
}

// IsRoot 判断是否为根节点
func (d *DocMetaEntry) IsRoot() bool {
	return d.ParentID == nil
}

// RelationType 关系类型枚举
// 根据设计文档要求，关系分为两类：
// 1. 显式关系（系统自动维护）：parent_child, sibling - 基于文档树结构自动生成和维护
// 2. 隐式关系（用户手动创建）：reference - 用户可以手动创建的依赖关系
type RelationType string

const (
	// RelationParentChild 父子关系 - 系统自动维护，基于文档层级结构
	// 当文档节点有父子层级关系时，系统自动创建和更新此关系
	RelationParentChild RelationType = "parent_child"

	// RelationSibling 兄弟关系 - 系统自动维护，基于文档树结构
	// 当文档节点处于同一层级且有相同父节点时，系统自动创建和更新此关系
	RelationSibling RelationType = "sibling"

	// RelationReference 引用关系 - 用户手动创建
	// 用户可以手动创建的跨文档依赖关系，支持数据、接口、配置三种依赖类型
	RelationReference RelationType = "reference"
)

// DependencyType 依赖类型枚举
// 仅适用于 reference 类型的关系，用于细化依赖关系的语义
type DependencyType string

const (
	// DepTypeData 数据依赖 - 表示文档内容或数据结构的依赖关系
	DepTypeData DependencyType = "data"

	// DepTypeInterface 接口依赖 - 表示API接口或服务契约的依赖关系
	DepTypeInterface DependencyType = "interface"

	// DepTypeConfig 配置依赖 - 表示配置参数或环境设置的依赖关系
	DepTypeConfig DependencyType = "config"
)

// Relationship 文档关系
type Relationship struct {
	ID             string          `json:"id"`
	FromID         string          `json:"from_id"`
	ToID           string          `json:"to_id"`
	Type           RelationType    `json:"type"`
	DependencyType *DependencyType `json:"dependency_type,omitempty"`
	Description    string          `json:"description,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// RelationshipsIndex 关系索引结构
type RelationshipsIndex struct {
	Relationships map[string]*Relationship `json:"relationships"`
	Version       int                      `json:"version"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

// ReferenceStatus 引用状态枚举
type ReferenceStatus string

const (
	RefStatusActive   ReferenceStatus = "active"
	RefStatusOutdated ReferenceStatus = "outdated"
	RefStatusBroken   ReferenceStatus = "broken"
)

// Reference 任务文档引用
type Reference struct {
	ID         string          `json:"id"`
	TaskID     string          `json:"task_id"`
	DocumentID string          `json:"document_id"`
	Anchor     string          `json:"anchor"`
	Context    string          `json:"context"`
	Status     ReferenceStatus `json:"status"`
	Version    int             `json:"version"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// ReferencesIndex 引用索引结构
type ReferencesIndex struct {
	References map[string]*Reference `json:"references"`
	Version    int                   `json:"version"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

// CreateRelationshipRequest 创建关系请求
type CreateRelationshipRequest struct {
	FromID         string          `json:"from_id"`
	ToID           string          `json:"to_id"`
	Type           RelationType    `json:"type"`
	DependencyType *DependencyType `json:"dependency_type,omitempty"`
	Description    string          `json:"description,omitempty"`
}

// CreateReferenceRequest 创建引用请求
type CreateReferenceRequest struct {
	TaskID     string  `json:"task_id"`
	DocumentID string  `json:"document_id"`
	Anchor     *string `json:"anchor,omitempty"`
	Context    *string `json:"context,omitempty"`
}
