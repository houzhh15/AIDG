import React, { useState, useEffect } from 'react';
import { message, Typography, Button, Spin, Input, Space } from 'antd';
import { ReloadOutlined, EyeOutlined, EditOutlined, SaveOutlined } from '@ant-design/icons';
import MarkdownViewer from './MarkdownViewer';
import { authedApi } from '../api/auth';

const { TextArea } = Input;

const { Title } = Typography;

interface MeetingSummaryProps {
  taskId: string;
}

export const MeetingSummary: React.FC<MeetingSummaryProps> = ({ taskId }) => {
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [isMarkdownMode, setIsMarkdownMode] = useState(true);

  // 加载会议总结内容 (meeting_summary.md)
  const loadContent = async () => {
    if (!taskId) return;
    setLoading(true);
    try {
      const response = await authedApi.get(`/tasks/${taskId}/meeting-summary`);
      setContent(response.data.content || '');
    } catch (error: any) {
      console.error('Failed to load meeting summary:', error);
      setContent('');
    } finally {
      setLoading(false);
    }
  };

  // 保存会议总结内容
  const saveContent = async (newContent: string) => {
    if (!taskId) return;
    setSaving(true);
    try {
      await authedApi.put(`/tasks/${taskId}/meeting-summary`, { content: newContent });
      message.success('保存成功');
    } catch (error: any) {
      message.error(`保存失败: ${error?.response?.data?.error || error.message}`);
    } finally {
      setSaving(false);
    }
  };

  // 防抖保存 - 已禁用自动保存
  // useEffect(() => {
  //   const timer = setTimeout(() => {
  //     if (content !== '' || taskId) {
  //       saveContent(content);
  //     }
  //   }, 1000); // 1秒后自动保存

  //   return () => clearTimeout(timer);
  // }, [content, taskId]);

  const handleManualSave = () => {
    saveContent(content);
    message.success('已手动保存');
  };

  // 当taskId改变时加载内容
  useEffect(() => {
    loadContent();
  }, [taskId]);

  if (!taskId) {
    return (
      <div style={{ 
        height: '100%', 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'center',
        color: '#999'
      }}>
        请选择一个任务以查看会议总结
      </div>
    );
  }

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexShrink: 0 }}>
        <Title level={4} style={{ margin: 0 }}>会议总结</Title>
        <Space>
          <Button 
            size="small" 
            icon={<ReloadOutlined />} 
            onClick={loadContent}
            loading={loading}
          >
            刷新
          </Button>
          <Button 
            type={isMarkdownMode ? "default" : "primary"}
            icon={isMarkdownMode ? <EditOutlined /> : <EyeOutlined />}
            onClick={() => setIsMarkdownMode(!isMarkdownMode)}
            size="small"
          >
            {isMarkdownMode ? "编辑" : "预览"}
          </Button>
          <Button 
            type="primary" 
            icon={<SaveOutlined />} 
            onClick={handleManualSave}
            disabled={saving}
            size="small"
          >
            保存
          </Button>
        </Space>
      </div>
      
      {isMarkdownMode ? (
        <div style={{ 
          flex: 1, 
          minHeight: '400px', 
          padding: '16px',
          border: '1px solid #d9d9d9',
          borderRadius: '6px',
          backgroundColor: '#fff',
          overflow: 'auto'
        }}>
          {loading ? (
            <div style={{ 
              display: 'flex', 
              justifyContent: 'center', 
              alignItems: 'center', 
              height: '200px' 
            }}>
              <Spin size="large" />
            </div>
          ) : content ? (
            <MarkdownViewer>{content}</MarkdownViewer>
          ) : (
            <div style={{ color: '#999', fontStyle: 'italic' }}>
              暂无内容，请切换到编辑模式添加会议总结信息...
            </div>
          )}
        </div>
      ) : (
        <TextArea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="请输入会议总结信息，支持Markdown格式...

# 会议总结

## 会议基本信息
- **会议时间**: 
- **会议类型**: 
- **参会人数**: 

## 主要议题
1. 
2. 
3. 

## 重要决议
- 
- 

## 行动项
- [ ] 任务描述 (负责人: , 截止时间: )
- [ ] 任务描述 (负责人: , 截止时间: )

## 风险与关注点
- 

## 下次会议
- **时间**: 
- **议题**: "
          style={{ flex: 1, resize: 'none', minHeight: '400px' }}
          autoSize={false}
          disabled={loading}
        />
      )}
      
      <div style={{ 
        fontSize: '12px', 
        color: '#999', 
        display: 'flex', 
        justifyContent: 'space-between',
        flexShrink: 0
      }}>
        <span>会议总结 • 支持Markdown格式 • 自动保存</span>
        <span>
          {saving ? '保存中...' : ''}
        </span>
      </div>
    </div>
  );
};
