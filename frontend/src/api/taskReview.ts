import { authedApi } from './auth';
import type { ProjectTask } from './tasks';

export interface ReviewDocument {
  content: string;
  version: number;
  etag: string;
  updated_at: string;
  exists: boolean;
}

export interface ReviewMutationResponse {
  success: boolean;
  data: { task: ProjectTask; review: ReviewDocument };
}

const base = (projectId: string, taskId: string) => `/projects/${projectId}/tasks/${taskId}/review`;

export async function getTaskReview(projectId: string, taskId: string): Promise<ReviewDocument | null> {
  try {
    const response = await authedApi.get<{ success: boolean; data: ReviewDocument }>(base(projectId, taskId));
    return response.data.data;
  } catch (error: any) {
    if (error?.response?.status === 404) return null;
    throw error;
  }
}

export async function requestTaskReview(
  projectId: string,
  taskId: string,
  input: { target_type: 'AIDG文档' | '文件' | '代码'; target_path: string; criteria: string; expected_revision?: number },
): Promise<ReviewMutationResponse> {
  const response = await authedApi.post<ReviewMutationResponse>(base(projectId, taskId), input);
  return response.data;
}

export async function updateTaskReviewResult(
  projectId: string,
  taskId: string,
  input: { result: '通过' | '不通过'; issues?: string; expected_revision?: number },
): Promise<ReviewMutationResponse> {
  const response = await authedApi.put<ReviewMutationResponse>(`${base(projectId, taskId)}/result`, input);
  return response.data;
}
