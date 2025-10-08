import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, message, Spin, Progress, Typography, Space } from 'antd';
import {
  CheckCircleOutlined,
  SyncOutlined,
  ClockCircleOutlined,
  RiseOutlined,
  FallOutlined
} from '@ant-design/icons';
import { fetchTaskStatistics, TaskDistribution } from '../api/statisticsApi';

const { Text, Title } = Typography;

interface Props {
  projectId: string;
  onTaskClick?: (status: 'completed' | 'in-progress' | 'todo') => void;
}

const TaskDashboard: React.FC<Props> = ({ projectId, onTaskClick }) => {
  const [statistics, setStatistics] = useState<TaskDistribution | null>(null);
  const [loading, setLoading] = useState(false);

  // 加载统计数据
  const loadStatistics = async () => {
    setLoading(true);
    try {
      const response = await fetchTaskStatistics(projectId);
      if (response.success && response.data) {
        setStatistics(response.data);
      } else {
        message.error(response.message || '加载统计数据失败');
      }
    } catch (error: any) {
      // 对403权限错误不显示提示，让无权限页面处理
      if (error?.response?.status !== 403) {
        message.error('加载统计数据失败: ' + (error as Error).message);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (projectId) {
      loadStatistics();
    }
  }, [projectId]);

  // 计算百分比
  const calculatePercentage = (count: number, total: number): number => {
    if (total === 0) return 0;
    return Math.round((count / total) * 100);
  };

  // 计算趋势
  const calculateTrend = (): { type: 'up' | 'down' | 'stable'; value: number } => {
    if (!statistics?.trend) {
      return { type: 'stable', value: 0 };
    }

    const { completed_this_week, completed_last_week } = statistics.trend;
    const diff = completed_this_week - completed_last_week;

    if (diff > 0) {
      return { type: 'up', value: diff };
    } else if (diff < 0) {
      return { type: 'down', value: Math.abs(diff) };
    }
    return { type: 'stable', value: 0 };
  };

  if (loading) {
    return (
      <Card>
        <Spin tip="加载中...">
          <div style={{ height: '200px' }} />
        </Spin>
      </Card>
    );
  }

  if (!statistics) {
    return <Card>暂无统计数据</Card>;
  }

  const completedPercent = calculatePercentage(statistics.completed, statistics.total);
  const inProgressPercent = calculatePercentage(statistics.in_progress, statistics.total);
  const todoPercent = calculatePercentage(statistics.todo, statistics.total);
  const trend = calculateTrend();

  return (
    <div>
      {/* 概览卡片 */}
      <Card
        title={
          <Space>
            <Title level={4} style={{ margin: 0 }}>
              任务仪表盘
            </Title>
            {statistics.trend && (
              <Space>
                {trend.type === 'up' && (
                  <Text type="success">
                    <RiseOutlined /> 本周完成 +{trend.value}
                  </Text>
                )}
                {trend.type === 'down' && (
                  <Text type="danger">
                    <FallOutlined /> 本周完成 {trend.value}
                  </Text>
                )}
                {trend.type === 'stable' && (
                  <Text type="secondary">本周完成与上周持平</Text>
                )}
              </Space>
            )}
          </Space>
        }
        style={{ marginBottom: '24px' }}
      >
        <Row gutter={16}>
          <Col span={8}>
            <Card
              hoverable
              onClick={() => onTaskClick?.('completed')}
              style={{
                borderColor: '#52c41a',
                cursor: onTaskClick ? 'pointer' : 'default'
              }}
            >
              <Statistic
                title="已完成"
                value={statistics.completed}
                suffix={`/ ${statistics.total}`}
                prefix={<CheckCircleOutlined style={{ color: '#52c41a' }} />}
                valueStyle={{ color: '#52c41a' }}
              />
              <Progress
                percent={completedPercent}
                strokeColor="#52c41a"
                size="small"
                style={{ marginTop: '12px' }}
              />
              <Text type="secondary" style={{ fontSize: '12px' }}>
                完成率: {completedPercent}%
              </Text>
            </Card>
          </Col>

          <Col span={8}>
            <Card
              hoverable
              onClick={() => onTaskClick?.('in-progress')}
              style={{
                borderColor: '#1890ff',
                cursor: onTaskClick ? 'pointer' : 'default'
              }}
            >
              <Statistic
                title="进行中"
                value={statistics.in_progress}
                suffix={`/ ${statistics.total}`}
                prefix={<SyncOutlined spin style={{ color: '#1890ff' }} />}
                valueStyle={{ color: '#1890ff' }}
              />
              <Progress
                percent={inProgressPercent}
                strokeColor="#1890ff"
                size="small"
                style={{ marginTop: '12px' }}
              />
              <Text type="secondary" style={{ fontSize: '12px' }}>
                占比: {inProgressPercent}%
              </Text>
            </Card>
          </Col>

          <Col span={8}>
            <Card
              hoverable
              onClick={() => onTaskClick?.('todo')}
              style={{
                borderColor: '#faad14',
                cursor: onTaskClick ? 'pointer' : 'default'
              }}
            >
              <Statistic
                title="待开始"
                value={statistics.todo}
                suffix={`/ ${statistics.total}`}
                prefix={<ClockCircleOutlined style={{ color: '#faad14' }} />}
                valueStyle={{ color: '#faad14' }}
              />
              <Progress
                percent={todoPercent}
                strokeColor="#faad14"
                size="small"
                style={{ marginTop: '12px' }}
              />
              <Text type="secondary" style={{ fontSize: '12px' }}>
                占比: {todoPercent}%
              </Text>
            </Card>
          </Col>
        </Row>
      </Card>

      {/* 周趋势详情（如果有数据） */}
      {statistics.trend && (
        <Card title="本周趋势" size="small">
          <Row gutter={16}>
            <Col span={12}>
              <Statistic
                title="上周完成"
                value={statistics.trend.completed_last_week}
                suffix="个任务"
              />
            </Col>
            <Col span={12}>
              <Statistic
                title="本周完成"
                value={statistics.trend.completed_this_week}
                suffix="个任务"
                valueStyle={{
                  color:
                    statistics.trend.completed_this_week >=
                    statistics.trend.completed_last_week
                      ? '#52c41a'
                      : '#ff4d4f'
                }}
              />
            </Col>
          </Row>
        </Card>
      )}
    </div>
  );
};

export default TaskDashboard;
