import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { getUserProfile, UserProfileData } from '../api/permissions';
import { onAuthChange } from '../api/auth';

/**
 * 权限上下文状态
 */
interface PermissionContextState {
  permissions: string[]; // 所有有效权限 (角色权限 + 默认权限)
  profile: UserProfileData | null;
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

const PermissionContext = createContext<PermissionContextState | undefined>(undefined);

const CACHE_TTL = 5 * 60 * 1000; // 5分钟缓存

/**
 * 权限提供者组件
 */
export const PermissionProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [permissions, setPermissions] = useState<string[]>([]);
  const [profile, setProfile] = useState<UserProfileData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastFetched, setLastFetched] = useState<number>(0);

  /**
   * 从用户档案中提取所有权限
   */
  const extractPermissions = useCallback((profileData: UserProfileData): string[] => {
    const allScopes: string[] = [];
    
    // 合并角色权限 - 添加防御性检查
    if (Array.isArray(profileData.roles)) {
      profileData.roles.forEach(role => {
        if (role && Array.isArray(role.scopes)) {
          allScopes.push(...role.scopes);
        }
      });
    }
    
    // 合并默认权限 - 添加防御性检查
    if (Array.isArray(profileData.default_permissions)) {
      profileData.default_permissions.forEach(perm => {
        if (perm && Array.isArray(perm.scopes)) {
          allScopes.push(...perm.scopes);
        }
      });
    }
    
    // 去重
    return Array.from(new Set(allScopes));
  }, []);

  /**
   * 获取用户权限
   */
  const fetchPermissions = useCallback(async () => {
    // 检查缓存是否有效
    const now = Date.now();
    if (lastFetched && now - lastFetched < CACHE_TTL) {
      return;
    }

    setLoading(true);
    setError(null);
    
    try {
      const profileData = await getUserProfile();
      
      // 确保返回的数据结构正确
      if (!profileData) {
        throw new Error('用户档案数据为空');
      }
      
      // 设置默认值以防止数据不完整
      const safeProfileData: UserProfileData = {
        ...profileData,
        roles: Array.isArray(profileData.roles) ? profileData.roles : [],
        default_permissions: Array.isArray(profileData.default_permissions) ? profileData.default_permissions : [],
      };
      
      setProfile(safeProfileData);
      setPermissions(extractPermissions(safeProfileData));
      setLastFetched(now);
    } catch (err: any) {
      const errorMessage = err.response?.data?.error?.message || err.message || '获取权限失败';
      setError(errorMessage);
      console.error('Failed to fetch permissions:', err);
      
      // 即使出错，也设置空数组防止后续逻辑出错
      setPermissions([]);
      setProfile(null);
    } finally {
      setLoading(false);
    }
  }, [lastFetched, extractPermissions]);

  /**
   * 强制刷新权限
   */
  const refetch = useCallback(async () => {
    setLastFetched(0); // 清除缓存
    await fetchPermissions();
  }, [fetchPermissions]);

  // 用户登录/登出时重新获取权限
  useEffect(() => {
    const unsubscribe = onAuthChange((auth) => {
      if (auth) {
        // 用户登录,获取权限
        fetchPermissions();
      } else {
        // 用户登出,清空权限
        setPermissions([]);
        setProfile(null);
        setLastFetched(0);
        setLoading(false);
        setError(null);
      }
    });

    // 组件挂载时获取权限
    fetchPermissions();

    return unsubscribe;
  }, [fetchPermissions]);

  const value: PermissionContextState = {
    permissions,
    profile,
    loading,
    error,
    refetch,
  };

  return (
    <PermissionContext.Provider value={value}>
      {children}
    </PermissionContext.Provider>
  );
};

/**
 * 使用权限上下文的 Hook
 * @throws 如果在 PermissionProvider 外部使用
 */
export function usePermissionContext(): PermissionContextState {
  const context = useContext(PermissionContext);
  if (!context) {
    throw new Error('usePermissionContext must be used within a PermissionProvider');
  }
  return context;
}
