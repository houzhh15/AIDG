/**
 * 项目文档章节编辑器
 * 与任务文档的 SectionEditor 保持相同的 UI 风格
 */
import React, { useState, useEffect } from 'react';
import { Layout, Spin, message, Modal } from 'antd';
import SectionTree, { FULL_DOCUMENT_ID } from '../SectionTree';
import SectionContentEditor from '../SectionContentEditor';
import { 
  getProjectDocSections, 
  getProjectDocSection, 
  updateProjectDocSection,
  exportProjectDoc,
  replaceProjectDocFull,
  type ProjectDocSlot,
} from '../../api/projectDocs';
import { SectionMeta, SectionContent } from '../../types/section';
import { useTaskRefresh } from '../../contexts/TaskRefreshContext';

const { Sider, Content } = Layout;

interface Props {
  projectId: string;
  slot: ProjectDocSlot;
  initialSectionId?: string;
  initialSectionTitle?: string;
  onCancel?: () => void;
  onSave?: () => void;
}

const ProjectDocSectionEditor: React.FC<Props> = ({ 
  projectId, 
  slot, 
  initialSectionId, 
  initialSectionTitle, 
  onCancel, 
  onSave: onSaveCallback 
}) => {
  const [sections, setSections] = useState<SectionMeta | null>(null);
  const [currentSectionId, setCurrentSectionId] = useState<string | null>(null);
  const [sectionContent, setSectionContent] = useState<SectionContent | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [isFullEditMode, setIsFullEditMode] = useState(false);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);

  const { triggerRefreshFor } = useTaskRefresh();

  // 构造一个完整的 SectionContent 对象（用于编辑器）
  const makeSectionContent = (id: string, title: string, content: string, level: number): SectionContent => ({
    id,
    title,
    content,
    level,
    order: 0,
    parent_id: null,
    file: '',
    children: [],
    hash: '',
  });

  // 加载章节列表
  useEffect(() => {
    loadSections();
  }, [projectId, slot]);

  // 设置初始选中的章节
  useEffect(() => {
    if (sections) {
      if (initialSectionId) {
        setCurrentSectionId(initialSectionId);
      } else if (initialSectionTitle) {
        const normalizeTitle = (title: string) => title.replace(/^#+\s+/, '').trim();
        const normalizedSearch = normalizeTitle(initialSectionTitle);
        const section = sections.sections.find(s => normalizeTitle(s.title) === normalizedSearch);
        if (section) {
          setCurrentSectionId(section.id);
        }
      }
    }
  }, [initialSectionId, initialSectionTitle, sections]);

  // 加载章节内容
  useEffect(() => {
    if (currentSectionId) {
      loadSectionContent(currentSectionId);
    }
  }, [currentSectionId, projectId, slot]);

  const loadSections = async () => {
    setLoading(true);
    try {
      const response = await getProjectDocSections(projectId, slot);
      setSections(response);

      // 如果没有选中章节，默认选中"全文"
      if (!currentSectionId && !initialSectionId && !initialSectionTitle) {
        setCurrentSectionId(FULL_DOCUMENT_ID);
      }
    } catch {
      // 如果没有章节数据，默认进入全文编辑模式
      setSections({ 
        version: 0, 
        updated_at: '', 
        root_level: 1,
        sections: [],
        etag: ''
      });
      setCurrentSectionId(FULL_DOCUMENT_ID);
    } finally {
      setLoading(false);
    }
  };

  const loadSectionContent = async (sectionId: string) => {
    setLoading(true);
    try {
      if (sectionId === FULL_DOCUMENT_ID) {
        // 加载全文
        const response = await exportProjectDoc(projectId, slot);
        setIsFullEditMode(true);
        setSectionContent(makeSectionContent(
          FULL_DOCUMENT_ID,
          '📄 全文',
          response.content || '',
          0
        ));
      } else {
        // 检查是否有子章节
        const section = sections?.sections.find(s => s.id === sectionId);
        const hasChildren = section && section.children && section.children.length > 0;

        if (hasChildren) {
          const response = await getProjectDocSection(projectId, slot, sectionId, true);
          const compiledContent = compileFullText(response);
          setIsFullEditMode(true);
          setSectionContent({
            ...response,
            content: compiledContent
          });
        } else {
          const response = await getProjectDocSection(projectId, slot, sectionId, false);
          setIsFullEditMode(false);
          setSectionContent(response);
        }
      }
      setHasUnsavedChanges(false);
    } catch {
      message.error('加载章节内容失败');
    } finally {
      setLoading(false);
    }
  };

  const handleSectionSelect = (sectionId: string) => {
    if (hasUnsavedChanges) {
      Modal.confirm({
        title: '未保存的更改',
        content: '当前章节有未保存的更改，切换章节将丢失这些更改。是否要保存？',
        okText: '保存',
        cancelText: '不保存',
        onOk: async () => {
          await handleSave();
          setCurrentSectionId(sectionId);
          setHasUnsavedChanges(false);
        },
        onCancel: () => {
          setCurrentSectionId(sectionId);
          setHasUnsavedChanges(false);
        }
      });
    } else {
      setCurrentSectionId(sectionId);
    }
  };

  const handleContentChange = (content: string) => {
    if (sectionContent) {
      setSectionContent({ ...sectionContent, content });
      setHasUnsavedChanges(true);
    }
  };

  const handleSave = async () => {
    if (!sectionContent) return;

    setSaving(true);
    try {
      if (sectionContent.id === FULL_DOCUMENT_ID || isFullEditMode) {
        // 全文模式：直接保存整个文档
        await replaceProjectDocFull(projectId, slot, sectionContent.content);
        message.success('保存成功');
        await loadSections();
        
        // 如果是全文模式，重新加载全文
        if (sectionContent.id === FULL_DOCUMENT_ID) {
          await loadSectionContent(FULL_DOCUMENT_ID);
        }
      } else {
        // 单章节编辑模式
        await updateProjectDocSection(
          projectId,
          slot,
          sectionContent.id,
          sectionContent.content,
          sections?.version
        );
        message.success('保存成功');
        await loadSections();
      }

      if (onSaveCallback) {
        onSaveCallback();
      }

      triggerRefreshFor('project-document');
      setHasUnsavedChanges(false);
    } catch (error: unknown) {
      const err = error as { response?: { status?: number } };
      if (err.response?.status === 409) {
        message.error('版本冲突，请刷新后重试');
      } else {
        message.error('保存失败');
      }
      console.error(error);
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setHasUnsavedChanges(false);
    if (onCancel) {
      onCancel();
    }
  };

  if (loading && !sections) {
    return (
      <div style={{ textAlign: 'center', padding: '50px' }}>
        <Spin tip="加载中..." />
      </div>
    );
  }

  return (
    <Layout style={{ height: '100%', overflow: 'hidden' }}>
      {/* 左侧：章节树 */}
      <Sider
        width={300}
        theme="light"
        style={{
          borderRight: '1px solid #f0f0f0',
          position: 'sticky',
          top: 0,
          height: '100vh',
          overflowY: 'auto',
          overflowX: 'hidden'
        }}
      >
        <SectionTree
          sections={sections?.sections || []}
          selectedSectionId={currentSectionId}
          onSelect={handleSectionSelect}
        />
      </Sider>

      {/* 主内容区：编辑器 */}
      <Content style={{ padding: '0 16px', position: 'relative' }}>
        {sectionContent ? (
          <SectionContentEditor
            section={sectionContent}
            onContentChange={handleContentChange}
            onSave={handleSave}
            onCancel={handleCancel}
            saving={saving}
            isFullEditMode={isFullEditMode}
          />
        ) : (
          <div style={{ padding: 24, textAlign: 'center', color: '#999' }}>
            请选择一个章节
          </div>
        )}
      </Content>
    </Layout>
  );
};

// 拼接父章节及所有子章节的完整文本
function compileFullText(section: SectionContent): string {
  let text = section.title + '\n\n';

  if (section.content) {
    text += section.content + '\n\n';
  }

  if (section.children_content && section.children_content.length > 0) {
    text += compileChildren(section.children_content);
  }

  return text.trim();
}

function compileChildren(children: SectionContent[]): string {
  let text = '';
  for (const child of children) {
    text += child.title + '\n\n';
    text += child.content + '\n\n';

    if (child.children_content && child.children_content.length > 0) {
      text += compileChildren(child.children_content);
    }
  }
  return text;
}

export default ProjectDocSectionEditor;
