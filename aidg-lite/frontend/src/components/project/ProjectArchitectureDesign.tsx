import React from 'react';
import ProjectDocument from './ProjectDocument';

interface Props { projectId: string; }

/**
 * 项目架构设计 - 使用统一的项目文档组件
 * 与任务文档保持相同的 UI 风格
 */
export const ProjectArchitectureDesign: React.FC<Props> = ({ projectId }) => {
  return (
    <ProjectDocument 
      projectId={projectId} 
      slot="architecture_design" 
      title="项目架构设计"
      color="#fa8c16"
    />
  );
};
