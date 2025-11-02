import { authedApi } from './auth';

const BASE_URL = '/projects'; // authedApi已配置baseURL='/api/v1'，这里只需相对路径

export interface RecommendationRequest {
  query_text: string;
  doc_type?: string;
  top_k?: number;
  threshold?: number;
  exclude_task_id?: string;
}

export interface Recommendation {
  task_id: string;
  doc_type: string;
  section_id: string;
  title: string;
  similarity: number;
  snippet: string;
}

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

export interface RecommendationsResponse {
  recommendations: Recommendation[];
  reason?: string; // 后端返回的跳过原因
}

// 写作前推荐
export async function getRecommendationsByQuery(
  projectId: string,
  taskId: string,
  request: RecommendationRequest
): Promise<ApiResponse<RecommendationsResponse>> {
  const response = await authedApi.post(
    `${BASE_URL}/${projectId}/tasks/${taskId}/recommendations/preview`,
    request
  );
  return response.data;
}

// 半实时增量推荐（支持请求取消）
export async function getRecommendationsLive(
  projectId: string,
  taskId: string,
  request: RecommendationRequest,
  signal?: AbortSignal
): Promise<ApiResponse<RecommendationsResponse>> {
  const response = await authedApi.post(
    `${BASE_URL}/${projectId}/tasks/${taskId}/recommendations/live`,
    request,
    { signal }
  );
  return response.data;
}
