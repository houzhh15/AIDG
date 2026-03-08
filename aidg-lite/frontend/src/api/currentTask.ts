import { ApiResponse } from '../types';
import { authedApi } from './auth';
import { ProjectTask } from './tasks';

export interface CurrentTaskInfo {
  project_id: string;
  task_id: string;
  task_info: ProjectTask;
  project_name: string;
  set_at: string;
}

export interface SetCurrentTaskRequest {
  project_id: string;
  task_id: string;
}

// 获取用户当前任务
export async function getCurrentTask(): Promise<ApiResponse<CurrentTaskInfo | null>> {
  const response = await authedApi.get<ApiResponse<CurrentTaskInfo | null>>('/user/current-task');
  return response.data;
}

// 设置用户当前任务
export async function setCurrentTask(data: SetCurrentTaskRequest): Promise<ApiResponse<void>> {
  const response = await authedApi.put<ApiResponse<void>>('/user/current-task', data);
  return response.data;
}