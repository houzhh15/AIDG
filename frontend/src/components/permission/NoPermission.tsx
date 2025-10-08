/**
 * 无权限提示组件
 * 
 * 功能:
 * - 显示 403 无权限页面
 * - 展示所需权限的描述信息
 * - 提供返回和查看权限页面入口
 * 
 * 使用示例:
 * ```tsx
 * // 单个权限
 * <NoPermission requiredPermission="project.doc.write" />
 * 
 * // 多个权限
 * <NoPermission 
 *   requiredPermission={["task.write", "task.plan.approve"]}
 *   description="执行此操作需要任务编辑和计划审批权限"
 * />
 * ```
 */

import React from 'react';
import { Result, Button } from 'antd';
import { LockOutlined } from '@ant-design/icons';
import { getScopeLabel, getScopeDescription } from '../../constants/permissions';

interface NoPermissionProps {
  /** 需要的权限 (单个或数组) */
  requiredPermission?: string | string[];
  
  /** 自定义描述文案 */
  description?: string;
  
  /** 是否显示返回按钮 (默认 true) */
  showBackButton?: boolean;
  
  /** 是否显示查看权限按钮 (默认 true) */
  showViewPermissionButton?: boolean;
  
  /** 自定义返回路径 (默认使用 window.history.back()) */
  backPath?: string;
  
  /** 自定义查看权限页面路径 (默认 /user-profile) */
  permissionPagePath?: string;
}

/**
 * 无权限提示组件
 */
export const NoPermission: React.FC<NoPermissionProps> = ({
  requiredPermission,
  description,
  showBackButton = true,
  showViewPermissionButton = true,
  backPath,
  permissionPagePath = '/user-profile',
}) => {

  // 生成权限描述
  const getPermissionDescription = () => {
    if (description) {
      return description;
    }

    if (!requiredPermission) {
      return '您没有访问此内容的权限。';
    }

    if (Array.isArray(requiredPermission)) {
      if (requiredPermission.length === 0) {
        return '您没有访问此内容的权限。';
      }

      const permissionLabels = requiredPermission
        .map(scope => getScopeLabel(scope))
        .filter(Boolean);

      if (permissionLabels.length === 0) {
        return '您没有访问此内容的权限。';
      }

      return `此操作需要以下权限:\n${permissionLabels.map((label, index) => `${index + 1}. ${label}`).join('\n')}`;
    } else {
      const label = getScopeLabel(requiredPermission);
      const desc = getScopeDescription(requiredPermission);
      
      if (label) {
        return `此操作需要 "${label}" 权限。${desc ? `\n${desc}` : ''}`;
      }
      
      return `此操作需要 "${requiredPermission}" 权限。`;
    }
  };

  // 返回上一页
  const handleBack = () => {
    if (backPath) {
      window.location.href = backPath;
    } else {
      window.history.back();
    }
  };

  // 查看我的权限
  const handleViewPermissions = () => {
    window.location.href = permissionPagePath;
  };

  return (
    <Result
      status="403"
      icon={<LockOutlined />}
      title="无权限访问"
      subTitle={
        <div style={{ whiteSpace: 'pre-line' }}>
          {getPermissionDescription()}
        </div>
      }
      extra={[
        showBackButton && (
          <Button key="back" onClick={handleBack}>
            返回上一页
          </Button>
        ),
        showViewPermissionButton && (
          <Button key="view-permissions" type="primary" onClick={handleViewPermissions}>
            查看我的权限
          </Button>
        ),
      ].filter(Boolean)}
    />
  );
};

export default NoPermission;
