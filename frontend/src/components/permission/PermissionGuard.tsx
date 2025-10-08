/**
 * 权限守卫组件 - 基于权限控制子组件可见性
 * 
 * 功能:
 * - 检查用户是否有指定权限
 * - 有权限时渲染 children
 * - 无权限时渲染 fallback 或 null
 * - 支持 loading 状态显示
 * 
 * 使用示例:
 * ```tsx
 * <PermissionGuard requiredPermission="task.write">
 *   <Button>编辑任务</Button>
 * </PermissionGuard>
 * 
 * <PermissionGuard 
 *   requiredPermission={["project.doc.write", "task.write"]}
 *   requireAll={true}
 *   fallback={<Button disabled>无权限编辑</Button>}
 * >
 *   <EditPanel />
 * </PermissionGuard>
 * ```
 */

import React from 'react';
import { Spin } from 'antd';
import { usePermission } from '../../hooks/usePermission';

interface PermissionGuardProps {
  /** 必需的权限 (单个或数组) */
  requiredPermission: string | string[];
  
  /** 当 requiredPermission 为数组时,是否需要全部权限 (默认 true) */
  requireAll?: boolean;
  
  /** 有权限时渲染的内容 */
  children: React.ReactNode;
  
  /** 无权限时渲染的内容 (默认不渲染任何内容) */
  fallback?: React.ReactNode;
  
  /** 是否显示 loading 状态 (默认 true) */
  showLoading?: boolean;
  
  /** 自定义 loading 组件 */
  loadingComponent?: React.ReactNode;
}

/**
 * 权限守卫组件
 */
export const PermissionGuard: React.FC<PermissionGuardProps> = ({
  requiredPermission,
  requireAll = true,
  children,
  fallback = null,
  showLoading = true,
  loadingComponent,
}) => {
  const { hasPermission, hasAnyPermission, hasAllPermissions, loading } = usePermission();

  // 调试日志
  console.log('[PermissionGuard] Props:', {
    requiredPermission,
    requireAll,
    loading,
    showLoading,
    requiredPermissionType: typeof requiredPermission,
    isArray: Array.isArray(requiredPermission),
  });

  // Loading 状态 - 显示加载提示
  if (loading && showLoading) {
    return (
      <div style={{ textAlign: 'center', padding: '20px' }}>
        {loadingComponent || <Spin tip="加载权限中..." />}
      </div>
    );
  }

  // Loading 状态 - 不显示加载提示，但不能执行权限检查
  // 防止在权限未加载时调用数组方法导致崩溃
  if (loading) {
    console.log('[PermissionGuard] Loading, returning fallback');
    return <>{fallback}</>;
  }

  // 检查权限
  let hasRequiredPermission = false;
  
  console.log('[PermissionGuard] Checking permission...');
  
  if (Array.isArray(requiredPermission)) {
    console.log('[PermissionGuard] requiredPermission is array, requireAll:', requireAll);
    if (requireAll) {
      hasRequiredPermission = hasAllPermissions(requiredPermission);
    } else {
      hasRequiredPermission = hasAnyPermission(requiredPermission);
    }
  } else {
    console.log('[PermissionGuard] requiredPermission is NOT array, calling hasPermission');
    hasRequiredPermission = hasPermission(requiredPermission);
  }

  console.log('[PermissionGuard] Result:', hasRequiredPermission);

  // 渲染
  if (hasRequiredPermission) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
};

export default PermissionGuard;
