import React, { useState, useEffect } from 'react';
import {
  Card,
  Button,
  List,
  Modal,
  DatePicker,
  Input,
  message,
  Space,
  Popconfirm,
  Empty,
  Spin,
  Typography,
  Tag
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CalendarOutlined
} from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import isoWeek from 'dayjs/plugin/isoWeek';
import MarkdownViewer from './MarkdownViewer';
import {
  fetchTaskSummaries,
  addTaskSummary,
  updateTaskSummary,
  deleteTaskSummary,
  TaskSummary
} from '../api/taskSummaryApi';

dayjs.extend(isoWeek);

const { TextArea } = Input;
const { Title, Text } = Typography;

interface Props {
  projectId: string;
  taskId: string;
}

interface EditingData {
  id?: string;
  time: Dayjs;
  content: string;
}

const TaskSummaryPanel: React.FC<Props> = ({ projectId, taskId }) => {
  const [summaries, setSummaries] = useState<TaskSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingData, setEditingData] = useState<EditingData | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // 加载总结列表
  const loadSummaries = async () => {
    setLoading(true);
    try {
      const response = await fetchTaskSummaries(projectId, taskId);
      if (response.success && response.data) {
        // 按时间倒序排列
        const sorted = [...response.data].sort((a, b) => {
          return dayjs(b.time).valueOf() - dayjs(a.time).valueOf();
        });
        setSummaries(sorted);
      } else {
        message.error(response.message || '加载总结列表失败');
      }
    } catch (error) {
      message.error('加载总结列表失败: ' + (error as Error).message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (projectId && taskId) {
      loadSummaries();
    }
  }, [projectId, taskId]);

  // 打开新增弹窗
  const handleAdd = () => {
    setEditingData({
      time: dayjs(),
      content: ''
    });
    setModalVisible(true);
  };

  // 打开编辑弹窗
  const handleEdit = (summary: TaskSummary) => {
    setEditingData({
      id: summary.id,
      time: dayjs(summary.time),
      content: summary.content
    });
    setModalVisible(true);
  };

  // 提交表单
  const handleSubmit = async () => {
    if (!editingData) return;

    if (!editingData.content.trim()) {
      message.warning('请输入总结内容');
      return;
    }

    setSubmitting(true);
    try {
      if (editingData.id) {
        // 更新
        const response = await updateTaskSummary(
          projectId,
          taskId,
          editingData.id,
          {
            time: editingData.time.toISOString(),
            content: editingData.content
          }
        );
        if (response.success) {
          message.success('更新成功');
          setModalVisible(false);
          setEditingData(null);
          loadSummaries();
        } else {
          message.error(response.message || '更新失败');
        }
      } else {
        // 新增
        const response = await addTaskSummary(projectId, taskId, {
          time: editingData.time.toISOString(),
          content: editingData.content
        });
        if (response.success) {
          message.success('添加成功');
          setModalVisible(false);
          setEditingData(null);
          loadSummaries();
        } else {
          message.error(response.message || '添加失败');
        }
      }
    } catch (error) {
      message.error('操作失败: ' + (error as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  // 删除总结
  const handleDelete = async (summaryId: string) => {
    try {
      const response = await deleteTaskSummary(projectId, taskId, summaryId);
      if (response.success) {
        message.success('删除成功');
        loadSummaries();
      } else {
        message.error(response.message || '删除失败');
      }
    } catch (error) {
      message.error('删除失败: ' + (error as Error).message);
    }
  };

  // 格式化周数显示
  const formatWeekNumber = (weekNumber: string) => {
    // weekNumber 格式: YYYY-WW
    if (!weekNumber || !weekNumber.match(/^\d{4}-W\d{2}$/)) {
      return weekNumber;
    }
    return weekNumber;
  };

  return (
    <Card
      title={
        <Space>
          <CalendarOutlined />
          <span>任务总结</span>
        </Space>
      }
      extra={
        <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
          添加总结
        </Button>
      }
    >
      <Spin spinning={loading}>
        {summaries.length === 0 ? (
          <Empty description="暂无总结记录" />
        ) : (
          <List
            itemLayout="vertical"
            dataSource={summaries}
            renderItem={(summary) => (
              <List.Item
                key={summary.id}
                actions={[
                  <Button
                    key="edit"
                    type="link"
                    icon={<EditOutlined />}
                    onClick={() => handleEdit(summary)}
                  >
                    编辑
                  </Button>,
                  <Popconfirm
                    key="delete"
                    title="确认删除这条总结吗？"
                    onConfirm={() => handleDelete(summary.id)}
                    okText="确认"
                    cancelText="取消"
                  >
                    <Button type="link" danger icon={<DeleteOutlined />}>
                      删除
                    </Button>
                  </Popconfirm>
                ]}
              >
                <List.Item.Meta
                  title={
                    <Space>
                      <Tag color="blue">{formatWeekNumber(summary.week_number)}</Tag>
                      <Text type="secondary">
                        {dayjs(summary.time).format('YYYY-MM-DD HH:mm')}
                      </Text>
                      <Text type="secondary" style={{ fontSize: '12px' }}>
                        创建者: {summary.creator}
                      </Text>
                    </Space>
                  }
                />
                <div
                  style={{
                    marginTop: '8px',
                    padding: '12px',
                    backgroundColor: '#fafafa',
                    borderRadius: '4px'
                  }}
                >
                  <MarkdownViewer>{summary.content}</MarkdownViewer>
                </div>
              </List.Item>
            )}
          />
        )}
      </Spin>

      {/* 添加/编辑弹窗 */}
      <Modal
        title={editingData?.id ? '编辑总结' : '添加总结'}
        open={modalVisible}
        onCancel={() => {
          setModalVisible(false);
          setEditingData(null);
        }}
        onOk={handleSubmit}
        confirmLoading={submitting}
        width={800}
        okText="保存"
        cancelText="取消"
      >
        {editingData && (
          <Space direction="vertical" style={{ width: '100%' }} size="large">
            <div>
              <Text strong>选择时间</Text>
              <DatePicker
                showTime
                value={editingData.time}
                onChange={(date) => {
                  if (date) {
                    setEditingData({ ...editingData, time: date });
                  }
                }}
                style={{ width: '100%', marginTop: '8px' }}
                format="YYYY-MM-DD HH:mm:ss"
              />
              <Text type="secondary" style={{ fontSize: '12px', marginTop: '4px' }}>
                当前周数: {editingData.time.format('YYYY-[W]WW')}
              </Text>
            </div>

            <div>
              <Text strong>总结内容 (支持 Markdown)</Text>
              <TextArea
                value={editingData.content}
                onChange={(e) => {
                  setEditingData({ ...editingData, content: e.target.value });
                }}
                placeholder="请输入总结内容，支持 Markdown 格式"
                rows={12}
                style={{ marginTop: '8px', fontFamily: 'monospace' }}
              />
            </div>

            {editingData.content && (
              <div>
                <Text strong>预览</Text>
                <div
                  style={{
                    marginTop: '8px',
                    padding: '12px',
                    border: '1px solid #d9d9d9',
                    borderRadius: '4px',
                    backgroundColor: '#fafafa',
                    maxHeight: '300px',
                    overflow: 'auto'
                  }}
                >
                  <MarkdownViewer>{editingData.content}</MarkdownViewer>
                </div>
              </div>
            )}
          </Space>
        )}
      </Modal>
    </Card>
  );
};

export default TaskSummaryPanel;
