import React, { useState, useEffect, useMemo } from 'react';
import { Button, List, Modal, Form, Input, Select, message, Spin, Dropdown, Tag, Collapse } from 'antd';
import type { MenuProps } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, UserOutlined, AppstoreOutlined, CheckCircleOutlined, ClockCircleOutlined, CopyOutlined, MoreOutlined } from '@ant-design/icons';
import { ProjectTask, TimeRangeFilter, getProjectTasks, createProjectTask, updateProjectTask, deleteProjectTask } from '../api/tasks';
import { getUsers, User } from '../api/users';
import { useTaskRefresh } from '../contexts/TaskRefreshContext';

const { TextArea } = Input;
const { Option } = Select;

const STATUS_OPTIONS = [
  { value: 'in-progress', label: '进行中', color: 'blue' },
  { value: 'todo', label: '待开始', color: 'default' },
  { value: 'completed', label: '已完成', color: 'green' },
  { value: 'cancelled', label: '已取消', color: 'red' },
];

// 获取状态配置
const getStatusConfig = (status?: string) => {
  return STATUS_OPTIONS.find(opt => opt.value === status) || { value: '', label: '未设置', color: 'default' };
};

interface Props {
  projectId: string;
  currentTask?: string;
  onTaskSelect: (taskId: string) => void;
}

const ProjectTaskSidebar: React.FC<Props> = ({ projectId, currentTask, onTaskSelect }) => {
  const [tasks, setTasks] = useState<ProjectTask[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingTask, setEditingTask] = useState<ProjectTask | null>(null);
  const [form] = Form.useForm();
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [assigneeFilter, setAssigneeFilter] = useState<string | undefined>();
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [timeRangeFilter, setTimeRangeFilter] = useState<TimeRangeFilter | undefined>();
  const { triggerRefresh } = useTaskRefresh();

  useEffect(() => {
    if (projectId) {
      loadTasks();
      loadUsers();
    }
  }, [projectId, searchQuery, timeRangeFilter]);

  const loadTasks = async () => {
    if (!projectId) return;
    setLoading(true);
    try {
      const result = await getProjectTasks(projectId, searchQuery || undefined, timeRangeFilter);
      setTasks(result.data || []);
    } catch (error: any) {
      // 对403权限错误不显示提示，让无权限页面处理
      if (error?.response?.status !== 403) {
        message.error('加载任务列表失败');
      }
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  const loadUsers = async () => {
    try {
      const result = await getUsers();
      setUsers(result.data || []);
    } catch (error) {
      console.error('Failed to load users:', error);
    }
  };

  const handleCreate = () => {
    setEditingTask(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (task: ProjectTask) => {
    setEditingTask(task);
    form.setFieldsValue(task);
    setModalVisible(true);
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      
      if (editingTask) {
        // 更新任务
        await updateProjectTask(projectId, editingTask.id, values);
        message.success('任务更新成功');
      } else {
        // 创建任务
        await createProjectTask(projectId, values);
        message.success('任务创建成功');
      }
      
      setModalVisible(false);
      loadTasks();
      // 触发任务统计数据刷新
      triggerRefresh();
    } catch (error) {
      message.error(editingTask ? '任务更新失败' : '任务创建失败');
      console.error(error);
    }
  };

  const handleDelete = async (taskId: string) => {
    try {
      await deleteProjectTask(projectId, taskId);
      message.success('任务删除成功');
      loadTasks();
      if (currentTask === taskId) {
        onTaskSelect(''); // 清除当前选中的任务
      }
      // 触发任务统计数据刷新
      triggerRefresh();
    } catch (error) {
      message.error('任务删除失败');
      console.error(error);
    }
  };

  const handleCopyTaskId = async (taskId: string) => {
    try {
      await navigator.clipboard.writeText(taskId);
      message.success(`任务ID已复制: ${taskId}`);
    } catch (error) {
      console.error('复制失败:', error);
      message.error('复制失败，请手动复制');
    }
  };

  const handleStatusChange = async (taskId: string, newStatus: string) => {
    try {
      await updateProjectTask(projectId, taskId, { status: newStatus });
      const statusConfig = getStatusConfig(newStatus);
      message.success(`任务状态已更新为：${statusConfig.label}`);
      loadTasks();
      // 触发任务统计数据刷新
      triggerRefresh();
    } catch (error) {
      message.error('状态更新失败');
      console.error(error);
    }
  };

  const visibleTasks = useMemo(() => {
    const filtered = tasks.filter(task => {
      if (assigneeFilter && task.assignee !== assigneeFilter) {
        return false;
      }
      return true;
    });

    // 按状态分组
    const grouped: Record<string, ProjectTask[]> = {
      'in-progress': [],
      'todo': [],
      'completed': [],
      'cancelled': [],
    };

    filtered.forEach(task => {
      const status = task.status || 'todo';
      if (grouped[status]) {
        grouped[status].push(task);
      } else {
        grouped['todo'].push(task); // 默认
      }
    });

    return grouped;
  }, [tasks, assigneeFilter]);

  const total = tasks.length;
  const filtered = Object.values(visibleTasks).flat().length;
  const filterApplied = Boolean(assigneeFilter || searchQuery);

  return (
    <div style={{ width: 250, borderRight: '1px solid #f0f0f0', height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div style={{ padding: 16, borderBottom: '1px solid #f0f0f0' }}>
        <Button
          block
          type="dashed"
          style={{ marginBottom: 12 }}
          onClick={() => onTaskSelect('')}
        >
          查看项目文档
        </Button>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
          <span style={{ fontWeight: 600, fontSize: 14 }}>项目任务</span>
          <Button
            type="primary"
            size="small"
            icon={<PlusOutlined />}
            onClick={handleCreate}
          >
            新建
          </Button>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <Input
            placeholder="搜索任务名"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            size="small"
            allowClear
          />
          <Select
            placeholder="按更新时间筛选"
            allowClear
            size="small"
            style={{ width: '100%' }}
            value={timeRangeFilter}
            onChange={(value) => setTimeRangeFilter(value as TimeRangeFilter | undefined)}
          >
            <Option value="today">今天</Option>
            <Option value="week">本周</Option>
            <Option value="month">本月</Option>
          </Select>
        </div>
        <div style={{ fontSize: 12, color: '#666', marginTop: 8 }}>
          共 {filterApplied ? `${filtered}/${total}` : total} 个任务
        </div>
      </div>

      <div style={{ flex: 1, overflow: 'auto' }}>
        <Spin spinning={loading}>
          <Collapse
            defaultActiveKey={['in-progress']}
            ghost
            items={STATUS_OPTIONS.map(status => ({
              key: status.value,
              label: (
                <span>
                  <Tag color={status.color} style={{ marginRight: 8 }}>{status.label}</Tag>
                  ({visibleTasks[status.value]?.length || 0})
                </span>
              ),
              children: (visibleTasks[status.value] || []).length === 0 ? (
                <div />
              ) : (
                <List
                  dataSource={visibleTasks[status.value] || []}
                  renderItem={(task) => {
                    const menu: MenuProps = {
                      items: [
                        {
                          key: 'status',
                          label: '修改状态',
                          icon: <CheckCircleOutlined />,
                          children: STATUS_OPTIONS.map(opt => ({
                            key: `status-${opt.value}`,
                            label: (
                              <span>
                                <Tag color={opt.color} style={{ marginRight: 8 }}>{opt.label}</Tag>
                                {task.status === opt.value && <CheckCircleOutlined style={{ color: '#52c41a' }} />}
                              </span>
                            ),
                            onClick: () => {
                              if (task.status !== opt.value) {
                                handleStatusChange(task.id, opt.value);
                              }
                            },
                          })),
                        },
                        { type: 'divider' },
                        {
                          key: 'copy-id',
                          icon: <CopyOutlined />,
                          label: '复制任务ID',
                        },
                        {
                          key: 'edit',
                          icon: <EditOutlined />,
                          label: '编辑任务',
                        },
                        {
                          key: 'delete',
                          icon: <DeleteOutlined />,
                          label: '删除任务',
                          danger: true,
                        },
                      ],
                      onClick: ({ key }) => {
                        if (key === 'copy-id') {
                          handleCopyTaskId(task.id);
                        }
                        if (key === 'edit') {
                          handleEdit(task);
                        }
                        if (key === 'delete') {
                          Modal.confirm({
                            title: '确定删除这个任务吗？',
                            okText: '删除',
                            okButtonProps: { danger: true },
                            cancelText: '取消',
                            onOk: () => handleDelete(task.id),
                          });
                        }
                      },
                    };

                    return (
                      <Dropdown key={task.id} trigger={['contextMenu']} menu={menu}>
                        <List.Item
                          key={task.id}
                          className={`task-item ${currentTask === task.id ? 'active' : ''}`}
                          style={{
                            padding: '12px 16px',
                            cursor: 'pointer',
                            backgroundColor: currentTask === task.id ? '#e6f7ff' : 'transparent',
                            borderLeft: currentTask === task.id ? '3px solid #1890ff' : '3px solid transparent',
                          }}
                          onClick={() => onTaskSelect(task.id)}
                        >
                          <div style={{ display: 'flex', width: '100%', alignItems: 'flex-start', gap: 8 }}>
                            <div style={{ flex: 1, minWidth: 0 }}>
                              <div style={{ fontSize: 13, fontWeight: 500, marginBottom: 4, display: 'flex', alignItems: 'center', gap: 6 }}>
                                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                  {task.name}({task.id})
                                </span>
                              </div>
                              <div style={{ fontSize: 11, color: '#666' }}>
                                <div style={{ marginBottom: 2 }}>
                                  <ClockCircleOutlined style={{ marginRight: 4 }} />
                                  {new Date(task.updated_at).toLocaleDateString('zh-CN')}
                                </div>
                                {task.assignee && (
                                  <div style={{ marginBottom: 2 }}>
                                    <UserOutlined style={{ marginRight: 4 }} />
                                    {task.assignee}
                                  </div>
                                )}
                              </div>
                            </div>
                            <Dropdown menu={menu} trigger={['click']} placement="bottomRight">
                              <Button
                                type="text"
                                size="small"
                                icon={<MoreOutlined />}
                                onClick={(e) => e.stopPropagation()}
                                style={{ color: '#666' }}
                              />
                            </Dropdown>
                          </div>
                        </List.Item>
                      </Dropdown>
                    );
                  }}
                />
              ),
            }))}
          />
        </Spin>
      </div>

      <Modal
        title={editingTask ? '编辑任务' : '创建任务'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={500}
        okText={editingTask ? '更新' : '创建'}
        cancelText="取消"
      >
        <Form
          form={form}
          layout="vertical"
          preserve={false}
        >
          <Form.Item
            name="name"
            label="任务名称"
            rules={[{ required: true, message: '请输入任务名称' }]}
          >
            <Input placeholder="输入任务名称" />
          </Form.Item>

          <Form.Item
            name="assignee"
            label="负责人"
          >
            <Select
              placeholder="选择负责人"
              allowClear
              showSearch
              filterOption={(input, option) =>
                String(option?.children || '').toLowerCase().includes(input.toLowerCase())
              }
            >
              {users.map(user => (
                <Option key={user.username} value={user.username}>
                  {user.username}
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="feature_id"
            label="关联特性ID"
          >
            <Input placeholder="输入特性ID，如 N-F01-S01" />
          </Form.Item>

          <Form.Item
            name="feature_name"
            label="特性名称"
          >
            <Input placeholder="输入特性名称" />
          </Form.Item>

          <Form.Item
            name="module"
            label="模块"
          >
            <Input placeholder="输入模块名称" />
          </Form.Item>

          <Form.Item
            name="status"
            label="状态"
          >
            <Select placeholder="选择状态" allowClear>
              <Option value="todo">待开始</Option>
              <Option value="in-progress">进行中</Option>
              <Option value="completed">已完成</Option>
              <Option value="cancelled">已取消</Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="description"
            label="描述"
          >
            <TextArea
              placeholder="输入任务描述"
              rows={3}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ProjectTaskSidebar;