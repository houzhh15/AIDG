import { authedApi } from './auth';

export interface RemoteSafe {
  id: string;
  name: string;
  url: string;
  created_at: string;
  updated_at: string;
}

export interface CreateRemoteRequest {
  name: string;
  url: string;
  secret?: string;
}

export interface UpdateRemoteRequest {
  name?: string;
  url?: string;
  secret?: string;
}

export interface TestResult {
  reachable: boolean;
  status: string;
  latency: string;
  error?: string;
  service?: string;
  version?: string;
}

export async function listRemotes(): Promise<RemoteSafe[]> {
  const r = await authedApi.get('/remotes');
  return r.data.remotes;
}

export async function createRemote(req: CreateRemoteRequest): Promise<RemoteSafe> {
  const r = await authedApi.post('/remotes', req);
  return r.data;
}

export async function updateRemote(id: string, req: UpdateRemoteRequest): Promise<RemoteSafe> {
  const r = await authedApi.put(`/remotes/${encodeURIComponent(id)}`, req);
  return r.data;
}

export async function deleteRemote(id: string): Promise<void> {
  await authedApi.delete(`/remotes/${encodeURIComponent(id)}`);
}

export async function testRemote(id: string): Promise<TestResult> {
  const r = await authedApi.post(`/remotes/${encodeURIComponent(id)}/test`);
  return r.data;
}

export async function testRemoteURL(url: string): Promise<TestResult> {
  const r = await authedApi.post('/remotes/test-url', { url });
  return r.data;
}
