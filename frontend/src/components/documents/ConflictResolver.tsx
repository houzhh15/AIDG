import React, { useState, useEffect } from 'react';
import { 
  Card, 
  List, 
  Button, 
  Typography, 
  Tag, 
  Space, 
  Modal, 
  Select, 
  Input, 
  Alert,
  Divider,
  Radio,
  message,
  Badge
} from 'antd';
import { 
  ExclamationCircleOutlined, 
  CheckCircleOutlined, 
  CloseCircleOutlined,
  MergeOutlined,
  EyeOutlined,
  EditOutlined
} from '@ant-design/icons';

const { Title, Text, Paragraph } = Typography;
const { Option } = Select;

// 冲突类型定义
export type ConflictType = 'content' | 'structure' | 'reference' | 'version';

export interface ConflictItem {
  id: string;
  type: ConflictType;
  nodeId: string;
  title: string;
  description: string;
  severity: 'high' | 'medium' | 'low';
  status: 'unresolved' | 'resolved' | 'ignored';
  conflictData: {
    baseVersion: number;
    branchVersions: number[];
    conflictContent: {
      base: string;
      current: string;
      incoming: string;
    };
  };
  createdAt: string;
  updatedAt: string;
}

interface ConflictResolverProps {
  projectId: string;
  conflicts: ConflictItem[];
  loading?: boolean;
  onResolveConflict?: (conflictId: string, resolution: ConflictResolution) => void;
  onIgnoreConflict?: (conflictId: string, reason: string) => void;
  onViewConflictDetail?: (conflictId: string) => void;
}

interface ConflictResolution {
  type: 'merge' | 'accept_current' | 'accept_incoming' | 'manual';
  mergedContent: string;
  reason?: string;
}

const ConflictResolver: React.FC<ConflictResolverProps> = ({
  projectId,
  conflicts,
  loading = false,
  onResolveConflict,
  onIgnoreConflict,
  onViewConflictDetail
}) => {
  const [resolveModalVisible, setResolveModalVisible] = useState<boolean>(false);
  const [currentConflict, setCurrentConflict] = useState<ConflictItem | null>(null);
  const [resolutionType, setResolutionType] = useState<'merge' | 'accept_current' | 'accept_incoming' | 'manual'>('merge');
  const [mergedContent, setMergedContent] = useState<string>('');
  const [ignoreReason, setIgnoreReason] = useState<string>('');
  
  // 冲突类型配置
  const conflictTypeConfig = {
    content: { label: '内容冲突', color: 'red', icon: <ExclamationCircleOutlined /> },
    structure: { label: '结构冲突', color: 'orange', icon: <ExclamationCircleOutlined /> },
    reference: { label: '引用冲突', color: 'blue', icon: <ExclamationCircleOutlined /> },
    version: { label: '版本冲突', color: 'purple', icon: <ExclamationCircleOutlined /> }
  };

  // 严重程度配置
  const severityConfig = {
    high: { label: '高', color: 'red' },
    medium: { label: '中', color: 'orange' },
    low: { label: '低', color: 'blue' }
  };

  // 状态配置
  const statusConfig = {
    unresolved: { label: '未解决', color: 'red', icon: <CloseCircleOutlined /> },
    resolved: { label: '已解决', color: 'green', icon: <CheckCircleOutlined /> },
    ignored: { label: '已忽略', color: 'gray', icon: <ExclamationCircleOutlined /> }
  };

  const handleResolveClick = (conflict: ConflictItem) => {
    setCurrentConflict(conflict);
    setResolutionType('merge');
    
    // 智能合并建议
    const { base, current, incoming } = conflict.conflictData.conflictContent;
    let nextResolution: typeof resolutionType = 'merge';
    let initialContent = '';

    if (current === incoming) {
      nextResolution = 'accept_current';
      initialContent = current;
    } else if (base === current) {
      nextResolution = 'accept_incoming';
      initialContent = incoming;
    } else if (base === incoming) {
      nextResolution = 'accept_current';
      initialContent = current;
    } else {
      // 尝试自动合并
      initialContent = attemptAutoMerge(base, current, incoming);
    }

    setResolutionType(nextResolution);
    setMergedContent(initialContent);
    
    setResolveModalVisible(true);
  };

  const attemptAutoMerge = (base: string, current: string, incoming: string): string => {
    // 简单的自动合并算法
    const baseLines = base.split('\n');
    const currentLines = current.split('\n');
    const incomingLines = incoming.split('\n');
    
    const merged = [];
    const maxLines = Math.max(baseLines.length, currentLines.length, incomingLines.length);
    
    for (let i = 0; i < maxLines; i++) {
      const baseLine = baseLines[i] || '';
      const currentLine = currentLines[i] || '';
      const incomingLine = incomingLines[i] || '';
      
      if (currentLine === incomingLine) {
        merged.push(currentLine);
      } else if (baseLine === currentLine) {
        merged.push(incomingLine);
      } else if (baseLine === incomingLine) {
        merged.push(currentLine);
      } else {
        // 冲突行，保留两个版本
        merged.push(`<<<<<<< 当前版本`);
        merged.push(currentLine);
        merged.push(`=======`);
        merged.push(incomingLine);
        merged.push(`>>>>>>> 传入版本`);
      }
    }
    
    return merged.join('\n');
  };

  const handleIgnoreClick = (conflict: ConflictItem) => {
    setCurrentConflict(conflict);
    Modal.confirm({
      title: '忽略冲突',
      content: (
        <div>
          <p>确定要忽略此冲突吗？</p>
          <Input.TextArea
            placeholder="请输入忽略原因（可选）"
            value={ignoreReason}
            onChange={(e) => setIgnoreReason(e.target.value)}
            rows={3}
          />
        </div>
      ),
      onOk: () => {
        onIgnoreConflict?.(conflict.id, ignoreReason);
        message.success('冲突已忽略');
        setIgnoreReason('');
      },
      onCancel: () => {
        setIgnoreReason('');
      }
    });
  };

  const handleViewDetail = (conflict: ConflictItem) => {
    onViewConflictDetail?.(conflict.id);
  };

  const handleResolveModalOk = () => {
    if (!currentConflict) return;

    const { base, current, incoming } = currentConflict.conflictData.conflictContent;

    let resolvedContent = mergedContent;

    switch (resolutionType) {
      case 'accept_current':
        resolvedContent = current;
        break;
      case 'accept_incoming':
        resolvedContent = incoming;
        break;
      case 'manual':
        if (!mergedContent.trim()) {
          message.warning('请填写手动合并后的内容');
          return;
        }
        resolvedContent = mergedContent;
        break;
      case 'merge':
      default:
        if (!mergedContent.trim()) {
          resolvedContent = attemptAutoMerge(base, current, incoming);
          setMergedContent(resolvedContent);
        }
        break;
    }
    
    if (!resolvedContent) {
      message.warning('未生成合并结果，请选择其他解决方式');
      return;
    }
    
    const resolution: ConflictResolution = {
      type: resolutionType,
      mergedContent: resolvedContent,
      reason: `使用${resolutionType}方式解决冲突`
    };
    
    onResolveConflict?.(currentConflict.id, resolution);
    message.success('冲突解决成功');
    setResolveModalVisible(false);
    setCurrentConflict(null);
    setMergedContent('');
  };

  const handleResolveModalCancel = () => {
    setResolveModalVisible(false);
    setCurrentConflict(null);
    setMergedContent('');
    setResolutionType('merge');
  };

  const renderConflictContent = () => {
    if (!currentConflict) return null;
    
    const { base, current, incoming } = currentConflict.conflictData.conflictContent;
    
    switch (resolutionType) {
      case 'accept_current':
        return (
          <div>
            <Text strong>将采用当前版本：</Text>
            <pre style={{ background: '#f6ffed', padding: 8, borderRadius: 4, marginTop: 8 }}>
              {current}
            </pre>
          </div>
        );
      case 'accept_incoming':
        return (
          <div>
            <Text strong>将采用传入版本：</Text>
            <pre style={{ background: '#f6ffed', padding: 8, borderRadius: 4, marginTop: 8 }}>
              {incoming}
            </pre>
          </div>
        );
      case 'manual':
        return (
          <div>
            <Text strong>手动编辑合并结果：</Text>
            <Input.TextArea
              value={mergedContent}
              onChange={(e) => setMergedContent(e.target.value)}
              rows={10}
              style={{ marginTop: 8 }}
            />
          </div>
        );
      default:
        return (
          <div>
            <Text strong>自动合并结果：</Text>
            <pre style={{ background: '#fff2e8', padding: 8, borderRadius: 4, marginTop: 8 }}>
              {mergedContent}
            </pre>
          </div>
        );
    }
  };

  const getConflictStats = () => {
    const stats = {
      total: conflicts.length,
      unresolved: conflicts.filter(c => c.status === 'unresolved').length,
      resolved: conflicts.filter(c => c.status === 'resolved').length,
      ignored: conflicts.filter(c => c.status === 'ignored').length
    };
    return stats;
  };

  const stats = getConflictStats();

  return (
    <div>
      {/* 统计信息 */}
      <Card size="small" style={{ marginBottom: 16 }}>
        <Space size="large">
          <div>
            <Badge count={stats.total} showZero>
              <Button type="text">总冲突</Button>
            </Badge>
          </div>
          <div>
            <Badge count={stats.unresolved} showZero>
              <Button type="text" danger>未解决</Button>
            </Badge>
          </div>
          <div>
            <Badge count={stats.resolved} showZero>
              <Button type="text" style={{ color: '#52c41a' }}>已解决</Button>
            </Badge>
          </div>
          <div>
            <Badge count={stats.ignored} showZero>
              <Button type="text" style={{ color: '#666' }}>已忽略</Button>
            </Badge>
          </div>
        </Space>
      </Card>

      {/* 冲突列表 */}
      <Card 
        title={<Title level={5} style={{ margin: 0 }}>文档冲突列表</Title>}
        loading={loading}
      >
        {stats.unresolved > 0 && (
          <Alert
            message="存在未解决的冲突"
            description="请及时处理文档冲突，以确保文档内容的一致性和完整性。"
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
          />
        )}

        <List
          dataSource={conflicts}
          locale={{ emptyText: '暂无冲突' }}
          renderItem={(conflict) => (
            <List.Item
              actions={conflict.status === 'unresolved' ? [
                <Button
                  key="resolve"
                  type="primary"
                  size="small"
                  icon={<MergeOutlined />}
                  onClick={() => handleResolveClick(conflict)}
                >
                  解决
                </Button>,
                <Button
                  key="ignore"
                  size="small"
                  onClick={() => handleIgnoreClick(conflict)}
                >
                  忽略
                </Button>,
                <Button
                  key="view"
                  size="small"
                  icon={<EyeOutlined />}
                  onClick={() => handleViewDetail(conflict)}
                >
                  详情
                </Button>
              ] : [
                <Button
                  key="view"
                  size="small"
                  icon={<EyeOutlined />}
                  onClick={() => handleViewDetail(conflict)}
                >
                  详情
                </Button>
              ]}
            >
              <List.Item.Meta
                title={
                  <Space>
                    <Tag 
                      color={conflictTypeConfig[conflict.type]?.color}
                      icon={conflictTypeConfig[conflict.type]?.icon}
                    >
                      {conflictTypeConfig[conflict.type]?.label}
                    </Tag>
                    <Tag color={severityConfig[conflict.severity]?.color}>
                      {severityConfig[conflict.severity]?.label}
                    </Tag>
                    <Tag 
                      color={statusConfig[conflict.status]?.color}
                      icon={statusConfig[conflict.status]?.icon}
                    >
                      {statusConfig[conflict.status]?.label}
                    </Tag>
                    <Text strong>{conflict.title}</Text>
                  </Space>
                }
                description={
                  <div>
                    <Paragraph ellipsis={{ rows: 2 }}>{conflict.description}</Paragraph>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      冲突版本: v{conflict.conflictData.baseVersion} vs v{conflict.conflictData.branchVersions.join(', v')}
                    </Text>
                  </div>
                }
              />
            </List.Item>
          )}
        />
      </Card>

      {/* 解决冲突模态框 */}
      <Modal
        title={`解决冲突: ${currentConflict?.title}`}
        open={resolveModalVisible}
        onOk={handleResolveModalOk}
        onCancel={handleResolveModalCancel}
        width={800}
        okText="解决冲突"
        cancelText="取消"
      >
        {currentConflict && (
          <div>
            <div style={{ marginBottom: 16 }}>
              <Text strong>冲突类型: </Text>
              <Tag color={conflictTypeConfig[currentConflict.type]?.color}>
                {conflictTypeConfig[currentConflict.type]?.label}
              </Tag>
              <Text strong style={{ marginLeft: 16 }}>严重程度: </Text>
              <Tag color={severityConfig[currentConflict.severity]?.color}>
                {severityConfig[currentConflict.severity]?.label}
              </Tag>
            </div>

            <Divider />

            <div style={{ marginBottom: 16 }}>
              <Text strong>解决方案:</Text>
              <Radio.Group 
                value={resolutionType} 
                onChange={(e) => setResolutionType(e.target.value)}
                style={{ marginTop: 8 }}
              >
                <Radio value="merge">自动合并</Radio>
                <Radio value="accept_current">采用当前版本</Radio>
                <Radio value="accept_incoming">采用传入版本</Radio>
                <Radio value="manual">手动编辑</Radio>
              </Radio.Group>
            </div>

            {renderConflictContent()}

            {resolutionType === 'merge' && (
              <Alert
                message="自动合并提示"
                description="系统会尝试智能合并冲突内容，请仔细检查合并结果。"
                type="info"
                showIcon
                style={{ marginTop: 16 }}
              />
            )}
          </div>
        )}
      </Modal>
    </div>
  );
};

export default ConflictResolver;