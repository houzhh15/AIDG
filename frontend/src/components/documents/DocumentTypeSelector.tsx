import React, { useState, useEffect } from 'react';
import { 
  Select, 
  Button, 
  Card, 
  Typography, 
  Space, 
  Tag, 
  Tooltip, 
  Drawer, 
  List,
  Input,
  Checkbox,
  Radio,
  Divider
} from 'antd';
import { 
  FilterOutlined, 
  SettingOutlined, 
  FileTextOutlined,
  FolderOutlined,
  ProjectOutlined,
  BulbOutlined,
  TeamOutlined
} from '@ant-design/icons';
import { DocumentType } from '../../types/documents';

const { Text, Title } = Typography;
const { Option } = Select;
const { Search } = Input;

interface DocumentTypeConfig {
  type: DocumentType;
  label: string;
  color: string;
  icon: React.ReactNode;
  description: string;
  templates?: string[];
}

interface DocumentTypeSelectorProps {
  selectedTypes: DocumentType[];
  onSelectionChange: (types: DocumentType[]) => void;
  showCount?: boolean;
  size?: 'small' | 'middle' | 'large';
  mode?: 'select' | 'filter' | 'tags';
  disabled?: boolean;
}

interface DocumentTypeFilterProps {
  availableTypes: DocumentType[];
  selectedTypes: DocumentType[];
  onFilterChange: (types: DocumentType[]) => void;
  counts?: Record<DocumentType, number>;
}

// 文档类型配置
const typeConfigs: DocumentTypeConfig[] = [
  {
    type: 'architecture',
    label: '架构设计',
    color: 'blue',
    icon: <ProjectOutlined />,
    description: '系统架构、技术架构等设计文档',
    templates: ['系统架构图', '组件架构图', '部署架构图']
  },
  {
    type: 'tech_design',
    label: '技术方案',
    color: 'green',
    icon: <BulbOutlined />,
    description: '具体功能模块的技术实现方案',
    templates: ['技术调研', '方案设计', '接口设计']
  },
  {
    type: 'requirements',
    label: '需求文档',
    color: 'orange',
    icon: <FileTextOutlined />,
    description: '产品需求、功能需求等文档',
    templates: ['PRD', '功能规格', '用户故事']
  },
  {
    type: 'meeting',
    label: '会议纪要',
    color: 'purple',
    icon: <TeamOutlined />,
    description: '会议记录、决策记录等',
    templates: ['会议纪要', '决策记录', '讨论总结']
  },
  {
    type: 'task',
    label: '任务文档',
    color: 'cyan',
    icon: <FolderOutlined />,
    description: '任务规划、进度跟踪等文档',
    templates: ['任务计划', '进度报告', '里程碑']
  }
];

// 文档类型选择器组件
const DocumentTypeSelector: React.FC<DocumentTypeSelectorProps> = ({
  selectedTypes,
  onSelectionChange,
  showCount = false,
  size = 'middle',
  mode = 'select',
  disabled = false
}) => {
  const [configDrawerVisible, setConfigDrawerVisible] = useState<boolean>(false);
  
  const getTypeConfig = (type: DocumentType) => {
    return typeConfigs.find(config => config.type === type);
  };

  const handleTypeChange = (value: DocumentType | DocumentType[]) => {
    const types = Array.isArray(value) ? value : [value];
    onSelectionChange(types);
  };

  const handleTagClose = (removedType: DocumentType) => {
    const newTypes = selectedTypes.filter(type => type !== removedType);
    onSelectionChange(newTypes);
  };

  const renderSelectMode = () => (
    <Select
      mode="multiple"
      placeholder="选择文档类型"
      value={selectedTypes}
      onChange={handleTypeChange}
      style={{ width: '100%', minWidth: 200 }}
      size={size}
      disabled={disabled}
      maxTagCount="responsive"
    >
      {typeConfigs.map(config => (
        <Option key={config.type} value={config.type}>
          <Space>
            {config.icon}
            <span>{config.label}</span>
          </Space>
        </Option>
      ))}
    </Select>
  );

  const renderTagMode = () => (
    <Space wrap>
      {selectedTypes.map(type => {
        const config = getTypeConfig(type);
        return config ? (
          <Tag
            key={type}
            closable={!disabled}
            color={config.color}
            onClose={() => handleTagClose(type)}
            icon={config.icon}
          >
            {config.label}
          </Tag>
        ) : null;
      })}
      {!disabled && (
        <Button 
          size="small" 
          type="dashed"
          icon={<FilterOutlined />}
          onClick={() => setConfigDrawerVisible(true)}
        >
          添加类型
        </Button>
      )}
    </Space>
  );

  const renderFilterMode = () => (
    <Space>
      {renderTagMode()}
      <Tooltip title="配置过滤器">
        <Button
          size="small"
          icon={<SettingOutlined />}
          onClick={() => setConfigDrawerVisible(true)}
          disabled={disabled}
        />
      </Tooltip>
    </Space>
  );

  const renderModeContent = () => {
    switch (mode) {
      case 'select':
        return renderSelectMode();
      case 'tags':
        return renderTagMode();
      case 'filter':
        return renderFilterMode();
      default:
        return renderSelectMode();
    }
  };

  return (
    <>
      {renderModeContent()}
      
      <Drawer
        title="文档类型配置"
        placement="right"
        open={configDrawerVisible}
        onClose={() => setConfigDrawerVisible(false)}
        width={400}
      >
        <List
          dataSource={typeConfigs}
          renderItem={(config) => (
            <List.Item>
              <List.Item.Meta
                avatar={
                  <Checkbox
                    checked={selectedTypes.includes(config.type)}
                    onChange={(e) => {
                      if (e.target.checked) {
                        onSelectionChange([...selectedTypes, config.type]);
                      } else {
                        onSelectionChange(selectedTypes.filter(t => t !== config.type));
                      }
                    }}
                  />
                }
                title={
                  <Space>
                    {config.icon}
                    <Text strong>{config.label}</Text>
                    <Tag color={config.color}>
                      {config.type}
                    </Tag>
                  </Space>
                }
                description={config.description}
              />
            </List.Item>
          )}
        />
      </Drawer>
    </>
  );
};

// 文档类型过滤器组件
const DocumentTypeFilter: React.FC<DocumentTypeFilterProps> = ({
  availableTypes,
  selectedTypes,
  onFilterChange,
  counts = {}
}) => {
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [filterMode, setFilterMode] = useState<'include' | 'exclude'>('include');

  const getTypeConfig = (type: DocumentType) => {
    return typeConfigs.find(config => config.type === type);
  };

  const filteredTypes = availableTypes.filter(type => {
    const config = getTypeConfig(type);
    if (!config) return false;
    
    if (searchQuery) {
      return config.label.toLowerCase().includes(searchQuery.toLowerCase()) ||
             config.description.toLowerCase().includes(searchQuery.toLowerCase());
    }
    return true;
  });

  const handleTypeToggle = (type: DocumentType, checked: boolean) => {
    if (checked) {
      onFilterChange([...selectedTypes, type]);
    } else {
      onFilterChange(selectedTypes.filter(t => t !== type));
    }
  };

  const handleSelectAll = () => {
    onFilterChange(availableTypes);
  };

  const handleClearAll = () => {
    onFilterChange([]);
  };

  const renderTypeItem = (type: DocumentType) => {
    const config = getTypeConfig(type);
    if (!config) return null;

    const count = counts[type] || 0;
    const isSelected = selectedTypes.includes(type);

    return (
      <div
        key={type}
        style={{
          padding: 8,
          border: '1px solid #f0f0f0',
          borderRadius: 4,
          marginBottom: 8,
          background: isSelected ? '#f6ffed' : '#fff',
          cursor: 'pointer'
        }}
        onClick={() => handleTypeToggle(type, !isSelected)}
      >
        <Space>
          <Checkbox checked={isSelected} />
          {config.icon}
          <div>
            <div>
              <Text strong>{config.label}</Text>
              {count > 0 && (
                <Tag color={config.color} style={{ marginLeft: 8 }}>
                  {count}
                </Tag>
              )}
            </div>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {config.description}
            </Text>
          </div>
        </Space>
      </div>
    );
  };

  return (
    <Card
      title="文档类型过滤"
      size="small"
      extra={
        <Space>
          <Button size="small" onClick={handleSelectAll}>
            全选
          </Button>
          <Button size="small" onClick={handleClearAll}>
            清空
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: '100%' }}>
        <Search
          placeholder="搜索文档类型"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          size="small"
        />

        <Radio.Group
          value={filterMode}
          onChange={(e) => setFilterMode(e.target.value)}
          size="small"
        >
          <Radio value="include">包含</Radio>
          <Radio value="exclude">排除</Radio>
        </Radio.Group>

        <Divider style={{ margin: '8px 0' }} />

        <div style={{ maxHeight: 300, overflowY: 'auto' }}>
          {filteredTypes.map(renderTypeItem)}
        </div>

        {selectedTypes.length > 0 && (
          <>
            <Divider style={{ margin: '8px 0' }} />
            <Text type="secondary" style={{ fontSize: 12 }}>
              已选择 {selectedTypes.length} 种类型: {' '}
              {selectedTypes.map(type => {
                const config = getTypeConfig(type);
                return config?.label;
              }).join(', ')}
            </Text>
          </>
        )}
      </Space>
    </Card>
  );
};

export { DocumentTypeSelector, DocumentTypeFilter, typeConfigs };
export type { DocumentTypeSelectorProps, DocumentTypeFilterProps, DocumentTypeConfig };