import React from 'react';
import { Result, Button } from 'antd';
import { LockOutlined, LeftOutlined } from '@ant-design/icons';

interface NoPermissionPageProps {
  projectId?: string;
  projectName?: string;
  onBack?: () => void;
  title?: string;
  description?: string;
}

const NoPermissionPage: React.FC<NoPermissionPageProps> = ({
  projectId,
  projectName,
  onBack,
  title = "无权限访问",
  description
}) => {
  const defaultDescription = projectName 
    ? `您没有访问项目 "${projectName}" 的权限。请联系管理员为您分配相应的项目角色。`
    : "您没有访问此项目的权限。请联系管理员为您分配相应的项目角色。";

  return (
    <div style={{ 
      height: '100%', 
      display: 'flex', 
      alignItems: 'center', 
      justifyContent: 'center',
      padding: '40px 20px'
    }}>
      <Result
        icon={<LockOutlined style={{ color: '#faad14' }} />}
        status="warning"
        title={title}
        subTitle={description || defaultDescription}
      />
    </div>
  );
};

export default NoPermissionPage;