import { ApiResponse } from '../types';
import { authedApi } from './auth';

export interface RoadmapNode {
  id: string;
  date: string; // YYYY-MM-DD
  goal: string;
  description: string;
  status: 'completed' | 'in-progress' | 'todo';
  created_at: string;
  updated_at: string;
}

export interface Roadmap {
  version: number;
  updated_at: string;
  nodes: RoadmapNode[];
}

export interface CreateNodeRequest {
  date: string;
  goal: string;
  description: string;
  status: 'completed' | 'in-progress' | 'todo';
}

export interface UpdateNodeRequest {
  date?: string;
  goal?: string;
  description?: string;
  status?: 'completed' | 'in-progress' | 'todo';
}

const BASE_URL = '/projects';

// 获取项目Roadmap
export async function fetchRoadmap(projectId: string): Promise<ApiResponse<Roadmap>> {
  const response = await authedApi.get<ApiResponse<Roadmap>>(
    `${BASE_URL}/${projectId}/roadmap`
  );
  return response.data;
}

// 添加Roadmap节点
export async function addRoadmapNode(
  projectId: string,
  data: CreateNodeRequest
): Promise<ApiResponse<RoadmapNode>> {
  const response = await authedApi.post<ApiResponse<RoadmapNode>>(
    `${BASE_URL}/${projectId}/roadmap/nodes`,
    data
  );
  return response.data;
}

// 更新Roadmap节点
export async function updateRoadmapNode(
  projectId: string,
  nodeId: string,
  data: UpdateNodeRequest,
  expectedVersion: number
): Promise<ApiResponse<void>> {
  const response = await authedApi.put<ApiResponse<void>>(
    `${BASE_URL}/${projectId}/roadmap/nodes/${nodeId}?expected_version=${expectedVersion}`,
    data
  );
  return response.data;
}

// 删除Roadmap节点
export async function deleteRoadmapNode(
  projectId: string,
  nodeId: string
): Promise<ApiResponse<void>> {
  const response = await authedApi.delete<ApiResponse<void>>(
    `${BASE_URL}/${projectId}/roadmap/nodes/${nodeId}`
  );
  return response.data;
}
