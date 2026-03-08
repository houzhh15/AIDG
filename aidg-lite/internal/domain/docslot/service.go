package docslot

import (
	"aidg-lite/internal/domain/taskdocs"
)

// UnifiedDocService 统一文档服务接口（lite版本仅支持 project scope）
type UnifiedDocService interface {
	// 项目文档操作
	Append(scope DocumentScope, scopeID, slotKey, content, user string, expectedVersion *int, op, source string) (*AppendResult, error)
	Export(scope DocumentScope, scopeID, slotKey string) (*ExportResult, error)
	ListChunks(scope DocumentScope, scopeID, slotKey string) ([]taskdocs.DocChunk, *taskdocs.DocMeta, error)
	Squash(scope DocumentScope, scopeID, slotKey, user, source string) (*taskdocs.DocMeta, error)

	// 项目文档章节操作
	GetSections(scope DocumentScope, scopeID, slotKey string) (*taskdocs.SectionMeta, error)
	GetSection(scope DocumentScope, scopeID, slotKey, sectionID string, includeChildren bool) (*taskdocs.SectionContent, error)
	UpdateSection(scope DocumentScope, scopeID, slotKey, sectionID, content string, expectedVersion int) error
	InsertSection(scope DocumentScope, scopeID, slotKey, title, content string, afterSectionID *string, expectedVersion int) (*taskdocs.Section, error)
	DeleteSection(scope DocumentScope, scopeID, slotKey, sectionID string, cascade bool, expectedVersion int) error

	PathResolver() PathResolver
}

type unifiedDocServiceImpl struct {
	pathResolver PathResolver
	docService   *taskdocs.DocService
	basePath     string
}

func NewUnifiedDocService(basePath string) UnifiedDocService {
	return &unifiedDocServiceImpl{
		pathResolver: NewPathResolver(basePath),
		docService:   taskdocs.NewDocService(),
		basePath:     basePath,
	}
}

func (s *unifiedDocServiceImpl) PathResolver() PathResolver { return s.pathResolver }

func (s *unifiedDocServiceImpl) resolveAndValidate(scope DocumentScope, scopeID, slotKey string) (string, error) {
	return s.pathResolver.ResolvePath(scope, scopeID, slotKey)
}

// Append 增量追加或全文替换
func (s *unifiedDocServiceImpl) Append(scope DocumentScope, scopeID, slotKey, content, user string, expectedVersion *int, op, source string) (*AppendResult, error) {
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, err
	}
	meta, chunk, duplicate, err := s.docService.AppendWithPath(basePath, content, op, user, source, expectedVersion)
	if err != nil {
		return nil, WrapError(scope, scopeID, slotKey, err)
	}
	result := &AppendResult{
		Version:   meta.Version,
		ETag:      meta.ETag,
		Duplicate: duplicate,
		Timestamp: meta.UpdatedAt,
	}
	if chunk != nil {
		result.Sequence = chunk.Sequence
	}
	return result, nil
}

// Export 导出完整文档内容
func (s *unifiedDocServiceImpl) Export(scope DocumentScope, scopeID, slotKey string) (*ExportResult, error) {
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, err
	}
	content, meta, err := taskdocs.ExportWithPath(basePath)
	if err != nil {
		return nil, WrapError(scope, scopeID, slotKey, err)
	}
	return &ExportResult{
		Content:   content,
		Version:   meta.Version,
		ETag:      meta.ETag,
		UpdatedAt: meta.UpdatedAt,
		Exists:    content != "" || meta.Version > 0,
	}, nil
}

// ListChunks 列出 chunk 历史
func (s *unifiedDocServiceImpl) ListChunks(scope DocumentScope, scopeID, slotKey string) ([]taskdocs.DocChunk, *taskdocs.DocMeta, error) {
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, nil, err
	}
	chunks, meta, err := s.docService.ListWithPath(basePath)
	if err != nil {
		return nil, nil, WrapError(scope, scopeID, slotKey, err)
	}
	return chunks, &meta, nil
}

// Squash 压缩合并 chunks
func (s *unifiedDocServiceImpl) Squash(scope DocumentScope, scopeID, slotKey, user, source string) (*taskdocs.DocMeta, error) {
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, err
	}
	meta, err := s.docService.SquashWithPath(basePath, user, source, nil)
	if err != nil {
		return nil, WrapError(scope, scopeID, slotKey, err)
	}
	return &meta, nil
}

// GetSections 获取章节列表
func (s *unifiedDocServiceImpl) GetSections(scope DocumentScope, scopeID, slotKey string) (*taskdocs.SectionMeta, error) {
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, err
	}
	svc := taskdocs.NewSectionServiceWithPath(basePath)
	meta, err := svc.GetSections()
	if err != nil {
		return nil, WrapError(scope, scopeID, slotKey, err)
	}
	return meta, nil
}

// GetSection 获取单个章节
func (s *unifiedDocServiceImpl) GetSection(scope DocumentScope, scopeID, slotKey, sectionID string, includeChildren bool) (*taskdocs.SectionContent, error) {
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, err
	}
	svc := taskdocs.NewSectionServiceWithPath(basePath)
	content, err := svc.GetSection(sectionID, includeChildren)
	if err != nil {
		return nil, WrapError(scope, scopeID, slotKey, err)
	}
	return content, nil
}

// UpdateSection 更新章节
func (s *unifiedDocServiceImpl) UpdateSection(scope DocumentScope, scopeID, slotKey, sectionID, content string, expectedVersion int) error {
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return err
	}
	svc := taskdocs.NewSectionServiceWithPath(basePath)
	if err := svc.UpdateSection(sectionID, content, expectedVersion); err != nil {
		return WrapError(scope, scopeID, slotKey, err)
	}
	return nil
}

// InsertSection 插入章节
func (s *unifiedDocServiceImpl) InsertSection(scope DocumentScope, scopeID, slotKey, title, content string, afterSectionID *string, expectedVersion int) (*taskdocs.Section, error) {
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, err
	}
	svc := taskdocs.NewSectionServiceWithPath(basePath)
	section, err := svc.InsertSection(title, content, afterSectionID, expectedVersion)
	if err != nil {
		return nil, WrapError(scope, scopeID, slotKey, err)
	}
	return section, nil
}

// DeleteSection 删除章节
func (s *unifiedDocServiceImpl) DeleteSection(scope DocumentScope, scopeID, slotKey, sectionID string, cascade bool, expectedVersion int) error {
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return err
	}
	svc := taskdocs.NewSectionServiceWithPath(basePath)
	if err := svc.DeleteSection(sectionID, cascade, expectedVersion); err != nil {
		return WrapError(scope, scopeID, slotKey, err)
	}
	return nil
}
