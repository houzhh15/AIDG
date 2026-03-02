import { authedApi } from './auth';

export type CopyMode = 'overwrite' | 'skip_existing';

export interface CopyResource {
  type: 'meeting' | 'project' | 'task';
  id: string;
  project_id?: string;
}

export interface CopyOptions {
  include_audio?: boolean;
  include_sub_tasks?: boolean;
}

export interface CopyPushRequest {
  remote_id?: string;
  remote_url?: string;
  resources: CopyResource[];
  mode: CopyMode;
  options?: CopyOptions;
}

export interface CopyResult {
  type: string;
  id: string;
  status: string;
  files: number;
  error?: string;
}

export interface CopyResultSummary {
  total: number;
  created: number;
  updated: number;
  skipped: number;
  errors: number;
}

export interface CopyReceiveResponse {
  success: boolean;
  resources: CopyResult[];
  summary: CopyResultSummary;
}

export interface CopyPushResponse {
  pushed_resources: number;
  remote_response: CopyReceiveResponse;
}

export interface RemoteResourceInfo {
  id: string;
  name: string;
  type: string;
}

export interface RemoteResourceList {
  meetings: RemoteResourceInfo[];
  projects: RemoteResourceInfo[];
}

export async function copyPush(req: CopyPushRequest): Promise<CopyPushResponse> {
  const r = await authedApi.post('/copy/push', req);
  return r.data;
}

export async function getCopyResources(): Promise<RemoteResourceList> {
  const r = await authedApi.get('/copy/resources');
  return r.data;
}
