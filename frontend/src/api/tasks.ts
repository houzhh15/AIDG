import { ApiResponse } from '../types';
import { authedApi } from './auth';

export interface ProjectTask {
  id: string;
  name: string;
  assignee?: string;
  feature_id?: string;
  feature_name?: string;
  module?: string;
  description?: string;
  status?: string;
  created_at: string;
  updated_at: string;
}

export interface TaskDocument {
  exists: boolean;
  content: string;
  recommendations?: Array<{
    task_id: string;
    doc_type: string;
    section_id: string;
    title: string;
    similarity: number;
    snippet: string;
  }>;
}

export interface CreateTaskRequest {
  name: string;
  assignee?: string;
  feature_id?: string;
  feature_name?: string;
  module?: string;
  description?: string;
  status?: string;
}

export interface UpdateTaskRequest {
  name?: string;
  assignee?: string;
  feature_id?: string;
  feature_name?: string;
  module?: string;
  description?: string;
  status?: string;
}

export interface TaskPrompt {
  id: string;
  username: string;
  content: string;
  created_at: string;
}

export interface CreatePromptRequest {
  username: string;
  content: string;
}

const BASE_URL = '/projects';

// 时间筛选范围类型
export type TimeRangeFilter = 'today' | 'week' | 'month';

// 获取项目任务列表
// 参数:
//   - projectId: 项目ID
//   - query: 搜索关键词
//   - timeRange: 时间筛选范围
//   - fuzzy: 是否启用模糊搜索（SimHash语义搜索），默认false
export async function getProjectTasks(
  projectId: string, 
  query?: string,
  timeRange?: TimeRangeFilter,
  fuzzy?: boolean
): Promise<ApiResponse<ProjectTask[]>> {
  const params = new URLSearchParams();
  if (query) params.append('q', query);
  if (timeRange) params.append('time_range', timeRange);
  if (fuzzy) params.append('fuzzy', 'true');
  const response = await authedApi.get<ApiResponse<ProjectTask[]>>(`${BASE_URL}/${projectId}/tasks?${params.toString()}`);
  return response.data;
}

// 创建项目任务
export async function createProjectTask(projectId: string, data: CreateTaskRequest): Promise<ApiResponse<ProjectTask>> {
  const response = await authedApi.post<ApiResponse<ProjectTask>>(`${BASE_URL}/${projectId}/tasks`, data);
  return response.data;
}

// 获取项目任务详情
export async function getProjectTask(projectId: string, taskId: string): Promise<ApiResponse<ProjectTask>> {
  const response = await authedApi.get<ApiResponse<ProjectTask>>(`${BASE_URL}/${projectId}/tasks/${taskId}`);
  return response.data;
}

// 更新项目任务
export async function updateProjectTask(projectId: string, taskId: string, data: UpdateTaskRequest): Promise<ApiResponse<void>> {
  const response = await authedApi.put<ApiResponse<void>>(`${BASE_URL}/${projectId}/tasks/${taskId}`, data);
  return response.data;
}

// 删除项目任务
export async function deleteProjectTask(projectId: string, taskId: string): Promise<ApiResponse<void>> {
  const response = await authedApi.delete<ApiResponse<void>>(`${BASE_URL}/${projectId}/tasks/${taskId}`);
  return response.data;
}

// 获取任务文档
export async function getTaskDocument(
  projectId: string, 
  taskId: string, 
  docType: 'requirements' | 'design' | 'test',
  includeRecommendations: boolean = false
): Promise<TaskDocument> {
  // 添加时间戳参数防止缓存
  const timestamp = new Date().getTime();
  let url = `${BASE_URL}/${projectId}/tasks/${taskId}/${docType}?_t=${timestamp}`;
  if (includeRecommendations) {
    url += '&include_recommendations=true';
  }
  const response = await authedApi.get<TaskDocument>(url);
  return response.data;
}

// 保存任务文档
export async function saveTaskDocument(projectId: string, taskId: string, docType: 'requirements' | 'design' | 'test', content: string): Promise<ApiResponse<void>> {
  const response = await authedApi.put<ApiResponse<void>>(`${BASE_URL}/${projectId}/tasks/${taskId}/${docType}`, { content });
  return response.data;
}

// 获取任务提示词列表
export async function getTaskPrompts(projectId: string, taskId: string): Promise<ApiResponse<TaskPrompt[]>> {
  const response = await authedApi.get<ApiResponse<TaskPrompt[]>>(`${BASE_URL}/${projectId}/tasks/${taskId}/prompts`);
  return response.data;
}

// 创建任务提示词记录
export async function createTaskPrompt(projectId: string, taskId: string, data: CreatePromptRequest): Promise<ApiResponse<TaskPrompt>> {
  const response = await authedApi.post<ApiResponse<TaskPrompt>>(`${BASE_URL}/${projectId}/tasks/${taskId}/prompts`, data);
  return response.data;
}

// 执行计划相关类型定义
export interface ExecutionPlanDependency {
  source: string;
  target: string;
}

export interface ExecutionPlanStep {
  id: string;
  status: 'pending' | 'in-progress' | 'succeeded' | 'failed' | 'cancelled';
  priority: string;
  description: string;
  output?: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface ExecutionPlan {
  plan_id: string;
  task_id: string;
  status: string;
  created_at: string;
  updated_at: string;
  dependencies: ExecutionPlanDependency[];
  steps: ExecutionPlanStep[];
  content: string;
}

export interface ApprovalRequest {
  comment?: string;
}

export interface RejectRequest {
  comment?: string;
  reason?: string;
}

// 获取执行计划
export async function getExecutionPlan(projectId: string, taskId: string): Promise<ApiResponse<ExecutionPlan>> {
  const response = await authedApi.get<ApiResponse<ExecutionPlan>>(`/projects/${projectId}/tasks/${taskId}/execution-plan`);
  return response.data;
}

// 提交执行计划（Draft -> Pending Approval）
export async function submitExecutionPlan(projectId: string, taskId: string, data: { comment?: string } = {}): Promise<ApiResponse<{ plan_id: string; status: string; message: string }>> {
  const response = await authedApi.post<ApiResponse<{ plan_id: string; status: string; message: string }>>(`/projects/${projectId}/tasks/${taskId}/execution-plan/submit`, data);
  return response.data;
}

// 批准执行计划
export async function approveExecutionPlan(projectId: string, taskId: string, data: ApprovalRequest): Promise<ApiResponse<{ plan_id: string; status: string; message: string }>> {
  const response = await authedApi.post<ApiResponse<{ plan_id: string; status: string; message: string }>>(`/projects/${projectId}/tasks/${taskId}/execution-plan/approve`, data);
  return response.data;
}

// 拒绝执行计划
export async function rejectExecutionPlan(projectId: string, taskId: string, data: RejectRequest): Promise<ApiResponse<{ plan_id: string; status: string; message: string }>> {
  const response = await authedApi.post<ApiResponse<{ plan_id: string; status: string; message: string }>>(`/projects/${projectId}/tasks/${taskId}/execution-plan/reject`, data);
  return response.data;
}

// 恢复执行计划到待审批状态
export async function restoreApproval(projectId: string, taskId: string, data: { comment?: string } = {}): Promise<ApiResponse<{ plan_id: string; status: string; message: string }>> {
  const response = await authedApi.post<ApiResponse<{ plan_id: string; status: string; message: string }>>(`/projects/${projectId}/tasks/${taskId}/execution-plan/restore-approval`, data);
  return response.data;
}

// 更新执行计划内容
export async function updateExecutionPlanContent(projectId: string, taskId: string, content: string): Promise<ApiResponse<{ plan_id: string; status: string; updated_at: string }>> {
  const response = await authedApi.put<ApiResponse<{ plan_id: string; status: string; updated_at: string }>>(`/projects/${projectId}/tasks/${taskId}/execution-plan`, { content });
  return response.data;
}

// 重置执行计划（生成默认模板）
export interface ResetPlanResponse {
  plan_id: string;
  status: string;
  created_at: string;
  updated_at: string;
  dependencies: ExecutionPlanDependency[];
  steps: ExecutionPlanStep[];
  content: string;
}

export async function resetExecutionPlan(
  projectId: string,
  taskId: string,
  force: boolean = false
): Promise<ApiResponse<ResetPlanResponse>> {
  const response = await authedApi.post<ApiResponse<ResetPlanResponse>>(
    `/projects/${projectId}/tasks/${taskId}/execution-plan/reset`,
    { force }
  );
  return response.data;
}

// ========== 章节管理 API ==========
import type {
  SectionMeta,
  SectionContent,
  UpdateSectionRequest,
  InsertSectionRequest,
  SyncSectionsRequest
} from '../types/section'

// 获取章节列表
export async function getTaskSections(
  projectId: string,
  taskId: string,
  docType: string
): Promise<SectionMeta> {
  const response = await authedApi.get<SectionMeta>(
    `/projects/${projectId}/tasks/${taskId}/${docType}/sections`
  )
  return response.data
}

// 获取单个章节
export async function getTaskSection(
  projectId: string,
  taskId: string,
  docType: string,
  sectionId: string,
  includeChildren = false
): Promise<SectionContent> {
  const response = await authedApi.get<SectionContent>(
    `/projects/${projectId}/tasks/${taskId}/${docType}/sections/${sectionId}`,
    { params: { include_children: includeChildren } }
  )
  return response.data
}

// 更新章节
export async function updateTaskSection(
  projectId: string,
  taskId: string,
  docType: string,
  sectionId: string,
  content: string,
  expectedVersion?: number
): Promise<{ success: boolean }> {
  const data: UpdateSectionRequest = {
    content,
    expected_version: expectedVersion
  }

  const response = await authedApi.put<{ success: boolean }>(
    `/projects/${projectId}/tasks/${taskId}/${docType}/sections/${sectionId}`,
    data
  )
  return response.data
}

// 更新父章节全文（包含所有子章节）
export async function updateTaskSectionFull(
  projectId: string,
  taskId: string,
  docType: string,
  sectionId: string,
  content: string,
  expectedVersion?: number
): Promise<{ success: boolean }> {
  const data: UpdateSectionRequest = {
    content,
    expected_version: expectedVersion
  }

  const response = await authedApi.put<{ success: boolean }>(
    `/projects/${projectId}/tasks/${taskId}/${docType}/sections/${sectionId}/full`,
    data
  )
  return response.data
}

// 插入章节
export async function insertTaskSection(
  projectId: string,
  taskId: string,
  docType: string,
  title: string,
  content: string,
  afterSectionId?: string,
  expectedVersion?: number
): Promise<SectionContent> {
  const data: InsertSectionRequest = {
    title,
    content,
    after_section_id: afterSectionId,
    expected_version: expectedVersion
  }

  const response = await authedApi.post<SectionContent>(
    `/projects/${projectId}/tasks/${taskId}/${docType}/sections`,
    data
  )
  return response.data
}

// 删除章节
export async function deleteTaskSection(
  projectId: string,
  taskId: string,
  docType: string,
  sectionId: string,
  cascade = false,
  expectedVersion?: number
): Promise<{ success: boolean }> {
  const response = await authedApi.delete<{ success: boolean }>(
    `/projects/${projectId}/tasks/${taskId}/${docType}/sections/${sectionId}`,
    {
      params: { cascade },
      data: { expected_version: expectedVersion }
    }
  )
  return response.data
}

// 调整章节顺序
export async function reorderTaskSection(
  projectId: string,
  taskId: string,
  docType: string,
  sectionId: string,
  afterSectionId?: string,
  expectedVersion?: number
): Promise<{ success: boolean }> {
  const data = {
    section_id: sectionId,
    after_section_id: afterSectionId,
    expected_version: expectedVersion
  }

  const response = await authedApi.patch<{ success: boolean }>(
    `/projects/${projectId}/tasks/${taskId}/${docType}/sections/reorder`,
    data
  )
  return response.data
}

// 手动同步章节
export async function syncTaskSections(
  projectId: string,
  taskId: string,
  docType: string,
  direction: 'from_compiled' | 'to_compiled'
): Promise<{ success: boolean }> {
  const data: SyncSectionsRequest = { direction }

  const response = await authedApi.post<{ success: boolean }>(
    `/projects/${projectId}/tasks/${taskId}/${docType}/sections/sync`,
    data
  )
  return response.data
}
