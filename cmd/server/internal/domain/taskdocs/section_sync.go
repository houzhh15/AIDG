package taskdocs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SyncManager 负责章节与 compiled.md 的同步
type SyncManager struct {
	basePath string
	docType  string
}

// NewSyncManager 创建同步管理器
func NewSyncManager(basePath, docType string) *SyncManager {
	return &SyncManager{
		basePath: basePath,
		docType:  docType,
	}
}

// SyncFromCompiled 从 compiled.md 同步到章节文件
func (sm *SyncManager) SyncFromCompiled() error {
	compiledPath := filepath.Join(sm.basePath, "compiled.md")
	sectionsDir := filepath.Join(sm.basePath, "sections")
	metaPath := filepath.Join(sm.basePath, "sections.json")

	// 1. 读取 compiled.md
	content, err := os.ReadFile(compiledPath)
	if err != nil {
		return fmt.Errorf("read compiled.md: %w", err)
	}

	// 2. 解析章节
	meta, err := ParseSections(string(content))
	if err != nil {
		return fmt.Errorf("parse sections: %w", err)
	}

	// 3. 清理旧的章节文件（确保幂等性）
	if err := os.RemoveAll(sectionsDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old sections dir: %w", err)
	}

	// 4. 创建新的 sections 目录
	if err := os.MkdirAll(sectionsDir, 0755); err != nil {
		return fmt.Errorf("create sections dir: %w", err)
	}

	// 5. 写入章节文件
	for _, section := range meta.Sections {
		// 提取章节内容（不含标题）
		sectionContent := extractSectionContent(string(content), section)

		if err := WriteSectionFile(sectionsDir, section, sectionContent); err != nil {
			return fmt.Errorf("write section %s: %w", section.ID, err)
		}
	}

	// 6. 保存 sections.json（完全覆盖）
	return saveSectionMeta(metaPath, meta)
}

// SyncToCompiled 从章节文件同步到 compiled.md
func (sm *SyncManager) SyncToCompiled() error {
	// 1. 读取 sections.json
	metaPath := filepath.Join(sm.basePath, "sections.json")
	meta, err := loadSectionMeta(metaPath)
	if err != nil {
		return fmt.Errorf("load sections.json: %w", err)
	}

	// 2. 拼接章节
	sectionsDir := filepath.Join(sm.basePath, "sections")
	compiled, err := CompileSections(meta, sectionsDir)
	if err != nil {
		return fmt.Errorf("compile sections: %w", err)
	}

	// 3. 写入 compiled.md
	compiledPath := filepath.Join(sm.basePath, "compiled.md")
	return os.WriteFile(compiledPath, []byte(compiled), 0644)
}

// CheckNeedSync 检查是否需要同步
// 返回: (needSync, direction, error)
// direction: "from_compiled" 或 "to_compiled"
func (sm *SyncManager) CheckNeedSync() (needSync bool, direction string, err error) {
	compiledPath := filepath.Join(sm.basePath, "compiled.md")
	sectionsPath := filepath.Join(sm.basePath, "sections.json")

	compiledStat, compiledErr := os.Stat(compiledPath)
	sectionsStat, sectionsErr := os.Stat(sectionsPath)

	// 如果 sections.json 不存在，需要从 compiled.md 同步
	if os.IsNotExist(sectionsErr) {
		if os.IsNotExist(compiledErr) {
			// 两者都不存在，无需同步
			return false, "", nil
		}
		return true, "from_compiled", nil
	}

	// 如果 compiled.md 不存在，需要从 sections 同步
	if os.IsNotExist(compiledErr) {
		return true, "to_compiled", nil
	}

	// 比较修改时间
	if compiledStat.ModTime().After(sectionsStat.ModTime()) {
		return true, "from_compiled", nil
	} else if sectionsStat.ModTime().After(compiledStat.ModTime()) {
		return true, "to_compiled", nil
	}

	return false, "", nil
}

// ForceSync 强制执行同步
func (sm *SyncManager) ForceSync(direction string) error {
	switch direction {
	case "from_compiled":
		return sm.SyncFromCompiled()
	case "to_compiled":
		return sm.SyncToCompiled()
	default:
		return fmt.Errorf("invalid sync direction: %s", direction)
	}
}

// saveSectionMeta 保存章节元数据到 sections.json
func saveSectionMeta(filePath string, meta *SectionMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write meta file: %w", err)
	}

	return nil
}

// loadSectionMeta 从 sections.json 加载章节元数据
func loadSectionMeta(filePath string) (*SectionMeta, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read meta file: %w", err)
	}

	var meta SectionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal meta: %w", err)
	}

	return &meta, nil
}

// AutoSync 自动检查并执行同步
func (sm *SyncManager) AutoSync() error {
	needSync, direction, err := sm.CheckNeedSync()
	if err != nil {
		return fmt.Errorf("check sync: %w", err)
	}

	if !needSync {
		return nil
	}

	return sm.ForceSync(direction)
}

// GetSectionByID 根据 ID 查找章节
func GetSectionByID(meta *SectionMeta, sectionID string) (*Section, error) {
	for i := range meta.Sections {
		if meta.Sections[i].ID == sectionID {
			return &meta.Sections[i], nil
		}
	}
	return nil, fmt.Errorf("section not found: %s", sectionID)
}

// UpdateSectionInMeta 更新元数据中的章节信息
func UpdateSectionInMeta(meta *SectionMeta, section Section) error {
	for i := range meta.Sections {
		if meta.Sections[i].ID == section.ID {
			meta.Sections[i] = section
			meta.Version++
			meta.ETag = generateETag(meta.Sections)
			return nil
		}
	}
	return fmt.Errorf("section not found: %s", section.ID)
}

// RemoveSectionFromMeta 从元数据中删除章节
func RemoveSectionFromMeta(meta *SectionMeta, sectionID string, cascade bool) error {
	// 查找要删除的章节
	index := -1
	for i := range meta.Sections {
		if meta.Sections[i].ID == sectionID {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("section not found: %s", sectionID)
	}

	// 收集要删除的章节 ID（如果级联删除）
	toDelete := []string{sectionID}
	if cascade {
		collectChildren(&meta.Sections[index], meta, &toDelete)
	}

	// 从列表中移除
	newSections := []Section{}
	for _, sec := range meta.Sections {
		shouldDelete := false
		for _, id := range toDelete {
			if sec.ID == id {
				shouldDelete = true
				break
			}
		}
		if !shouldDelete {
			newSections = append(newSections, sec)
		}
	}

	// 更新父章节的 children 列表
	for i := range newSections {
		if newSections[i].Children != nil {
			newChildren := []string{}
			for _, childID := range newSections[i].Children {
				shouldRemove := false
				for _, id := range toDelete {
					if childID == id {
						shouldRemove = true
						break
					}
				}
				if !shouldRemove {
					newChildren = append(newChildren, childID)
				}
			}
			newSections[i].Children = newChildren
		}
	}

	meta.Sections = newSections
	meta.Version++
	meta.ETag = generateETag(meta.Sections)

	return nil
}

// collectChildren 递归收集所有子章节 ID
func collectChildren(section *Section, meta *SectionMeta, result *[]string) {
	for _, childID := range section.Children {
		*result = append(*result, childID)
		child, err := GetSectionByID(meta, childID)
		if err == nil {
			collectChildren(child, meta, result)
		}
	}
}

// InsertSectionInMeta 在元数据中插入新章节
func InsertSectionInMeta(meta *SectionMeta, title, content string, afterSectionID *string) (*Section, error) {
	// 生成新章节 ID
	maxOrder := 0
	for _, sec := range meta.Sections {
		if sec.Order > maxOrder {
			maxOrder = sec.Order
		}
	}

	newOrder := maxOrder + 1
	newSection := Section{
		ID:       fmt.Sprintf("section_%03d", newOrder),
		Title:    title,
		Level:    getHeadingLevel(title),
		Order:    newOrder,
		File:     fmt.Sprintf("section_%03d.md", newOrder),
		Children: []string{},
		Hash:     hashContent(content),
	}

	// 确定插入位置
	insertIndex := len(meta.Sections) // 默认插入到末尾

	if afterSectionID != nil && *afterSectionID != "" {
		for i, sec := range meta.Sections {
			if sec.ID == *afterSectionID {
				insertIndex = i + 1
				break
			}
		}
	}

	// 插入章节
	newSections := make([]Section, 0, len(meta.Sections)+1)
	newSections = append(newSections, meta.Sections[:insertIndex]...)
	newSections = append(newSections, newSection)
	newSections = append(newSections, meta.Sections[insertIndex:]...)

	// 重新调整 order
	for i := range newSections {
		newSections[i].Order = i + 1
	}

	meta.Sections = newSections
	meta.Version++
	meta.ETag = generateETag(meta.Sections)

	// 重新构建层级关系
	buildHierarchy(meta.Sections)

	return &newSection, nil
}

// ReplaceSectionRange 替换 compiled.md 中父章节及其所有子章节的内容
func ReplaceSectionRange(
	compiledContent string,
	parentSection *Section,
	newContent string,
	meta *SectionMeta,
) (string, error) {
	lines := strings.Split(compiledContent, "\n")

	// 找到父章节的开始位置
	startIdx := -1
	for i, line := range lines {
		if line == parentSection.Title {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return "", fmt.Errorf("parent section title not found: %s", parentSection.Title)
	}

	// 🔧 新逻辑：检查新内容中是否包含同级或更高级别的标题
	// 如果包含，说明是全文编辑，需要替换到文档末尾
	newContentLines := strings.Split(strings.TrimSpace(newContent), "\n")
	hasHigherLevelHeading := false
	inCodeBlock := false

	for _, line := range newContentLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if !inCodeBlock && isHeading(line) {
			level := getHeadingLevel(line)
			// 跳过第一行（标题本身）
			if line != parentSection.Title && level <= parentSection.Level {
				hasHigherLevelHeading = true
				break
			}
		}
	}

	// 找到父章节范围的结束位置
	var endIdx int
	if hasHigherLevelHeading {
		// 如果新内容包含同级或更高级别标题，替换到文档末尾
		endIdx = len(lines)
	} else {
		// 否则，只替换到下一个同级或更高级别标题之前
		endIdx = len(lines)
		inCodeBlock = false
		for i := startIdx + 1; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			// 检测代码块边界
			if strings.HasPrefix(trimmed, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			// 只在代码块外检测标题
			if !inCodeBlock && isHeading(lines[i]) {
				level := getHeadingLevel(lines[i])
				if level <= parentSection.Level {
					endIdx = i
					break
				}
			}
		}
	}

	// 构建新的 compiled.md
	var builder strings.Builder

	// 1. 保留开始位置之前的内容
	for i := 0; i < startIdx; i++ {
		builder.WriteString(lines[i] + "\n")
	}

	// 2. 插入新内容（所见即所得）
	trimmedContent := strings.TrimSpace(newContent)
	if trimmedContent != "" {
		builder.WriteString(trimmedContent)
		if !strings.HasSuffix(trimmedContent, "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	// 3. 保留结束位置之后的内容
	for i := endIdx; i < len(lines); i++ {
		builder.WriteString(lines[i] + "\n")
	}

	return strings.TrimSpace(builder.String()), nil
}
