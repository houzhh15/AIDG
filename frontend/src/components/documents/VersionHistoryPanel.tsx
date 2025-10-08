import React, { useState, useEffect } from 'react';
import { List, Typography, Tag, Space, Button, Avatar, Spin, Empty } from 'antd';
import { HistoryOutlined, EyeOutlined, SwapOutlined } from '@ant-design/icons';
import { VersionHistoryPanelProps, SnapshotMeta } from '../../types/documents';

const { Text } = Typography;

const VersionHistoryPanel: React.FC<VersionHistoryPanelProps> = ({
  projectId,
  nodeId,
  onVersionSelect,
  currentVersion,
  currentTitle,
  onCompareWithCurrent,
  onCompareSelected,
  refreshKey
}) => {
  const [versions, setVersions] = useState<SnapshotMeta[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [selectedVersions, setSelectedVersions] = useState<number[]>([]);

  useEffect(() => {
    loadVersionHistory();
  }, [projectId, nodeId, refreshKey]);

  const loadVersionHistory = async () => {
    setLoading(true);
    try {
      if (!nodeId) {
        setVersions([]);
        return;
      }
      // 调用版本历史API - 需要先导入documentsAPI
      const { default: documentsAPI } = await import('../../api/documents');
      const result = await documentsAPI.getVersionHistory(projectId, nodeId);
      setVersions(result.versions);
    } catch (error) {
      console.error('加载版本历史失败:', error);
      // 保留一个fallback版本，避免UI空白
      setVersions([{
        version: 1,
        created_at: new Date().toISOString(),
        path: `.history/${nodeId}/1.md`
      }]);
    } finally {
      setLoading(false);
    }
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const handleVersionClick = (version: number) => {
    onVersionSelect(version);
  };

  const handleCompareSelect = (version: number) => {
    setSelectedVersions(prev => {
      if (prev.includes(version)) {
        return prev.filter(v => v !== version);
      }
      if (prev.length >= 2) {
        return [prev[1], version];
      }
      return [...prev, version];
    });
  };

  const handleCompare = () => {
    if (selectedVersions.length === 2 && onCompareSelected) {
      const [first, second] = selectedVersions;
      onCompareSelected([first, second]);
    }
  };

  const renderVersionItem = (item: SnapshotMeta) => {
    const isSelected = selectedVersions.includes(item.version);
    const maxVersion = versions.length ? Math.max(...versions.map(v => v.version)) : undefined;
    const isLatest = typeof maxVersion === 'number' ? item.version === maxVersion : false;
    const isCurrent = typeof currentVersion === 'number' ? item.version === currentVersion : false;
    const canCompareWithCurrent = typeof currentVersion === 'number' && currentVersion > 0 && !isCurrent;
    
    return (
      <List.Item
        key={item.version}
        style={{
          backgroundColor: isSelected ? '#e6f4ff' : 'transparent',
          border: isSelected ? '1px solid #1890ff' : '1px solid transparent',
          borderRadius: 4,
          padding: 12,
          marginBottom: 8,
          cursor: 'pointer'
        }}
        onClick={() => handleVersionClick(item.version)}
      >
        <List.Item.Meta
          avatar={
            <Avatar 
              icon={<HistoryOutlined />}
              style={{ backgroundColor: isLatest ? '#52c41a' : '#1890ff' }}
            />
          }
          title={
            <Space>
              <Text strong>版本 {item.version}</Text>
              {isCurrent && <Tag color="processing">当前</Tag>}
              {!isCurrent && isLatest && <Tag color="success">最新</Tag>}
            </Space>
          }
          description={
            <div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {formatDate(item.created_at)}
              </Text>
            </div>
          }
        />
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'flex-end',
            gap: 4
          }}
        >
          <Button 
            type="text" 
            size="small" 
            icon={<EyeOutlined />}
            onClick={(e) => {
              e.stopPropagation();
              handleVersionClick(item.version);
            }}
            style={{ paddingInline: 8 }}
          >
            查看
          </Button>
          <Button 
            type={isSelected ? 'primary' : 'text'}
            size="small" 
            icon={<SwapOutlined />}
            onClick={(e) => {
              e.stopPropagation();
              handleCompareSelect(item.version);
            }}
            style={{ paddingInline: 8 }}
          >
            {isSelected ? '已选' : '对比'}
          </Button>
          {canCompareWithCurrent && (
            <Button
              type="link"
              size="small"
              onClick={(e) => {
                e.stopPropagation();
                onCompareWithCurrent?.(item.version);
              }}
              style={{ paddingInline: 8 }}
            >
              对比当前
            </Button>
          )}
        </div>
      </List.Item>
    );
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '40px 0' }}>
        <Spin size="large" />
        <div style={{ marginTop: 16 }}>
          <Text type="secondary">加载版本历史中...</Text>
        </div>
      </div>
    );
  }

  return (
    <div className="version-history-panel">
      <div
        style={{
          marginBottom: 16,
          padding: '12px 16px',
          border: '1px solid #e6ebf2',
          borderRadius: 8,
          backgroundColor: '#f8fbff'
        }}
      >
        <Space direction="vertical" size={4} style={{ width: '100%' }}>
          <Text strong>当前版本</Text>
          {typeof currentVersion === 'number' && currentVersion > 0 ? (
            <Space size={8} wrap>
              <Tag color="processing">v{currentVersion}</Tag>
              {currentTitle && (
                <Text type="secondary" style={{ maxWidth: '100%' }} ellipsis>
                  {currentTitle}
                </Text>
              )}
            </Space>
          ) : (
            <Text type="secondary">暂无当前版本信息，请先选择文档。</Text>
          )}
        </Space>
      </div>

      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Text strong>版本历史 ({versions.length})</Text>
        {selectedVersions.length === 2 && (
          <Button 
            type="primary" 
            size="small"
            icon={<SwapOutlined />}
            onClick={handleCompare}
          >
            对比版本 {selectedVersions.sort((a, b) => b - a).join(' 与 ')}
          </Button>
        )}
      </div>

      {versions.length === 0 ? (
        <Empty description="暂无版本历史" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <List
          dataSource={versions}
          renderItem={renderVersionItem}
          style={{ maxHeight: 400, overflowY: 'auto' }}
        />
      )}
      
      {selectedVersions.length > 0 && selectedVersions.length < 2 && (
        <div style={{ marginTop: 16, padding: 8, backgroundColor: '#f0f2f5', borderRadius: 4 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            请再选择一个版本进行对比
          </Text>
        </div>
      )}
    </div>
  );
};

export default VersionHistoryPanel;