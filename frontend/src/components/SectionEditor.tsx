import React, { useState, useEffect } from 'react'
import { Layout, Spin, message } from 'antd'
import SectionTree from './SectionTree'
import SectionContentEditor from './SectionContentEditor'
import { getTaskSections, getTaskSection, updateTaskSection, updateTaskSectionFull } from '../api/tasks'
import type { SectionMeta, SectionContent } from '../types/section'

const { Sider, Content } = Layout

interface Props {
  projectId: string
  taskId: string
  docType: string
  onCancel?: () => void
  onSave?: () => void  // 新增：保存成功后的回调
}

const SectionEditor: React.FC<Props> = ({ projectId, taskId, docType, onCancel, onSave: onSaveCallback }) => {
  const [sections, setSections] = useState<SectionMeta | null>(null)
  const [currentSectionId, setCurrentSectionId] = useState<string | null>(null)
  const [sectionContent, setSectionContent] = useState<SectionContent | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [isFullEditMode, setIsFullEditMode] = useState(false) // 新增：是否为全文编辑模式

  // 加载章节列表
  useEffect(() => {
    loadSections()
  }, [projectId, taskId, docType])

  // 加载章节内容
  useEffect(() => {
    if (currentSectionId) {
      loadSectionContent(currentSectionId)
    }
  }, [currentSectionId])

  const loadSections = async () => {
    setLoading(true)
    try {
      const response = await getTaskSections(projectId, taskId, docType)
      setSections(response)

      // 自动选中第一个章节
      if (response.sections.length > 0) {
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
    } catch (error) {
      message.error('加载章节内容失败')
      console.error(error)
    } finally {
      setLoading(false)
    }
  }

  const handleSectionSelect = (sectionId: string) => {
    // 如果有未保存的更改，提示用户
    // TODO: 实现未保存检测
    setCurrentSectionId(sectionId)
  }

  const handleContentChange = (content: string) => {
    if (sectionContent) {
      setSectionContent({ ...sectionContent, content })
    }
  }

  const handleSave = async () => {
    if (!sectionContent || !sections) return

    setSaving(true)
    try {
      if (isFullEditMode) {
        // 全文编辑模式：调用全文更新API
        await updateTaskSectionFull(
          projectId,
          taskId,
          docType,
          sectionContent.id,
          sectionContent.content,
          sections.version
        )
        message.success('保存成功，已重新拆分章节')
      } else {
        // 单章节编辑模式：调用普通更新API
        await updateTaskSection(
          projectId,
          taskId,
          docType,
          sectionContent.id,
          sectionContent.content,
          sections.version
        )
        message.success('保存成功')
      }

      // 重新加载章节列表（版本号已更新）
      await loadSections()
      
      // 通知父组件刷新文档
      if (onSaveCallback) {
        onSaveCallback()
      }
    } catch (error: any) {
      if (error.response?.status === 409) {
        message.error('版本冲突，请刷新后重试')
      } else {
        message.error('保存失败')
      }
      console.error(error)
    } finally {
      setSaving(false)
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

      <Content style={{ padding: '0 16px' }}>
        {sectionContent ? (
          <SectionContentEditor
            section={sectionContent}
            onContentChange={handleContentChange}
            onSave={handleSave}
            onCancel={onCancel}
            saving={saving}
            isFullEditMode={isFullEditMode}  // 传递全文编辑模式状态
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
