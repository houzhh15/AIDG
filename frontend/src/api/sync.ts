import { authedApi } from './auth';

export type SyncMode = 'client_overwrite' | 'server_overwrite' | 'merge_no_overwrite' | 'pull_overwrite';

export interface SyncFile {
  path: string;
  hash: string;
  content: string;
  size: number;
}

// legacy functions (kept temporarily for backward compatibility)
export async function syncPrepare(baseUrl?: string){
  const r = await authedApi.get('/sync/prepare');
  return r.data.files as SyncFile[];
}
export async function syncExecute(targetBaseUrl: string, mode: SyncMode, files: SyncFile[], options?: any){
  const body = { mode, client_host: window.location.hostname, timestamp: new Date().toISOString(), files, options: options||{} };
  const r = await authedApi.post(targetBaseUrl + '/api/v1/sync', body);
  return r.data;
}

// new single-step dispatch (server-to-server); backend will collect & send
export async function dispatchSync(target: string, mode: SyncMode, returnFiles=false){
  const body = { target, mode, return_files: returnFiles };
  const r = await authedApi.post('/sync/dispatch', body);
  return r.data;
}
