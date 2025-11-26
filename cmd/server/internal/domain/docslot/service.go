package docslot

import (
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
)

// UnifiedDocService 统一文档服务接口
type UnifiedDocService interface {
	// ========== 文档操作（项目/会议作用域） ==========

	// Append 增量追加或全文替换（项目/会议文档）
	Append(scope DocumentScope, scopeID, slotKey, content, user string, expectedVersion *int, op, source string) (*AppendResult, error)

	// Export 导出完整文档内容（项目/会议文档）
	Export(scope DocumentScope, scopeID, slotKey string) (*ExportResult, error)

	// ListChunks 列出 chunk 历史（项目/会议文档）
	ListChunks(scope DocumentScope, scopeID, slotKey string) ([]taskdocs.DocChunk, *taskdocs.DocMeta, error)

	// Squash 压缩合并 chunks（项目/会议文档）
	Squash(scope DocumentScope, scopeID, slotKey, user, source string) (*taskdocs.DocMeta, error)

	// ========== 文档操作（任务作用域） ==========

	// AppendTask 任务文档追加
	AppendTask(projectID, taskID, slotKey, content, user string, expectedVersion *int, op, source string) (*AppendResult, error)

	// ExportTask 导出任务文档
	ExportTask(projectID, taskID, slotKey string) (*ExportResult, error)

	// ListTaskChunks 列出任务文档 chunk 历史
	ListTaskChunks(projectID, taskID, slotKey string) ([]taskdocs.DocChunk, *taskdocs.DocMeta, error)

	// SquashTask 压缩任务文档 chunks
	SquashTask(projectID, taskID, slotKey, user, source string) (*taskdocs.DocMeta, error)

	// ========== 章节操作（项目/会议作用域） ==========

	// GetSections 获取章节列表
	GetSections(scope DocumentScope, scopeID, slotKey string) (*taskdocs.SectionMeta, error)

	// GetSection 获取单个章节
	GetSection(scope DocumentScope, scopeID, slotKey, sectionID string, includeChildren bool) (*taskdocs.SectionContent, error)

	// UpdateSection 更新章节
	UpdateSection(scope DocumentScope, scopeID, slotKey, sectionID, content string, expectedVersion int) error

	// InsertSection 插入章节
	InsertSection(scope DocumentScope, scopeID, slotKey, title, content string, afterSectionID *string, expectedVersion int) (*taskdocs.Section, error)

	// DeleteSection 删除章节
	DeleteSection(scope DocumentScope, scopeID, slotKey, sectionID string, cascade bool, expectedVersion int) error

	// ========== 章节操作（任务作用域） ==========

	// GetTaskSections 获取任务文档章节列表
	GetTaskSections(projectID, taskID, slotKey string) (*taskdocs.SectionMeta, error)

	// GetTaskSection 获取任务文档单个章节
	GetTaskSection(projectID, taskID, slotKey, sectionID string, includeChildren bool) (*taskdocs.SectionContent, error)

	// UpdateTaskSection 更新任务文档章节
	UpdateTaskSection(projectID, taskID, slotKey, sectionID, content string, expectedVersion int) error

	// InsertTaskSection 插入任务文档章节
	InsertTaskSection(projectID, taskID, slotKey, title, content string, afterSectionID *string, expectedVersion int) (*taskdocs.Section, error)

	// DeleteTaskSection 删除任务文档章节
	DeleteTaskSection(projectID, taskID, slotKey, sectionID string, cascade bool, expectedVersion int) error

	// ========== 辅助方法 ==========

	// PathResolver 返回路径解析器
	PathResolver() PathResolver
}

// unifiedDocServiceImpl UnifiedDocService 实现
type unifiedDocServiceImpl struct {
	pathResolver PathResolver         // 路径解析器
	docService   *taskdocs.DocService // 文档服务（chunk 操作）
	basePath     string               // 数据根目录
}

// NewUnifiedDocService 创建统一文档服务实例
func NewUnifiedDocService(basePath string) UnifiedDocService {
	return &unifiedDocServiceImpl{
		pathResolver: NewPathResolver(basePath),
		docService:   taskdocs.NewDocService(),
		basePath:     basePath,
	}
}

// PathResolver 返回路径解析器
func (s *unifiedDocServiceImpl) PathResolver() PathResolver {
	return s.pathResolver
}

// resolveAndValidate 解析并验证路径
// 对于 ScopeTask 会返回错误，需要使用 resolveTaskPath
func (s *unifiedDocServiceImpl) resolveAndValidate(scope DocumentScope, scopeID, slotKey string) (string, error) {
	return s.pathResolver.ResolvePath(scope, scopeID, slotKey)
}

// resolveTaskPath 解析任务文档路径
func (s *unifiedDocServiceImpl) resolveTaskPath(projectID, taskID, slotKey string) (string, error) {
	return s.pathResolver.ResolveTaskPath(projectID, taskID, slotKey)
}

// getSectionService 获取指定路径的章节服务
// basePath 应为项目根目录（data/projects）
func (s *unifiedDocServiceImpl) getSectionService(projectsRoot string) taskdocs.SectionService {
	return taskdocs.NewSectionService(projectsRoot)
}

// getSectionServiceForPath 获取指定文档路径的章节服务（用于项目/会议文档）
// docPath 应为完整文档目录路径
// TODO: 在 step-06 中实现 taskdocs.NewSectionServiceWithPath
func (s *unifiedDocServiceImpl) getSectionServiceForPath(docPath string) taskdocs.SectionService {
	// 临时实现：使用现有构造函数
	return taskdocs.NewSectionService(docPath)
}

// ========== 文档操作方法（step-04 实现） ==========

// Append 增量追加或全文替换（项目/会议文档）
func (s *unifiedDocServiceImpl) Append(scope DocumentScope, scopeID, slotKey, content, user string, expectedVersion *int, op, source string) (*AppendResult, error) {
	// 解析路径
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, err
	}

	// 委托给 DocService
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
	// 解析路径
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, err
	}

	// 读取内容
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
	// 解析路径
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
	// 解析路径
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

// AppendTask 任务文档追加
func (s *unifiedDocServiceImpl) AppendTask(projectID, taskID, slotKey, content, user string, expectedVersion *int, op, source string) (*AppendResult, error) {
	// 解析路径
	basePath, err := s.resolveTaskPath(projectID, taskID, slotKey)
	if err != nil {
		return nil, err
	}

	// 委托给 DocService
	meta, chunk, duplicate, err := s.docService.AppendWithPath(basePath, content, op, user, source, expectedVersion)
	if err != nil {
		return nil, WrapError(ScopeTask, projectID+"/"+taskID, slotKey, err)
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

// ExportTask 导出任务文档
func (s *unifiedDocServiceImpl) ExportTask(projectID, taskID, slotKey string) (*ExportResult, error) {
	// 解析路径
	basePath, err := s.resolveTaskPath(projectID, taskID, slotKey)
	if err != nil {
		return nil, err
	}

	// 读取内容
	content, meta, err := taskdocs.ExportWithPath(basePath)
	if err != nil {
		return nil, WrapError(ScopeTask, projectID+"/"+taskID, slotKey, err)
	}

	return &ExportResult{
		Content:   content,
		Version:   meta.Version,
		ETag:      meta.ETag,
		UpdatedAt: meta.UpdatedAt,
		Exists:    content != "" || meta.Version > 0,
	}, nil
}

// ListTaskChunks 列出任务文档 chunk 历史
func (s *unifiedDocServiceImpl) ListTaskChunks(projectID, taskID, slotKey string) ([]taskdocs.DocChunk, *taskdocs.DocMeta, error) {
	// 解析路径
	basePath, err := s.resolveTaskPath(projectID, taskID, slotKey)
	if err != nil {
		return nil, nil, err
	}

	chunks, meta, err := s.docService.ListWithPath(basePath)
	if err != nil {
		return nil, nil, WrapError(ScopeTask, projectID+"/"+taskID, slotKey, err)
	}

	return chunks, &meta, nil
}

// SquashTask 压缩任务文档 chunks
func (s *unifiedDocServiceImpl) SquashTask(projectID, taskID, slotKey, user, source string) (*taskdocs.DocMeta, error) {
	// 解析路径
	basePath, err := s.resolveTaskPath(projectID, taskID, slotKey)
	if err != nil {
		return nil, err
	}

	meta, err := s.docService.SquashWithPath(basePath, user, source, nil)
	if err != nil {
		return nil, WrapError(ScopeTask, projectID+"/"+taskID, slotKey, err)
	}

	return &meta, nil
}

// ========== 章节操作方法（step-05 实现） ==========

// GetSections 获取章节列表（项目/会议文档）
func (s *unifiedDocServiceImpl) GetSections(scope DocumentScope, scopeID, slotKey string) (*taskdocs.SectionMeta, error) {
	// 解析路径
	basePath, err := s.resolveAndValidate(scope, scopeID, slotKey)
	if err != nil {
		return nil, err
	}

	// 使用 SectionServiceWithPath
	svc := taskdocs.NewSectionServiceWithPath(basePath)
	meta, err := svc.GetSections()
	if err != nil {
		return nil, WrapError(scope, scopeID, slotKey, err)
	}

	return meta, nil
}

// GetSection 获取单个章节（项目/会议文档）
func (s *unifiedDocServiceImpl) GetSection(scope DocumentScope, scopeID, slotKey, sectionID string, includeChildren bool) (*taskdocs.SectionContent, error) {
	// 解析路径
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

// UpdateSection 更新章节（项目/会议文档）
func (s *unifiedDocServiceImpl) UpdateSection(scope DocumentScope, scopeID, slotKey, sectionID, content string, expectedVersion int) error {
	// 解析路径
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

// InsertSection 插入章节（项目/会议文档）
func (s *unifiedDocServiceImpl) InsertSection(scope DocumentScope, scopeID, slotKey, title, content string, afterSectionID *string, expectedVersion int) (*taskdocs.Section, error) {
	// 解析路径
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

// DeleteSection 删除章节（项目/会议文档）
func (s *unifiedDocServiceImpl) DeleteSection(scope DocumentScope, scopeID, slotKey, sectionID string, cascade bool, expectedVersion int) error {
	// 解析路径
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

// GetTaskSections 获取任务文档章节列表
func (s *unifiedDocServiceImpl) GetTaskSections(projectID, taskID, slotKey string) (*taskdocs.SectionMeta, error) {
	// 解析路径
	basePath, err := s.resolveTaskPath(projectID, taskID, slotKey)
	if err != nil {
		return nil, err
	}

	svc := taskdocs.NewSectionServiceWithPath(basePath)
	meta, err := svc.GetSections()
	if err != nil {
		return nil, WrapError(ScopeTask, projectID+"/"+taskID, slotKey, err)
	}

	return meta, nil
}

// GetTaskSection 获取任务文档单个章节
func (s *unifiedDocServiceImpl) GetTaskSection(projectID, taskID, slotKey, sectionID string, includeChildren bool) (*taskdocs.SectionContent, error) {
	// 解析路径
	basePath, err := s.resolveTaskPath(projectID, taskID, slotKey)
	if err != nil {
		return nil, err
	}

	svc := taskdocs.NewSectionServiceWithPath(basePath)
	content, err := svc.GetSection(sectionID, includeChildren)
	if err != nil {
		return nil, WrapError(ScopeTask, projectID+"/"+taskID, slotKey, err)
	}

	return content, nil
}

// UpdateTaskSection 更新任务文档章节
func (s *unifiedDocServiceImpl) UpdateTaskSection(projectID, taskID, slotKey, sectionID, content string, expectedVersion int) error {
	// 解析路径
	basePath, err := s.resolveTaskPath(projectID, taskID, slotKey)
	if err != nil {
		return err
	}

	svc := taskdocs.NewSectionServiceWithPath(basePath)
	if err := svc.UpdateSection(sectionID, content, expectedVersion); err != nil {
		return WrapError(ScopeTask, projectID+"/"+taskID, slotKey, err)
	}

	return nil
}

// InsertTaskSection 插入任务文档章节
func (s *unifiedDocServiceImpl) InsertTaskSection(projectID, taskID, slotKey, title, content string, afterSectionID *string, expectedVersion int) (*taskdocs.Section, error) {
	// 解析路径
	basePath, err := s.resolveTaskPath(projectID, taskID, slotKey)
	if err != nil {
		return nil, err
	}

	svc := taskdocs.NewSectionServiceWithPath(basePath)
	section, err := svc.InsertSection(title, content, afterSectionID, expectedVersion)
	if err != nil {
		return nil, WrapError(ScopeTask, projectID+"/"+taskID, slotKey, err)
	}

	return section, nil
}

// DeleteTaskSection 删除任务文档章节
func (s *unifiedDocServiceImpl) DeleteTaskSection(projectID, taskID, slotKey, sectionID string, cascade bool, expectedVersion int) error {
	// 解析路径
	basePath, err := s.resolveTaskPath(projectID, taskID, slotKey)
	if err != nil {
		return err
	}

	svc := taskdocs.NewSectionServiceWithPath(basePath)
	if err := svc.DeleteSection(sectionID, cascade, expectedVersion); err != nil {
		return WrapError(ScopeTask, projectID+"/"+taskID, slotKey, err)
	}

	return nil
}
