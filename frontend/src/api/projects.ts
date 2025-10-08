import { api } from './client';

export interface ProjectSummary {
  id: string;
  name: string;
  product_line?: string;
  created_at: string;
  updated_at?: string;
}

export async function listProjects(): Promise<ProjectSummary[]> {
  const r = await api.get('/projects');
  return r.data.projects || [];
}

export async function createProject(payload: { id?: string; name: string; product_line?: string; from_task_id?: string; }): Promise<ProjectSummary> {
  const r = await api.post('/projects', payload);
  return r.data;
}

export async function getProject(id: string): Promise<ProjectSummary> {
  const r = await api.get(`/projects/${id}`);
  return r.data;
}

export async function patchProject(id: string, patch: { name?: string; product_line?: string; }): Promise<ProjectSummary> {
  const r = await api.patch(`/projects/${id}`, patch);
  return r.data;
}

export async function deleteProject(id: string): Promise<{deleted: string}> {
  const r = await api.delete(`/projects/${id}`);
  return r.data;
}

// Deliverables
export async function getProjectFeatureList(id: string){
  const r = await api.get(`/projects/${id}/feature-list`); return r.data;
}
export async function saveProjectFeatureList(id: string, content: string){
  await api.put(`/projects/${id}/feature-list`, { content });
}
export async function getProjectFeatureListHistory(id: string){
  const r = await api.get(`/projects/${id}/feature-list/history`); return r.data.history || [];
}
export async function deleteProjectFeatureListHistory(id: string, version: number){
  await api.delete(`/projects/${id}/feature-list/history/${version}`);
}

export async function getProjectFeatureListJson(id: string){
  const r = await api.get(`/projects/${id}/feature-list.json`); 
  return r.data;
}

export async function getProjectArchitecture(id: string){
  const r = await api.get(`/projects/${id}/architecture-design`); return r.data;
}
export async function saveProjectArchitecture(id: string, content: string){
  await api.put(`/projects/${id}/architecture-design`, { content });
}
export async function getProjectArchitectureHistory(id: string){
  const r = await api.get(`/projects/${id}/architecture-design/history`); return r.data.history || [];
}
export async function deleteProjectArchitectureHistory(id: string, version: number){
  await api.delete(`/projects/${id}/architecture-design/history/${version}`);
}

export async function getProjectTechDesign(id: string){
  const r = await api.get(`/projects/${id}/tech-design`); return r.data;
}
export async function saveProjectTechDesign(id: string, content: string, filename?: string){
  await api.put(`/projects/${id}/tech-design`, { content, filename });
}
export async function getProjectTechDesignHistory(id: string){
  const r = await api.get(`/projects/${id}/tech-design/history`); return r.data.history || [];
}
export async function deleteProjectTechDesignHistory(id: string, version: number){
  await api.delete(`/projects/${id}/tech-design/history/${version}`);
}

export async function copyDeliverablesFromTask(id: string, sourceTaskId: string, kinds: string[]){
  await api.post(`/projects/${id}/copy-from-task`, { sourceTaskId, kinds });
}

// Project Tasks Management
export interface ProjectTask {
  id: string;
  name: string;
  description?: string;
  status: 'todo' | 'in-progress' | 'review' | 'completed';
  assignee?: string;
  module?: string;
  feature_id?: string;
  feature_name?: string;
  created_at?: string;
  updated_at?: string;
}

export interface ProjectTasksResponse {
  data: ProjectTask[];
  total?: number;
  success: boolean;
  message?: string;
}

export async function getProjectTasks(projectId: string): Promise<ProjectTask[]> {
  const r = await api.get(`/projects/${projectId}/tasks`);
  return r.data.data || r.data || [];
}

export async function getProjectTask(projectId: string, taskId: string): Promise<ProjectTask> {
  const r = await api.get(`/projects/${projectId}/tasks/${taskId}`);
  return r.data;
}

export async function createProjectTask(projectId: string, task: Omit<ProjectTask, 'id' | 'created_at' | 'updated_at'>): Promise<ProjectTask> {
  const r = await api.post(`/projects/${projectId}/tasks`, task);
  return r.data;
}

export async function updateProjectTask(projectId: string, taskId: string, task: Partial<ProjectTask>): Promise<ProjectTask> {
  const r = await api.put(`/projects/${projectId}/tasks/${taskId}`, task);
  return r.data;
}

export async function deleteProjectTask(projectId: string, taskId: string): Promise<{deleted: string}> {
  const r = await api.delete(`/projects/${projectId}/tasks/${taskId}`);
  return r.data;
}
