import { TaskSummary, ChunkFlag, SegmentsFile, AvDevice } from '../types';
import { authedApi as api } from './auth';
export { api }; // re-export for modules expecting { api }

export async function listTasks(): Promise<TaskSummary[]> {
  const r = await api.get('/tasks');
  return r.data.tasks;
}

export async function createTask(): Promise<TaskSummary> {
  const r = await api.post('/tasks', {});
  return r.data;
}

export async function deleteTask(id: string) {
  await api.delete(`/tasks/${encodeURIComponent(id)}`);
}

export async function startTask(id: string) {
  await api.post(`/tasks/${encodeURIComponent(id)}/start`);
}

export async function stopTask(id: string) {
  await api.post(`/tasks/${encodeURIComponent(id)}/stop`);
}

export async function reprocessTask(id: string) {
  await api.post(`/tasks/${encodeURIComponent(id)}/reprocess`);
}

export async function updateTaskDevice(id: string, ffmpegDevice: string) {
  await api.patch(`/tasks/${encodeURIComponent(id)}/config`, { ffmpeg_device: ffmpegDevice });
}

export async function updateTaskDiarization(id: string, backend: string) {
  await api.patch(`/tasks/${encodeURIComponent(id)}/config`, { diarization_backend: backend });
}

export async function updateTaskEmbeddingScript(id: string, script: string) {
  await api.patch(`/tasks/${encodeURIComponent(id)}/config`, { embedding_script: script });
}

export async function statusTask(id: string) {
  const r = await api.get(`/tasks/${encodeURIComponent(id)}/status`);
  return r.data;
}

export async function listChunks(id: string): Promise<ChunkFlag[]> {
  const r = await api.get(`/tasks/${encodeURIComponent(id)}/chunks`);
  return r.data.chunks;
}

export async function mergeOnly(taskId: string): Promise<string> {
  try {
    const r = await api.post(`/tasks/${encodeURIComponent(taskId)}/merge_only`);
    return r.data.merged_all;
  } catch(e:any){
    if(e.response?.data) throw new Error(JSON.stringify(e.response.data));
    throw e;
  }
}

export async function mergeChunk(taskId: string, chunkId: string): Promise<string> {
  try {
    const r = await api.post(`/tasks/${encodeURIComponent(taskId)}/chunks/${chunkId}/merge`);
    return r.data.merged;
  } catch(e:any){
    if(e.response?.data) throw new Error(JSON.stringify(e.response.data));
    throw e;
  }
}

export async function debugChunk(taskId: string, chunkId: string): Promise<any> {
  const r = await api.get(`/tasks/${encodeURIComponent(taskId)}/chunks/${chunkId}/debug`);
  return r.data;
}

export async function redoSpeakers(taskId: string, chunkId: string) {
  const r = await api.post(`/tasks/${encodeURIComponent(taskId)}/chunks/${chunkId}/redo/speakers`);
  return r.data;
}

export async function redoEmbeddings(taskId: string, chunkId: string) {
  const r = await api.post(`/tasks/${encodeURIComponent(taskId)}/chunks/${chunkId}/redo/embeddings`);
  return r.data;
}

export async function redoMapped(taskId: string, chunkId: string) {
  const r = await api.post(`/tasks/${encodeURIComponent(taskId)}/chunks/${chunkId}/redo/mapped`);
  return r.data;
}

export async function getChunkFile(taskId: string, chunkId: string, kind: string) {
  const r = await api.get(`/tasks/${encodeURIComponent(taskId)}/chunks/${chunkId}/${kind}`);
  return r.data;
}

export async function getChunkFileRaw(taskId: string, chunkId: string, kind: string, noCache = false) {
  const suffix = noCache ? `?_ts=${Date.now()}` : '';
  const r = await api.get(`/tasks/${encodeURIComponent(taskId)}/chunks/${chunkId}/${kind}${suffix}`, { responseType: 'text' });
  return r.data;
}

export async function updateSegments(taskId: string, chunkId: string, data: SegmentsFile) {
  await api.put(`/tasks/${encodeURIComponent(taskId)}/chunks/${chunkId}/segments`, data);
}

export async function asrOnce(taskId: string, chunkId: string, model: string, segments: string, temperature: number = 0.0){
  const r = await api.post(`/tasks/${encodeURIComponent(taskId)}/chunks/${chunkId}/asr_once`, { model, segments, temperature });
  return r.data;
}

export async function listAvfoundationDevices(): Promise<AvDevice[]> {
  const r = await api.get('/devices/avfoundation');
  return r.data.devices;
}

// Admin maintenance
export async function adminReload(): Promise<{success:boolean; message:string}> {
  const r = await api.post('/admin/reload', {});
  return r.data;
}

// Services status
export interface ServicesStatus {
  whisper_available: boolean;
  deps_service_available: boolean;
  whisper_mode?: string;
  dependency_mode?: string;
}

export async function getServicesStatus(): Promise<ServicesStatus> {
  const r = await api.get('/services/status');
  return r.data;
}
