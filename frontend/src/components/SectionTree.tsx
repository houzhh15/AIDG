import React from 'react'
import { Tree } from 'antd'
import type { DataNode } from 'antd/es/tree'
import type { Section } from '../types/section'

interface Props {
  sections: Section[]
  selectedSectionId: string | null
  onSelect: (sectionId: string) => void
}

const SectionTree: React.FC<Props> = ({ sections, selectedSectionId, onSelect }) => {
  // 将扁平的 sections 转换为树形结构
  const buildTreeData = (sections: Section[]): DataNode[] => {
    const map = new Map<string, DataNode>()
    const roots: DataNode[] = []

    // 第一遍：创建所有节点
    sections.forEach(section => {
      map.set(section.id, {
        key: section.id,
        title: formatTitle(section.title),
        children: [],
      })
    })

    // 第二遍：建立父子关系
    sections.forEach(section => {
      const node = map.get(section.id)!

      if (section.parent_id) {
        const parent = map.get(section.parent_id)
        if (parent) {
          parent.children = parent.children || []
          parent.children.push(node)
        }
      } else {
        roots.push(node)
      }
    })

    return roots
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

  const treeData = buildTreeData(sections)

  return (
    <div style={{ 
      padding: '16px 8px',
      height: '100%',
      overflowY: 'auto',
      overflowX: 'hidden'
    }}>
      <Tree
        treeData={treeData}
        selectedKeys={selectedSectionId ? [selectedSectionId] : []}
        onSelect={handleSelect}
        defaultExpandAll
        showLine
      />
    </div>
  )
}

export default SectionTree
