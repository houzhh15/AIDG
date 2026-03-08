package taskdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SectionService 章节服务接口
type SectionService interface {
	// GetSections 获取章节列表
	GetSections(projectID, taskID, docType string) (*SectionMeta, error)

	// GetSection 获取单个章节内容
	GetSection(projectID, taskID, docType, sectionID string, includeChildren bool) (*SectionContent, error)

	// UpdateSection 更新章节内容
	UpdateSection(projectID, taskID, docType, sectionID string, content string, expectedVersion int) error

	// InsertSection 插入新章节
	InsertSection(projectID, taskID, docType, title, content string, afterSectionID *string, expectedVersion int) (*Section, error)

	// DeleteSection 删除章节
	DeleteSection(projectID, taskID, docType, sectionID string, cascade bool, expectedVersion int) error

	// ReorderSection 调整章节顺序
	ReorderSection(projectID, taskID, docType, sectionID string, afterSectionID *string, expectedVersion int) error

	// SyncSections 同步章节与 compiled.md
	SyncSections(projectID, taskID, docType string, direction string) error

	// UpdateSectionFull 更新父章节的全文内容（包含所有子章节）
	UpdateSectionFull(projectID, taskID, docType, sectionID string, fullContent string, expectedVersion int) error
}

// sectionServiceImpl Service 实现
type sectionServiceImpl struct {
	basePath   string      // 项目根目录
	docService *DocService // 文档服务（用于记录历史）
}

// NewSectionService 创建 Service 实例
func NewSectionService(basePath string) SectionService {
	return &sectionServiceImpl{
		basePath:   basePath,
		docService: NewDocService(), // 创建文档服务实例
	}
}

// getDocPath 获取文档路径
func (s *sectionServiceImpl) getDocPath(projectID, taskID, docType string) string {
	return filepath.Join(s.basePath, projectID, "tasks", taskID, "docs", docType)
}

// GetSections 获取章节列表
func (s *sectionServiceImpl) GetSections(projectID, taskID, docType string) (*SectionMeta, error) {
	docPath := s.getDocPath(projectID, taskID, docType)
	metaPath := filepath.Join(docPath, "sections.json")
	compiledPath := filepath.Join(docPath, "compiled.md")

	// 检查 sections.json 是否存在
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		// 检查 compiled.md 是否存在
		if _, compErr := os.Stat(compiledPath); os.IsNotExist(compErr) {
			// 两者都不存在：返回空的章节元数据（新文档）
			return &SectionMeta{
				Version:   0,
				UpdatedAt: time.Now(),
				RootLevel: 1,
				Sections:  []Section{},
				ETag:      generateETag([]Section{}),
			}, nil
		}

		// compiled.md 存在但 sections.json 不存在：首次初始化
		sm := NewSyncManager(docPath, docType)
		if err := sm.SyncFromCompiled(); err != nil {
			return nil, fmt.Errorf("init from compiled: %w", err)
		}
	}
	// 注意：不再执行 AutoSync，避免重复解析
	// 如果需要同步，应该通过显式的 API 调用（如 POST /sections/sync）

	// 读取 sections.json
	meta, err := loadSectionMeta(metaPath)
	if err != nil {
		return nil, fmt.Errorf("load meta: %w", err)
	}

	return meta, nil
}

// GetSection 获取单个章节内容
func (s *sectionServiceImpl) GetSection(projectID, taskID, docType, sectionID string, includeChildren bool) (*SectionContent, error) {
	docPath := s.getDocPath(projectID, taskID, docType)

	// 加载元数据
	metaPath := filepath.Join(docPath, "sections.json")
	meta, err := loadSectionMeta(metaPath)
	if err != nil {
		return nil, fmt.Errorf("load meta: %w", err)
	}

	// 查找章节
	section, err := GetSectionByID(meta, sectionID)
	if err != nil {
		return nil, err
	}

	// 读取章节内容
	sectionsDir := filepath.Join(docPath, "sections")
	content, err := ReadSectionFile(sectionsDir, *section)
	if err != nil {
		return nil, fmt.Errorf("read section content: %w", err)
	}

	result := &SectionContent{
		Section: *section,
		Content: content,
	}

	// 如果需要包含子章节
	if includeChildren && len(section.Children) > 0 {
		result.ChildrenContent = []SectionContent{}
		for _, childID := range section.Children {
			childContent, err := s.GetSection(projectID, taskID, docType, childID, true)
			if err != nil {
				return nil, fmt.Errorf("get child %s: %w", childID, err)
			}
			result.ChildrenContent = append(result.ChildrenContent, *childContent)
		}
	}

	return result, nil
}

// UpdateSection 更新章节内容
func (s *sectionServiceImpl) UpdateSection(
	projectID, taskID, docType, sectionID string,
	content string, expectedVersion int,
) error {
	docPath := s.getDocPath(projectID, taskID, docType)
	metaPath := filepath.Join(docPath, "sections.json")

	// 1. 加载并验证版本
	meta, err := loadSectionMeta(metaPath)
	if err != nil {
		return fmt.Errorf("load meta: %w", err)
	}

	if expectedVersion > 0 && meta.Version != expectedVersion {
		return fmt.Errorf("version conflict: expected %d, got %d", expectedVersion, meta.Version)
	}

	// 2. 查找章节
	section, err := GetSectionByID(meta, sectionID)
	if err != nil {
		return err
	}

	// 3. 写入章节文件
	sectionsDir := filepath.Join(docPath, "sections")
	if err := WriteSectionFile(sectionsDir, *section, content); err != nil {
		return fmt.Errorf("write section file: %w", err)
	}

	// 4. 更新哈希和版本
	section.Hash = hashContent(content)
	if err := UpdateSectionInMeta(meta, *section); err != nil {
		return fmt.Errorf("update meta: %w", err)
	}

	meta.UpdatedAt = time.Now()

	// 5. 保存元数据
	if err := saveSectionMeta(metaPath, meta); err != nil {
		return fmt.Errorf("save meta: %w", err)
	}

	// 6. 同步到 compiled.md
	sm := NewSyncManager(docPath, docType)
	if err := sm.SyncToCompiled(); err != nil {
		return fmt.Errorf("sync to compiled: %w", err)
	}

	// 7. 🔧 修复：使用特殊操作类型避免重复解析
	// 读取新的 compiled.md 并通过 DocService 保存（记录到 chunks.ndjson）
	compiledPath := filepath.Join(docPath, "compiled.md")
	newCompiled, err := os.ReadFile(compiledPath)
	if err != nil {
		return fmt.Errorf("read compiled.md: %w", err)
	}

	// 通过 DocService 记录变更历史
	docMeta, err := LoadOrInitMeta(projectID, taskID, docType)
	if err != nil {
		return fmt.Errorf("load doc meta: %w", err)
	}

	_, _, _, err = s.docService.Append(
		projectID, taskID, docType,
		string(newCompiled),       // 完整文档内容
		"section_edit",            // 用户标识
		&docMeta.Version,          // 版本号
		"section_update_no_parse", // 🔧 特殊操作：不触发 SyncFromCompiled
		"update_section",          // 来源：单章节更新
	)
	if err != nil {
		return fmt.Errorf("save through doc service: %w", err)
	}

	return nil
}

// UpdateSectionFull 更新父章节的全文内容（包含所有子章节）
// 确保所见即所得：用户看到的内容范围与实际替换的范围完全一致
func (s *sectionServiceImpl) UpdateSectionFull(
	projectID, taskID, docType, sectionID string,
	fullContent string, expectedVersion int,
) error {
	docPath := s.getDocPath(projectID, taskID, docType)
	metaPath := filepath.Join(docPath, "sections.json")
	sectionsDir := filepath.Join(docPath, "sections")

	// 1. 加载并验证版本（sections.json 的版本）
	meta, err := loadSectionMeta(metaPath)
	if err != nil {
		return fmt.Errorf("load meta: %w", err)
	}

	if expectedVersion > 0 && meta.Version != expectedVersion {
		return fmt.Errorf("version conflict: expected %d, got %d", expectedVersion, meta.Version)
	}

	// 2. 查找父章节
	section, err := GetSectionByID(meta, sectionID)
	if err != nil {
		return err
	}

	// 3. 收集要删除的所有子章节（确保删除范围与用户看到的一致）
	childrenToDelete := []*Section{}
	s.collectChildSections(meta, section, &childrenToDelete)

	// 4. 删除所有子章节（级联删除）
	// 4.1 删除子章节文件
	for _, child := range childrenToDelete {
		if err := DeleteSectionFile(sectionsDir, *child); err != nil {
			// 继续删除，不因为单个文件失败而中止
			fmt.Printf("Warning: delete child section file %s: %v\n", child.ID, err)
		}
	}

	// 4.2 从元数据中删除子章节
	for _, child := range childrenToDelete {
		if err := RemoveSectionFromMeta(meta, child.ID, false); err != nil {
			return fmt.Errorf("remove child section %s from meta: %w", child.ID, err)
		}
	}

	// 5. 删除父章节本身（因为我们要用新内容完全替换这个区域）
	if err := DeleteSectionFile(sectionsDir, *section); err != nil {
		fmt.Printf("Warning: delete parent section file %s: %v\n", section.ID, err)
	}
	if err := RemoveSectionFromMeta(meta, sectionID, false); err != nil {
		return fmt.Errorf("remove parent section from meta: %w", err)
	}

	meta.UpdatedAt = time.Now()

	// 6. 保存元数据
	if err := saveSectionMeta(metaPath, meta); err != nil {
		return fmt.Errorf("save meta: %w", err)
	}

	// 7. 直接使用用户编辑的完整内容替换 compiled.md
	// 注意：需要保留其他章节的内容，只替换被编辑的部分
	compiledPath := filepath.Join(docPath, "compiled.md")
	oldCompiled, err := os.ReadFile(compiledPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read compiled.md: %w", err)
	}

	// 构建新的 compiled.md：保留其他章节，替换当前编辑的父章节区域
	newCompiled := s.replaceSection(string(oldCompiled), section, fullContent)

	// 8. 通过 DocService 保存（记录到 chunks.ndjson）
	// 重要：使用 replace_full 操作，这会触发 SyncFromCompiled 重新解析章节
	docMeta, err := LoadOrInitMeta(projectID, taskID, docType)
	if err != nil {
		return fmt.Errorf("load doc meta: %w", err)
	}

	_, _, _, err = s.docService.Append(
		projectID, taskID, docType,
		newCompiled,             // 新的完整文档内容
		"section_edit",          // 用户标识
		&docMeta.Version,        // 使用 doc meta 的版本号进行并发检查
		"section_full_no_parse", // 🔧 特殊操作：不触发 SyncFromCompiled，避免重复解析
		"update_section_full",   // 来源：章节全文更新
	)
	if err != nil {
		return fmt.Errorf("save through doc service: %w", err)
	}

	// 🔧 手动触发 SyncFromCompiled 来重建章节结构
	sm := NewSyncManager(docPath, docType)
	if err := sm.SyncFromCompiled(); err != nil {
		return fmt.Errorf("sync from compiled after update: %w", err)
	}

	return nil
}

// replaceSection 在 compiled.md 中替换指定章节的内容
// 保留其他章节不变，只替换被编辑的章节区域
func (s *sectionServiceImpl) replaceSection(compiledContent string, section *Section, newContent string) string {
	lines := strings.Split(compiledContent, "\n")

	// 🔧 修复：section.Title 已经包含完整的标题标记，直接使用
	sectionTitle := section.Title

	// 查找章节开始位置
	startIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == strings.TrimSpace(sectionTitle) {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		// 如果找不到原章节，直接返回新内容
		// 这种情况可能发生在章节被删除或标题被修改时
		return newContent
	}

	// 查找章节结束位置（下一个同级或更高级的标题）
	endIdx := len(lines)
	for i := startIdx + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "#") {
			// 检查是否是同级或更高级的标题
			level := 0
			for _, ch := range line {
				if ch == '#' {
					level++
				} else {
					break
				}
			}
			if level <= section.Level {
				endIdx = i
				break
			}
		}
	}

	// 构建新的 compiled.md
	var result strings.Builder

	// 保留章节之前的内容
	if startIdx > 0 {
		result.WriteString(strings.Join(lines[:startIdx], "\n"))
		result.WriteString("\n")
	}

	// 插入新内容
	result.WriteString(newContent)

	// 保留章节之后的内容
	if endIdx < len(lines) {
		result.WriteString("\n")
		result.WriteString(strings.Join(lines[endIdx:], "\n"))
	}

	return result.String()
}

// InsertSection 插入新章节
func (s *sectionServiceImpl) InsertSection(
	projectID, taskID, docType, title, content string,
	afterSectionID *string, expectedVersion int,
) (*Section, error) {
	docPath := s.getDocPath(projectID, taskID, docType)
	metaPath := filepath.Join(docPath, "sections.json")

	// 1. 加载并验证版本
	meta, err := loadSectionMeta(metaPath)
	if err != nil {
		return nil, fmt.Errorf("load meta: %w", err)
	}

	if expectedVersion > 0 && meta.Version != expectedVersion {
		return nil, fmt.Errorf("version conflict: expected %d, got %d", expectedVersion, meta.Version)
	}

	// 2. 插入新章节到元数据
	newSection, err := InsertSectionInMeta(meta, title, content, afterSectionID)
	if err != nil {
		return nil, fmt.Errorf("insert section: %w", err)
	}

	meta.UpdatedAt = time.Now()

	// 3. 写入章节文件
	sectionsDir := filepath.Join(docPath, "sections")
	if err := WriteSectionFile(sectionsDir, *newSection, content); err != nil {
		return nil, fmt.Errorf("write section file: %w", err)
	}

	// 4. 保存元数据
	if err := saveSectionMeta(metaPath, meta); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}

	// 5. 同步到 compiled.md
	sm := NewSyncManager(docPath, docType)
	if err := sm.SyncToCompiled(); err != nil {
		return nil, fmt.Errorf("sync to compiled: %w", err)
	}

	// 6. 🔧 修复：直接更新 doc meta 的版本号，不触发 SyncFromCompiled
	// 读取新的 compiled.md
	compiledPath := filepath.Join(docPath, "compiled.md")
	newCompiled, err := os.ReadFile(compiledPath)
	if err != nil {
		return nil, fmt.Errorf("read compiled.md: %w", err)
	}

	// 加载 doc meta
	docMeta, err := LoadOrInitMeta(projectID, taskID, docType)
	if err != nil {
		return nil, fmt.Errorf("load doc meta: %w", err)
	}

	// 使用特殊的操作类型 "section_insert_no_parse" 避免重新解析
	_, _, _, err = s.docService.Append(
		projectID, taskID, docType,
		string(newCompiled),       // 完整文档内容
		"section_edit",            // 用户标识
		&docMeta.Version,          // 版本号
		"section_insert_no_parse", // 🔧 特殊操作：不触发 SyncFromCompiled
		"insert_section",          // 来源：插入章节
	)
	if err != nil {
		return nil, fmt.Errorf("save through doc service: %w", err)
	}

	return newSection, nil
}

// DeleteSection 删除章节
func (s *sectionServiceImpl) DeleteSection(
	projectID, taskID, docType, sectionID string,
	cascade bool, expectedVersion int,
) error {
	docPath := s.getDocPath(projectID, taskID, docType)
	metaPath := filepath.Join(docPath, "sections.json")

	// 1. 加载并验证版本
	meta, err := loadSectionMeta(metaPath)
	if err != nil {
		return fmt.Errorf("load meta: %w", err)
	}

	if expectedVersion > 0 && meta.Version != expectedVersion {
		return fmt.Errorf("version conflict: expected %d, got %d", expectedVersion, meta.Version)
	}

	// 2. 查找要删除的章节
	section, err := GetSectionByID(meta, sectionID)
	if err != nil {
		return err
	}

	// 3. 收集要删除的所有章节（如果级联删除）
	toDelete := []*Section{section}
	if cascade {
		s.collectChildSections(meta, section, &toDelete)
	}

	// 4. 删除章节文件
	sectionsDir := filepath.Join(docPath, "sections")
	for _, sec := range toDelete {
		if err := DeleteSectionFile(sectionsDir, *sec); err != nil {
			// 继续删除，不因为单个文件失败而中止
			fmt.Printf("Warning: delete section file %s: %v\n", sec.ID, err)
		}
	}

	// 5. 从元数据中删除
	if err := RemoveSectionFromMeta(meta, sectionID, cascade); err != nil {
		return fmt.Errorf("remove from meta: %w", err)
	}

	meta.UpdatedAt = time.Now()

	// 6. 保存元数据
	if err := saveSectionMeta(metaPath, meta); err != nil {
		return fmt.Errorf("save meta: %w", err)
	}

	// 7. 同步到 compiled.md
	sm := NewSyncManager(docPath, docType)
	if err := sm.SyncToCompiled(); err != nil {
		return fmt.Errorf("sync to compiled: %w", err)
	}

	// 8. 🔧 修复：使用特殊操作类型避免重复解析
	// 读取新的 compiled.md 并通过 DocService 保存（记录到 chunks.ndjson）
	compiledPath := filepath.Join(docPath, "compiled.md")
	newCompiled, err := os.ReadFile(compiledPath)
	if err != nil {
		return fmt.Errorf("read compiled.md: %w", err)
	}

	// 通过 DocService 记录变更历史
	docMeta, err := LoadOrInitMeta(projectID, taskID, docType)
	if err != nil {
		return fmt.Errorf("load doc meta: %w", err)
	}

	_, _, _, err = s.docService.Append(
		projectID, taskID, docType,
		string(newCompiled),       // 完整文档内容
		"section_edit",            // 用户标识
		&docMeta.Version,          // 版本号
		"section_delete_no_parse", // 🔧 特殊操作：不触发 SyncFromCompiled
		"delete_section",          // 来源：删除章节
	)
	if err != nil {
		return fmt.Errorf("save through doc service: %w", err)
	}

	return nil
}

// collectChildSections 递归收集所有子章节
func (s *sectionServiceImpl) collectChildSections(meta *SectionMeta, parent *Section, result *[]*Section) {
	for _, childID := range parent.Children {
		child, err := GetSectionByID(meta, childID)
		if err == nil {
			*result = append(*result, child)
			s.collectChildSections(meta, child, result)
		}
	}
}

// ReorderSection 调整章节顺序
func (s *sectionServiceImpl) ReorderSection(
	projectID, taskID, docType, sectionID string,
	afterSectionID *string, expectedVersion int,
) error {
	docPath := s.getDocPath(projectID, taskID, docType)
	metaPath := filepath.Join(docPath, "sections.json")

	// 1. 加载并验证版本
	meta, err := loadSectionMeta(metaPath)
	if err != nil {
		return fmt.Errorf("load meta: %w", err)
	}

	if expectedVersion > 0 && meta.Version != expectedVersion {
		return fmt.Errorf("version conflict: expected %d, got %d", expectedVersion, meta.Version)
	}

	// 2. 查找要移动的章节
	var targetSection *Section
	targetIndex := -1
	for i := range meta.Sections {
		if meta.Sections[i].ID == sectionID {
			targetSection = &meta.Sections[i]
			targetIndex = i
			break
		}
	}

	if targetSection == nil {
		return fmt.Errorf("section not found: %s", sectionID)
	}

	// 3. 确定新位置
	newIndex := len(meta.Sections) - 1 // 默认移到末尾

	if afterSectionID != nil && *afterSectionID != "" {
		for i := range meta.Sections {
			if meta.Sections[i].ID == *afterSectionID {
				newIndex = i
				break
			}
		}
	}

	// 4. 重新排列
	newSections := []Section{}

	// 先添加目标位置之前的章节（不包括要移动的）
	for i := 0; i <= newIndex && i < len(meta.Sections); i++ {
		if i != targetIndex {
			newSections = append(newSections, meta.Sections[i])
		}
	}

	// 添加要移动的章节
	newSections = append(newSections, *targetSection)

	// 添加剩余章节（不包括要移动的）
	for i := newIndex + 1; i < len(meta.Sections); i++ {
		if i != targetIndex {
			newSections = append(newSections, meta.Sections[i])
		}
	}

	// 5. 重新调整 order
	for i := range newSections {
		newSections[i].Order = i + 1
	}

	meta.Sections = newSections
	meta.Version++
	meta.UpdatedAt = time.Now()
	meta.ETag = generateETag(meta.Sections)

	// 6. 重新构建层级关系
	buildHierarchy(meta.Sections)

	// 7. 保存元数据
	if err := saveSectionMeta(metaPath, meta); err != nil {
		return fmt.Errorf("save meta: %w", err)
	}

	// 8. 同步到 compiled.md
	sm := NewSyncManager(docPath, docType)
	if err := sm.SyncToCompiled(); err != nil {
		return fmt.Errorf("sync to compiled: %w", err)
	}

	return nil
}

// SyncSections 同步章节与 compiled.md
func (s *sectionServiceImpl) SyncSections(projectID, taskID, docType string, direction string) error {
	docPath := s.getDocPath(projectID, taskID, docType)
	sm := NewSyncManager(docPath, docType)
	return sm.ForceSync(direction)
}
