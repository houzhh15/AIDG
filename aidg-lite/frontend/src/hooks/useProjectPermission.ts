import { useState, useCallback } from 'react';
import { authedApi } from '../api/auth';

interface UseProjectPermissionResult {
  hasPermission: boolean;
  loading: boolean;
  error: string | null;
  checkPermission: (projectId: string) => Promise<void>;
}

/**
 * 检查用户是否有特定项目的访问权限
 * 通过尝试获取项目任务列表来验证权限
 */
export function useProjectPermission(): UseProjectPermissionResult {
  const [hasPermission, setHasPermission] = useState(false); // 默认假设没有权限，等待检查结果
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const checkPermission = useCallback(async (projectId: string) => {
    if (!projectId) {
      setHasPermission(true);
      return;
    }

    console.log(`正在检查项目权限: ${projectId}`);
    setLoading(true);
    setError(null);

    try {
      // 尝试获取项目任务列表作为权限检查
      // 如果没有权限，后端会返回403错误
      await authedApi.get(`/projects/${projectId}/tasks`, {
        params: { limit: 1 } // 只获取一条记录，减少数据传输
      });
      
      console.log(`项目权限检查通过: ${projectId}`);
      setHasPermission(true);
    } catch (err: any) {
      if (err.response?.status === 403) {
        console.log(`项目权限检查失败: ${projectId} - 403 Forbidden`);
        setHasPermission(false);
        setError('没有访问此项目的权限');
      } else {
        // 对于其他错误，也认为没有权限，避免显示错误的内容
        console.warn(`检查项目权限时发生错误: ${projectId}`, err);
        setHasPermission(false);
        setError('权限检查失败');
      }
    } finally {
      setLoading(false);
    }
  }, []); // 空依赖数组，因为函数内部不依赖任何外部变量

  return {
    hasPermission,
    loading,
    error,
    checkPermission
  };
}