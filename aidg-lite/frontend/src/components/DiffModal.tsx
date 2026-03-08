import React from 'react';
import { Modal, Button, Typography, Divider } from 'antd';
import { ExclamationCircleOutlined } from '@ant-design/icons';

const { Title, Text } = Typography;

interface DiffModalProps {
  visible: boolean;
  title: string;
  currentContent: string;
  sourceContent: string;
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}

export const DiffModal: React.FC<DiffModalProps> = ({
  visible,
  title,
  currentContent,
  sourceContent,
  onConfirm,
  onCancel,
  loading = false
}) => {
  const hasCurrentContent = currentContent && currentContent.trim().length > 0;
  const hasSourceContent = sourceContent && sourceContent.trim().length > 0;

  // 简单的差异检测
  const contentDifferent = currentContent.trim() !== sourceContent.trim();

  return (
    <Modal
      title={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <ExclamationCircleOutlined style={{ color: '#faad14' }} />
          {title}
        </div>
      }
      open={visible}
      onCancel={onCancel}
      width={800}
      footer={[
        <Button key="cancel" onClick={onCancel}>
          取消
        </Button>,
        <Button 
          key="confirm" 
          type="primary" 
          onClick={onConfirm}
          loading={loading}
          danger={!!(hasCurrentContent && contentDifferent)}
        >
          {hasCurrentContent ? '确认覆盖' : '确认复制'}
        </Button>
      ]}
    >
      <div style={{ maxHeight: '60vh', overflow: 'auto' }}>
        {hasCurrentContent && contentDifferent && (
          <div style={{ 
            background: '#fff2e8', 
            border: '1px solid #ffbb96',
            borderRadius: 6,
            padding: 12,
            marginBottom: 16
          }}>
            <Text strong style={{ color: '#d4380d' }}>
              ⚠️ 警告：当前任务已存在此文件，复制操作将覆盖现有内容！
            </Text>
          </div>
        )}

        {hasCurrentContent && (
          <>
            <Title level={5} style={{ color: '#fa541c', marginBottom: 8 }}>
              📄 当前内容 (将被覆盖)
            </Title>
            <div style={{
              background: '#fff2e8',
              border: '1px solid #ffbb96',
              borderRadius: 6,
              padding: 12,
              marginBottom: 16,
              maxHeight: '200px',
              overflow: 'auto'
            }}>
              <pre style={{
                fontSize: '12px',
                lineHeight: '1.4',
                margin: 0,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word'
              }}>
                {currentContent || '(空内容)'}
              </pre>
            </div>
          </>
        )}

        <Divider style={{ margin: '16px 0' }} />

        <Title level={5} style={{ color: '#52c41a', marginBottom: 8 }}>
          📥 源内容 (将要复制)
        </Title>
        <div style={{
          background: '#f6ffed',
          border: '1px solid #b7eb8f',
          borderRadius: 6,
          padding: 12,
          maxHeight: '200px',
          overflow: 'auto'
        }}>
          <pre style={{
            fontSize: '12px',
            lineHeight: '1.4',
            margin: 0,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word'
          }}>
            {sourceContent || '(空内容)'}
          </pre>
        </div>

        {!hasCurrentContent && (
          <div style={{ 
            background: '#f0f8ff', 
            border: '1px solid #91d5ff',
            borderRadius: 6,
            padding: 12,
            marginTop: 16
          }}>
            <Text style={{ color: '#1890ff' }}>
              ℹ️ 当前任务尚无此文件，将直接创建新文件。
            </Text>
          </div>
        )}

        {hasCurrentContent && !contentDifferent && (
          <div style={{ 
            background: '#f6ffed', 
            border: '1px solid #b7eb8f',
            borderRadius: 6,
            padding: 12,
            marginTop: 16
          }}>
            <Text style={{ color: '#52c41a' }}>
              ✅ 内容相同，无需复制。
            </Text>
          </div>
        )}
      </div>
    </Modal>
  );
};