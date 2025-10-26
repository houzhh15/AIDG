/**
 * MCP Resources API Client
 * 管理 MCP 协议资源的 REST API 客户端
 */

import { authedApi } from './auth';

/**
 * MCP 资源接口定义（前端 camelCase 格式）
 */
export interface Resource {
  /** 资源唯一标识 */
  resourceId: string;
  /** 资源名称（用户友好的显示名称） */
  name: string;
  /** 资源描述 */
  description: string;
  /** 所属项目ID（用于项目相关资源） */
  projectId?: string;
  /** 所属任务ID（用于任务相关资源） */
  taskId?: string;
  /** 资源可见性：public | private */
  visibility: 'public' | 'private';
  /** 是否为系统自动添加的资源 */
  autoAdded: boolean;
  /** 资源内容（用于 resources/read 的返回） */
  content: string;
  /** 资源创建时间 */
  createdAt: string;
}

/**
 * 后端返回的资源接口（snake_case 格式）
 */
interface ResourceDTO {
  resource_id: string;
  name: string;
  description: string;
  project_id?: string;
  task_id?: string;
  visibility: 'public' | 'private';
  auto_added: boolean;
  content: string;
  created_at: string;
}

/**
 * 将后端 DTO 转换为前端 Resource 格式
 */
function transformResource(dto: ResourceDTO): Resource {
  return {
    resourceId: dto.resource_id,
    name: dto.name,
    description: dto.description,
    projectId: dto.project_id,
    taskId: dto.task_id,
    visibility: dto.visibility,
    autoAdded: dto.auto_added,
    content: dto.content,
    createdAt: dto.created_at
  };
}

/**
 * 获取用户的资源列表
 * 
 * @param username - 用户名
 * @param filters - 可选的过滤条件
 * @returns 资源列表
 */
export async function getUserResources(
  username: string,
  filters?: {
    visibility?: 'public' | 'private';
    projectId?: string;
    taskId?: string;
    autoAdded?: boolean;
  }
): Promise<Resource[]> {
  const params = new URLSearchParams();
  if (filters?.visibility) params.append('visibility', filters.visibility);
  if (filters?.projectId) params.append('projectId', filters.projectId);
  if (filters?.taskId) params.append('taskId', filters.taskId);
  if (filters?.autoAdded !== undefined) params.append('autoAdded', filters.autoAdded.toString());
  
  const queryString = params.toString();
  const url = `/users/${username}/resources${queryString ? `?${queryString}` : ''}`;
  
  const response = await authedApi.get<{ data: ResourceDTO[]; success: boolean }>(url);
  // 转换后端 snake_case 格式到前端 camelCase 格式
  return response.data.data.map(transformResource);
}

/**
 * 添加自定义资源
 * 
 * @param username - 用户名
 * @param resource - 资源数据（部分字段）
 * @returns 创建成功的资源
 */
export async function addCustomResource(
  username: string,
  resource: {
    name: string;
    description: string;
    content: string;
    visibility?: 'public' | 'private';
    projectId?: string;
    taskId?: string;
  }
): Promise<Resource> {
  const response = await authedApi.post<Resource>(
    `/users/${username}/resources`,
    resource
  );
  return response.data;
}

/**
 * 更新资源
 * 
 * @param username - 用户名
 * @param resourceId - 资源ID
 * @param updates - 要更新的字段
 * @returns 更新后的资源
 */
export async function updateResource(
  username: string,
  resourceId: string,
  updates: {
    name?: string;
    description?: string;
    content?: string;
    visibility?: 'public' | 'private';
    projectId?: string;
    taskId?: string;
  }
): Promise<Resource> {
  const response = await authedApi.put<Resource>(
    `/users/${username}/resources/${resourceId}`,
    updates
  );
  return response.data;
}

/**
 * 删除资源
 * 
 * @param username - 用户名
 * @param resourceId - 资源ID
 */
export async function deleteResource(
  username: string,
  resourceId: string
): Promise<void> {
  await authedApi.delete(`/users/${username}/resources/${resourceId}`);
}
