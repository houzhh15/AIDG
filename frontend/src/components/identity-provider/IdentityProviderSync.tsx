/**
 * 身份源同步管理组件
 */
import React, { useState, useEffect } from 'react';
import {
  Card,
  Button,
  Table,
  Tag,
  Spin,
  Statistic,
  Row,
  Col,
  Space,
  message,
  Result,
  Typography,
} from 'antd';
import {
  ArrowLeftOutlined,
  SyncOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import {
  getSyncStatus,
  getSyncLogs,
  triggerSync,
} from '../../api/identityProviders';
import { SyncStatus, SyncLog, SyncStatusType } from '../../types/identityProvider';
import { usePermission } from '../../hooks/usePermission';

const { Title, Text } = Typography;

interface IdentityProviderSyncProps {
  idpId: string;
  onBack?: () => void;
}

const IdentityProviderSync: React.FC<IdentityProviderSyncProps> = ({
  idpId,
  onBack,
}) => {
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null);
  const [syncLogs, setSyncLogs] = useState<SyncLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);

  const { hasPermission } = usePermission();
  const hasWritePermission = hasPermission('idp.write');

  // 加载数据
  const loadData = async () => {
    setLoading(true);
    try {
      const [statusRes, logsRes] = await Promise.all([
        getSyncStatus(idpId),
        getSyncLogs(idpId, 10),
      ]);

      if (statusRes.success && statusRes.data) {
        setSyncStatus(statusRes.data);
      }
      if (logsRes.success && logsRes.data) {
        setSyncLogs(logsRes.data);
      }
    } catch (err: any) {
      console.error('[IdP Sync] Failed to load sync data:', err);
      message.error('加载同步状态失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [idpId]);

  // 触发同步
  const handleSync = async () => {
    setSyncing(true);
    try {
      const res = await triggerSync(idpId);
      if (res.success) {
        message.success('同步已触发');
        // 延迟刷新状态
        setTimeout(() => {
          loadData();
        }, 1000);
      } else {
        message.error(res.error || '触发同步失败');
      }
    } catch (err: any) {
      console.error('[IdP Sync] Trigger sync error:', err);
      message.error(err.response?.data?.error || '触发同步失败');
    } finally {
      setSyncing(false);
    }
  };

  // 渲染同步状态图标
  const renderStatusIcon = (status: SyncStatusType) => {
    switch (status) {
      case 'running':
        return <LoadingOutlined style={{ color: '#1890ff' }} spin />;
      case 'completed':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'failed':
        return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
      default:
        return null;
    }
  };

  // 渲染状态标签
  const renderStatusTag = (status: SyncStatusType) => {
    const config = {
      running: { color: 'processing', text: '同步中' },
      completed: { color: 'success', text: '已完成' },
      failed: { color: 'error', text: '失败' },
    };
    const { color, text } = config[status] || { color: 'default', text: status };
    return <Tag color={color}>{text}</Tag>;
  };

  // 日志表格列
  const columns: ColumnsType<SyncLog> = [
    {
      title: '同步 ID',
      dataIndex: 'sync_id',
      key: 'sync_id',
      width: 150,
      ellipsis: true,
    },
    {
      title: '开始时间',
      dataIndex: 'started_at',
      key: 'started_at',
      width: 180,
      render: (time: string) => new Date(time).toLocaleString('zh-CN'),
    },
    {
      title: '结束时间',
      dataIndex: 'finished_at',
      key: 'finished_at',
      width: 180,
      render: (time: string) => time ? new Date(time).toLocaleString('zh-CN') : '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: SyncStatusType) => renderStatusTag(status),
    },
    {
      title: '统计',
      key: 'stats',
      width: 280,
      render: (_, record) => {
        const { stats } = record;
        if (!stats) return '-';
        return (
          <Space size="small" wrap>
            <Text type="secondary">拉取: {stats.total_fetched}</Text>
            <Text type="success">新增: {stats.created}</Text>
            <Text style={{ color: '#1890ff' }}>更新: {stats.updated}</Text>
            <Text type="warning">禁用: {stats.disabled}</Text>
            {stats.errors > 0 && <Text type="danger">错误: {stats.errors}</Text>}
          </Space>
        );
      },
    },
    {
      title: '错误信息',
      dataIndex: 'error',
      key: 'error',
      ellipsis: true,
      render: (error: string) => error ? <Text type="danger">{error}</Text> : '-',
    },
  ];

  if (loading) {
    return (
      <Card>
        <div style={{ textAlign: 'center', padding: '50px 0' }}>
          <Spin size="large" />
        </div>
      </Card>
    );
  }

  return (
    <Card
      title="用户同步管理"
      extra={
        <Button icon={<ArrowLeftOutlined />} onClick={onBack}>
          返回列表
        </Button>
      }
    >
      {/* 同步状态卡片 */}
      <Card
        type="inner"
        title={
          <Space>
            <Title level={5} style={{ margin: 0 }}>同步状态</Title>
            {syncStatus?.is_running && (
              <Tag icon={<LoadingOutlined spin />} color="processing">
                同步中
              </Tag>
            )}
          </Space>
        }
        extra={
          <Button
            type="primary"
            icon={<SyncOutlined spin={syncing} />}
            onClick={handleSync}
            loading={syncing}
            disabled={!hasWritePermission || syncStatus?.is_running}
          >
            立即同步
          </Button>
        }
        style={{ marginBottom: 24 }}
      >
        {syncStatus?.last_sync ? (
          <>
            <Row gutter={16} style={{ marginBottom: 16 }}>
              <Col span={8}>
                <Space>
                  {renderStatusIcon(syncStatus.last_sync.status)}
                  <Text strong>
                    {syncStatus.last_sync.status === 'running' ? '同步进行中' :
                     syncStatus.last_sync.status === 'completed' ? '最近同步成功' :
                     '最近同步失败'}
                  </Text>
                </Space>
              </Col>
              <Col span={8}>
                <Text type="secondary">开始时间: </Text>
                <Text>{new Date(syncStatus.last_sync.started_at).toLocaleString('zh-CN')}</Text>
              </Col>
              {syncStatus.last_sync.finished_at && (
                <Col span={8}>
                  <Text type="secondary">结束时间: </Text>
                  <Text>{new Date(syncStatus.last_sync.finished_at).toLocaleString('zh-CN')}</Text>
                </Col>
              )}
            </Row>
            
            {syncStatus.last_sync.stats && (
              <Row gutter={16}>
                <Col span={4}>
                  <Statistic
                    title="拉取用户"
                    value={syncStatus.last_sync.stats.total_fetched}
                    valueStyle={{ color: '#1890ff' }}
                  />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="新增"
                    value={syncStatus.last_sync.stats.created}
                    valueStyle={{ color: '#52c41a' }}
                  />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="更新"
                    value={syncStatus.last_sync.stats.updated}
                    valueStyle={{ color: '#1890ff' }}
                  />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="禁用"
                    value={syncStatus.last_sync.stats.disabled}
                    valueStyle={{ color: '#faad14' }}
                  />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="跳过"
                    value={syncStatus.last_sync.stats.skipped}
                    valueStyle={{ color: '#8c8c8c' }}
                  />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="错误"
                    value={syncStatus.last_sync.stats.errors}
                    valueStyle={{ color: syncStatus.last_sync.stats.errors > 0 ? '#ff4d4f' : '#8c8c8c' }}
                  />
                </Col>
              </Row>
            )}

            {syncStatus.last_sync.error && (
              <div style={{ marginTop: 16 }}>
                <Text type="danger">错误信息: {syncStatus.last_sync.error}</Text>
              </div>
            )}
          </>
        ) : (
          <Result
            status="info"
            title="暂无同步记录"
            subTitle="点击「立即同步」开始首次用户同步"
          />
        )}
      </Card>

      {/* 同步历史日志 */}
      <Card type="inner" title="同步历史记录">
        <Table
          columns={columns}
          dataSource={syncLogs}
          rowKey="sync_id"
          pagination={false}
          size="small"
          locale={{ emptyText: '暂无同步记录' }}
        />
      </Card>
    </Card>
  );
};

export default IdentityProviderSync;
