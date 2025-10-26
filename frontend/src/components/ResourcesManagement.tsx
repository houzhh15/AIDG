/**
 * MCP Resources Management Component
 * 用户资源管理界面 - 支持查看、添加、编辑、删除 MCP 协议资源
 */

import React, { useState, useEffect } from 'react';
import {
  Table,
  Button,
  Modal,
  Input,
  Select,
  Space,
  Tag,
  message,
  Row,
  Col,
  Tooltip,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  EyeOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import {
  Resource,
  getUserResources,
  deleteResource,
} from '../api/resourceApi';
import ResourceEditorModal from './resources/ResourceEditorModal';

const { Option } = Select;

interface ResourcesManagementProps {
  username: string;
  className?: string;
}

const ResourcesManagement: React.FC<ResourcesManagementProps> = ({
  username,
  className = '',
}) => {
  const [resources, setResources] = useState<Resource[]>([]);
  const [loading, setLoading] = useState(false);
  const [viewModalVisible, setViewModalVisible] = useState(false);
  const [selectedResource, setSelectedResource] = useState<Resource | null>(null);

  // 编辑器 Modal 状态
  const [modalState, setModalState] = useState<{
    visible: boolean;
    mode: 'create' | 'edit';
    resource?: Resource;
  }>({
    visible: false,
    mode: 'create',
    resource: undefined
  });

  // 过滤器状态
  const [filters, setFilters] = useState<{
    visibility?: 'public' | 'private';
    projectId?: string;
    taskId?: string;
    autoAdded?: boolean;
  }>({});

  // 加载资源列表
  const loadResources = async () => {
    try {
      setLoading(true);
      const data = await getUserResources(username, filters);
      setResources(data);
    } catch (err) {
      message.error(`加载资源失败: ${err instanceof Error ? err.message : '未知错误'}`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadResources();
  }, [username, filters]);

  // 打开添加资源对话框
  const handleAdd = () => {
    setModalState({
      visible: true,
      mode: 'create',
      resource: undefined
    });
  };

  // 打开编辑资源对话框
  const handleEdit = (resource: Resource) => {
    setModalState({
      visible: true,
      mode: 'edit',
      resource
    });
  };

  // 查看资源详情
  const handleView = (resource: Resource) => {
    setSelectedResource(resource);
    setViewModalVisible(true);
  };

  // Modal 关闭回调
  const handleModalClose = () => {
    setModalState({
      ...modalState,
      visible: false
    });
  };

  // Modal 保存成功回调
  const handleModalSuccess = () => {
    handleModalClose();
    loadResources(); // 刷新资源列表
  };

  // 删除资源
  const handleDelete = (resource: Resource) => {
    Modal.confirm({
      title: '确认删除',
      content: (
        <div>
          <p>确定要删除资源 <strong>{resource.name}</strong> 吗?</p>
          <p style={{ color: '#999', fontSize: '12px' }}>ID: {resource.resourceId}</p>
        </div>
      ),
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          setLoading(true);
          await deleteResource(username, resource.resourceId);
          message.success('资源删除成功');
          loadResources();
        } catch (err) {
          message.error(`删除失败: ${err instanceof Error ? err.message : '未知错误'}`);
        } finally {
          setLoading(false);
        }
      },
    });
  };

  // 表格列定义
  const columns: ColumnsType<Resource> = [
    {
      title: '资源名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      ellipsis: true,
      render: (text) => <strong>{text}</strong>,
    },
    {
      title: '资源ID',
      dataIndex: 'resourceId',
      key: 'resourceId',
      width: 180,
      ellipsis: true,
      render: (text) => (
        <Tooltip title={text}>
          <code style={{ fontSize: '12px' }}>{text}</code>
        </Tooltip>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (text) => text || <span style={{ color: '#999' }}>-</span>,
    },
    {
      title: '可见性',
      dataIndex: 'visibility',
      key: 'visibility',
      width: 100,
      render: (visibility: 'public' | 'private') => (
        <Tag color={visibility === 'public' ? 'blue' : 'orange'}>
          {visibility === 'public' ? '公开' : '私有'}
        </Tag>
      ),
    },
    {
      title: '类型',
      dataIndex: 'autoAdded',
      key: 'autoAdded',
      width: 100,
      render: (autoAdded: boolean) => (
        <Tag color={autoAdded ? 'green' : 'default'}>
          {autoAdded ? '系统' : '自定义'}
        </Tag>
      ),
    },
    {
      title: '关联',
      key: 'association',
      width: 150,
      render: (_, record) => (
        <div style={{ fontSize: '12px' }}>
          {record.projectId && (
            <div>
              <Tag color="purple" style={{ marginBottom: 4 }}>
                项目: {record.projectId}
              </Tag>
            </div>
          )}
          {record.taskId && (
            <div>
              <Tag color="cyan">任务: {record.taskId}</Tag>
            </div>
          )}
          {!record.projectId && !record.taskId && (
            <span style={{ color: '#999' }}>-</span>
          )}
        </div>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 150,
      fixed: 'right',
      render: (_, record) => (
        <Space size="small">
          <Tooltip title="查看">
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handleView(record)}
            />
          </Tooltip>
          {!record.autoAdded && (
            <>
              <Tooltip title="编辑">
                <Button
                  type="link"
                  size="small"
                  icon={<EditOutlined />}
                  onClick={() => handleEdit(record)}
                />
              </Tooltip>
              <Tooltip title="删除">
                <Button
                  type="link"
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => handleDelete(record)}
                />
              </Tooltip>
            </>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div className={className} style={{ padding: '16px' }}>
      {/* 工具栏 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col flex="auto">
          <Space size="middle">
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
              添加资源
            </Button>
            <Button icon={<ReloadOutlined />} onClick={loadResources}>
              刷新
            </Button>
          </Space>
        </Col>
      </Row>

      {/* 过滤器 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Select
            placeholder="可见性"
            allowClear
            style={{ width: '100%' }}
            value={filters.visibility}
            onChange={(value) => setFilters({ ...filters, visibility: value })}
          >
            <Option value="public">公开</Option>
            <Option value="private">私有</Option>
          </Select>
        </Col>
        <Col span={6}>
          <Select
            placeholder="资源类型"
            allowClear
            style={{ width: '100%' }}
            value={filters.autoAdded}
            onChange={(value) => setFilters({ ...filters, autoAdded: value })}
          >
            <Option value={true}>系统资源</Option>
            <Option value={false}>自定义资源</Option>
          </Select>
        </Col>
        <Col span={6}>
          <Input
            placeholder="项目ID"
            allowClear
            value={filters.projectId}
            onChange={(e) => setFilters({ ...filters, projectId: e.target.value || undefined })}
          />
        </Col>
        <Col span={6}>
          <Input
            placeholder="任务ID"
            allowClear
            value={filters.taskId}
            onChange={(e) => setFilters({ ...filters, taskId: e.target.value || undefined })}
          />
        </Col>
      </Row>

      {/* 资源表格 */}
      <Table
        columns={columns}
        dataSource={resources}
        rowKey="resourceId"
        loading={loading}
        pagination={{
          defaultPageSize: 10,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条资源`,
        }}
        scroll={{ x: 1200 }}
      />

      {/* 资源编辑器 Modal（使用新的 ResourceEditorModal） */}
      <ResourceEditorModal
        mode={modalState.mode}
        visible={modalState.visible}
        initialResource={modalState.resource}
        username={username}
        onClose={handleModalClose}
        onSuccess={handleModalSuccess}
      />

      {/* 查看详情对话框 */}
      <Modal
        title="资源详情"
        open={viewModalVisible}
        onCancel={() => {
          setViewModalVisible(false);
          setSelectedResource(null);
        }}
        width={800}
        footer={[
          <Button key="close" onClick={() => setViewModalVisible(false)}>
            关闭
          </Button>,
        ]}
      >
        {selectedResource && (
          <div style={{ maxHeight: '600px', overflowY: 'auto' }}>
            <div style={{ marginBottom: 16 }}>
              <h3>{selectedResource.name}</h3>
              <p style={{ color: '#999', fontSize: '12px', fontFamily: 'monospace' }}>
                {selectedResource.resourceId}
              </p>
            </div>

            <div style={{ marginBottom: 16 }}>
              <strong>描述:</strong>
              <p>{selectedResource.description}</p>
            </div>

            <Row gutter={16} style={{ marginBottom: 16 }}>
              <Col span={8}>
                <strong>可见性:</strong>
                <div>
                  <Tag color={selectedResource.visibility === 'public' ? 'blue' : 'orange'}>
                    {selectedResource.visibility === 'public' ? '公开' : '私有'}
                  </Tag>
                </div>
              </Col>
              <Col span={8}>
                <strong>类型:</strong>
                <div>
                  <Tag color={selectedResource.autoAdded ? 'green' : 'default'}>
                    {selectedResource.autoAdded ? '系统资源' : '自定义资源'}
                  </Tag>
                </div>
              </Col>
              <Col span={8}>
                <strong>创建时间:</strong>
                <div>{new Date(selectedResource.createdAt).toLocaleString('zh-CN')}</div>
              </Col>
            </Row>

            {(selectedResource.projectId || selectedResource.taskId) && (
              <div style={{ marginBottom: 16 }}>
                <strong>关联信息:</strong>
                <div>
                  {selectedResource.projectId && (
                    <Tag color="purple" style={{ marginTop: 8 }}>
                      项目: {selectedResource.projectId}
                    </Tag>
                  )}
                  {selectedResource.taskId && (
                    <Tag color="cyan" style={{ marginTop: 8 }}>
                      任务: {selectedResource.taskId}
                    </Tag>
                  )}
                </div>
              </div>
            )}

            <div>
              <strong>资源内容:</strong>
              <pre
                style={{
                  background: '#f5f5f5',
                  padding: '12px',
                  borderRadius: '4px',
                  maxHeight: '300px',
                  overflow: 'auto',
                  fontSize: '12px',
                  lineHeight: '1.5',
                }}
              >
                {selectedResource.content}
              </pre>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default ResourcesManagement;
