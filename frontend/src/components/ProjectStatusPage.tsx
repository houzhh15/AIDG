import React, { useState, useEffect } from 'react';
import { Space, Divider, Alert, Spin } from 'antd';
import ProjectOverview from './ProjectOverview';
import RoadmapTimeline from './RoadmapTimeline';
import TaskDashboard from './TaskDashboard';
import TimeProgress from './TimeProgress';
import TaskSummaryPanel from './TaskSummaryPanel';

interface Props {
  projectId: string;
  taskId?: string; // 可选，用于任务总结面板
}

const ProjectStatusPage: React.FC<Props> = ({ projectId, taskId }) => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // 页面加载时的初始化逻辑
    if (!projectId) {
      setError('缺少项目ID');
      return;
    }
    
    setError(null);
  }, [projectId]);

  // 错误边界处理
  if (error) {
    return (
      <div style={{ padding: '24px' }}>
        <Alert
          message="页面加载错误"
          description={error}
          type="error"
          showIcon
        />
      </div>
    );
  }

  // 处理任务仪表盘的点击事件
  const handleTaskClick = (status: 'completed' | 'in-progress' | 'todo') => {
    // 可以在这里实现跳转到任务列表页面，并按状态筛选
    console.log('Navigate to tasks with status:', status);
  };

  return (
    <Spin spinning={loading}>
      <div style={{ padding: '24px', backgroundColor: '#f0f2f5', minHeight: '100vh' }}>
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          {/* 1. 任务仪表盘（含本周趋势） */}
          <div id="task-dashboard">
            <TaskDashboard projectId={projectId} onTaskClick={handleTaskClick} />
          </div>

          <Divider />

          {/* 2. 项目基本信息 */}
          <div id="project-overview">
            <ProjectOverview projectId={projectId} />
          </div>

          <Divider />

          {/* 3. 项目roadmap */}
          <div id="roadmap">
            <RoadmapTimeline projectId={projectId} />
          </div>

          <Divider />

          {/* 4. 时间维度进展 */}
          <div id="time-progress">
            <TimeProgress projectId={projectId} />
          </div>

          {taskId && (
            <>
              <Divider />

              {/* 5. 任务总结（可选） */}
              <div id="task-summary">
                <TaskSummaryPanel projectId={projectId} taskId={taskId} />
              </div>
            </>
          )}
        </Space>
      </div>
    </Spin>
  );
};

export default ProjectStatusPage;
