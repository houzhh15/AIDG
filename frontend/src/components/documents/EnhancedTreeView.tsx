import React, { useState, useEffect, useRef, useMemo } from 'react';
import { 
  Tree, 
  Input, 
  Button, 
  Typography, 
  Space, 
  Tooltip, 
  Dropdown, 
  Modal,
  Form,
  Select,
  Tag,
  message
} from 'antd';
import type { MenuProps } from 'antd';
import { 
  SearchOutlined,
  PlusOutlined,
  FormOutlined,
  DeleteOutlined,
  MoreOutlined,
  FolderOutlined,
  FileOutlined,
  DragOutlined,
  FullscreenOutlined,
  CompressOutlined,
  ShareAltOutlined,
  LinkOutlined,
  DisconnectOutlined,
  UploadOutlined
} from '@ant-design/icons';
import { DocumentTreeNode, DocumentType } from '../../types/documents';
import { ImportMeta } from '../../api/documents';
import FileUploadArea from './FileUploadArea';

const { Title, Text } = Typography;
const { Search } = Input;
const { Option } = Select;

export type ReferenceSourceValue =
  | 'task_requirements'
  | 'task_design'
  | 'task_test'
  | 'project_feature_list'
  | 'project_architecture'
  | 'meeting_details'
  | 'meeting_summary'
  | 'file_import';

export type ReferenceContextType = 'task' | 'meeting';

export interface ReferenceOption {
  value: ReferenceSourceValue;
  label: string;
  description?: string;
  documentType: DocumentType;
  disabled?: boolean;
  contextType?: ReferenceContextType;
}

export interface ReferenceOptionGroup {
  label: string;
  options: ReferenceOption[];
}

export interface ReferenceContextOption {
  value: string;
  label: string;
  description?: string;
}

export type ReferenceContextOptionsMap = Partial<Record<ReferenceContextType, ReferenceContextOption[]>>;

export interface ReferenceContextSelection {
  type: ReferenceContextType;
  id: string;
}

export interface AddNodePayload {
  title: string;
  type: DocumentType;
  referenceSource?: ReferenceSourceValue | null;
  referenceContext?: ReferenceContextSelection;
  importedContent?: string;    // 新增：文件导入的内容
  importMeta?: ImportMeta;     // 新增：文件导入的元数据
}

const treeTypographyStyle: React.CSSProperties = {
  fontSize: 12,
  lineHeight: '18px'
};

const compactTagStyle: React.CSSProperties = {
  fontSize: 10,
  lineHeight: '16px',
  padding: '0 6px',
  borderRadius: 6
};

interface EnhancedTreeViewProps {
  projectId: string;
  treeData: DocumentTreeNode[];
  selectedKeys: string[];
  expandedKeys: string[];
  loading?: boolean;
  searchable?: boolean;
  draggable?: boolean;
  showContextMenu?: boolean;
  showToolbar?: boolean;
  onSelect?: (selectedKeys: string[], info: any) => void;
  onExpand?: (expandedKeys: string[]) => void;
  onDrop?: (info: any) => void;
  onAdd?: (parentId: string, payload: AddNodePayload) => void | Promise<void>;
  onRename?: (nodeId: string, title: string) => Promise<void> | void;
  onDelete?: (nodeId: string) => void;
  onMove?: (dragNodeId: string, targetNodeId: string, position: 'before' | 'after' | 'inside') => void;
  onAddToResource?: (nodeId: string) => void;
  onLinkToTask?: (nodeId: string) => void;
  onUnlinkTask?: (nodeId: string) => void;
  referenceOptions?: ReferenceOptionGroup[];
  referenceContextOptions?: ReferenceContextOptionsMap;
}

interface TreeNodeData extends Omit<DocumentTreeNode, 'title' | 'children'> {
  key: string;
  title: React.ReactNode;
  icon?: React.ReactNode;
  children?: TreeNodeData[];
  disabled?: boolean;
  disableCheckbox?: boolean;
  selectable?: boolean;
}

const EnhancedTreeView: React.FC<EnhancedTreeViewProps> = ({
  projectId,
  treeData,
  selectedKeys,
  expandedKeys,
  loading = false,
  searchable = true,
  draggable = true,
  showContextMenu = true,
  showToolbar = true,
  onSelect,
  onExpand,
  onDrop,
  onAdd,
  onRename,
  onDelete,
  onMove,
  onAddToResource,
  onLinkToTask,
  onUnlinkTask,
  referenceOptions,
  referenceContextOptions
}) => {
  const [searchValue, setSearchValue] = useState<string>('');
  const [filteredTreeData, setFilteredTreeData] = useState<TreeNodeData[]>([]);
  const [autoExpandParent, setAutoExpandParent] = useState<boolean>(true);
  const [contextMenuNode, setContextMenuNode] = useState<DocumentTreeNode | null>(null);
  const [addModalVisible, setAddModalVisible] = useState<boolean>(false);
  const [fullscreen, setFullscreen] = useState<boolean>(false);
  const [addForm] = Form.useForm();
  const [renameForm] = Form.useForm();
  const treeRef = useRef<any>(null);
  const [renameModalVisible, setRenameModalVisible] = useState<boolean>(false);
  const [renameTarget, setRenameTarget] = useState<DocumentTreeNode | null>(null);
  const [renameLoading, setRenameLoading] = useState<boolean>(false);
  const [pendingParentId, setPendingParentId] = useState<string | null>(null);
  const referenceSource = Form.useWatch<ReferenceSourceValue | undefined>('referenceSource', addForm);
  
  // 文件导入相关状态
  const [importedContent, setImportedContent] = useState<string>('');
  const [importMeta, setImportMeta] = useState<ImportMeta | null>(null);

  // 文件导入始终可用，所以 hasReferenceOptions 总为 true
  const hasReferenceOptions = true;

  const referenceSelectOptions = useMemo(() => {
    // 直接使用传入的 referenceOptions，file_import 已经在 REFERENCE_SOURCE_META 中定义
    return referenceOptions?.map(group => ({
      label: group.label,
      options: group.options.map(option => ({
        label: option.label,
        value: option.value,
        disabled: option.disabled,
        description: option.description,
        contextType: option.contextType
      }))
    })) ?? [];
  }, [referenceOptions]);

  const referenceOptionLookup = useMemo(() => {
    const map = new Map<ReferenceSourceValue, ReferenceOption>();
    referenceOptions?.forEach(group => {
      group.options.forEach(option => {
        map.set(option.value, option);
      });
    });
    return map;
  }, [referenceOptions]);

  const referenceTypeMap = useMemo(() => {
    const map = new Map<ReferenceSourceValue, DocumentType>();
    referenceOptionLookup.forEach((option, value) => {
      map.set(value, option.documentType);
    });
    return map;
  }, [referenceOptionLookup]);

  const referenceContextTypeMap = useMemo(() => {
    const map = new Map<ReferenceSourceValue, ReferenceContextType | undefined>();
    referenceOptionLookup.forEach((option, value) => {
      map.set(value, option.contextType);
    });
    return map;
  }, [referenceOptionLookup]);

  const currentReferenceContextType = referenceSource ? referenceContextTypeMap.get(referenceSource) : undefined;
  const currentReferenceOptions = currentReferenceContextType ? referenceContextOptions?.[currentReferenceContextType] ?? [] : [];

  useEffect(() => {
    addForm.setFieldsValue({ referenceContextId: undefined });
    if (!referenceSource) {
      return;
    }
    const inferredType = referenceTypeMap.get(referenceSource);
    if (inferredType) {
      addForm.setFieldsValue({ type: inferredType });
    }
  }, [referenceSource, referenceTypeMap, addForm]);

  // 文档类型图标配置
  const typeIcons = {
    feature_list: <FileOutlined style={{ color: '#1890ff' }} />,
    architecture: <FolderOutlined style={{ color: '#52c41a' }} />,
    tech_design: <FileOutlined style={{ color: '#fa8c16' }} />,
    background: <FileOutlined style={{ color: '#722ed1' }} />,
    requirements: <FileOutlined style={{ color: '#eb2f96' }} />,
    meeting: <FileOutlined style={{ color: '#13c2c2' }} />,
    task: <FileOutlined style={{ color: '#a0d911' }} />
  };

  useEffect(() => {
    buildTreeData();
  }, [treeData, searchValue]);

  const buildTreeData = () => {
    const processNode = (node: DocumentTreeNode): TreeNodeData => {
      const hasSearchValue = searchValue && node.title.toLowerCase().includes(searchValue.toLowerCase());
      
      const processedNode: TreeNodeData = {
        ...node,
        key: node.id,
        title: renderNodeTitle(node, !!hasSearchValue),
        icon: typeIcons[node.type] || <FileOutlined />,
        children: node.children ? node.children.map(processNode) : undefined
      };

      return processedNode;
    };

    const filtered = treeData.map(processNode);
    setFilteredTreeData(filtered);
  };

  const renderNodeTitle = (node: DocumentTreeNode, highlighted: boolean = false) => {
    const baseTitleStyle: React.CSSProperties = {
      fontSize: treeTypographyStyle.fontSize,
      lineHeight: treeTypographyStyle.lineHeight
    };

    const titleText = highlighted ? (
      <Text style={{ ...baseTitleStyle, backgroundColor: '#fff2e8', padding: '0 2px', borderRadius: 4 }}>
        {node.title}
      </Text>
    ) : (
      <Text style={baseTitleStyle}>{node.title}</Text>
    );

    const menuProps = getContextMenuProps(node);

    const handleMenuOpenChange = (visible: boolean) => {
      if (visible) {
        setContextMenuNode(node);
        setPendingParentId(node.id);
      }
    };

    const content = (
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          width: '100%',
          fontSize: treeTypographyStyle.fontSize,
          lineHeight: treeTypographyStyle.lineHeight
        }}
      >
        <Space size={4} style={{ fontSize: treeTypographyStyle.fontSize }}>
          {titleText}
          <Tag color="blue" style={compactTagStyle}>
            v{node.version}
          </Tag>
        </Space>

        {showContextMenu && (
          <Dropdown
            menu={menuProps}
            trigger={['click']}
            onOpenChange={handleMenuOpenChange}
          >
            <Button
              type="text"
              size="small"
              icon={<MoreOutlined />}
              onClick={(e) => {
                e.stopPropagation();
              }}
            />
          </Dropdown>
        )}
      </div>
    );

    if (!showContextMenu) {
      return content;
    }

    return (
      <Dropdown
        menu={menuProps}
        trigger={['contextMenu']}
        onOpenChange={handleMenuOpenChange}
      >
        {content}
      </Dropdown>
    );
  };

  const findNodeInTree = (nodes: DocumentTreeNode[], id: string): DocumentTreeNode | null => {
    for (const node of nodes) {
      if (node.id === id) {
        return node;
      }
      if (node.children && node.children.length > 0) {
        const found = findNodeInTree(node.children, id);
        if (found) {
          return found;
        }
      }
    }
    return null;
  };

  const getContextMenuProps = (node: DocumentTreeNode): MenuProps => {
    const items: MenuProps['items'] = [];

    if (onAdd) {
      items.push({
        key: 'add',
        icon: <PlusOutlined />,
        label: '添加子节点'
      });
    }

    if (onRename) {
      items.push({
        key: 'rename',
        icon: <FormOutlined />,
        label: '重命名'
      });
    }

    if (onLinkToTask) {
      if (items.length > 0) items.push({ type: 'divider' });
      items.push({
        key: 'link-to-task',
        icon: <LinkOutlined />,
        label: '关联任务'
      });
    }

    if (onUnlinkTask) {
      items.push({
        key: 'unlink-task',
        icon: <DisconnectOutlined />,
        label: '解除关联'
      });
    }

    if (onAddToResource) {
      items.push({
        key: 'add-to-resource',
        icon: <ShareAltOutlined />,
        label: '添加到MCP资源'
      });
    }

    if (onDelete) {
      if (items.length > 0) items.push({ type: 'divider' });
      items.push({
        key: 'delete',
        icon: <DeleteOutlined />,
        label: '删除节点',
        danger: true
      });
    }

    return {
      items,
      onClick: ({ key }) => {
        if (key === 'add') {
          handleAddNode(node.id);
        } else if (key === 'rename' && onRename) {
          handleRenameStart(node);
        } else if (key === 'link-to-task' && onLinkToTask) {
          onLinkToTask(node.id);
        } else if (key === 'unlink-task' && onUnlinkTask) {
          onUnlinkTask(node.id);
        } else if (key === 'add-to-resource' && onAddToResource) {
          onAddToResource(node.id);
        } else if (key === 'delete') {
          handleDeleteNode(node.id);
        }
      }
    };
  };

  const handleSearch = (value: string) => {
    setSearchValue(value);
    if (value) {
      // 搜索时展开所有匹配的父节点
      const expandKeys = getAllParentKeys(treeData, value);
      onExpand?.(expandKeys);
      setAutoExpandParent(true);
    } else {
      setAutoExpandParent(false);
    }
  };

  const getAllParentKeys = (data: DocumentTreeNode[], searchValue: string): string[] => {
    const keys: string[] = [];
    
    const findParents = (nodes: DocumentTreeNode[], parentKey?: string) => {
      nodes.forEach(node => {
        if (node.title.toLowerCase().includes(searchValue.toLowerCase())) {
          if (parentKey) {
            keys.push(parentKey);
          }
        }
        if (node.children) {
          findParents(node.children, node.id);
        }
      });
    };

    findParents(data);
    return keys;
  };

  const handleAddNode = (parentId: string) => {
    const resolvedParentId = parentId && parentId.length > 0 ? parentId : 'root';
    const parentNode = parentId ? findNodeInTree(treeData, parentId) : null;

    setContextMenuNode(parentNode);
    setPendingParentId(resolvedParentId);
    setAddModalVisible(true);
    addForm.resetFields();
  };

  const resetAddModalState = () => {
    setAddModalVisible(false);
    addForm.resetFields();
    setPendingParentId(null);
    setContextMenuNode(null);
    // 重置文件导入状态
    setImportedContent('');
    setImportMeta(null);
  };

  const handleDeleteNode = (nodeId: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除此节点及其所有子节点吗？此操作不可恢复。',
      onOk: () => {
        onDelete?.(nodeId);
        message.success('节点删除成功');
      }
    });
  };

  const handleDrop = (info: any) => {
    const dropKey = info.node.key;
    const dragKey = info.dragNode.key;
    const dropPos = info.node.pos.split('-');
    const dropPosition = info.dropPosition - Number(dropPos[dropPos.length - 1]);

    if (dragKey === dropKey || info.dragNode.parentKey === dropKey) {
      return;
    }

    let position: 'before' | 'after' | 'inside';
    if (info.dropToGap) {
      position = dropPosition === -1 ? 'before' : 'after';
    } else {
      position = 'inside';
    }

    onMove?.(dragKey, dropKey, position);
  };

  const handleModalOk = () => {
    addForm.validateFields().then((values: { title: string; type: DocumentType; referenceSource?: ReferenceSourceValue | null; referenceContextId?: string }) => {
      // 校验文件导入场景下必须上传文件
      if (values.referenceSource === 'file_import' && !importedContent) {
        message.error('请先上传文件');
        return;
      }

      const parentId = pendingParentId ?? (contextMenuNode ? contextMenuNode.id : 'root');
      const referenceSourceValue = values.referenceSource ?? null;
      let referenceContext: ReferenceContextSelection | undefined;

      if (referenceSourceValue && referenceSourceValue !== 'file_import') {
        const option = referenceOptionLookup.get(referenceSourceValue);
        if (option?.contextType && values.referenceContextId) {
          referenceContext = {
            type: option.contextType,
            id: values.referenceContextId
          };
        }
      }

      const payload: AddNodePayload = {
        title: values.title?.trim() || '',
        type: values.type,
        referenceSource: referenceSourceValue,
        referenceContext,
        importedContent: referenceSourceValue === 'file_import' ? importedContent : undefined,
        importMeta: referenceSourceValue === 'file_import' ? importMeta ?? undefined : undefined
      };

      const maybePromise = onAdd?.(parentId, payload);
      if (maybePromise && typeof (maybePromise as Promise<void>).then === 'function') {
        (maybePromise as Promise<void>)
          .then(() => {
            resetAddModalState();
          })
          .catch((error) => {
            console.error('添加节点失败:', error);
          });
      } else {
        resetAddModalState();
      }
    });
  };

  const handleRenameStart = (node: DocumentTreeNode) => {
    setRenameTarget(node);
    setRenameModalVisible(true);
    renameForm.setFieldsValue({ title: node.title });
  };

  const handleRenameModalOk = () => {
    renameForm.validateFields().then((values: { title: string }) => {
      if (!renameTarget || !onRename) {
        setRenameModalVisible(false);
        setRenameTarget(null);
        renameForm.resetFields();
        return;
      }

      const nextTitle = values.title?.trim() || '';
      if (!nextTitle) {
        message.error('请输入节点标题');
        return;
      }

      setRenameLoading(true);
      Promise.resolve(onRename(renameTarget.id, nextTitle))
        .then(() => {
          setRenameModalVisible(false);
          setRenameTarget(null);
          renameForm.resetFields();
        })
        .catch(err => {
          console.error('重命名节点失败:', err);
        })
        .finally(() => {
          setRenameLoading(false);
        });
    });
  };

  const renderToolbar = () => (
    <div style={{ 
      marginBottom: 16, 
      padding: 8, 
      background: '#fafafa', 
      borderRadius: 4,
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'center'
    }}>
      <Space>
        <Button
          type="primary"
          size="small"
          icon={<PlusOutlined />}
          onClick={() => handleAddNode('')}
        >
          添加根节点
        </Button>
        
        <Tooltip title="展开所有层级的节点">
          <Button
            size="small"
            onClick={() => {
              const allKeys = getAllKeys(treeData);
              onExpand?.(allKeys);
            }}
          >
            展开全部
          </Button>
        </Tooltip>

        <Tooltip title="折叠所有节点">
          <Button
            size="small"
            onClick={() => onExpand?.([])}
          >
            折叠全部
          </Button>
        </Tooltip>
      </Space>

      <Space>
        <Tooltip title={fullscreen ? '退出全屏' : '全屏显示'}>
          <Button
            size="small"
            icon={fullscreen ? <CompressOutlined /> : <FullscreenOutlined />}
            onClick={() => setFullscreen(!fullscreen)}
          />
        </Tooltip>
      </Space>
    </div>
  );

  const getAllKeys = (data: DocumentTreeNode[]): string[] => {
    const keys: string[] = [];
    
    const traverse = (nodes: DocumentTreeNode[]) => {
      nodes.forEach(node => {
        keys.push(node.id);
        if (node.children) {
          traverse(node.children);
        }
      });
    };

    traverse(data);
    return keys;
  };

  const treeContent = (
    <div style={{ height: fullscreen ? 'calc(100vh - 120px)' : '600px', overflow: 'auto' }}>
      {showToolbar && renderToolbar()}
      
      {searchable && (
        <Search
          placeholder="搜索文档节点"
          value={searchValue}
          onChange={(e) => handleSearch(e.target.value)}
          onSearch={handleSearch}
          style={{ marginBottom: 16 }}
          allowClear
        />
      )}

      <Tree
        style={treeTypographyStyle}
        ref={treeRef}
        treeData={filteredTreeData}
        selectedKeys={selectedKeys}
        expandedKeys={expandedKeys}
        autoExpandParent={autoExpandParent}
        showIcon
        showLine={{ showLeafIcon: false }}
        draggable={draggable && {
          icon: <DragOutlined />,
          nodeDraggable: (node) => !node.disabled
        }}

        onSelect={(selectedKeysValue, info) => {
          const keys = selectedKeysValue.map(key => String(key));
          onSelect?.(keys, info);
        }}
        onExpand={(keys) => {
          onExpand?.(keys.map(key => String(key)));
          setAutoExpandParent(false);
        }}
        onDrop={handleDrop}
        height={fullscreen ? window.innerHeight - 200 : 500}
      />

      {/* 添加节点模态框 */}
      <Modal
        title="添加文档节点"
        open={addModalVisible}
        onOk={handleModalOk}
        onCancel={resetAddModalState}
        okText="确定"
        cancelText="取消"
      >
        <Form
          form={addForm}
          layout="vertical"
          requiredMark={false}
        >
          <Form.Item
            label="节点标题"
            name="title"
            rules={[{ required: true, message: '请输入节点标题' }]}
          >
            <Input placeholder="输入节点标题" />
          </Form.Item>

          <Form.Item
            label="文档类型"
            name="type"
            rules={[{ required: true, message: '请选择文档类型' }]}
          >
            <Select placeholder="选择文档类型">
              <Option value="feature_list">特性列表</Option>
              <Option value="architecture">架构设计</Option>
              <Option value="tech_design">技术方案</Option>
              <Option value="background">背景信息</Option>
              <Option value="requirements">需求文档</Option>
              <Option value="meeting">会议纪要</Option>
              <Option value="task">任务文档</Option>
            </Select>
          </Form.Item>

          {hasReferenceOptions && (
            <Form.Item
              label="引用来源"
              name="referenceSource"
              tooltip="可选，从既有文档中复制内容到新节点，并自动同步文档类型"
            >
              <Select
                placeholder="选择要引用的文档（可选）"
                allowClear
                options={referenceSelectOptions}
                optionRender={(option) => {
                  const data = option.data as { label: string; description?: string; contextType?: ReferenceContextType };
                  const contextHint = data.contextType === 'task'
                    ? '需选任务'
                    : data.contextType === 'meeting'
                      ? '需选会议'
                      : null;
                  return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span>{data.label}</span>
                        {contextHint && (
                          <Tag style={compactTagStyle} color={data.contextType === 'task' ? 'blue' : 'cyan'}>
                            {contextHint}
                          </Tag>
                        )}
                      </div>
                      {data.description && (
                        <span style={{ fontSize: 12, color: '#94a3b8' }}>{data.description}</span>
                      )}
                    </div>
                  );
                }}
                optionLabelProp="label"
              />
            </Form.Item>
          )}

          {currentReferenceContextType && (
            <Form.Item
              key={`reference-context-${currentReferenceContextType}`}
              label={currentReferenceContextType === 'task' ? '引用任务' : '引用会议'}
              name="referenceContextId"
              rules={[{
                required: true,
                message: currentReferenceContextType === 'task' ? '请选择要引用的任务' : '请选择会议对应的任务'
              }]}
              tooltip={currentReferenceOptions.length === 0 ? '暂无可选项，请先在项目中维护任务/会议' : undefined}
            >
              <Select
                showSearch
                placeholder={currentReferenceContextType === 'task' ? '选择要引用的任务文档' : '选择会议来源任务'}
                optionFilterProp="label"
                options={currentReferenceOptions.map(option => ({
                  label: option.label,
                  value: option.value,
                  description: option.description
                }))}
                notFoundContent={currentReferenceOptions.length === 0 ? '暂无可选项' : undefined}
                optionRender={(option) => {
                  const data = option.data as ReferenceContextOption;
                  return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                      <span>{data.label}</span>
                      {data.description && (
                        <span style={{ fontSize: 12, color: '#94a3b8' }}>{data.description}</span>
                      )}
                    </div>
                  );
                }}
                optionLabelProp="label"
              />
            </Form.Item>
          )}

          {referenceSource === 'file_import' && (
            <Form.Item
              label="上传文件"
              required
              tooltip="支持 PDF、PPT、DOC、EXCEL、SVG 文件，最大20MB"
            >
              <FileUploadArea
                projectId={projectId}
                onImportComplete={(content, meta) => {
                  setImportedContent(content);
                  setImportMeta(meta);
                }}
              />
              {importMeta && (
                <div style={{ marginTop: 8, fontSize: 12, color: '#52c41a' }}>
                  已导入: {importMeta.original_filename} ({(importMeta.file_size / 1024).toFixed(1)} KB)
                </div>
              )}
            </Form.Item>
          )}
        </Form>
      </Modal>

      {/* 重命名节点模态框 */}
      <Modal
        title="重命名文档节点"
        open={renameModalVisible}
        onOk={handleRenameModalOk}
        onCancel={() => {
          setRenameModalVisible(false);
          setRenameTarget(null);
          renameForm.resetFields();
        }}
        okText="确定"
        cancelText="取消"
        confirmLoading={renameLoading}
      >
        <Form
          form={renameForm}
          layout="vertical"
          requiredMark={false}
        >
          <Form.Item
            label="节点标题"
            name="title"
            rules={[{ required: true, message: '请输入节点标题' }]}
          >
            <Input placeholder="输入新的节点标题" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );

  if (fullscreen) {
    return (
      <Modal
        title="文档树视图"
        open={fullscreen}
        onCancel={() => setFullscreen(false)}
        footer={null}
        width="100vw"
        style={{ top: 0, maxWidth: 'none' }}
        bodyStyle={{ padding: 16, height: '100vh' }}
      >
        {treeContent}
      </Modal>
    );
  }

  return treeContent;
};

export default EnhancedTreeView;