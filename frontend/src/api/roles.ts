/**
 * 角色管理 API
 * 
 * 提供角色的 CRUD 操作接口
 */

import { authedApi } from './auth';

/**
 * 角色信息
 */
export interface Role {
  role_id: string;      // 后端返回的是 role_id
  id?: string;          // 为了兼容性保留 id
  project_id: string;
  name: string;
  description?: string; // 描述可能为空
  scopes: string[];
  created_at: string;
  updated_at: string;
}

/**
 * 创建角色请求
 */
export interface CreateRoleRequest {
  project_id: string;
  name: string;
  description?: string;
  scopes: string[];
}

/**
 * 更新角色请求
 */
export interface UpdateRoleRequest {
  name?: string;
  description?: string;
  scopes?: string[];
}

/**
 * API 响应
 */
export interface RoleApiResponse<T = any> {
  success: boolean;
  message: string;
  data?: T;
}

/**
 * 获取项目的所有角色
 */
export async function getRoles(projectId: string): Promise<RoleApiResponse<Role[]>> {
  try {
    const response = await authedApi.get(`/projects/${projectId}/roles`);
    // 检查响应格式：可能是 {success, data} 或直接是数组
    const data = response.data?.data || response.data || [];
    return {
      success: true,
      message: '获取角色列表成功',
      data: Array.isArray(data) ? data : [],
    };
  } catch (error: any) {
    const errorData = error.response?.data?.error;
    const errorMessage = typeof errorData === 'string' 
      ? errorData 
      : errorData?.message || error.message || '获取角色列表失败';
    
    return {
      success: false,
      message: errorMessage,
      data: [],
    };
  }
}

/**
 * 获取单个角色详情
 */
export async function getRole(projectId: string, roleId: string): Promise<RoleApiResponse<Role>> {
  try {
    const response = await authedApi.get(`/projects/${projectId}/roles/${roleId}`);
    return {
      success: true,
      message: '获取角色详情成功',
      data: response.data,
    };
  } catch (error: any) {
    const errorData = error.response?.data?.error;
    const errorMessage = typeof errorData === 'string' 
      ? errorData 
      : errorData?.message || error.message || '获取角色详情失败';
    
    return {
      success: false,
      message: errorMessage,
    };
  }
}

/**
 * 创建角色
 */
export async function createRole(data: CreateRoleRequest): Promise<RoleApiResponse<Role>> {
  try {
    // 从请求体中排除 project_id，因为它已经在 URL 中
    const { project_id, ...body } = data;
    const response = await authedApi.post(`/projects/${project_id}/roles`, body);
    return {
      success: true,
      message: '创建角色成功',
      data: response.data,
    };
  } catch (error: any) {
    const errorData = error.response?.data?.error;
    const errorMessage = typeof errorData === 'string' 
      ? errorData 
      : errorData?.message || error.message || '创建角色失败';
    
    return {
      success: false,
      message: errorMessage,
    };
  }
}

/**
 * 更新角色
 */
export async function updateRole(
  projectId: string,
  roleId: string,
  data: UpdateRoleRequest
): Promise<RoleApiResponse<Role>> {
  try {
    const response = await authedApi.put(`/projects/${projectId}/roles/${roleId}`, data);
    return {
      success: true,
      message: '更新角色成功',
      data: response.data,
    };
  } catch (error: any) {
    const errorData = error.response?.data?.error;
    const errorMessage = typeof errorData === 'string' 
      ? errorData 
      : errorData?.message || error.message || '更新角色失败';
    
    return {
      success: false,
      message: errorMessage,
    };
  }
}

/**
 * 删除角色
 */
export async function deleteRole(projectId: string, roleId: string): Promise<RoleApiResponse<void>> {
  try {
    await authedApi.delete(`/projects/${projectId}/roles/${roleId}`);
    return {
      success: true,
      message: '删除角色成功',
    };
  } catch (error: any) {
    const errorData = error.response?.data?.error;
    const errorMessage = typeof errorData === 'string' 
      ? errorData 
      : errorData?.message || error.message || '删除角色失败';
    
    return {
      success: false,
      message: errorMessage,
    };
  }
}
