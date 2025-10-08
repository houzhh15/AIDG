import React, { useState, useEffect } from 'react';
import {
  Card,
  Timeline,
  Button,
  Modal,
  Form,
  Input,
  DatePicker,
  Select,
  message,
  Spin,
  Tag,
  Space,
  Popconfirm,
  Empty,
  Radio
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  SyncOutlined
} from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import {
  fetchRoadmap,
  addRoadmapNode,
  updateRoadmapNode,
  deleteRoadmapNode,
  RoadmapNode,
  CreateNodeRequest
} from '../api/roadmapApi';

const { TextArea } = Input;
const { Option } = Select;

interface Props {
  projectId: string;
}

type StatusFilter = 'all' | 'completed' | 'in-progress' | 'todo';

const RoadmapTimeline: React.FC<Props> = ({ projectId }) => {
  const [loading, setLoading] = useState(false);
  const [nodes, setNodes] = useState<RoadmapNode[]>([]);
  const [version, setVersion] = useState(0);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingNode, setEditingNode] = useState<RoadmapNode | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [form] = Form.useForm();

  // 加载Roadmap
  const loadRoadmap = async () => {
    setLoading(true);
    try {
      const response = await fetchRoadmap(projectId);
      if (response.success && response.data) {
        setNodes(response.data.nodes || []);
        setVersion(response.data.version);
      } else {
        message.error(response.message || '加载Roadmap失败');
      }
    } catch (error: any) {
      // 对403权限错误不显示提示，让无权限页面处理
      if (error?.response?.status !== 403) {
        message.error('加载Roadmap失败: ' + (error as Error).message);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (projectId) {
      loadRoadmap();
    }
  }, [projectId]);

  // 打开添加弹窗
  const handleAdd = () => {
    setEditingNode(null);
    form.resetFields();
    form.setFieldsValue({
      status: 'todo'
    });
    setModalVisible(true);
  };

  // 打开编辑弹窗
  const handleEdit = (node: RoadmapNode) => {
    setEditingNode(node);
    form.setFieldsValue({
      date: dayjs(node.date),
      goal: node.goal,
      description: node.description,
      status: node.status
    });
    setModalVisible(true);
  };

  // 提交表单
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      const data: CreateNodeRequest = {
        date: values.date.format('YYYY-MM-DD'),
        goal: values.goal,
        description: values.description || '',
        status: values.status
      };

      setSubmitting(true);

      if (editingNode) {
        // 更新节点
        const response = await updateRoadmapNode(
          projectId,
          editingNode.id,
          data,
          version
        );
        if (response.success) {
          message.success('更新成功');
          setModalVisible(false);
          loadRoadmap();
        } else {
          message.error(response.message || '更新失败');
        }
      } else {
        // 添加节点
        const response = await addRoadmapNode(projectId, data);
        if (response.success) {
          message.success('添加成功');
          setModalVisible(false);
          loadRoadmap();
        } else {
          message.error(response.message || '添加失败');
        }
      }
    } catch (error) {
      if (error instanceof Error) {
        message.error('操作失败: ' + error.message);
      }
    } finally {
      setSubmitting(false);
    }
  };

  // 删除节点
  const handleDelete = async (nodeId: string) => {
    try {
      const response = await deleteRoadmapNode(projectId, nodeId);
      if (response.success) {
        message.success('删除成功');
        loadRoadmap();
      } else {
        message.error(response.message || '删除失败');
      }
    } catch (error) {
      message.error('删除失败: ' + (error as Error).message);
    }
  };

  // 获取状态标签
  const getStatusTag = (status: string) => {
    switch (status) {
      case 'completed':
        return <Tag color="success">已完成</Tag>;
      case 'in-progress':
        return <Tag color="processing">进行中</Tag>;
      case 'todo':
        return <Tag color="default">待开始</Tag>;
      default:
        return <Tag>{status}</Tag>;
    }
  };

  // 获取状态图标
  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'in-progress':
        return <SyncOutlined spin style={{ color: '#1890ff' }} />;
      case 'todo':
        return <ClockCircleOutlined style={{ color: '#faad14' }} />;
      default:
        return null;
    }
  };

  // 过滤节点
  const filteredNodes = nodes.filter((node) => {
    if (statusFilter === 'all') return true;
    return node.status === statusFilter;
  });

  // 按日期排序
  const sortedNodes = [...filteredNodes].sort((a, b) => {
    return new Date(a.date).getTime() - new Date(b.date).getTime();
  });

  return (
    <Card
      title="项目Roadmap"
      extra={
        <Space>
          <Radio.Group
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            size="small"
          >
            <Radio.Button value="all">全部</Radio.Button>
            <Radio.Button value="todo">待开始</Radio.Button>
            <Radio.Button value="in-progress">进行中</Radio.Button>
            <Radio.Button value="completed">已完成</Radio.Button>
          </Radio.Group>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加节点
          </Button>
        </Space>
      }
    >
      <Spin spinning={loading}>
        {sortedNodes.length === 0 ? (
          <Empty description="暂无Roadmap节点" />
        ) : (
          <Timeline mode="left">
            {sortedNodes.map((node) => (
              <Timeline.Item
                key={node.id}
                dot={getStatusIcon(node.status)}
                color={
                  node.status === 'completed'
                    ? 'green'
                    : node.status === 'in-progress'
                    ? 'blue'
                    : 'gray'
                }
              >
                <div style={{ paddingBottom: '16px' }}>
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <Space>
                      <strong>{dayjs(node.date).format('YYYY-MM-DD')}</strong>
                      {getStatusTag(node.status)}
                    </Space>
                    <div style={{ fontSize: '16px', fontWeight: 'bold' }}>
                      {node.goal}
                    </div>
                    {node.description && (
                      <div style={{ color: '#666' }}>{node.description}</div>
                    )}
                    <Space>
                      <Button
                        type="link"
                        size="small"
                        icon={<EditOutlined />}
                        onClick={() => handleEdit(node)}
                      >
                        编辑
                      </Button>
                      <Popconfirm
                        title="确认删除这个节点吗？"
                        onConfirm={() => handleDelete(node.id)}
                        okText="确认"
                        cancelText="取消"
                      >
                        <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                          删除
                        </Button>
                      </Popconfirm>
                    </Space>
                  </Space>
                </div>
              </Timeline.Item>
            ))}
          </Timeline>
        )}
      </Spin>

      {/* 添加/编辑弹窗 */}
      <Modal
        title={editingNode ? '编辑节点' : '添加节点'}
        open={modalVisible}
        onCancel={() => {
          setModalVisible(false);
          form.resetFields();
        }}
        onOk={handleSubmit}
        confirmLoading={submitting}
        width={600}
        okText="保存"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item
            label="日期"
            name="date"
            rules={[{ required: true, message: '请选择日期' }]}
          >
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            label="目标"
            name="goal"
            rules={[
              { required: true, message: '请输入目标' },
              { max: 50, message: '目标长度不能超过50字' }
            ]}
          >
            <Input placeholder="请输入目标" />
          </Form.Item>

          <Form.Item
            label="描述"
            name="description"
            rules={[{ max: 500, message: '描述长度不能超过500字' }]}
          >
            <TextArea rows={4} placeholder="请输入描述（可选）" />
          </Form.Item>

          <Form.Item
            label="状态"
            name="status"
            rules={[{ required: true, message: '请选择状态' }]}
          >
            <Select>
              <Option value="todo">待开始</Option>
              <Option value="in-progress">进行中</Option>
              <Option value="completed">已完成</Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
};

export default RoadmapTimeline;
