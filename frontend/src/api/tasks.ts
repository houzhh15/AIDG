import { ApiResponse } from '../types';
import { authedApi } from './auth';

export type TaskStatus = 'todo' | 'in-progress' | 'review' | 'completed';

export interface ConstraintReadiness {
  ready_count: number;
  requirements: boolean;
  design: boolean;
  test: boolean;
}

export interface ProjectTask {
  id: string;
  name: string;
  assignee?: string;
  feature_id?: string;
  feature_name?: string;
  module?: string;
  description?: string;
  status: TaskStatus;
  revision?: number;
  ready_count?: number;
  constraint_readiness?: ConstraintReadiness;
  created_at: string;
  updated_at: string;
}

export interface TaskDocument {
  exists: boolean;
  content: string;
  version?: number;
  etag?: string;
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
  status?: TaskStatus;
}

export interface UpdateTaskRequest {
  expected_revision?: number;
  name?: string;
  assignee?: string;
  feature_id?: string;
  feature_name?: string;
  module?: string;
  description?: string;
  status?: TaskStatus;
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

// 转移项目任务到另一个项目
export async function transferProjectTask(
  sourceProjectId: string,
  taskId: string,
  targetProjectId: string
): Promise<ApiResponse<{ task_id: string; source_project_id: string; target_project_id: string }>> {
  const response = await authedApi.post<ApiResponse<{ task_id: string; source_project_id: string; target_project_id: string }>>(
    `${BASE_URL}/${sourceProjectId}/tasks/${taskId}/transfer`,
    { target_project_id: targetProjectId }
  );
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

// 保存任务文档：先取得明确基线，再以 expected_version 提交 replace_full。
// 409 由编辑器保留本地草稿并提示刷新对比，绝不静默覆盖远端版本。
export async function saveTaskDocument(projectId: string, taskId: string, docType: 'requirements' | 'design' | 'test', content: string, expectedVersion: number): Promise<ApiResponse<void>> {
  const response = await authedApi.post<ApiResponse<void>>(`${BASE_URL}/${projectId}/tasks/${taskId}/${docType}/append`, {
    content,
    expected_version: expectedVersion,
    op: 'replace_full',
    source: 'ui',
  });
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

// 历史执行计划只读查询。新版入口不得暴露任何 mutation。
export async function getExecutionPlan(projectId: string, taskId: string): Promise<ApiResponse<ExecutionPlan>> {
  const response = await authedApi.get<ApiResponse<ExecutionPlan>>(`/projects/${projectId}/tasks/${taskId}/execution-plan`);
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
