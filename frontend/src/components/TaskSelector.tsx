import React, { useState, useEffect } from 'react';
import { Select, Tooltip } from 'antd';
import { FolderOutlined } from '@ant-design/icons';
import { authedApi } from '../api/auth';

const { Option } = Select;

interface TaskSelectorProps {
  currentTaskId: string;
  placeholder?: string;
  style?: React.CSSProperties;
  onChange?: (taskId: string) => void;
  loading?: boolean;
}

interface Task {
  id: string;
  name?: string;
  createdAt?: string;
  status?: string;
}

export const TaskSelector: React.FC<TaskSelectorProps> = ({
  currentTaskId,
  placeholder = "选择源任务",
  style,
  onChange,
  loading: externalLoading = false
}) => {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState<boolean>(false);

  useEffect(() => {
    const fetchTasks = async () => {
      setLoading(true);
      try {
        const response = await authedApi.get('/tasks');
        // 后端返回的数据结构是 {tasks: [...]}
        const taskList = response.data.tasks || [];
        // 过滤掉当前任务
        const filteredTasks = taskList.filter((task: Task) => task.id !== currentTaskId);
        setTasks(filteredTasks);
      } catch (error) {
        console.error('Failed to fetch tasks:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchTasks();
  }, [currentTaskId]);

  // 返回显示文本（不截断）
  const getDisplayText = (task: Task) => {
    if (task.name && task.name.trim().length > 0) return task.name.trim();
    return task.id; // 无 name 显示完整 ID
  };

  return (
    <Select
      placeholder={placeholder}
      style={{ width: '100%', ...style }}
      onChange={onChange}
      loading={loading || externalLoading}
      showSearch
      optionFilterProp="children"
      filterOption={(input, option) =>
        option?.children?.toString().toLowerCase().includes(input.toLowerCase()) || false
      }
    >
      {tasks.map(task => {
        const text = getDisplayText(task);
        return (
          <Option key={task.id} value={task.id}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
              <FolderOutlined style={{ color: '#1890ff', flexShrink: 0 }} />
              <Tooltip title={<div style={{ maxWidth: 480, whiteSpace: 'normal', wordBreak: 'break-all' }}>{text}</div>}>
                <span style={{
                  display: 'inline-block',
                  maxWidth: 360,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap'
                }}>{text}</span>
              </Tooltip>
              {task.status && (
                <span style={{ 
                  fontSize: '12px', 
                  color: '#999',
                  marginLeft: 'auto',
                  flexShrink: 0
                }}>
                  {task.status}
                </span>
              )}
            </div>
          </Option>
        );
      })}
    </Select>
  );
};

export default TaskSelector;