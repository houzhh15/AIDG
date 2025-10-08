import { ApiResponse } from '../types';
import { authedApi } from './auth';

export interface TaskTrend {
  completed_this_week: number;
  completed_last_week: number;
}

export interface TaskDistribution {
  total: number;
  completed: number;
  in_progress: number;
  todo: number;
  distribution: {
    completed: number;
    in_progress: number;
    todo: number;
  };
  trend?: TaskTrend;
}

const BASE_URL = '/projects';

// 获取任务状态统计
export async function fetchTaskStatistics(projectId: string): Promise<ApiResponse<TaskDistribution>> {
  const response = await authedApi.get<ApiResponse<TaskDistribution>>(
    `${BASE_URL}/${projectId}/tasks/statistics`
  );
  return response.data;
}

// 注意：fetchProjectOverview 已移至 projectApi.ts，请从那里导入
