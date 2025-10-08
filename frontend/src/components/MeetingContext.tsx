import React, { useState, useEffect } from 'react';
import { message, Typography, Button, Spin, Space, Input } from 'antd';
import { ReloadOutlined, EyeOutlined, EditOutlined, SaveOutlined } from '@ant-design/icons';
import MarkdownViewer from './MarkdownViewer';
import { MeetingTopic } from './MeetingTopic';
import { authedApi } from '../api/auth';

const { TextArea } = Input;
const { Title } = Typography;

interface MeetingContextProps {
  taskId: string;
}

export const MeetingContext: React.FC<MeetingContextProps> = ({ taskId }) => {
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [isMarkdownMode, setIsMarkdownMode] = useState(true);

  // 加载会议背景内容
  const loadContent = async () => {
    if (!taskId) return;
    setLoading(true);
    try {
      const response = await authedApi.get(`/tasks/${taskId}/meeting-context`);
      setContent(response.data.content || '');
    } catch (error: any) {
      console.error('Failed to load meeting context:', error);
      // 如果文件不存在，不显示错误，只是保持空内容
    } finally {
      setLoading(false);
    }
  };

  // 保存会议背景内容
  const saveContent = async (newContent: string) => {
    if (!taskId) return;
    setSaving(true);
    try {
      await authedApi.put(`/tasks/${taskId}/meeting-context`, { content: newContent });
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

  // 当taskId改变时加载内容
  useEffect(() => {
    loadContent();
  }, [taskId]);

  const handleManualSave = () => {
    saveContent(content);
    message.success('已手动保存');
  };

  if (!taskId) {
    return (
      <div style={{ 
        height: '100%', 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'center',
        color: '#999'
      }}>
        请选择一个任务以编辑会议背景
      </div>
    );
  }

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexShrink: 0 }}>
        <Title level={4} style={{ margin: 0 }}>会议背景</Title>
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
          {content ? (
            <MarkdownViewer>{content}</MarkdownViewer>
          ) : (
            <div style={{ color: '#999', fontStyle: 'italic' }}>
              暂无内容，请切换到编辑模式添加会议背景信息...
            </div>
          )}
        </div>
      ) : (
        <TextArea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="请输入会议背景信息，支持Markdown格式...

## 会议基本信息
- **会议主题**: 
- **会议时间**: 
- **参与人员**: 
- **会议目标**: 

## 会议背景
项目背景、讨论要点、相关文档等...

## 预期成果
希望达成的目标、需要做出的决策等..."
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
        alignItems: 'center',
        flexShrink: 0
      }}>
        <span>内容将自动保存</span>
        {saving && <span>保存中...</span>}
      </div>

      {/* 会议主题提取窗口 */}
      <MeetingTopic taskId={taskId} />
    </div>
  );
};
