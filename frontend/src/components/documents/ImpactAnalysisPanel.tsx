import React, { useState, useEffect } from 'react';
import { 
  Card, 
  Tree, 
  Button, 
  Typography, 
  Space, 
  Tag, 
  Spin, 
  Alert,
  Tooltip,
  Select,
  Radio,
  Divider,
  List,
  Progress,
  Badge
} from 'antd';
import { 
  NodeIndexOutlined, 
  ExclamationCircleOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  ReloadOutlined,
  FilterOutlined,
  DownOutlined
} from '@ant-design/icons';
import { AnalysisMode, ImpactResult } from '../../types/documents';

const { Title, Text } = Typography;
const { Option } = Select;

interface ImpactAnalysisPanelProps {
  projectId: string;
  nodeId: string;
  analysisMode: AnalysisMode;
  impactResults?: ImpactResult[];
  loading?: boolean;
  onAnalyze?: (nodeId: string, mode: AnalysisMode) => void;
  onNodeSelect?: (nodeId: string) => void;
}

interface ImpactTreeNode {
  key: string;
  title: React.ReactNode;
  children?: ImpactTreeNode[];
  isLeaf?: boolean;
  disabled?: boolean;
  data?: ImpactResult;
}

const ImpactAnalysisPanel: React.FC<ImpactAnalysisPanelProps> = ({
  projectId,
  nodeId,
  analysisMode,
  impactResults = [],
  loading = false,
  onAnalyze,
  onNodeSelect
}) => {
  const [selectedMode, setSelectedMode] = useState<AnalysisMode>(analysisMode);
  const [expandedKeys, setExpandedKeys] = useState<string[]>([]);
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const [filterType, setFilterType] = useState<'all' | 'high' | 'medium' | 'low'>('all');
  const [treeData, setTreeData] = useState<ImpactTreeNode[]>([]);

  // 影响级别配置
  const impactLevelConfig = {
    high: { label: '高影响', color: 'red', icon: <ExclamationCircleOutlined /> },
    medium: { label: '中影响', color: 'orange', icon: <ClockCircleOutlined /> },
    low: { label: '低影响', color: 'blue', icon: <CheckCircleOutlined /> }
  };

  // 分析模式配置
  const analysisModeConfig = {
    upstream: { label: '上游依赖', description: '分析哪些节点依赖当前节点' },
    downstream: { label: '下游影响', description: '分析当前节点影响哪些节点' },
    bidirectional: { label: '双向分析', description: '同时分析上游依赖和下游影响' }
  };

  useEffect(() => {
    buildTreeData();
  }, [impactResults, filterType]);

  const buildTreeData = () => {
    const filteredResults = filterType === 'all' 
      ? impactResults 
      : impactResults.filter(result => result.impact_level === filterType);

    // 按影响级别分组
    const grouped = filteredResults.reduce((acc, result) => {
      const level = result.impact_level;
      if (!acc[level]) {
        acc[level] = [];
      }
      acc[level].push(result);
      return acc;
    }, {} as Record<string, ImpactResult[]>);

    // 构建树形结构
    const treeNodes: ImpactTreeNode[] = [];
    
    Object.entries(grouped).forEach(([level, results]) => {
      const config = impactLevelConfig[level as keyof typeof impactLevelConfig];
      if (!config) return;

      const groupNode: ImpactTreeNode = {
        key: `level-${level}`,
        title: (
          <Space>
            {config.icon}
            <Text strong>{config.label}</Text>
            <Badge count={results.length} showZero />
          </Space>
        ),
        children: results.map(result => ({
          key: result.affected_node_id,
          title: (
            <div>
              <div style={{ marginBottom: 4 }}>
                <Text strong>{result.title}</Text>
                <Tag color={config.color} style={{ marginLeft: 8 }}>
                  {config.label}
                </Tag>
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {result.description}
              </Text>
              {result.change_probability && (
                <div style={{ marginTop: 4 }}>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    变更概率: 
                  </Text>
                  <Progress 
                    percent={result.change_probability * 100} 
                    size="small" 
                    style={{ width: 100, marginLeft: 4 }}
                  />
                </div>
              )}
            </div>
          ),
          isLeaf: true,
          data: result
        }))
      };

      treeNodes.push(groupNode);
    });

    setTreeData(treeNodes);
    
    // 默认展开所有节点
    const allKeys = treeNodes.map(node => node.key);
    setExpandedKeys(allKeys);
  };

  const handleAnalyze = () => {
    onAnalyze?.(nodeId, selectedMode);
  };

  const handleModeChange = (mode: AnalysisMode) => {
    setSelectedMode(mode);
  };

  const handleNodeClick = (selectedKeysValue: React.Key[], info: any) => {
    const keys = selectedKeysValue.map(key => String(key));
    setSelectedKeys(keys);
    const nodeData = info.node?.data;
    if (nodeData) {
      onNodeSelect?.(nodeData.affected_node_id);
    }
  };

  const handleExpand = (expandedKeysValue: React.Key[]) => {
    const keys = expandedKeysValue.map(key => String(key));
    setExpandedKeys(keys);
  };

  const getStatistics = () => {
    const stats = {
      total: impactResults.length,
      high: impactResults.filter(r => r.impact_level === 'high').length,
      medium: impactResults.filter(r => r.impact_level === 'medium').length,
      low: impactResults.filter(r => r.impact_level === 'low').length
    };

    const avgProbability = impactResults.reduce((sum, r) => sum + (r.change_probability || 0), 0) / impactResults.length;

    return { ...stats, avgProbability: avgProbability || 0 };
  };

  const stats = getStatistics();

  const renderStatistics = () => (
    <Card size="small" style={{ marginBottom: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Space size="large">
          <div>
            <Text type="secondary">总影响节点</Text>
            <div><Text strong style={{ fontSize: 18 }}>{stats.total}</Text></div>
          </div>
          <div>
            <Text type="secondary">高影响</Text>
            <div><Text strong style={{ fontSize: 18, color: '#ff4d4f' }}>{stats.high}</Text></div>
          </div>
          <div>
            <Text type="secondary">中影响</Text>
            <div><Text strong style={{ fontSize: 18, color: '#fa8c16' }}>{stats.medium}</Text></div>
          </div>
          <div>
            <Text type="secondary">低影响</Text>
            <div><Text strong style={{ fontSize: 18, color: '#1890ff' }}>{stats.low}</Text></div>
          </div>
          <div>
            <Text type="secondary">平均变更概率</Text>
            <div>
              <Text strong style={{ fontSize: 18 }}>
                {(stats.avgProbability * 100).toFixed(1)}%
              </Text>
            </div>
          </div>
        </Space>
      </div>
    </Card>
  );

  const renderAnalysisControls = () => (
    <Card size="small" style={{ marginBottom: 16 }}>
      <Space direction="vertical" style={{ width: '100%' }}>
        <div>
          <Text strong>分析模式:</Text>
          <Radio.Group 
            value={selectedMode} 
            onChange={(e) => handleModeChange(e.target.value)}
            style={{ marginLeft: 16 }}
          >
            {Object.entries(analysisModeConfig).map(([key, config]) => (
              <Tooltip key={key} title={config.description}>
                <Radio value={key}>{config.label}</Radio>
              </Tooltip>
            ))}
          </Radio.Group>
        </div>

        <div>
          <Text strong>影响级别筛选:</Text>
          <Select
            value={filterType}
            onChange={setFilterType}
            style={{ width: 120, marginLeft: 16 }}
            size="small"
          >
            <Option value="all">全部</Option>
            <Option value="high">高影响</Option>
            <Option value="medium">中影响</Option>
            <Option value="low">低影响</Option>
          </Select>
        </div>

        <div style={{ textAlign: 'right' }}>
          <Button 
            type="primary" 
            icon={<NodeIndexOutlined />}
            onClick={handleAnalyze}
            loading={loading}
          >
            重新分析
          </Button>
        </div>
      </Space>
    </Card>
  );

  const renderImpactTree = () => (
    <Card
      title={
        <Space>
          <Title level={5} style={{ margin: 0 }}>影响分析结果</Title>
          <Tooltip title="显示当前文档变更对其他文档的影响">
            <Button type="text" size="small" icon={<FilterOutlined />} />
          </Tooltip>
        </Space>
      }
      loading={loading}
      bodyStyle={{ padding: '12px 0' }}
    >
      {impactResults.length > 0 ? (
        <Tree
          treeData={treeData}
          expandedKeys={expandedKeys}
          selectedKeys={selectedKeys}
          onExpand={handleExpand}
          onSelect={handleNodeClick}
          showLine={{ showLeafIcon: false }}
          switcherIcon={<DownOutlined />}
          height={400}
        />
      ) : (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <NodeIndexOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />
          <div style={{ marginTop: 16 }}>
            <Text type="secondary">暂无影响分析结果</Text>
          </div>
          <div style={{ marginTop: 8 }}>
            <Button type="primary" onClick={handleAnalyze} loading={loading}>
              开始分析
            </Button>
          </div>
        </div>
      )}
    </Card>
  );

  const renderAlerts = () => {
    if (stats.high > 0) {
      return (
        <Alert
          message="检测到高影响变更"
          description={`当前修改可能会影响 ${stats.high} 个重要文档，请谨慎操作并通知相关人员。`}
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          action={
            <Button size="small" onClick={handleAnalyze}>
              重新分析
            </Button>
          }
        />
      );
    }
    
    if (stats.total > 10) {
      return (
        <Alert
          message="影响范围较大"
          description={`当前修改将影响 ${stats.total} 个文档，建议分阶段进行变更。`}
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
      );
    }

    return null;
  };

  return (
    <div>
      {renderAlerts()}
      {renderAnalysisControls()}
      {stats.total > 0 && renderStatistics()}
      {renderImpactTree()}
    </div>
  );
};

export default ImpactAnalysisPanel;