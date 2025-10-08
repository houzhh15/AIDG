import { ApiResponse } from '../types';
import { authedApi } from './auth';

export interface BasicInfo {
  id: string;
  name: string;
  product_line: string;
  description?: string;
  owner?: string;
  start_date?: string;
  estimated_end_date?: string;
  created_at: string;
  updated_at: string;
}

export interface ProjectOverview {
  basic_info: BasicInfo;
}

export interface UpdateMetadataRequest {
  description?: string;
  owner?: string;
  start_date?: string;
  estimated_end_date?: string;
}

const BASE_URL = '/projects';

// 获取项目概述
export async function fetchProjectOverview(
  projectId: string
): Promise<ApiResponse<ProjectOverview>> {
  const response = await authedApi.get<ApiResponse<ProjectOverview>>(
    `${BASE_URL}/${projectId}/overview`
  );
  return response.data;
}

// 更新项目元数据
export async function updateProjectMetadata(
  projectId: string,
  data: UpdateMetadataRequest
): Promise<ApiResponse<void>> {
  const response = await authedApi.patch<ApiResponse<void>>(
    `${BASE_URL}/${projectId}/metadata`,
    data
  );
  return response.data;
}
