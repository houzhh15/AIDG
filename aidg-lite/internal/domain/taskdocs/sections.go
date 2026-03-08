package taskdocs

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// Section 表示文档的一个章节
type Section struct {
	ID       string   `json:"id"`        // 章节唯一标识，如 "section_001"
	Title    string   `json:"title"`     // 章节标题（含 Markdown 标记），如 "## 1. 项目概述"
	Level    int      `json:"level"`     // 标题等级（1-6），对应 Markdown 的 # 数量
	Order    int      `json:"order"`     // 章节顺序（全局序号）
	ParentID *string  `json:"parent_id"` // 父章节 ID，根章节为 nil
	File     string   `json:"file"`      // 章节文件名，如 "section_001.md"
	Children []string `json:"children"`  // 子章节 ID 列表
	Hash     string   `json:"hash"`      // 内容 SHA256 哈希（用于检测变更）
}

// SectionMeta 存储章节元数据（对应 sections.json）
type SectionMeta struct {
	Version   int       `json:"version"`    // 版本号（每次修改递增）
	UpdatedAt time.Time `json:"updated_at"` // 最后更新时间
	RootLevel int       `json:"root_level"` // 根章节的标题等级
	Sections  []Section `json:"sections"`   // 所有章节列表（按 order 排序）
	ETag      string    `json:"etag"`       // 用于并发控制
}

// SectionContent 表示章节的完整内容
type SectionContent struct {
	Section
	Content         string           `json:"content"`                    // 章节 Markdown 内容
	ChildrenContent []SectionContent `json:"children_content,omitempty"` // 子章节内容（可选）
}

// hashContent 计算内容的 SHA256 哈希值（取前 16 字节）
func hashContent(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("sha256:%x", h.Sum(nil)[:16])
}

// generateETag 根据章节列表生成 ETag
func generateETag(sections []Section) string {
	h := sha256.New()
	for _, s := range sections {
		h.Write([]byte(s.ID + s.Hash))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
