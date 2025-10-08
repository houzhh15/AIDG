import React from 'react';
import { List, Typography, Tag, Space, Breadcrumb, Spin, Empty } from 'antd';
import { FileTextOutlined, HomeOutlined } from '@ant-design/icons';
import { SearchResultsViewProps, DocumentType } from '../../types/documents';

const { Text, Paragraph } = Typography;

// 文档类型配置
const documentTypeConfig: Record<DocumentType, { color: string; label: string }> = {
  feature_list: { color: 'blue', label: '特性列表' },
  architecture: { color: 'green', label: '架构设计' },
  tech_design: { color: 'orange', label: '技术方案' },
  background: { color: 'purple', label: '背景资料' },
  requirements: { color: 'magenta', label: '需求文档' },
  meeting: { color: 'cyan', label: '会议纪要' },
  task: { color: 'geekblue', label: '任务文档' }
};

const SearchResultsView: React.FC<SearchResultsViewProps> = ({
  results,
  loading,
  onNodeSelect
}) => {
  const getDocumentTypeTag = (type: DocumentType) => {
    const config = documentTypeConfig[type];
    if (!config) {
      return <Tag color="default">{type}</Tag>;
    }
    return <Tag color={config.color}>{config.label}</Tag>;
  };

  const renderBreadcrumbs = (breadcrumbs: string[]) => {
    const items = breadcrumbs.map((crumb, index) => ({
      title: index === 0 ? (
        <Space>
          <HomeOutlined />
          <span>{crumb}</span>
        </Space>
      ) : crumb
    }));

    return (
      <Breadcrumb
        items={items}
        style={{ marginBottom: 4, fontSize: 12 }}
      />
    );
  };

  const renderResultItem = (item: any) => (
    <List.Item
      key={item.nodeId}
      style={{ cursor: 'pointer' }}
      onClick={() => onNodeSelect(item.nodeId)}
      onMouseEnter={(e) => {
        e.currentTarget.style.backgroundColor = '#f5f5f5';
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.backgroundColor = 'transparent';
      }}
    >
      <List.Item.Meta
        avatar={<FileTextOutlined style={{ fontSize: 16, color: '#1890ff' }} />}
        title={
          <Space>
            <Text strong>{item.title}</Text>
            {getDocumentTypeTag(item.type)}
            <Text type="secondary" style={{ fontSize: 12 }}>
              相关度: {Math.round(item.relevanceScore * 100)}%
            </Text>
          </Space>
        }
        description={
          <div>
            {item.breadcrumbs && item.breadcrumbs.length > 0 && renderBreadcrumbs(item.breadcrumbs)}
            <Paragraph
              ellipsis={{
                rows: 2,
                expandable: false,
                tooltip: item.content
              }}
              style={{ margin: 0, color: '#666' }}
            >
              {item.content}
            </Paragraph>
          </div>
        }
      />
    </List.Item>
  );

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '40px 0' }}>
        <Spin size="large" />
        <div style={{ marginTop: 16 }}>
          <Text type="secondary">搜索中...</Text>
        </div>
      </div>
    );
  }

  if (!results || results.length === 0) {
    return (
      <Empty
        description="未找到相关文档"
        image={Empty.PRESENTED_IMAGE_SIMPLE}
      />
    );
  }

  return (
    <div className="search-results-view">
      <div style={{ marginBottom: 16 }}>
        <Text type="secondary">
          找到 {results.length} 个相关结果
        </Text>
      </div>
      <List
        itemLayout="vertical"
        dataSource={results}
        renderItem={renderResultItem}
        pagination={
          results.length > 10
            ? {
                pageSize: 10,
                size: 'small',
                showSizeChanger: false,
                showQuickJumper: true
              }
            : false
        }
      />
    </div>
  );
};

export default SearchResultsView;