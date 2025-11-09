import { authedApi as api } from '../api/auth';

// ==================== 类型定义 ====================

export interface TagInfo {
  tag_name: string;
  created_at: string;
  md5_hash: string;
  file_size: number;
  creator?: string;
}

export interface TagCreateRequest {
  tagName: string;
}

export interface TagSwitchRequest {
  tagName: string;
  force?: boolean; // 强制切换，放弃未保存的修改
}

export interface TagSwitchResponse {
  success: boolean;
  message: string;
  needConfirm?: boolean;  // 是否需要用户确认
  currentMd5?: string;    // 当前版本的MD5
  targetMd5?: string;     // 目标版本的MD5
}

export interface TagListResponse {
  tags: TagInfo[];
}

export interface TagCreateResponse {
  success: boolean;
  tagName: string;
  md5Hash: string;
}

export interface ExecutionPlanTagInfo extends TagInfo {
  planVersion?: number;
}

// ==================== 任务文档Tag服务 ====================

/**
 * 创建文档Tag
 * @param projectId 项目ID
 * @param taskId 任务ID
 * @param docType 文档类型
 * @param tagName Tag名称
 */
export async function createTag(
  projectId: string,
  taskId: string,
  docType: 'requirements' | 'design' | 'test',
  tagName: string
): Promise<TagCreateResponse> {
  const response = await api.post(
    `/projects/${projectId}/tasks/${taskId}/docs/${docType}/tags`,
    { tag_name: tagName }
  );
  return response.data;
}

/**
 * 获取文档Tag列表
 * @param projectId 项目ID
 * @param taskId 任务ID
 * @param docType 文档类型
 */
export async function listTags(
  projectId: string,
  taskId: string,
  docType: 'requirements' | 'design' | 'test'
): Promise<TagListResponse> {
  const response = await api.get(
    `/projects/${projectId}/tasks/${taskId}/docs/${docType}/tags`
  );
  return response.data;
}

/**
 * 切换到指定Tag版本
 * @param projectId 项目ID
 * @param taskId 任务ID
 * @param docType 文档类型
 * @param tagName Tag名称
 * @param force 是否强制切换（放弃未保存的修改）
 */
export async function switchTag(
  projectId: string,
  taskId: string,
  docType: 'requirements' | 'design' | 'test',
  tagName: string,
  force: boolean = false
): Promise<TagSwitchResponse> {
  const response = await api.post(
    `/projects/${projectId}/tasks/${taskId}/docs/${docType}/tags/${tagName}/switch`,
    { force }
  );
  return response.data;
}

/**
 * 获取指定Tag的信息
 * @param projectId 项目ID
 * @param taskId 任务ID
 * @param docType 文档类型
 * @param tagName Tag名称
 */
export async function getTagInfo(
  projectId: string,
  taskId: string,
  docType: 'requirements' | 'design' | 'test',
  tagName: string
): Promise<TagInfo> {
  const response = await api.get(
    `/projects/${projectId}/tasks/${taskId}/docs/${docType}/tags/${tagName}`
  );
  return response.data;
}

// ==================== 执行计划Tag服务 ====================

/**
 * 创建执行计划Tag
 * @param projectId 项目ID
 * @param taskId 任务ID
 * @param tagName Tag名称
 */
export async function createExecutionPlanTag(
  projectId: string,
  taskId: string,
  tagName: string
): Promise<TagCreateResponse> {
  const response = await api.post(
    `/projects/${projectId}/tasks/${taskId}/execution-plan/tags`,
    { tag_name: tagName }
  );
  return response.data;
}

/**
 * 获取执行计划Tag列表
 * @param projectId 项目ID
 * @param taskId 任务ID
 */
export async function listExecutionPlanTags(
  projectId: string,
  taskId: string
): Promise<TagListResponse> {
  const response = await api.get(
    `/projects/${projectId}/tasks/${taskId}/execution-plan/tags`
  );
  return response.data;
}

/**
 * 切换执行计划到指定Tag版本
 * @param projectId 项目ID
 * @param taskId 任务ID
 * @param tagName Tag名称
 * @param force 是否强制切换
 */
export async function switchExecutionPlanTag(
  projectId: string,
  taskId: string,
  tagName: string,
  force: boolean = false
): Promise<TagSwitchResponse> {
  const response = await api.post(
    `/projects/${projectId}/tasks/${taskId}/execution-plan/tags/${tagName}/switch`,
    { force }
  );
  return response.data;
}

/**
 * 获取执行计划指定Tag的信息
 * @param projectId 项目ID
 * @param taskId 任务ID
 * @param tagName Tag名称
 */
export async function getExecutionPlanTagInfo(
  projectId: string,
  taskId: string,
  tagName: string
): Promise<ExecutionPlanTagInfo> {
  const response = await api.get(
    `/projects/${projectId}/tasks/${taskId}/execution-plan/tags/${tagName}`
  );
  return response.data;
}

// ==================== 统一导出 ====================

export const tagService = {
  // 任务文档Tag
  createTag,
  listTags,
  switchTag,
  getTagInfo,
  
  // 执行计划Tag
  createExecutionPlanTag,
  listExecutionPlanTags,
  switchExecutionPlanTag,
  getExecutionPlanTagInfo,
};

export default tagService;
