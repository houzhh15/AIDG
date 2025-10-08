import React, { useState, useEffect } from 'react';
import { message, Typography, Button, Spin, Space } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { AnnotatableMarkdown } from './AnnotatableMarkdown';
import { authedApi } from '../api/auth';

const { Title } = Typography;

interface MeetingDetailsProps {
  taskId: string;
}

export const MeetingDetails: React.FC<MeetingDetailsProps> = ({ taskId }) => {
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(false);

  // 加载会议详情内容 (polish_all.md)
  const loadContent = async () => {
    if (!taskId) return;
    setLoading(true);
    try {
      const response = await authedApi.get(`/tasks/${taskId}/polish`);
      setContent(response.data.content || '');
    } catch (error: any) {
      // 如果文件不存在，显示提示信息
      setContent('');
      console.error('Failed to load meeting details:', error);
      setContent('');
    } finally {
      setLoading(false);
    }
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
        请选择一个任务以查看会议详情
      </div>
    );
  }

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexShrink: 0 }}>
        <Title level={4} style={{ margin: 0 }}>会议详情</Title>
        <Space>
          <Button 
            size="small" 
            icon={<ReloadOutlined />} 
            onClick={loadContent}
            loading={loading}
          >
            刷新
          </Button>
        </Space>
      </div>
      
      <div style={{ 
        flex: 1, 
        overflow: 'auto', 
        background: '#fafafa', 
        padding: 16, 
        borderRadius: 6,
        border: '1px solid #f0f0f0',
        minHeight: 0  // 确保flex容器能正确overflow
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
          <div style={{ 
            fontSize: '14px', 
            lineHeight: '1.6',
            color: '#333'
          }}>
            <AnnotatableMarkdown 
              content={content} 
              taskId={taskId}
              editable={true}
            />
          </div>
        ) : (
          <div style={{ 
            textAlign: 'center', 
            color: '#999', 
            padding: '60px 20px',
            fontSize: '14px'
          }}>
            <p>暂无会议详情内容</p>
          </div>
        )}
      </div>
      
      <div style={{ 
        fontSize: '12px', 
        color: '#999', 
        flexShrink: 0
      }}>
        会议详情由AI自动分析转录内容生成
      </div>
    </div>
  );
};
