import React, { useState } from 'react';
import { Card, Space, Button, Typography, Row, Col, Tabs, message } from 'antd';
import { 
  GlobalSearchBox, 
  SearchResultsView, 
  VersionHistoryPanel,
  DiffViewModal,
  ReferencePanel,
  RelationshipGraph,
  ConflictResolver,
  DocumentTypeSelector,
  MarkdownEditor,
  ImpactAnalysisPanel,
  EnhancedTreeView,
  SearchResult,
  Reference,
  ConflictItem,
  ImpactResult,
  DocumentTreeNode,
  DocumentType
} from '../components/documents';

const { Title } = Typography;
const { TabPane } = Tabs;

// 模拟数据
const mockSearchResults: SearchResult[] = [
  {
    nodeId: 'doc1',
    title: '系统架构设计',
    type: 'architecture',
    content: '本文档描述了系统的整体架构设计，包括前端、后端和数据库层的设计方案...',
    breadcrumbs: ['项目根目录', '设计文档', '架构设计'],
    relevanceScore: 0.95
  },
  {
    nodeId: 'doc2', 
    title: '用户管理模块技术方案',
    type: 'tech_design',
    content: '用户管理模块的详细技术实现方案，包括用户注册、登录、权限管理等功能...',
    breadcrumbs: ['项目根目录', '技术方案', '用户管理'],
    relevanceScore: 0.87
  }
];

const mockReferences: Reference[] = [
  {
    id: 'ref1',
    task_id: 'task-1',
    document_id: 'doc-1',
    anchor: '2.1.3',
    context: '系统架构设计中的数据库层设计章节',
    status: 'active',
    version: 1,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z'
  }
];

const mockConflicts: ConflictItem[] = [
  {
    id: 'conflict1',
    type: 'content',
    nodeId: 'doc1',
    title: '架构设计文档内容冲突',
    description: '数据库连接配置部分存在不同版本的冲突',
    severity: 'high',
    status: 'unresolved',
    conflictData: {
      baseVersion: 1,
      branchVersions: [2, 3],
      conflictContent: {
        base: '使用 MySQL 8.0 数据库',
        current: '使用 PostgreSQL 14 数据库',
        incoming: '使用 MongoDB 数据库'
      }
    },
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z'
  }
];

const mockImpactResults: ImpactResult[] = [
  {
    affected_node_id: 'doc2',
    title: '用户管理模块',
    description: '需要更新用户认证流程',
    impact_level: 'high',
    change_probability: 0.85,
    relationship_type: 'dependency'
  },
  {
    affected_node_id: 'doc3',
    title: 'API 文档',
    description: '需要更新接口定义',
    impact_level: 'medium',
    change_probability: 0.60,
    relationship_type: 'reference'
  }
];

const mockTreeData: DocumentTreeNode[] = [
  {
    id: 'root1',
    parent_id: undefined,
    title: '系统架构',
    type: 'architecture',
    level: 1,
    position: 1,
    version: 2,
    updated_at: '2024-01-01T00:00:00Z',
    children: [
      {
        id: 'child1',
        parent_id: 'root1',
        title: '前端架构',
        type: 'tech_design',
        level: 2,
        position: 1,
        version: 1,
        updated_at: '2024-01-01T00:00:00Z'
      },
      {
        id: 'child2',
        parent_id: 'root1',
        title: '后端架构',
        type: 'tech_design',
        level: 2,
        position: 2,
        version: 1,
        updated_at: '2024-01-01T00:00:00Z'
      }
    ]
  }
];

const DocumentsDemo: React.FC = () => {
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searchLoading, setSearchLoading] = useState<boolean>(false);
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null);
  const [diffModalVisible, setDiffModalVisible] = useState<boolean>(false);
  const [selectedDocTypes, setSelectedDocTypes] = useState<DocumentType[]>(['architecture', 'tech_design']);
  const [markdownContent, setMarkdownContent] = useState<string>('# 示例文档\n\n这是一个**Markdown**编辑器示例。\n\n## 功能特性\n\n- 支持实时预览\n- 支持快捷键\n- 支持自动保存\n\n```javascript\nconst demo = "Hello World";\nconsole.log(demo);\n```');

  const handleSearch = (query: string) => {
    console.log('搜索查询:', query);
    setSearchLoading(true);
    
    setTimeout(() => {
      const filteredResults = mockSearchResults.filter(result => 
        result.title.toLowerCase().includes(query.toLowerCase()) ||
        result.content.toLowerCase().includes(query.toLowerCase())
      );
      setSearchResults(filteredResults);
      setSearchLoading(false);
    }, 800);
  };

  const handleNodeSelect = (nodeId: string) => {
    console.log('选择文档节点:', nodeId);
    message.info(`选择了节点: ${nodeId}`);
  };

  const handleVersionSelect = (version: number) => {
    setSelectedVersion(version);
    console.log('选择版本:', version);
  };

  const showDiffModal = () => {
    setDiffModalVisible(true);
  };

  return (
    <div style={{ padding: 24 }}>
      <Title level={2}>文档管理组件完整演示</Title>
      
      <Tabs defaultActiveKey="search" type="card">
        <TabPane tab="搜索与版本" key="search">
          <Row gutter={[24, 24]}>
            <Col span={12}>
              <Card title="T-M3-07a: 全局搜索功能" size="small">
                <Space direction="vertical" style={{ width: '100%' }}>
                  <GlobalSearchBox
                    projectId="test-project"
                    onSearch={handleSearch}
                    placeholder="搜索项目文档..."
                  />
                  <SearchResultsView
                    results={searchResults}
                    loading={searchLoading}
                    onNodeSelect={handleNodeSelect}
                  />
                </Space>
              </Card>
            </Col>
            
            <Col span={12}>
              <Card title="T-M3-07b: 版本历史管理" size="small">
                <Space direction="vertical" style={{ width: '100%' }}>
                  <VersionHistoryPanel
                    projectId="test-project"
                    nodeId="doc1"
                    onVersionSelect={handleVersionSelect}
                  />
                  {selectedVersion && (
                    <Button type="primary" onClick={showDiffModal}>
                      查看版本 {selectedVersion} 差异
                    </Button>
                  )}
                </Space>
              </Card>
            </Col>
          </Row>
        </TabPane>

        <TabPane tab="引用与关系" key="reference">
          <Row gutter={[24, 24]}>
            <Col span={12}>
              <Card title="T-M3-07c: 引用管理" size="small">
                <ReferencePanel
                  projectId="test-project"
                  nodeId="doc1"
                  references={mockReferences}
                  onReferenceClick={handleNodeSelect}
                />
              </Card>
            </Col>
            <Col span={12}>
              <Card title="T-M3-07d: 关系图" size="small">
                <RelationshipGraph
                  projectId="test-project"
                  nodes={[
                    { id: 'doc1', label: '架构设计', type: 'architecture' },
                    { id: 'doc2', label: '技术方案', type: 'tech_design' }
                  ]}
                  edges={[
                    { id: 'edge1', source: 'doc1', target: 'doc2', type: 'references' }
                  ]}
                  onNodeSelect={handleNodeSelect}
                />
              </Card>
            </Col>
          </Row>
        </TabPane>

        <TabPane tab="冲突与分析" key="conflict">
          <Row gutter={[24, 24]}>
            <Col span={12}>
              <Card title="T-M3-07e: 冲突解决器" size="small">
                <ConflictResolver
                  projectId="test-project"
                  conflicts={mockConflicts}
                  onViewConflictDetail={handleNodeSelect}
                />
              </Card>
            </Col>
            <Col span={12}>
              <Card title="T-M3-08: 影响分析面板" size="small">
                <ImpactAnalysisPanel
                  projectId="test-project"
                  nodeId="doc1"
                  analysisMode="bidirectional"
                  impactResults={mockImpactResults}
                  onNodeSelect={handleNodeSelect}
                />
              </Card>
            </Col>
          </Row>
        </TabPane>

        <TabPane tab="类型与编辑" key="editor">
          <Row gutter={[24, 24]}>
            <Col span={8}>
              <Card title="T-M1-08: 文档类型选择器" size="small">
                <DocumentTypeSelector
                  selectedTypes={selectedDocTypes}
                  onSelectionChange={setSelectedDocTypes}
                  mode="filter"
                />
              </Card>
            </Col>
            <Col span={16}>
              <Card title="T-M1-09: Markdown编辑器" size="small">
                <MarkdownEditor
                  value={markdownContent}
                  onChange={setMarkdownContent}
                  onSave={(content) => message.success('保存成功')}
                  height={300}
                  showPreview
                  showToolbar
                />
              </Card>
            </Col>
          </Row>
        </TabPane>

        <TabPane tab="增强树视图" key="tree">
          <Card title="T-M1-07增强: 增强树视图" size="small">
            <EnhancedTreeView
              treeData={mockTreeData}
              selectedKeys={[]}
              expandedKeys={['root1']}
              searchable
              draggable
              showContextMenu
              showToolbar
              onSelect={(keys, info) => {
                console.log('选择节点:', keys);
                if (keys.length > 0) {
                  message.info(`选择了节点: ${keys[0]}`);
                }
              }}
              onExpand={(keys) => console.log('展开节点:', keys)}
              onAdd={(parentId, payload) => {
                message.success(`添加节点: ${payload.type} 到 ${parentId}`);
              }}
              onDelete={(nodeId) => message.success(`删除节点: ${nodeId}`)}
            />
          </Card>
        </TabPane>
      </Tabs>

      <DiffViewModal
        projectId="test-project"
        nodeId="doc1"
        fromVersion={1}
        toVersion={2}
        currentVersion={2}
        visible={diffModalVisible}
        onClose={() => setDiffModalVisible(false)}
      />
    </div>
  );
};

export default DocumentsDemo;