import React from 'react';
import ProjectDocument from './ProjectDocument';

interface Props { projectId: string; }

/**
 * 项目特性列表 - 使用统一的项目文档组件
 * 与任务文档保持相同的 UI 风格
 */
export const ProjectFeatureList: React.FC<Props> = ({ projectId }) => {
  return (
    <ProjectDocument 
      projectId={projectId} 
      slot="feature_list" 
      title="项目特性列表"
      color="#52c41a"
    />
  );
};
