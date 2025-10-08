import { useCallback } from 'react';
import { usePermissionContext } from '../contexts/PermissionContext';

/**
 * 权限检查 Hook
 * 
 * 用于在组件中判断当前用户是否具有特定权限
 * 
 * @example
 * ```tsx
 * const { hasPermission, permissions, loading } = usePermission();
 * 
 * if (loading) return <Spin />;
 * 
 * return (
 *   <div>
 *     {hasPermission('task.write') && <Button>编辑任务</Button>}
 *     {hasPermission(['task.read', 'task.write']) && <Button>高级操作</Button>}
 *   </div>
 * );
 * ```
 */
export interface UsePermissionReturn {
  /**
   * 检查是否具有指定的所有权限
   * @param scope 单个权限或权限数组
   * @returns 如果是数组,返回是否具有所有权限;如果是字符串,返回是否具有该权限
   */
  hasPermission: (scope: string | string[]) => boolean;
  
  /**
   * 检查是否具有指定权限中的任意一个
   * @param scopes 权限数组
   * @returns 是否具有任意一个权限
   */
  hasAnyPermission: (scopes: string[]) => boolean;
  
  /**
   * 检查是否具有指定的所有权限
   * @param scopes 权限数组
   * @returns 是否具有所有权限
   */
  hasAllPermissions: (scopes: string[]) => boolean;
  
  /**
   * 当前用户的所有权限列表 (角色权限 + 默认权限)
   */
  permissions: string[];
  
  /**
   * 权限加载状态
   */
  loading: boolean;
  
  /**
   * 权限加载错误信息
   */
  error: string | null;
  
  /**
   * 强制刷新权限
   */
  refetch: () => Promise<void>;
}

export function usePermission(): UsePermissionReturn {
  const { permissions, loading, error, refetch } = usePermissionContext();

  /**
   * 检查单个或多个权限
   */
  const hasPermission = useCallback((scope: string | string[]): boolean => {
    // 添加防御性检查 - permissions
    if (!Array.isArray(permissions)) {
      console.warn('[hasPermission] permissions is not an array:', typeof permissions, permissions);
      return false;
    }
    
    // 检查 scope 参数类型
    if (Array.isArray(scope)) {
      // 数组: 检查是否具有所有权限
      return scope.every(s => permissions.includes(s));
    }
    
    // 字符串或其他类型: 检查是否具有该权限
    if (typeof scope === 'string') {
      return permissions.includes(scope);
    }
    
    // 其他类型: 返回 false
    console.warn('[hasPermission] scope is neither string nor array:', typeof scope, scope);
    return false;
  }, [permissions]);

  /**
   * 检查是否具有任意一个权限
   */
  const hasAnyPermission = useCallback((scopes: string[]): boolean => {
    // 添加防御性检查 - permissions
    if (!Array.isArray(permissions)) {
      console.warn('[hasAnyPermission] permissions is not an array:', typeof permissions, permissions);
      return false;
    }
    
    // 添加防御性检查 - scopes 参数
    if (!Array.isArray(scopes)) {
      console.warn('[hasAnyPermission] scopes is not an array:', typeof scopes, scopes);
      return false;
    }
    
    // 安全的 some 调用
    return scopes.some(s => permissions.includes(s));
  }, [permissions]);

  /**
   * 检查是否具有所有权限
   */
  const hasAllPermissions = useCallback((scopes: string[]): boolean => {
    // 添加防御性检查 - permissions
    if (!Array.isArray(permissions)) {
      console.warn('[hasAllPermissions] permissions is not an array:', typeof permissions, permissions);
      return false;
    }
    
    // 添加防御性检查 - scopes 参数
    if (!Array.isArray(scopes)) {
      console.warn('[hasAllPermissions] scopes is not an array:', typeof scopes, scopes);
      return false;
    }
    
    // 安全的 every 调用
    return scopes.every(s => permissions.includes(s));
  }, [permissions]);

  return {
    hasPermission,
    hasAnyPermission,
    hasAllPermissions,
    permissions,
    loading,
    error,
    refetch,
  };
}

export default usePermission;
