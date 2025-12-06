import { ApiResponse } from '../types';
import { authedApi } from './auth';

// 用户来源类型
export type UserSource = 'local' | 'external';

export interface User {
  username: string;
  scopes: string[];
  created_at: string;
  updated_at: string;
  // 外部用户扩展字段
  source?: UserSource;
  idp_id?: string;
  idp_name?: string;           // 前端展示用，由后端填充
  external_id?: string;
  email?: string;
  fullname?: string;
  disabled?: boolean;
  last_login_at?: string;
  synced_at?: string;
}

export interface CreateUserRequest {
  username: string;
  password?: string;
  scopes: string[];
}

export interface UpdateUserRequest {
  scopes: string[];
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}

// 全局权限：仅包含跨项目的权限
export const AVAILABLE_SCOPES = [
  { value: 'project.admin', label: '项目管理 - 创建与全局配置' },
  { value: 'user.manage', label: '用户管理' },
  { value: 'meeting.read', label: '会议记录 - 读取' },
  { value: 'meeting.write', label: '会议记录 - 写入' },
  { value: 'idp.read', label: '身份源管理 - 读取' },
  { value: 'idp.write', label: '身份源管理 - 写入' },
];

const BASE_URL = '/users';

// 获取所有用户
export async function getUsers(): Promise<ApiResponse<User[]>> {
  const response = await authedApi.get<ApiResponse<User[]>>(BASE_URL);
  return response.data;
}

// 获取单个用户
export async function getUser(username: string): Promise<ApiResponse<User>> {
  const response = await authedApi.get<ApiResponse<User>>(`${BASE_URL}/${username}`);
  return response.data;
}

// 创建用户
export async function createUser(data: CreateUserRequest): Promise<ApiResponse<User>> {
  const response = await authedApi.post<ApiResponse<User>>(BASE_URL, data);
  return response.data;
}

// 更新用户权限
export async function updateUserScopes(username: string, data: UpdateUserRequest): Promise<ApiResponse<User>> {
  const response = await authedApi.patch<ApiResponse<User>>(`${BASE_URL}/${username}`, data);
  return response.data;
}

// 删除用户
export async function deleteUser(username: string): Promise<ApiResponse<void>> {
  const response = await authedApi.delete<ApiResponse<void>>(`${BASE_URL}/${username}`);
  return response.data;
}

// 修改密码
export async function changePassword(username: string, data: ChangePasswordRequest): Promise<ApiResponse<void>> {
  const response = await authedApi.put<ApiResponse<void>>(`${BASE_URL}/${username}/password`, data);
  return response.data;
}

// 禁用用户（仅限外部用户）
export async function disableUser(username: string): Promise<ApiResponse<void>> {
  const response = await authedApi.post<ApiResponse<void>>(`${BASE_URL}/${username}/disable`);
  return response.data;
}

// 启用用户（仅限外部用户）
export async function enableUser(username: string): Promise<ApiResponse<void>> {
  const response = await authedApi.post<ApiResponse<void>>(`${BASE_URL}/${username}/enable`);
  return response.data;
}

// 身份源权限
export const IDP_SCOPES = [
  { value: 'idp.read', label: '身份源管理 - 读取' },
  { value: 'idp.write', label: '身份源管理 - 写入' },
];