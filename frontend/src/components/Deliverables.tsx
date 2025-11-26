import React from 'react';
import { Tabs } from 'antd';
import { FileSearchOutlined, SettingOutlined, FolderOutlined, DashboardOutlined, FileTextOutlined } from '@ant-design/icons';
import { FeatureList } from './FeatureList';
import { ArchitectureDesign } from './ArchitectureDesign';
import { TechDesign } from './TechDesign';
import { ProjectFeatureList } from './project/ProjectFeatureList';
import { ProjectArchitectureDesign } from './project/ProjectArchitectureDesign';
import { ProjectTechDesign } from './project/ProjectTechDesign';
import DocumentManagementSystem from './DocumentManagementSystem';
import ProjectStatusPage from './ProjectStatusPage';
import PromptsManagement from './PromptsManagement';
import { loadAuth } from '../api/auth';

type DeliverableMode = 'task' | 'project';

const { TabPane } = Tabs;

interface DeliverablesProps {
  taskId?: string; // 兼容旧调用
  mode?: DeliverableMode;
  targetId?: string; // 当 mode=project 时使用
}

export const Deliverables: React.FC<DeliverablesProps> = ({ taskId, mode='task', targetId }) => {
  const effectiveTaskId = mode === 'task' ? (taskId || '') : '';
  const effectiveProjectId = mode === 'project' ? (targetId || '') : '';

  // TODO: project 模式下将引入 ProjectFeatureList / ProjectArchitecture / ProjectTechDesign
  const renderFeature = () => {
    if(mode==='task') return <FeatureList taskId={effectiveTaskId} />;
    return effectiveProjectId ? <ProjectFeatureList projectId={effectiveProjectId} /> : <div style={{ padding:12, color:'#999' }}>请选择项目</div>;
  };
  const renderArchitecture = () => {
    if(mode==='task') return <ArchitectureDesign taskId={effectiveTaskId} />;
    return effectiveProjectId ? <ProjectArchitectureDesign projectId={effectiveProjectId} /> : <div style={{ padding:12, color:'#999' }}>请选择项目</div>;
  };
  const renderTech = () => {
    if(mode==='task') return <TechDesign taskId={effectiveTaskId} />;
    return effectiveProjectId ? <ProjectTechDesign projectId={effectiveProjectId} /> : <div style={{ padding:12, color:'#999' }}>请选择项目</div>;
  };
  
  const renderDocumentManagement = () => {
    if(mode==='task') return <div style={{ padding:12, color:'#999' }}>任务模式下请查看文档管理标签页</div>;
    return effectiveProjectId ? (
      <div style={{ height: '100%', padding: '16px 0' }}>
        <DocumentManagementSystem 
          projectId={effectiveProjectId}
          taskId="project-level" // 项目级别文档管理，使用特殊标识
        />
      </div>
    ) : <div style={{ padding:12, color:'#999' }}>请选择项目</div>;
  };
  
  const renderPrompts = () => {
    if(mode==='task') return <div style={{ padding:12, color:'#999' }}>任务模式下暂不支持 Prompts 管理</div>;
    const currentUsername = loadAuth()?.username || '';
    return effectiveProjectId ? (
      <div style={{ height: '100%', padding: '16px 0' }}>
        <PromptsManagement 
          scope="project"
          projectId={effectiveProjectId}
          username={currentUsername}
        />
      </div>
    ) : <div style={{ padding:12, color:'#999' }}>请选择项目</div>;
  };
  const containerStyle: React.CSSProperties = {
    height: '100%',
    display: 'flex',
    flexDirection: 'column',
  };

  const tabsStyle: React.CSSProperties = {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    minHeight: 0,
  };

  const items = [
    // 项目模式下显示项目状态页面（第一个标签页）
    ...(mode === 'project' && effectiveProjectId ? [{
      key: 'status',
      label: (
        <span>
          <DashboardOutlined />
          项目状态
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'auto' }}>
          <ProjectStatusPage projectId={effectiveProjectId} />
        </div>
      ),
    }] : []),
    {
      key: 'features',
      label: (
        <span>
          <FileSearchOutlined />
          特性列表
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'hidden' }}>
          {renderFeature()}
        </div>
      ),
    },
    {
      key: 'architecture',
      label: (
        <span>
          <SettingOutlined />
          架构设计
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'hidden' }}>
          {renderArchitecture()}
        </div>
      ),
    },
    {
      key: 'documents',
      label: (
        <span>
          <FolderOutlined />
          文档管理
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'hidden' }}>
          {renderDocumentManagement()}
        </div>
      ),
    },
    // 项目模式下显示 Prompts 管理标签页
    ...(mode === 'project' && effectiveProjectId ? [{
      key: 'prompts',
      label: (
        <span>
          <FileTextOutlined />
          Prompts
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'hidden' }}>
          {renderPrompts()}
        </div>
      ),
    }] : []),
  ];

  return (
    <div style={containerStyle}>
      <Tabs
        defaultActiveKey={mode === 'project' && effectiveProjectId ? 'status' : 'features'}
        items={items}
        size="small"
        style={tabsStyle}
        tabBarStyle={{ 
          marginBottom: 16,
          paddingLeft: 8,
          paddingRight: 8,
          flexShrink: 0
        }}
        className="full-height-tabs"
      />
    </div>
  );
};