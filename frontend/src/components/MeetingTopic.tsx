import React, { useState, useEffect } from 'react';
import { Card, Typography, Spin, Empty } from 'antd';
import { BulbOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown'; // keep for custom styling
import remarkGfm from 'remark-gfm';
import { authedApi } from '../api/auth';

const { Title } = Typography;

interface MeetingTopicProps {
  taskId: string;
}

export const MeetingTopic: React.FC<MeetingTopicProps> = ({ taskId }) => {
  const [topicContent, setTopicContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [topicExists, setTopicExists] = useState(false);

  // 加载会议主题内容
  const loadTopicContent = async () => {
    if (!taskId) return;
    setLoading(true);
    try {
      const response = await authedApi.get(`/tasks/${taskId}/topic`);
      setTopicContent(response.data.content || '');
      setTopicExists(response.data.exists || false);
    } catch (error) {
      console.error('Failed to load topic content:', error);
      setTopicExists(false);
    } finally {
      setLoading(false);
    }
  };

  // 当taskId改变时加载内容
  useEffect(() => {
    loadTopicContent();
  }, [taskId]);

  // 如果topic.md不存在，不显示组件
  if (!topicExists && !loading) {
    return null;
  }

  return (
    <Card
      style={{ 
        marginTop: 16,
        borderRadius: 8,
        boxShadow: '0 2px 8px rgba(0,0,0,0.1)'
      }}
      bodyStyle={{ padding: 16 }}
    >
      <div style={{ 
        display: 'flex', 
        alignItems: 'center', 
        marginBottom: 12,
        gap: 8
      }}>
        <BulbOutlined style={{ 
          color: '#ff9500', 
          fontSize: '16px' 
        }} />
        <Title 
          level={5} 
          style={{ 
            margin: 0, 
            color: '#ff9500',
            fontWeight: 600
          }}
        >
          会议主题提取
        </Title>
      </div>

      {loading ? (
        <div style={{ 
          textAlign: 'center', 
          padding: '20px 0' 
        }}>
          <Spin size="small" />
          <div style={{ 
            marginTop: 8, 
            fontSize: '12px', 
            color: '#999' 
          }}>
            正在加载主题内容...
          </div>
        </div>
      ) : topicContent ? (
        <div style={{
          backgroundColor: '#fff7e6',
          border: '1px solid #ffd591',
          borderRadius: 6,
          padding: 12,
          fontSize: '14px',
          lineHeight: '1.6'
        }}>
          <ReactMarkdown
            remarkPlugins={[remarkGfm] as any}
            components={{
              p: ({ children }) => (
                <p style={{ margin: '8px 0', color: '#8c4400' }}>{children}</p>
              ),
              h1: ({ children }) => (
                <h1 style={{ fontSize: '16px', color: '#d46b08', margin: '12px 0 8px 0' }}>{children}</h1>
              ),
              h2: ({ children }) => (
                <h2 style={{ fontSize: '15px', color: '#d46b08', margin: '10px 0 6px 0' }}>{children}</h2>
              ),
              h3: ({ children }) => (
                <h3 style={{ fontSize: '14px', color: '#d46b08', margin: '8px 0 4px 0' }}>{children}</h3>
              ),
              ul: ({ children }) => (
                <ul style={{ margin: '8px 0', paddingLeft: 20, color: '#8c4400' }}>{children}</ul>
              ),
              ol: ({ children }) => (
                <ol style={{ margin: '8px 0', paddingLeft: 20, color: '#8c4400' }}>{children}</ol>
              ),
              li: ({ children }) => (
                <li style={{ margin: '2px 0' }}>{children}</li>
              ),
              strong: ({ children }) => (
                <strong style={{ color: '#d46b08', fontWeight: 600 }}>{children}</strong>
              ),
              em: ({ children }) => (
                <em style={{ color: '#ad6800' }}>{children}</em>
              )
            }}
          >
            {topicContent}
          </ReactMarkdown>
        </div>
      ) : (
        <Empty 
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={
            <span style={{ color: '#999', fontSize: '12px' }}>
              暂无主题提取内容
            </span>
          }
          style={{ margin: '10px 0' }}
        />
      )}

      <div style={{ 
        fontSize: '11px', 
        color: '#999', 
        marginTop: 8,
        textAlign: 'center'
      }}>
        💡 AI自动提取的会议主题和关键议题
      </div>
    </Card>
  );
};