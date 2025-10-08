import { ApiResponse } from '../types';
import { authedApi } from './auth';

export interface TaskSummary {
  id: string;
  task_id?: string;
  task_name?: string;
  time: string; // RFC3339 format
  week_number: string; // YYYY-WW format
  content: string; // Markdown content
  creator: string;
  created_at: string;
  updated_at: string;
}

export interface CreateSummaryRequest {
  time: string; // RFC3339 format
  content: string;
}

export interface UpdateSummaryRequest {
  time?: string;
  content?: string;
}

const BASE_URL = '/projects';

// 获取任务总结列表
export async function fetchTaskSummaries(
  projectId: string,
  taskId: string,
  startWeek?: string,
  endWeek?: string
): Promise<ApiResponse<TaskSummary[]>> {
  const params = new URLSearchParams();
  if (startWeek) params.append('start_week', startWeek);
  if (endWeek) params.append('end_week', endWeek);
  
  const queryString = params.toString();
  const url = `${BASE_URL}/${projectId}/tasks/${taskId}/summaries${queryString ? `?${queryString}` : ''}`;
  
  const response = await authedApi.get<ApiResponse<TaskSummary[]>>(url);
  return response.data;
}

// 添加任务总结
export async function addTaskSummary(
  projectId: string,
  taskId: string,
  data: CreateSummaryRequest
): Promise<ApiResponse<{ id: string; week_number: string }>> {
  const response = await authedApi.post<ApiResponse<{ id: string; week_number: string }>>(
    `${BASE_URL}/${projectId}/tasks/${taskId}/summaries`,
    data
  );
  return response.data;
}

// 更新任务总结
export async function updateTaskSummary(
  projectId: string,
  taskId: string,
  summaryId: string,
  data: UpdateSummaryRequest
): Promise<ApiResponse<void>> {
  const response = await authedApi.put<ApiResponse<void>>(
    `${BASE_URL}/${projectId}/tasks/${taskId}/summaries/${summaryId}`,
    data
  );
  return response.data;
}

// 删除任务总结
export async function deleteTaskSummary(
  projectId: string,
  taskId: string,
  summaryId: string
): Promise<ApiResponse<void>> {
  const response = await authedApi.delete<ApiResponse<void>>(
    `${BASE_URL}/${projectId}/tasks/${taskId}/summaries/${summaryId}`
  );
  return response.data;
}

// 跨任务按周范围检索总结
export async function fetchSummariesByWeek(
  projectId: string,
  startWeek: string,
  endWeek: string
): Promise<ApiResponse<TaskSummary[]>> {
  const params = new URLSearchParams();
  params.append('start_week', startWeek);
  params.append('end_week', endWeek);
  
  const response = await authedApi.get<ApiResponse<TaskSummary[]>>(
    `${BASE_URL}/${projectId}/summaries/by-week?${params.toString()}`
  );
  return response.data;
}
