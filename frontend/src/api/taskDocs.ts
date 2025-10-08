import { authedApi } from './auth';

export type DocType = 'requirements' | 'design' | 'test';

export interface DocChunk {
  sequence: number;
  timestamp: string;
  op: 'add_full' | 'replace_full';
  content: string;
  user: string;
  source: string;
  hash: string;
  active: boolean;
}

export interface DocMeta {
  version: number;
  last_sequence: number;
  created_at?: string;
  updated_at?: string;
  doc_type?: DocType;
  hash_window?: string[];
  chunk_count: number;
  deleted_count: number;
  etag: string;
}

export interface AppendRequest {
  content: string;
  expected_version?: number;
  op?: 'add_full' | 'replace_full';
  source?: string;
}

export interface AppendResponse {
  version: number;
  duplicate: boolean;
  etag: string;
  sequence?: number;
  timestamp?: string;
  last_sequence: number;
  chunk_count: number;
  deleted_count: number;
  compiled_size?: number;
}

export interface ListChunksResponse {
  chunks: DocChunk[];
  meta: DocMeta;
}

export interface ExportResponse {
  content: string;
  version: number;
  etag: string;
}

function base(projectId: string, taskId: string, doc: DocType) {
  return `/projects/${projectId}/tasks/${taskId}/${doc}`;
}

export async function appendDoc(projectId: string, taskId: string, doc: DocType, req: AppendRequest): Promise<AppendResponse> {
  const r = await authedApi.post<AppendResponse>(`${base(projectId, taskId, doc)}/append`, req);
  return r.data;
}

export async function listDocChunks(projectId: string, taskId: string, doc: DocType): Promise<ListChunksResponse> {
  const r = await authedApi.get<ListChunksResponse>(`${base(projectId, taskId, doc)}/chunks`);
  return r.data;
}

export async function deleteDocChunk(projectId: string, taskId: string, doc: DocType, seq: number) {
  const r = await authedApi.delete(`${base(projectId, taskId, doc)}/chunks/${seq}`);
  return r.data as { version: number };
}

export async function exportDoc(projectId: string, taskId: string, doc: DocType): Promise<ExportResponse> {
  const r = await authedApi.get<ExportResponse>(`${base(projectId, taskId, doc)}/export`);
  return r.data;
}

// toggle active/inactive
export async function toggleDocChunk(projectId: string, taskId: string, doc: DocType, seq: number) {
  const r = await authedApi.patch(`${base(projectId, taskId, doc)}/chunks/${seq}/toggle`, {});
  return r.data as { version: number };
}

// squash chunks
export async function squashDoc(projectId: string, taskId: string, doc: DocType, req: { expected_version?: number }) {
  const r = await authedApi.post(`${base(projectId, taskId, doc)}/squash`, req);
  return r.data as { version: number };
}

// legacy full get (兼容)
export async function legacyGet(projectId: string, taskId: string, doc: DocType): Promise<{content: string; version?: number; etag?: string; exists?: boolean;}> {
  const r = await authedApi.get(`${base(projectId, taskId, doc)}`);
  return r.data;
}

// convenience replace_full
export async function replaceFull(projectId: string, taskId: string, doc: DocType, content: string, expected_version?: number) {
  return appendDoc(projectId, taskId, doc, { content, expected_version, op: 'replace_full', source: 'ui' });
}
