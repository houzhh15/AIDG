package taskdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SectionServiceWithPath 支持自定义路径的章节服务
// 与 sectionServiceImpl 不同，它直接使用完整的文档路径，而不是从 projectID/taskID 推导
type SectionServiceWithPath interface {
	// GetSections 获取章节列表
	GetSections() (*SectionMeta, error)

	// GetSection 获取单个章节内容
	GetSection(sectionID string, includeChildren bool) (*SectionContent, error)

	// UpdateSection 更新章节内容
	UpdateSection(sectionID string, content string, expectedVersion int) error

	// InsertSection 插入新章节
	InsertSection(title, content string, afterSectionID *string, expectedVersion int) (*Section, error)

	// DeleteSection 删除章节
	DeleteSection(sectionID string, cascade bool, expectedVersion int) error

	// SyncSections 同步章节与 compiled.md
	SyncSections(direction string) error
}

// sectionServiceWithPathImpl 实现
type sectionServiceWithPathImpl struct {
	docPath    string      // 完整文档目录路径
	docType    string      // 文档类型（从路径推导）
	docService *DocService // 文档服务
}

// NewSectionServiceWithPath 创建支持自定义路径的章节服务
func NewSectionServiceWithPath(docPath string) SectionServiceWithPath {
	docType := filepath.Base(docPath)
	return &sectionServiceWithPathImpl{
		docPath:    docPath,
		docType:    docType,
		docService: NewDocService(),
	}
}

// GetSections 获取章节列表
func (s *sectionServiceWithPathImpl) GetSections() (*SectionMeta, error) {
	metaPath := filepath.Join(s.docPath, "sections.json")
	compiledPath := filepath.Join(s.docPath, "compiled.md")

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
		sm := NewSyncManager(s.docPath, s.docType)
		if err := sm.SyncFromCompiled(); err != nil {
			return nil, fmt.Errorf("init from compiled: %w", err)
		}
	}

	// 读取 sections.json
	meta, err := loadSectionMeta(metaPath)
	if err != nil {
		return nil, fmt.Errorf("load meta: %w", err)
	}

	return meta, nil
}

// GetSection 获取单个章节内容
func (s *sectionServiceWithPathImpl) GetSection(sectionID string, includeChildren bool) (*SectionContent, error) {
	// 加载元数据
	metaPath := filepath.Join(s.docPath, "sections.json")
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
	sectionsDir := filepath.Join(s.docPath, "sections")
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
			childContent, err := s.GetSection(childID, true)
			if err != nil {
				return nil, fmt.Errorf("get child %s: %w", childID, err)
			}
			result.ChildrenContent = append(result.ChildrenContent, *childContent)
		}
	}

	return result, nil
}

// UpdateSection 更新章节内容
func (s *sectionServiceWithPathImpl) UpdateSection(sectionID string, content string, expectedVersion int) error {
	metaPath := filepath.Join(s.docPath, "sections.json")

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
	sectionsDir := filepath.Join(s.docPath, "sections")
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
	sm := NewSyncManager(s.docPath, s.docType)
	if err := sm.SyncToCompiled(); err != nil {
		return fmt.Errorf("sync to compiled: %w", err)
	}

	// 7. 更新 doc meta
	compiledPath := filepath.Join(s.docPath, "compiled.md")
	newCompiled, err := os.ReadFile(compiledPath)
	if err != nil {
		return fmt.Errorf("read compiled.md: %w", err)
	}

	docMeta, err := loadOrInitMetaWithPath(s.docPath)
	if err != nil {
		return fmt.Errorf("load doc meta: %w", err)
	}

	_, _, _, err = s.docService.AppendWithPath(
		s.docPath,
		string(newCompiled),
		"section_update_no_parse",
		"section_edit",
		"update_section",
		&docMeta.Version,
	)
	if err != nil {
		return fmt.Errorf("save through doc service: %w", err)
	}

	return nil
}

// InsertSection 插入新章节
func (s *sectionServiceWithPathImpl) InsertSection(title, content string, afterSectionID *string, expectedVersion int) (*Section, error) {
	metaPath := filepath.Join(s.docPath, "sections.json")

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
	sectionsDir := filepath.Join(s.docPath, "sections")
	if err := WriteSectionFile(sectionsDir, *newSection, content); err != nil {
		return nil, fmt.Errorf("write section file: %w", err)
	}

	// 4. 保存元数据
	if err := saveSectionMeta(metaPath, meta); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}

	// 5. 同步到 compiled.md
	sm := NewSyncManager(s.docPath, s.docType)
	if err := sm.SyncToCompiled(); err != nil {
		return nil, fmt.Errorf("sync to compiled: %w", err)
	}

	// 6. 更新 doc meta
	compiledPath := filepath.Join(s.docPath, "compiled.md")
	newCompiled, err := os.ReadFile(compiledPath)
	if err != nil {
		return nil, fmt.Errorf("read compiled.md: %w", err)
	}

	docMeta, err := loadOrInitMetaWithPath(s.docPath)
	if err != nil {
		return nil, fmt.Errorf("load doc meta: %w", err)
	}

	_, _, _, err = s.docService.AppendWithPath(
		s.docPath,
		string(newCompiled),
		"section_insert_no_parse",
		"section_edit",
		"insert_section",
		&docMeta.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("save through doc service: %w", err)
	}

	return newSection, nil
}

// DeleteSection 删除章节
func (s *sectionServiceWithPathImpl) DeleteSection(sectionID string, cascade bool, expectedVersion int) error {
	metaPath := filepath.Join(s.docPath, "sections.json")

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

	// 3. 收集要删除的所有章节
	toDelete := []*Section{section}
	if cascade {
		collectChildSectionsHelper(meta, section, &toDelete)
	}

	// 4. 删除章节文件
	sectionsDir := filepath.Join(s.docPath, "sections")
	for _, sec := range toDelete {
		if err := DeleteSectionFile(sectionsDir, *sec); err != nil {
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
	sm := NewSyncManager(s.docPath, s.docType)
	if err := sm.SyncToCompiled(); err != nil {
		return fmt.Errorf("sync to compiled: %w", err)
	}

	// 8. 更新 doc meta
	compiledPath := filepath.Join(s.docPath, "compiled.md")
	newCompiled, err := os.ReadFile(compiledPath)
	if err != nil {
		return fmt.Errorf("read compiled.md: %w", err)
	}

	docMeta, err := loadOrInitMetaWithPath(s.docPath)
	if err != nil {
		return fmt.Errorf("load doc meta: %w", err)
	}

	_, _, _, err = s.docService.AppendWithPath(
		s.docPath,
		string(newCompiled),
		"section_delete_no_parse",
		"section_edit",
		"delete_section",
		&docMeta.Version,
	)
	if err != nil {
		return fmt.Errorf("save through doc service: %w", err)
	}

	return nil
}

// SyncSections 同步章节与 compiled.md
func (s *sectionServiceWithPathImpl) SyncSections(direction string) error {
	sm := NewSyncManager(s.docPath, s.docType)
	return sm.ForceSync(direction)
}

// collectChildSectionsHelper 递归收集所有子章节
func collectChildSectionsHelper(meta *SectionMeta, parent *Section, result *[]*Section) {
	for _, childID := range parent.Children {
		child, err := GetSectionByID(meta, childID)
		if err == nil {
			*result = append(*result, child)
			collectChildSectionsHelper(meta, child, result)
		}
	}
}
