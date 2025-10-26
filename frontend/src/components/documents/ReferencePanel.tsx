import React, { useState, useEffect } from 'react';
import { 
  List, 
  Button, 
  Input, 
  Select, 
  Typography, 
  Space, 
  Tag, 
  Popconfirm,
  Modal,
  Form,
  message,
  Tooltip,
  Badge
} from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, LinkOutlined, FileTextOutlined } from '@ant-design/icons';
import { Reference, ReferenceStatus } from '../../types/documents';

const { Text } = Typography;
const { Option } = Select;

interface ReferencePanelProps {
  projectId: string;
  nodeId: string;
  references: Reference[];
  loading?: boolean;
  onAddReference?: (reference: Omit<Reference, 'id' | 'created_at' | 'updated_at'>) => void | Promise<void>;
  onDeleteReference?: (referenceId: string) => void | Promise<void>;
  onUpdateReference?: (referenceId: string, reference: Partial<Reference>) => void | Promise<void>;
  onReferenceClick?: (documentId: string, reference?: Reference) => void;
}

interface ReferenceFormData {
  task_id: string;
  document_id: string;
  anchor?: string;
  context?: string;
}

const ReferencePanel: React.FC<ReferencePanelProps> = ({
  projectId,
  nodeId,
  references,
  loading = false,
  onAddReference,
  onDeleteReference,
  onUpdateReference,
  onReferenceClick
}) => {
  const [form] = Form.useForm<ReferenceFormData>();
  const [addModalVisible, setAddModalVisible] = useState<boolean>(false);
  const [editingReference, setEditingReference] = useState<Reference | null>(null);
  const [taskList, setTaskList] = useState<any[]>([]);
  const [documentList, setDocumentList] = useState<any[]>([]);

  const ANCHOR_MAX_LENGTH = 120;

  // 引用状态配置
  const statusConfig = {
    active: { label: '有效', color: 'green' },
    outdated: { label: '过期', color: 'orange' },
    broken: { label: '失效', color: 'red' }
  };

  useEffect(() => {
    loadTasksAndDocuments();
  }, [projectId]);

  const loadTasksAndDocuments = async () => {
    try {
      // 调用API获取任务列表和文档列表
      const { default: documentsAPI } = await import('../../api/documents');
      
      // 1. 获取项目任务列表 (从项目任务管理API获取)
      const { getProjectTasks } = await import('../../api/projects');
      const tasks = await getProjectTasks(projectId);

      // 2. 获取文档列表 (从文档树获取)
      const treeResult = await documentsAPI.getTree(projectId);

      const normalizeTree = (tree: any): any[] => {
        if (!tree) {
          return [];
        }
        const base = Array.isArray(tree) ? tree : [tree];
        if (base.length === 1 && base[0]?.node?.id === 'virtual_root') {
          return base[0].children ?? [];
        }
        return base;
      };

      // 从文档树提取所有文档信息（包括任务文档，它们是关于任务的文档，不同于项目任务）
      const extractDocuments = (roots: any[]): any[] => {
        const documents: any[] = [];
        
        const traverse = (node: any) => {
          if (node.node) {
            if (node.node.id === 'virtual_root') {
              // 跳过虚拟根节点，但继续遍历其子节点
              if (node.children && Array.isArray(node.children)) {
                node.children.forEach((child: any) => traverse(child));
              }
              return;
            }

            // 添加所有文档节点（文档、任务文档等都是文档类型）
            documents.push({
              id: node.node.id,
              name: node.node.title,
              type: node.node.type
            });
            
            if (node.children && Array.isArray(node.children)) {
              node.children.forEach((child: any) => traverse(child));
            }
          }
        };
        
        roots.forEach(rootNode => traverse(rootNode));
        return documents;
      };
      
      const normalizedTree = normalizeTree(treeResult.tree);
      const documents = extractDocuments(normalizedTree);
      
      setTaskList(tasks);
      setDocumentList(documents);
    } catch (error) {
      console.error('加载任务和文档列表失败:', error);
      // 如果API调用失败，设置空数据并显示提示
      setTaskList([]);
      setDocumentList([]);
      message.warning('加载任务和文档列表失败，请检查网络连接后重试');
    }
  };

  const handleAddReference = () => {
    setEditingReference(null);
    form.resetFields();
    setAddModalVisible(true);
  };

  const handleEditReference = (reference: Reference) => {
    setEditingReference(reference);
    form.setFieldsValue({
      task_id: reference.task_id,
      document_id: reference.document_id,
      anchor: reference.anchor ?? undefined,
      context: reference.context ?? undefined
    });
    setAddModalVisible(true);
  };

  const handleDeleteReference = async (referenceId: string) => {
    if (!onDeleteReference) {
      message.warning('当前暂不支持删除引用');
      return;
    }
    try {
      await onDeleteReference(referenceId);
    } catch (error) {
      console.error('引用删除失败:', error);
      message.error('引用删除失败，请稍后重试');
    }
  };

  const handleModalOk = async () => {
    try {
      const values = await form.validateFields();
      const trimmedAnchor = values.anchor?.trim();
      const trimmedContext = values.context?.trim();

      const basePayload = {
        task_id: values.task_id,
        document_id: values.document_id
      };

      if (editingReference) {
        if (!onUpdateReference) {
          message.warning('当前暂不支持更新引用');
          return;
        }
        const updatePayload: Partial<Reference> = {
          ...basePayload
        };
        if (trimmedAnchor) {
          updatePayload.anchor = trimmedAnchor;
        }
        if (trimmedContext) {
          updatePayload.context = trimmedContext;
        }
        await onUpdateReference(editingReference.id, updatePayload);
        message.success('引用更新成功');
      } else {
        if (!onAddReference) {
          message.warning('当前暂不支持添加引用');
          return;
        }
        const newReference: Omit<Reference, 'id' | 'created_at' | 'updated_at'> = {
          ...basePayload,
          status: 'active' as ReferenceStatus,
          version: 1
        };
        if (trimmedAnchor) {
          newReference.anchor = trimmedAnchor;
        }
        if (trimmedContext) {
          newReference.context = trimmedContext;
        }
        await onAddReference(newReference);
        message.success('引用添加成功');
      }

      setAddModalVisible(false);
      setEditingReference(null);
      form.resetFields();
    } catch (error: any) {
      if (!error?.errorFields) {
        console.error('引用保存失败:', error);
        message.error('引用保存失败，请稍后重试');
      }
    }
  };

  const handleModalCancel = () => {
    setAddModalVisible(false);
    setEditingReference(null);
    form.resetFields();
  };

  const getTaskName = (taskId: string): string => {
    const task = taskList.find(t => t.id === taskId);
    return task?.name || taskId;
  };

  const getDocumentName = (documentId: string): string => {
    const document = documentList.find(d => d.id === documentId);
    return document?.name || documentId;
  };

  const handleReferenceClick = (reference: Reference) => {
    console.log('[ReferencePanel] handleReferenceClick called with reference:', reference);
    console.log('[ReferencePanel] Passing document_id to onReferenceClick:', reference.document_id);
    if (reference.document_id.startsWith('task_')) {
      console.error('[ReferencePanel] WARNING: reference.document_id is a taskId:', reference.document_id);
    }
    onReferenceClick?.(reference.document_id, reference);
  };

  return (
    <div style={{ fontSize: 12, color: '#1f2937' }}>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Text strong style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 14 }}>
          <LinkOutlined /> 文档引用关系
        </Text>
        <Button 
          type="primary" 
          size="small" 
          icon={<PlusOutlined />}
          onClick={handleAddReference}
        >
          添加引用
        </Button>
      </div>

      <List
        size="small"
        loading={loading}
        dataSource={references}
        style={{ fontSize: 12 }}
        locale={{ emptyText: '暂无引用关系' }}
        renderItem={(reference) => (
          <List.Item
            style={{ alignItems: 'flex-start', padding: '8px 0' }}
            actions={[
              <Button
                key="edit"
                type="text"
                size="small"
                icon={<EditOutlined />}
                onClick={() => handleEditReference(reference)}
              />,
              <Popconfirm
                key="delete"
                title="确定删除此引用吗？"
                onConfirm={() => handleDeleteReference(reference.id)}
                okText="确定"
                cancelText="取消"
              >
                <Button
                  type="text"
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                />
              </Popconfirm>
            ]}
          >
            <List.Item.Meta
              avatar={<FileTextOutlined />}
              title={
                <Space>
                  <Tag color={statusConfig[reference.status]?.color} style={{ fontSize: 12, padding: '2px 6px' }}>
                    {statusConfig[reference.status]?.label}
                  </Tag>
                  <Button
                    type="link"
                    size="small"
                    onClick={() => handleReferenceClick(reference)}
                    style={{ padding: 0, fontSize: 12 }}
                  >
                    {getDocumentName(reference.document_id)}
                  </Button>
                  <Badge count={`v${reference.version}`} showZero style={{ backgroundColor: '#e5e7eb', color: '#111827', fontSize: 10 }} />
                </Space>
              }
              description={
                <div style={{ display: 'grid', gap: 4 }}>
                  <div><span style={{ color: '#6b7280', marginRight: 4 }}>任务</span>{getTaskName(reference.task_id)}</div>
                  <div>
                    <span style={{ color: '#6b7280', marginRight: 4 }}>锚点</span>
                    {reference.anchor && reference.anchor.trim() ? reference.anchor : '未设置'}
                  </div>
                  <div style={{ color: '#4b5563' }}>
                    {reference.context && reference.context.trim() ? reference.context : '未填写引用上下文'}
                  </div>
                </div>
              }
            />
          </List.Item>
        )}
      />

      <Modal
        title={editingReference ? '编辑引用' : '添加引用'}
        open={addModalVisible}
        onOk={handleModalOk}
        onCancel={handleModalCancel}
        okText="确定"
        cancelText="取消"
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          requiredMark={false}
        >
          <Form.Item
            label="关联任务"
            name="task_id"
            rules={[{ required: true, message: '请选择关联任务' }]}
          >
            <Select 
              placeholder="选择关联任务" 
              showSearch
              optionFilterProp="children"
              filterOption={(input, option) =>
                (option?.children as unknown as string)?.toLowerCase().includes(input.toLowerCase())
              }
            >
              {taskList.map(task => (
                <Option key={task.id} value={task.id}>
                  <Space>
                    <span>{task.name}</span>
                    {task.status && <Tag color={
                      task.status === 'completed' ? 'green' :
                      task.status === 'in-progress' ? 'blue' :
                      task.status === 'review' ? 'orange' : 'default'
                    } style={{ fontSize: '11px', padding: '1px 4px' }}>{task.status}</Tag>}
                  </Space>
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            label="引用文档"
            name="document_id"
            rules={[{ required: true, message: '请选择引用文档' }]}
          >
            <Select placeholder="选择引用文档" showSearch>
              {documentList.map(doc => (
                <Option key={doc.id} value={doc.id}>
                  {doc.name}
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            label="锚点定位"
            name="anchor"
            rules={[
              {
                validator: (_rule, value) => {
                  if (!value || !value.trim()) {
                    return Promise.resolve();
                  }
                  const trimmed = value.trim();
                  if (trimmed.length > ANCHOR_MAX_LENGTH) {
                    return Promise.reject(new Error(`锚点长度不能超过 ${ANCHOR_MAX_LENGTH} 个字符`));
                  }
                  if (/\r|\n|\t/.test(trimmed)) {
                    return Promise.reject(new Error('锚点不能包含换行或制表符'));
                  }
                  return Promise.resolve();
                }
              }
            ]}
            extra="选填：例如章节号、段落编号、代码行号等"
          >
            <Input placeholder="输入锚点定位信息，如 '2.1.3' 或 'line:156'" allowClear />
          </Form.Item>

          <Form.Item
            label="引用上下文"
            name="context"
          >
            <Input.TextArea
              rows={4}
              placeholder="描述引用的具体内容和上下文信息"
              maxLength={500}
              showCount
              allowClear
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ReferencePanel;