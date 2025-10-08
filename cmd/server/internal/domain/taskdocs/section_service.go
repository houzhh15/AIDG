package taskdocs

import (
	"fmt"
	"os"
	"path/filepath"
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
	basePath string // 项目根目录
}

// NewSectionService 创建 Service 实例
func NewSectionService(basePath string) SectionService {
	return &sectionServiceImpl{basePath: basePath}
}

// getDocPath 获取文档路径
func (s *sectionServiceImpl) getDocPath(projectID, taskID, docType string) string {
	return filepath.Join(s.basePath, projectID, "tasks", taskID, "docs", docType)
}

// GetSections 获取章节列表
func (s *sectionServiceImpl) GetSections(projectID, taskID, docType string) (*SectionMeta, error) {
	docPath := s.getDocPath(projectID, taskID, docType)
	metaPath := filepath.Join(docPath, "sections.json")

	// 检查 sections.json 是否存在
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		// 首次初始化：从 compiled.md 解析
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

	return nil
}

// UpdateSectionFull 更新父章节的全文内容（包含所有子章节）
func (s *sectionServiceImpl) UpdateSectionFull(
	projectID, taskID, docType, sectionID string,
	fullContent string, expectedVersion int,
) error {
	docPath := s.getDocPath(projectID, taskID, docType)
	compiledPath := filepath.Join(docPath, "compiled.md")
	metaPath := filepath.Join(docPath, "sections.json")

	// 1. 加载并验证版本
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

	// 3. 备份当前 compiled.md（用于回滚）
	compiledContent, err := os.ReadFile(compiledPath)
	if err != nil {
		return fmt.Errorf("read compiled.md: %w", err)
	}
	backup := compiledContent

	// 4. 替换父章节范围
	newCompiled, err := ReplaceSectionRange(string(compiledContent), section, fullContent, meta)
	if err != nil {
		return fmt.Errorf("replace section range: %w", err)
	}

	// 5. 写入 compiled.md
	if err := os.WriteFile(compiledPath, []byte(newCompiled), 0644); err != nil {
		return fmt.Errorf("write compiled.md: %w", err)
	}

	// 6. 重新从 compiled.md 解析章节
	sm := NewSyncManager(docPath, docType)
	if err := sm.SyncFromCompiled(); err != nil {
		// 解析失败，回滚 compiled.md
		os.WriteFile(compiledPath, backup, 0644)
		return fmt.Errorf("sync from compiled failed, rolled back: %w", err)
	}

	return nil
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
