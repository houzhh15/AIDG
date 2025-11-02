import React, { useState, useEffect } from 'react';
import { Card, List, Tag, Typography, Space, Tooltip } from 'antd';
import { FileTextOutlined, BulbOutlined } from '@ant-design/icons';
import { getProjectTask } from '../api/tasks';

const { Text, Link } = Typography;

interface Recommendation {
  task_id: string;
  doc_type: string;
  section_id: string;
  title: string;
  similarity: number;
  snippet: string;
}

interface RecommendationPanelProps {
  recommendations: Recommendation[];
  projectId: string;
  onRecommendationClick?: (taskId: string, sectionId: string) => void;
  inDrawer?: boolean; // 是否在抽屉中显示
}

const RecommendationPanel: React.FC<RecommendationPanelProps> = ({
  recommendations,
  projectId,
  onRecommendationClick,
  inDrawer = false
}) => {
  const [taskNames, setTaskNames] = useState<Record<string, string>>({});

  // 获取所有推荐任务的名称
  useEffect(() => {
    const loadTaskNames = async () => {
      const uniqueTaskIds = Array.from(new Set(recommendations.map(r => r.task_id)));
      const names: Record<string, string> = {};
      
      await Promise.all(
        uniqueTaskIds.map(async (taskId) => {
          try {
            const response = await getProjectTask(projectId, taskId);
            names[taskId] = response.data?.name || taskId;
          } catch (error) {
            console.error(`Failed to load task name for ${taskId}:`, error);
            names[taskId] = taskId;
          }
        })
      );
      
      setTaskNames(names);
    };

    if (recommendations.length > 0) {
      loadTaskNames();
    }
  }, [recommendations, projectId]);

  if (!recommendations || recommendations.length === 0) {
    return null;
  }

  const getSimilarityColor = (similarity: number): string => {
    if (similarity >= 0.8) return 'green';
    if (similarity >= 0.7) return 'blue';
    return 'orange';
  };

  const getDocTypeLabel = (docType: string): string => {
    const labels: Record<string, string> = {
      'requirements': '需求',
      'design': '设计',
      'test': '测试'
    };
    return labels[docType] || docType;
  };

  const listContent = (
    <List
      dataSource={recommendations}
      renderItem={(item) => (
        <List.Item style={{ padding: inDrawer ? '12px 0' : undefined }}>
          <List.Item.Meta
            avatar={<FileTextOutlined style={{ fontSize: 20, color: '#1890ff' }} />}
            title={
              <Space>
                <Tooltip title="点击查看该章节的详细内容">
                  <Link
                    onClick={() => onRecommendationClick?.(item.task_id, item.section_id)}
                    style={{ fontWeight: 500 }}
                  >
                    {item.title}
                  </Link>
                </Tooltip>
                <Tag color={getSimilarityColor(item.similarity)}>
                  相似度 {(item.similarity * 100).toFixed(1)}%
                </Tag>
              </Space>
            }
            description={
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {item.snippet}
                </Text>
                <div style={{ marginTop: 4 }}>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    来源: {taskNames[item.task_id] || item.task_id} ({item.task_id}) / {getDocTypeLabel(item.doc_type)}文档 / {item.section_id}
                  </Text>
                </div>
              </div>
            }
          />
        </List.Item>
      )}
    />
  );

  // 如果在抽屉中，直接返回列表，不包装Card
  if (inDrawer) {
    return listContent;
  }

  // 否则返回带Card的版本
  return (
    <Card
      title={
        <Space>
          <BulbOutlined />
          <span>相似历史参考（基于语义检索）</span>
        </Space>
      }
      size="small"
      style={{ marginBottom: 16 }}
    >
      {listContent}
    </Card>
  );
};

export default RecommendationPanel;
