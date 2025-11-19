import React, { useState } from 'react'
import { Tree, Dropdown, Modal, Form, Input, message } from 'antd'
import type { MenuProps } from 'antd'
import type { DataNode } from 'antd/es/tree'
import type { Section } from '../types/section'
import { CopyOutlined, PlusOutlined } from '@ant-design/icons'
import { addCustomResource } from '../api/resourceApi'
import { getTaskSection } from '../api/tasks'
import { loadAuth } from '../api/auth'

interface Props {
  sections: Section[]
  selectedSectionId: string | null
  onSelect: (sectionId: string) => void
  projectId?: string
  taskId?: string
  docType?: 'requirements' | 'design' | 'test'
}

// 特殊的全文模式ID
export const FULL_DOCUMENT_ID = '__FULL_DOCUMENT__'

const SectionTree: React.FC<Props> = ({ sections, selectedSectionId, onSelect, projectId, taskId, docType }) => {
  const [contextMenuSection, setContextMenuSection] = useState<Section | null>(null)
  const [modalVisible, setModalVisible] = useState(false)
  const [form] = Form.useForm()
  const [saving, setSaving] = useState(false)

  // 将扁平的 sections 转换为树形结构
  const buildTreeData = (sections: Section[]): DataNode[] => {
    console.log(`[SectionTree] 构建树结构，sections数量: ${sections.length}`)
    const map = new Map<string, DataNode>()
    const roots: DataNode[] = []

    // 第一遍：创建所有节点
    sections.forEach(section => {
      const node: DataNode = {
        key: section.id,
        title: formatTitle(section.title),
        children: [],
      }
      map.set(section.id, node)
    })

    // 检测循环引用的辅助函数
    const hasCircularReference = (sectionId: string, parentId: string, visited = new Set<string>()): boolean => {
      // 如果父节点就是自己，或者已经访问过这个父节点，说明有循环
      if (parentId === sectionId || visited.has(parentId)) {
        return true
      }
      
      const parent = sections.find(s => s.id === parentId)
      if (!parent || !parent.parent_id) {
        return false // 到达根节点
      }
      
      visited.add(parentId)
      return hasCircularReference(sectionId, parent.parent_id, visited)
    }

    // 第二遍：建立父子关系，跳过有循环引用的节点
    sections.forEach(section => {
      const node = map.get(section.id)!

      if (section.parent_id) {
        // 检查是否存在循环引用
        if (hasCircularReference(section.id, section.parent_id)) {
          console.error(`[SectionTree] 数据中存在循环引用: 节点 ${section.id}(${section.title}) -> 父节点 ${section.parent_id}，将其作为根节点处理`)
          // 将其作为根节点处理
          roots.push(node)
          return
        }

        const parent = map.get(section.parent_id)
        if (parent) {
          parent.children = parent.children || []
          parent.children.push(node)
        } else {
          // 父节点不存在，作为根节点
          console.warn(`[SectionTree] 父节点不存在: 节点 ${section.id}(${section.title}) 的父节点 ${section.parent_id}，将其作为根节点处理`)
          roots.push(node)
        }
      } else {
        roots.push(node)
      }
    })

    console.log(`[SectionTree] 构建完成，根节点数量: ${roots.length}`)
    
    // 在最前面添加"全文"节点
    const fullDocNode: DataNode = {
      key: FULL_DOCUMENT_ID,
      title: '📄 全文',
      children: [],
    }

    return [fullDocNode, ...roots]
  }

  // 包装树节点，为非全文节点添加右键菜单（内部递归函数）
  const wrapTreeNodeRecursive = (node: DataNode, visited: Set<React.Key>, path: string[] = []): DataNode => {
    // 防止循环引用
    if (visited.has(node.key)) {
      const pathStr = path.join('->')
      console.warn(`[SectionTree] 检测到循环引用: ${node.key}, 路径: ${pathStr}`)
      // 使用完整路径创建唯一的 key，避免重复
      const uniqueKey = `${node.key}-dup-${pathStr.replace(/[^a-zA-Z0-9]/g, '_')}`
      return {
        ...node,
        key: uniqueKey,
        children: [], // 中断循环
      }
    }
    
    visited.add(node.key)
    const currentPath = [...path, String(node.key)]

    // 如果是全文节点，不添加右键菜单
    if (node.key === FULL_DOCUMENT_ID) {
      return {
        ...node,
        children: node.children?.map(child => wrapTreeNodeRecursive(child, visited, currentPath)),
      }
    }

    // 找到对应的 section
    const section = sections.find(s => s.id === node.key)
    if (!section) {
      return {
        ...node,
        children: node.children?.map(child => wrapTreeNodeRecursive(child, visited, currentPath)),
      }
    }

    // 为节点添加右键菜单
    const titleText = typeof node.title === 'string' ? node.title : String(node.title)
    const wrappedTitle = (
      <Dropdown menu={getContextMenu(section)} trigger={['contextMenu']}>
        <span>{titleText}</span>
      </Dropdown>
    )

    return {
      ...node,
      title: wrappedTitle,
      children: node.children?.map(child => wrapTreeNodeRecursive(child, visited, currentPath)),
    }
  }

  // 包装树节点的入口函数
  const wrapTreeNode = (node: DataNode): DataNode => {
    return wrapTreeNodeRecursive(node, new Set<React.Key>(), [])
  }

  // 格式化标题：去除 Markdown 标记
  const formatTitle = (title: string): string => {
    // 去除 # 标记
    const formatted = title.replace(/^#+\s*/, '')

    // 可选：去除序号前缀（如 "1. "、"1、"）
    // formatted = formatted.replace(/^\d+[.、)]\s*/, '')

    return formatted
  }

  const handleSelect = (selectedKeys: React.Key[]) => {
    if (selectedKeys.length > 0) {
      onSelect(selectedKeys[0] as string)
    }
  }

  // 获取章节及其所有子章节的内容
  const getSectionWithChildren = async (sectionId: string): Promise<string> => {
    if (!projectId || !taskId || !docType) {
      throw new Error('缺少项目、任务或文档类型信息')
    }

    try {
      // 获取章节内容（包含子章节）
      const sectionContent = await getTaskSection(projectId, taskId, docType, sectionId, true)
      
      // 构建完整内容
      let content = `${sectionContent.title}\n\n${sectionContent.content}\n\n`
      
      // 如果有子章节内容，递归添加
      if (sectionContent.children_content && sectionContent.children_content.length > 0) {
        const buildChildrenContent = (children: any[]): string => {
          let childContent = ''
          children.forEach(child => {
            childContent += `${child.title}\n\n${child.content}\n\n`
            if (child.children_content && child.children_content.length > 0) {
              childContent += buildChildrenContent(child.children_content)
            }
          })
          return childContent
        }
        content += buildChildrenContent(sectionContent.children_content)
      }

      return content
    } catch (error) {
      console.error('获取章节内容失败:', error)
      throw error
    }
  }

  // 复制章节名
  const handleCopySectionName = (section: Section) => {
    if (!taskId || !docType) {
      message.error('缺少任务或文档类型信息')
      return
    }

    const docTypeMap = {
      requirements: '需求文档',
      design: '设计文档',
      test: '测试文档'
    }

    const copyText = `${taskId}::${docTypeMap[docType]}::${section.title.replace(/^#+\s*/, '')}`
    
    navigator.clipboard.writeText(copyText).then(() => {
      message.success(`已复制: ${copyText}`)
    }).catch(err => {
      console.error('复制失败:', err)
      message.error('复制失败')
    })
  }

  // 添加到MCP资源
  const handleAddToMCPResource = (section: Section) => {
    setContextMenuSection(section)
    const sectionTitle = section.title.replace(/^#+\s*/, '')
    form.setFieldsValue({
      name: `${sectionTitle} - ${taskId}`,
      description: `来自任务 ${taskId} 的章节内容`,
    })
    setModalVisible(true)
  }

  // 提交MCP资源
  const handleSubmitMCPResource = async () => {
    if (!contextMenuSection) return

    try {
      const values = await form.validateFields()
      const auth = loadAuth()
      if (!auth) {
        message.error('请先登录')
        return
      }

      setSaving(true)

      // 获取章节及其子章节的内容
      const content = await getSectionWithChildren(contextMenuSection.id)

      await addCustomResource(auth.username, {
        name: values.name,
        description: values.description,
        content: content,
        visibility: 'private',
        projectId: projectId,
        taskId: taskId,
      })

      message.success('已添加到MCP资源')
      setModalVisible(false)
      form.resetFields()
    } catch (error: any) {
      console.error('添加MCP资源失败:', error)
      message.error('添加失败: ' + (error.message || '未知错误'))
    } finally {
      setSaving(false)
    }
  }

  // 右键菜单
  const getContextMenu = (section: Section): MenuProps => ({
    items: [
      {
        key: 'copy-name',
        icon: <CopyOutlined />,
        label: '复制章节名',
        onClick: () => handleCopySectionName(section),
      },
      {
        key: 'add-to-mcp',
        icon: <PlusOutlined />,
        label: '添加到MCP资源',
        onClick: () => handleAddToMCPResource(section),
      },
    ],
  })

  const treeData = buildTreeData(sections)
  // 每个顶级节点都有自己的 visited Set，避免不同树之间的节点被误判为循环
  const wrappedTreeData = treeData.map(node => wrapTreeNode(node))

  return (
    <>
      <div style={{ 
        padding: '16px 8px',
        height: '100%',
        overflowY: 'auto',
        overflowX: 'hidden'
      }}>
        <Tree
          treeData={wrappedTreeData}
          selectedKeys={selectedSectionId ? [selectedSectionId] : []}
          onSelect={handleSelect}
          defaultExpandAll
          showLine
        />
      </div>

      {/* 添加到MCP资源的模态框 */}
      <Modal
        title="添加到MCP资源"
        open={modalVisible}
        onOk={handleSubmitMCPResource}
        onCancel={() => {
          setModalVisible(false)
          form.resetFields()
        }}
        confirmLoading={saving}
        okText="添加"
        cancelText="取消"
      >
        <Form
          form={form}
          layout="vertical"
        >
          <Form.Item
            name="name"
            label="资源名称"
            rules={[{ required: true, message: '请输入资源名称' }]}
          >
            <Input placeholder="请输入资源名称" />
          </Form.Item>
          <Form.Item
            name="description"
            label="资源描述"
          >
            <Input.TextArea
              rows={3}
              placeholder="请输入资源描述（可选）"
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}

export default SectionTree
