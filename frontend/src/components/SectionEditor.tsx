import React, { useState, useEffect } from 'react'
import { Alert, Button, Input, Layout, Space, Spin, message, Modal } from 'antd'
import SectionTree, { FULL_DOCUMENT_ID } from './SectionTree'
import SectionContentEditor from './SectionContentEditor'
import { getTaskSections, getTaskSection, updateTaskSection, updateTaskSectionFull, getTaskDocument, saveTaskDocument } from '../api/tasks'
import type { SectionMeta, SectionContent } from '../types/section'
import { useTaskRefresh } from '../contexts/TaskRefreshContext'

const { Sider, Content } = Layout

interface Props {
  projectId: string
  taskId: string
  docType: string
  initialSectionId?: string  // 新增：初始选中的章节ID
  initialSectionTitle?: string  // 新增：初始选中的章节标题（将根据标题查找ID）
  onCancel?: () => void
  onSave?: () => void  // 新增：保存成功后的回调
  onUnsavedChanges?: (hasChanges: boolean) => void  // 新增：通知父组件是否有未保存的更改
}

const SectionEditor: React.FC<Props> = ({ projectId, taskId, docType, initialSectionId, initialSectionTitle, onCancel, onSave: onSaveCallback, onUnsavedChanges }) => {
  const [sections, setSections] = useState<SectionMeta | null>(null)
  const [currentSectionId, setCurrentSectionId] = useState<string | null>(null)
  const [sectionContent, setSectionContent] = useState<SectionContent | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [isFullEditMode, setIsFullEditMode] = useState(false) // 新增：是否为全文编辑模式
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false) // 新增：跟踪是否有未保存的更改
  const [documentVersion, setDocumentVersion] = useState<number | null>(null)
  const [conflict, setConflict] = useState<{ remoteContent: string; remoteVersion: number } | null>(null)
  
  // 获取刷新函数
  const { triggerRefreshForMultiple } = useTaskRefresh()

  // 通知父组件 hasUnsavedChanges 的变化
  useEffect(() => {
    if (onUnsavedChanges) {
      onUnsavedChanges(hasUnsavedChanges)
    }
  }, [hasUnsavedChanges, onUnsavedChanges])

  // 加载章节列表
  useEffect(() => {
    setConflict(null)
    loadSections()
  }, [projectId, taskId, docType])

  // 设置初始选中的章节
  useEffect(() => {
    if (sections) {
      // 如果提供了章节ID，直接使用
      if (initialSectionId) {
        setCurrentSectionId(initialSectionId)
      }
      // 如果提供了章节标题，根据标题查找章节ID
      else if (initialSectionTitle) {
        console.log('[SectionEditor] Searching for section with title:', initialSectionTitle)
        console.log('[SectionEditor] Available sections:', sections.sections.map(s => ({ id: s.id, title: s.title })))
        
        // 规范化标题：移除 Markdown 标题符号和多余空格
        const normalizeTitle = (title: string) => {
          return title
            .replace(/^#+\s+/, '') // 移除开头的 # 符号和空格
            .trim()
        }
        
        const normalizedSearch = normalizeTitle(initialSectionTitle)
        console.log('[SectionEditor] Normalized search title:', normalizedSearch)
        
        // 先尝试精确匹配（忽略 Markdown 标题符号）
        const section = sections.sections.find(s => normalizeTitle(s.title) === normalizedSearch)
        
        if (section) {
          console.log('[SectionEditor] Found exact match:', section.id, section.title)
          setCurrentSectionId(section.id)
        } else {
          console.log('[SectionEditor] No exact match found, trying partial match...')
          // 尝试部分匹配
          const matchedSection = sections.sections.find(s => {
            const normalized = normalizeTitle(s.title)
            return normalized.includes(normalizedSearch) || normalizedSearch.includes(normalized)
          })
          
          if (matchedSection) {
            console.log('[SectionEditor] Found partial match:', matchedSection.id, matchedSection.title)
            setCurrentSectionId(matchedSection.id)
          } else {
            console.log('[SectionEditor] No match found at all')
          }
        }
      }
    }
  }, [initialSectionId, initialSectionTitle, sections])

  // 加载章节内容（依赖任务参数，确保任务切换时重新加载）
  useEffect(() => {
    if (currentSectionId) {
      loadSectionContent(currentSectionId)
    }
  }, [currentSectionId, projectId, taskId, docType])

  const loadSections = async () => {
    setLoading(true)
    try {
      const response = await getTaskSections(projectId, taskId, docType)
      setSections(response)
      setDocumentVersion(response.version)

      // 如果当前没有选中任何章节，并且没有提供初始章节，才自动选中第一个章节
      // 这样可以避免覆盖用户通过 initialSectionTitle/initialSectionId 指定的章节
      if (!currentSectionId && !initialSectionId && !initialSectionTitle && response.sections.length > 0) {
        setCurrentSectionId(response.sections[0].id)
      }
    } catch (error) {
      message.error('加载章节列表失败')
      console.error(error)
    } finally {
      setLoading(false)
    }
  }

  const loadSectionContent = async (sectionId: string) => {
    setLoading(true)
    try {
      // 检查是否为"全文"模式
      if (sectionId === FULL_DOCUMENT_ID) {
        console.log('[SectionEditor] Loading full document...')
        // 加载整个 compiled.md
        const response = await getTaskDocument(projectId, taskId, docType as 'requirements' | 'design' | 'test')
        console.log('[SectionEditor] Full document loaded, length:', response.content.length)
        console.log('[SectionEditor] Content preview (first 200 chars):', response.content.substring(0, 200))
        setIsFullEditMode(true)
        if (typeof response.version !== 'number') throw new Error('document revision is unavailable')
        setDocumentVersion(response.version)
        setSectionContent({
          id: FULL_DOCUMENT_ID,
          title: '📄 全文',
          content: response.content,
          level: 0,
          order: 0,
          parent_id: null,
          file: '',
          children: [],
          hash: '',
          children_content: []
        })
      } else {
        // 检查是否为父章节（有子章节）
        const section = sections?.sections.find(s => s.id === sectionId)
        const hasChildren = section && section.children && section.children.length > 0

        if (hasChildren) {
          // 全文编辑模式：获取包含所有子章节的完整内容
          const response = await getTaskSection(projectId, taskId, docType, sectionId, true)
          // 拼接父章节和所有子章节内容
          const compiledContent = compileFullText(response)
          setIsFullEditMode(true)
          setSectionContent({
            ...response,
            content: compiledContent
          })
        } else {
          // 单章节编辑模式
          const response = await getTaskSection(projectId, taskId, docType, sectionId, false)
          setIsFullEditMode(false)
          setSectionContent(response)
        }
      }
      
      // 加载新章节内容时，重置未保存状态
      setHasUnsavedChanges(false)
    } catch (error) {
      message.error('加载章节内容失败')
      console.error(error)
    } finally {
      setLoading(false)
    }
  }

  const handleSectionSelect = (sectionId: string) => {
    // 如果有未保存的更改，提示用户
    if (hasUnsavedChanges) {
      Modal.confirm({
        title: '未保存的更改',
        content: '当前章节有未保存的更改，切换章节将丢失这些更改。是否要保存？',
        okText: '保存',
        cancelText: '不保存',
        onOk: async () => {
          // 保存当前章节
          const saved = await handleSave()
          if (!saved) return
          // 仅保存成功后切换章节；409/422 等失败继续保留当前草稿。
          setCurrentSectionId(sectionId)
          setHasUnsavedChanges(false)
        },
        onCancel: () => {
          // 不保存，直接切换章节
          setCurrentSectionId(sectionId)
          setHasUnsavedChanges(false)
        }
      })
    } else {
      // 没有未保存的更改，直接切换
      setCurrentSectionId(sectionId)
    }
  }

  const handleContentChange = (content: string) => {
    if (sectionContent) {
      setSectionContent({ ...sectionContent, content })
      setHasUnsavedChanges(true) // 标记有未保存的更改
    }
  }

  const handleSave = async () => {
    if (!sectionContent) return false
    
    // 全文模式不需要 sections
    if (sectionContent.id !== FULL_DOCUMENT_ID && !sections) return

    setSaving(true)
    try {
      // 检查是否为"全文"模式
      if (sectionContent.id === FULL_DOCUMENT_ID) {
        console.log('[SectionEditor] Saving full document, content length:', sectionContent.content.length)
        console.log('[SectionEditor] Content preview (first 200 chars):', sectionContent.content.substring(0, 200))
        
        // 全文档模式：使用加载草稿时捕获的基线版本，禁止保存前重新 GET 扩大覆盖窗口。
        if (documentVersion === null) throw new Error('document revision is unavailable')
        await saveTaskDocument(projectId, taskId, docType as 'requirements' | 'design' | 'test', sectionContent.content, documentVersion)
        message.success('保存成功')
        
        console.log('[SectionEditor] Save completed, reloading sections...')
        // 重新加载章节列表
        await loadSections()
        
        console.log('[SectionEditor] Sections reloaded, now reloading full document content...')
        // 重要：全文保存后，保持"全文"视图，重新加载全文内容
        await loadSectionContent(FULL_DOCUMENT_ID)
        console.log('[SectionEditor] Full document reloaded')
      } else if (isFullEditMode) {
        // 章节全文编辑模式：调用全文更新API
        await updateTaskSectionFull(
          projectId,
          taskId,
          docType,
          sectionContent.id,
          sectionContent.content,
          sections!.version  // 已在上面检查了 sections 不为 null
        )
        message.success('保存成功，已重新拆分章节')
        
        // 重新加载章节列表
        await loadSections()
      } else {
        // 单章节编辑模式：调用普通更新API
        await updateTaskSection(
          projectId,
          taskId,
          docType,
          sectionContent.id,
          sectionContent.content,
          sections!.version  // 已在上面检查了 sections 不为 null
        )
        message.success('保存成功')
        
        // 重新加载章节列表
        await loadSections()
      }
      
      // 通知父组件刷新文档
      if (onSaveCallback) {
        onSaveCallback()
      }
      
      // 触发全局刷新事件，让其他组件也能响应
      triggerRefreshForMultiple(['task-document', 'task-detail'])
      
      // 重置未保存状态
      setHasUnsavedChanges(false)
      return true
    } catch (error: any) {
      if (error.response?.status === 409) {
        message.error('版本冲突：本地草稿已保留。请刷新远端版本、对比差异后再重试')
        try {
          const remote = await getTaskDocument(projectId, taskId, docType as 'requirements' | 'design' | 'test')
          if (typeof remote.version === 'number') {
            setConflict({ remoteContent: remote.content || '', remoteVersion: remote.version })
          }
        } catch {
          // 对比加载失败不应清除本地草稿。
        }
      } else {
        message.error('保存失败')
      }
      console.error(error)
      return false
    } finally {
      setSaving(false)
    }
  }

  const handleCancel = () => {
    // 如果有未保存的更改，提示用户
    if (hasUnsavedChanges) {
      Modal.confirm({
        title: '未保存的更改',
        content: '当前有未保存的更改，关闭将丢失这些更改。是否要保存？',
        okText: '保存',
        cancelText: '不保存',
        onOk: async () => {
          // 仅保存成功后关闭；冲突时继续停留并保留草稿。
          const saved = await handleSave()
          if (!saved) return Promise.reject(new Error('save failed'))
          setHasUnsavedChanges(false)
          onCancel?.()
        },
        onCancel: () => {
          // 不保存，直接关闭
          setHasUnsavedChanges(false)
          if (onCancel) {
            onCancel()
          }
        }
      })
    } else {
      // 没有未保存的更改，直接关闭
      if (onCancel) {
        onCancel()
      }
    }
  }

  if (loading && !sections) {
    return (
      <div style={{ textAlign: 'center', padding: '50px' }}>
        <Spin tip="加载中..." />
      </div>
    )
  }

  return (
    <Layout style={{ height: '100%', overflow: 'hidden' }}>
      {conflict && (
        <Alert
          type="warning"
          showIcon
          style={{ position: 'absolute', top: 8, left: 316, right: 16, zIndex: 10 }}
          message={`检测到远端新版本 v${conflict.remoteVersion}；本地草稿仍保留`}
          description={(
            <Space wrap>
              <Button size="small" onClick={() => Modal.info({
                width: 900,
                title: '本地草稿与远端版本对比',
                content: (
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <strong>本地草稿</strong>
                    <Input.TextArea rows={7} value={sectionContent?.content || ''} readOnly />
                    <strong>远端 v{conflict.remoteVersion}</strong>
                    <Input.TextArea rows={7} value={conflict.remoteContent} readOnly />
                  </Space>
                ),
              })}>对比差异</Button>
              <Button size="small" type="primary" onClick={() => {
                setDocumentVersion(conflict.remoteVersion)
                setConflict(null)
                message.info('已更新保存基线；本地草稿未改变，请确认后重试保存')
              }}>以远端版本为新基线</Button>
            </Space>
          )}
        />
      )}
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
          projectId={projectId}
          taskId={taskId}
          docType={docType as 'requirements' | 'design' | 'test'}
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
  )
}

// 拼接父章节及所有子章节的完整文本
function compileFullText(section: SectionContent): string {
  let text = section.title + '\n\n'
  
  // 父章节的直接内容（通常为空）
  if (section.content) {
    text += section.content + '\n\n'
  }
  
  // 递归拼接所有子章节
  if (section.children_content && section.children_content.length > 0) {
    text += compileChildren(section.children_content)
  }
  
  return text.trim()
}

// 递归拼接子章节
function compileChildren(children: SectionContent[]): string {
  let text = ''
  for (const child of children) {
    text += child.title + '\n\n'
    text += child.content + '\n\n'
    
    // 递归处理孙章节
    if (child.children_content && child.children_content.length > 0) {
      text += compileChildren(child.children_content)
    }
  }
  return text
}

export default SectionEditor
