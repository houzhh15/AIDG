import React from 'react';
import { Tabs } from 'antd';
import { FileSearchOutlined, SettingOutlined } from '@ant-design/icons';
import { FeatureList } from './FeatureList';
import { ArchitectureDesign } from './ArchitectureDesign';
import { TechDesign } from './TechDesign';
import { ProjectFeatureList } from './project/ProjectFeatureList';
import { ProjectArchitectureDesign } from './project/ProjectArchitectureDesign';
import { ProjectTechDesign } from './project/ProjectTechDesign';
// Lite: removed DocumentManagementSystem, ProjectStatusPage, PromptsManagement

type DeliverableMode = 'task' | 'project';

const { TabPane } = Tabs;

interface DeliverablesProps {
  taskId?: string; // 兼容旧调用
  mode?: DeliverableMode;
  targetId?: string; // 当 mode=project 时使用
  liteMode?: boolean; // lite 模式下仅显示特性列表和架构设计
}

export const Deliverables: React.FC<DeliverablesProps> = ({ taskId, mode='task', targetId, liteMode = false }) => {
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

  ];

  return (
    <div style={containerStyle}>
      <Tabs
        defaultActiveKey="features"
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