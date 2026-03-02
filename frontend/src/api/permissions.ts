import { authedApi as api } from './auth';

/**
 * 用户权限 API
 */

export interface UserRoleInfo {
  username: string;
  project_id: string;
  project_name: string;
  role_id: string;
  role_name: string;
  scopes: string[];
  assigned_at: string;
}

export interface DefaultPermission {
  source: 'task_owner' | 'meeting_owner' | 'meeting_acl';
  task_id?: string;
  task_name?: string;
  meeting_id?: string;
  meeting_name?: string;
  scopes: string[];
}

export interface UserProfileData {
  username: string;
  email?: string;
  roles: UserRoleInfo[];
  default_permissions: DefaultPermission[];
}

export interface UserProfileResponse {
  success: boolean;
  data: UserProfileData;
}

/**
 * 获取当前用户的权限档案
 * 包含项目角色和默认权限 (任务负责人、会议创建者等)
 */
export async function getUserProfile(): Promise<UserProfileData> {
  const response = await api.get<UserProfileResponse>('/user/profile');
  return response.data.data;
}

/**
 * 修改密码
 */
export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}

export interface ChangePasswordResponse {
  success: boolean;
  message: string;
}

export async function changePassword(data: ChangePasswordRequest): Promise<string> {
  const response = await api.post<ChangePasswordResponse>('/user/change-password', data);
  return response.data.message;
}

/**
 * 用户项目可见性
 */
export interface UserProjectItem {
  id: string;
  name: string;
  product_line?: string;
  visible: boolean;
}

export interface UserProjectsResponse {
  success: boolean;
  data: UserProjectItem[];
}

/**
 * 获取当前用户的项目列表（包含可见性信息）
 */
export async function getUserProjects(): Promise<UserProjectItem[]> {
  const response = await api.get<UserProjectsResponse>('/user/projects');
  return response.data.data;
}

/**
 * 更新用户的项目可见性设置
 */
export async function updateUserProjects(hiddenProjectIds: string[]): Promise<void> {
  await api.put('/user/projects', { hidden_project_ids: hiddenProjectIds });
}
