/**
 * 用户-角色映射 API
 * 
 * 提供用户角色分配的操作接口
 */

import { authedApi } from './auth';

/**
 * 用户角色映射
 */
export interface UserRoleMapping {
  id: string;
  project_id: string;
  username: string;
  role_id: string;
  created_at: string;
}

/**
 * 用户角色映射带角色详情
 */
export interface UserRoleMappingWithRole extends UserRoleMapping {
  role_name: string;
  role_description: string;
  role_scopes: string[];
}

/**
 * 分配角色请求
 */
export interface AssignRoleRequest {
  project_id: string;
  username: string;
  role_id: string; // 前端API接口保持单个ID，内部会转为数组
}

/**
 * API 响应
 */
export interface UserRoleApiResponse<T = any> {
  success: boolean;
  message: string;
  data?: T;
}

/**
 * 获取用户在项目中的角色列表
 */
export async function getUserRoles(
  projectId: string,
  username: string
): Promise<UserRoleApiResponse<UserRoleMappingWithRole[]>> {
  try {
    const response = await authedApi.get(`/projects/${projectId}/users/${username}/roles`);
    return {
      success: true,
      message: '获取用户角色列表成功',
      data: response.data || [],
    };
  } catch (error: any) {
    return {
      success: false,
      message: error.response?.data?.error || error.message || '获取用户角色列表失败',
    };
  }
}

/**
 * 获取项目中所有用户角色映射
 */
export async function getProjectUserRoles(
  projectId: string
): Promise<UserRoleApiResponse<UserRoleMappingWithRole[]>> {
  try {
    const response = await authedApi.get(`/projects/${projectId}/user-roles`);
    return {
      success: true,
      message: '获取项目用户角色映射成功',
      data: response.data || [],
    };
  } catch (error: any) {
    return {
      success: false,
      message: error.response?.data?.error || error.message || '获取项目用户角色映射失败',
    };
  }
}

/**
 * 分配角色给用户
 */
export async function assignRole(data: AssignRoleRequest): Promise<UserRoleApiResponse<UserRoleMapping>> {
  try {
    // 后端API期望 role_ids 数组，而前端接口使用单个 role_id
    const requestBody = {
      username: data.username,
      project_id: data.project_id,
      role_ids: [data.role_id], // 转换为数组
    };
    
    const response = await authedApi.post(
      `/projects/${data.project_id}/users/${data.username}/roles`,
      requestBody
    );
    return {
      success: true,
      message: '分配角色成功',
      data: response.data,
    };
  } catch (error: any) {
    const errorData = error.response?.data?.error;
    const errorMessage = typeof errorData === 'string' 
      ? errorData 
      : errorData?.message || error.message || '分配角色失败';
    
    return {
      success: false,
      message: errorMessage,
    };
  }
}

/**
 * 移除用户的角色
 */
export async function removeUserRole(
  projectId: string,
  username: string,
  roleId: string
): Promise<UserRoleApiResponse<void>> {
  try {
    await authedApi.delete(`/projects/${projectId}/users/${username}/roles/${roleId}`);
    return {
      success: true,
      message: '移除角色成功',
    };
  } catch (error: any) {
    return {
      success: false,
      message: error.response?.data?.error || error.message || '移除角色失败',
    };
  }
}

/**
 * 用户项目角色信息（用于profile）
 */
export interface UserProjectRoleInfo {
  username: string;
  project_id: string;
  role_id: string;
  role_name: string;
  scopes: string[];
  assigned_at: string;
}

/**
 * 用户profile数据
 */
export interface UserProfileData {
  username: string;
  project_roles: UserProjectRoleInfo[];
}

/**
 * 获取用户的profile（包含所有项目角色）
 */
export async function getUserProfile(
  username: string
): Promise<UserRoleApiResponse<UserProfileData>> {
  try {
    const response = await authedApi.get(`/users/${username}/profile`);
    // 后端返回格式: { success: true, data: { username, project_roles } }
    return {
      success: true,
      message: '获取用户profile成功',
      data: response.data.data || { username, project_roles: [] },
    };
  } catch (error: any) {
    return {
      success: false,
      message: error.response?.data?.error || error.message || '获取用户profile失败',
    };
  }
}
