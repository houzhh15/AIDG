import React, { useState, useEffect } from 'react';
import { Select, Spin, message, Tooltip, Button } from 'antd';
import { FolderOpenOutlined } from '@ant-design/icons';
import { listProjects } from '../api/projects';
import { getProjectTasks, ProjectTask } from '../api/tasks';
import { getCurrentTask, setCurrentTask, CurrentTaskInfo } from '../api/currentTask';
import { loadAuth, onAuthChange } from '../api/auth';
import { useTaskRefresh } from '../contexts/TaskRefreshContext';

interface ProjectOptionGroup {
  label: string;
  options: { label: string; value: string }[];
}

const ProjectTaskSelector: React.FC = () => {
  const [projects, setProjects] = useState<{ id: string; name: string }[]>([]);
  const [tasksByProject, setTasksByProject] = useState<Record<string, ProjectTask[]>>({});
  const [loadingProjects, setLoadingProjects] = useState(false);
  const [loadingTasks, setLoadingTasks] = useState(false);
  const [saving, setSaving] = useState(false);
  const [current, setCurrent] = useState<CurrentTaskInfo | null>(null);
  const { refreshTrigger } = useTaskRefresh();

  // 初始与登录状态变化时再加载当前任务，避免未登录 401 噪音
  useEffect(() => {
    const auth = loadAuth();
    if (auth) {
      loadCurrent();
    }
    loadProjects();

    // 监听登录/登出变化，动态刷新当前任务
    const dispose = onAuthChange(a => {
      if (a) {
        loadCurrent();
      } else {
        setCurrent(null);
      }
    });
    return () => dispose();
  }, []);

  // 监听任务刷新触发器，当任务状态改变时重新加载任务列表
  useEffect(() => {
    if (projects.length > 0) {
      // 重新加载所有项目的任务列表
      const reloadAllTasks = async () => {
        setLoadingTasks(true);
        try {
          for (const p of projects) {
            const r = await getProjectTasks(p.id);
            if (r.success && r.data) {
              setTasksByProject(prev => ({ ...prev, [p.id]: r.data! }));
            }
          }
        } catch (e) {
          console.error('reloadAllTasks failed', e);
        } finally {
          setLoadingTasks(false);
        }
      };
      reloadAllTasks();
    }
  }, [refreshTrigger, projects]);

  const loadCurrent = async () => {
    try {
      const r = await getCurrentTask();
      if (r.success) setCurrent(r.data || null);
    } catch (e) {
      console.warn('load current task failed', e);
    }
  };

  const loadProjects = async () => {
    setLoadingProjects(true);
    try {
      const ps = await listProjects();
      setProjects(ps);
    } catch (e) {
      console.error('listProjects failed', e);
    } finally {
      setLoadingProjects(false);
    }
  };

  const ensureProjectTasks = async (projectId: string) => {
    if (tasksByProject[projectId]) return;
    setLoadingTasks(true);
    try {
      const r = await getProjectTasks(projectId);
      if (r.success && r.data) {
        setTasksByProject(prev => ({ ...prev, [projectId]: r.data! }));
      }
    } catch (e) {
      console.error('getProjectTasks failed', e);
    } finally {
      setLoadingTasks(false);
    }
  };

  const handleOpen = async (open: boolean) => {
    if (open) {
      for (const p of projects) {
        // 顺序加载，避免大并发
        // eslint-disable-next-line no-await-in-loop
        await ensureProjectTasks(p.id);
      }
    }
  };

  const onChange = async (value?: string) => {
    if (!value) {
      message.info('未选择任务');
      return;
    }
    const [project_id, task_id] = value.split('::');
    setSaving(true);
    try {
      await setCurrentTask({ project_id, task_id });
      await loadCurrent();
      message.success('已设置当前任务');
    } catch (e) {
      message.error('设置失败');
    } finally {
      setSaving(false);
    }
  };

  // 基于当前登录用户的任务视图构建（按钮所有登录用户可见）
  const auth = loadAuth();
  const username = auth?.username;
  const [showAll, setShowAll] = useState(false); // showAll toggle 所有登录用户可切换

  const grouped: ProjectOptionGroup[] = projects.map(p => {
    let tasks = tasksByProject[p.id] || [];
    // 只显示进行中的任务
    tasks = tasks.filter(t => t.status === 'in-progress');
    if (!showAll) {
      tasks = tasks.filter(t => t.assignee && t.assignee === username);
    }
    
    // 如果当前任务属于这个项目，但不在过滤结果中，则添加它
    if (current && current.project_id === p.id) {
      const currentTaskInList = tasks.find(t => t.id === current.task_id);
      if (!currentTaskInList && current.task_info) {
        // 将当前任务添加到列表开头
        tasks = [current.task_info, ...tasks];
      }
    }
    
    return {
      label: p.name || p.id,
      options: tasks.map(t => ({
        label: t.name || t.id,
        value: `${p.id}::${t.id}`
      }))
    };
  }).filter(g => g.options.length > 0);

  // 构造 value
  const value = current ? `${current.project_id}::${current.task_id}` : undefined;

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <span style={{ color: '#d0e6f9', fontSize: 12 }}>当前任务:</span>
      {username && (
        <Tooltip title={showAll ? '已显示全部任务' : '切换：显示全部任务'}>
          <Button
            size="small"
            type={showAll ? 'primary' : 'default'}
            onClick={() => setShowAll(v => !v)}
            style={{ padding: '0 10px' }}
          >{showAll ? '全部' : '我的'}</Button>
        </Tooltip>
      )}
      <Tooltip title={username ? (showAll ? '当前显示全部任务' : '当前仅显示指派给你的任务') : '请先登录以选择指派任务'}>
        <Select
        style={{ width: 300 }}
        placeholder="选择项目任务"
        loading={loadingProjects || loadingTasks || saving}
        onDropdownVisibleChange={handleOpen}
        value={value}
        onChange={onChange}
        showSearch
        optionFilterProp="label"
        suffixIcon={(loadingProjects || loadingTasks) ? <Spin size="small" /> : <FolderOpenOutlined />}
        options={grouped}
        disabled={!username}
        />
      </Tooltip>
    </div>
  );
};

export default ProjectTaskSelector;
