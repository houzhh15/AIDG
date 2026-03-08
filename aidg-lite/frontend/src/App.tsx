import React, { useEffect, useState } from 'react';
import { Layout, message } from 'antd';
import { ProjectSidebar } from './components/ProjectSidebar';
import { Deliverables } from './components/Deliverables';
import ProjectTaskSidebar from './components/ProjectTaskSidebar';
import TaskDocuments from './components/TaskDocuments';
import { smartLogin, loadAuth, onAuthChange } from './api/auth';
import { TaskRefreshProvider } from './contexts/TaskRefreshContext';

const { Content } = Layout;

const App: React.FC = () => {
  const [auth, setAuth] = useState(loadAuth());
  const [loggingIn, setLoggingIn] = useState(false);
  const [currentProject, setCurrentProject] = useState<string | undefined>();
  const [currentProjectTask, setCurrentProjectTask] = useState<string | undefined>();
  const [projectTaskSidebarCollapsed, setProjectTaskSidebarCollapsed] = useState(false);

  // Auto-login for lite mode
  useEffect(() => {
    if (!auth && !loggingIn) {
      setLoggingIn(true);
      smartLogin('local', 'lite-admin-default')
        .then(a => setAuth(a))
        .catch(() => {
          smartLogin('local', '').then(a => setAuth(a)).catch(() => {});
        })
        .finally(() => setLoggingIn(false));
    }
  }, [auth, loggingIn]);

  // Listen for auth changes
  useEffect(() => {
    const unsubscribe = onAuthChange((newAuth) => {
      setAuth(newAuth);
      if (!newAuth) {
        setCurrentProject(undefined);
      }
    });
    return unsubscribe;
  }, []);

  if (!auth) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100dvh', background: '#f5f7fa' }}>
        <div style={{ color: '#999' }}>正在连接服务器...</div>
      </div>
    );
  }

  return (
    <TaskRefreshProvider>
      <Layout className="app-root-layout" style={{ height: '100dvh', overflow: 'hidden' }}>
        <Layout style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
          <ProjectSidebar
            current={currentProject}
            onSelect={(projectId) => {
              setCurrentProject(projectId);
              setCurrentProjectTask(undefined);
            }}
            scopes={auth?.scopes || []}
          />
          {currentProject && (
            <ProjectTaskSidebar
              projectId={currentProject}
              currentTask={currentProjectTask}
              onTaskSelect={setCurrentProjectTask}
              collapsed={projectTaskSidebarCollapsed}
              onCollapse={setProjectTaskSidebarCollapsed}
              scopes={auth?.scopes || []}
            />
          )}
          <Content style={{ display: 'flex', height: '100%', minHeight: 0 }}>
            <div className="scroll-region" style={{ flex: 1, padding: 12, minWidth: 0, height: '100%' }}>
              {!currentProject ? (
                <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#999' }}>
                  请选择或创建一个项目
                </div>
              ) : !currentProjectTask ? (
                <Deliverables mode="project" targetId={currentProject} liteMode={true} />
              ) : (
                <TaskDocuments projectId={currentProject} taskId={currentProjectTask} liteMode={true} />
              )}
            </div>
          </Content>
        </Layout>
      </Layout>
    </TaskRefreshProvider>
  );
};

export default App;
