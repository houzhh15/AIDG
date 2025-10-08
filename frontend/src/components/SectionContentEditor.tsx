import React, { useState } from 'react'
import { Button, Space, Typography, Input, Alert } from 'antd'
import { SaveOutlined, CloseOutlined } from '@ant-design/icons'
import type { SectionContent } from '../types/section'

const { Title } = Typography
const { TextArea } = Input

interface Props {
  section: SectionContent
  onContentChange: (content: string) => void
  onSave: () => void
  onCancel?: () => void
  saving: boolean
  isFullEditMode?: boolean  // 新增：是否为全文编辑模式
}

const SectionContentEditor: React.FC<Props> = ({
  section,
  onContentChange,
  onSave,
  onCancel,
  saving,
  isFullEditMode = false  // 默认为单章节编辑模式
}) => {
  const [originalContent] = useState(section.content)

  const handleCancel = () => {
    onContentChange(originalContent)
    if (onCancel) {
      onCancel()
    }
  }
  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* 头部：标题和操作按钮 */}
      <div
        style={{
          padding: '16px 0',
          borderBottom: '1px solid #f0f0f0',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center'
        }}
      >
        <Title level={4} style={{ margin: 0 }}>
          {section.title}
          {isFullEditMode && (
            <span style={{ marginLeft: '10px', fontSize: '14px', color: '#1890ff', fontWeight: 'normal' }}>
              [全文编辑]
            </span>
          )}
        </Title>

        <Space>
          <Button
            icon={<CloseOutlined />}
            onClick={handleCancel}
            disabled={saving}
          >
            取消
          </Button>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={onSave}
            loading={saving}
          >
            保存
          </Button>
        </Space>
      </div>

      {/* 全文编辑模式提示 */}
      {isFullEditMode && (
        <Alert
          message="全文编辑模式"
          description="当前正在编辑父章节的完整内容（包含所有子章节）。保存时将自动重新拆分章节结构。请保留所有子章节的标题以确保正确拆分。"
          type="info"
          showIcon
          style={{ marginTop: 16 }}
        />
      )}

      {/* 编辑器 */}
      <div style={{ flex: 1, marginTop: 16 }}>
        <TextArea
          value={section.content}
          onChange={(e) => onContentChange(e.target.value)}
          style={{ 
            height: '100%', 
            fontFamily: 'monospace',
            fontSize: '14px'
          }}
          placeholder="请输入章节内容（Markdown 格式）"
        />
      </div>
    </div>
  )
}

export default SectionContentEditor
