/**
 * 身份源 API 封装
 * @module api/identityProviders
 */

import axios from 'axios';
import { ApiResponse } from '../types';
import { authedApi } from './auth';
import {
  IdentityProvider,
  PublicIdentityProvider,
  CreateIdPRequest,
  UpdateIdPRequest,
  TestConnectionRequest,
  TestResult,
  SyncStatus,
  SyncLog,
  SyncResult,
} from '../types/identityProvider';

const BASE_URL = '/identity-providers';

/**
 * 获取身份源列表
 */
export async function getIdentityProviders(): Promise<ApiResponse<IdentityProvider[]>> {
  const response = await authedApi.get<ApiResponse<IdentityProvider[]>>(BASE_URL);
  return response.data;
}

/**
 * 获取单个身份源详情
 */
export async function getIdentityProvider(id: string): Promise<ApiResponse<IdentityProvider>> {
  const response = await authedApi.get<ApiResponse<IdentityProvider>>(`${BASE_URL}/${id}`);
  return response.data;
}

/**
 * 创建身份源
 */
export async function createIdentityProvider(data: CreateIdPRequest): Promise<ApiResponse<IdentityProvider>> {
  const response = await authedApi.post<ApiResponse<IdentityProvider>>(BASE_URL, data);
  return response.data;
}

/**
 * 更新身份源
 */
export async function updateIdentityProvider(id: string, data: UpdateIdPRequest): Promise<ApiResponse<IdentityProvider>> {
  const response = await authedApi.put<ApiResponse<IdentityProvider>>(`${BASE_URL}/${id}`, data);
  return response.data;
}

/**
 * 删除身份源
 */
export async function deleteIdentityProvider(id: string): Promise<ApiResponse<void>> {
  const response = await authedApi.delete<ApiResponse<void>>(`${BASE_URL}/${id}`);
  return response.data;
}

/**
 * 测试身份源连接
 */
export async function testConnection(data: TestConnectionRequest): Promise<ApiResponse<TestResult>> {
  const response = await authedApi.post<ApiResponse<TestResult>>(`${BASE_URL}/test`, data);
  return response.data;
}

/**
 * 获取公开身份源列表（无需认证，用于登录页）
 * 使用原始 axios 而非 authedApi，因为此接口不需要认证
 */
export async function getPublicIdentityProviders(): Promise<ApiResponse<PublicIdentityProvider[]>> {
  const response = await axios.get<ApiResponse<PublicIdentityProvider[]>>('/api/v1/identity-providers/public');
  return response.data;
}

/**
 * 触发用户同步
 */
export async function triggerSync(idpId: string): Promise<ApiResponse<SyncResult>> {
  const response = await authedApi.post<ApiResponse<SyncResult>>(`${BASE_URL}/${idpId}/sync`);
  return response.data;
}

/**
 * 获取同步状态
 */
export async function getSyncStatus(idpId: string): Promise<ApiResponse<SyncStatus>> {
  const response = await authedApi.get<ApiResponse<SyncStatus>>(`${BASE_URL}/${idpId}/sync/status`);
  return response.data;
}

/**
 * 获取同步日志列表
 */
export async function getSyncLogs(idpId: string, limit: number = 10): Promise<ApiResponse<SyncLog[]>> {
  const response = await authedApi.get<ApiResponse<SyncLog[]>>(`${BASE_URL}/${idpId}/sync/logs`, {
    params: { limit }
  });
  return response.data;
}
