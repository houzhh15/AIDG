/**
 * 项目文档统一 API
 * 与任务文档 (taskDocs.ts) 保持一致的接口风格
 */
import { authedApi } from './auth';
import { Section, SectionMeta, SectionContent } from '../types/section';

export type ProjectDocSlot = 'feature_list' | 'architecture_design';

// 重新导出 section 类型，方便其他模块使用
export type { Section, SectionMeta, SectionContent };

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
  doc_type?: string;
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

function base(projectId: string, slot: ProjectDocSlot) {
  return `/projects/${projectId}/docs/${slot}`;
}

// ========== 文档操作 ==========

export async function appendProjectDoc(projectId: string, slot: ProjectDocSlot, req: AppendRequest): Promise<AppendResponse> {
  const r = await authedApi.post<AppendResponse>(`${base(projectId, slot)}/append`, req);
  return r.data;
}

export async function listProjectDocChunks(projectId: string, slot: ProjectDocSlot): Promise<ListChunksResponse> {
  const r = await authedApi.get<ListChunksResponse>(`${base(projectId, slot)}/chunks`);
  return r.data;
}

export async function exportProjectDoc(projectId: string, slot: ProjectDocSlot): Promise<ExportResponse> {
  const r = await authedApi.get<ExportResponse>(`${base(projectId, slot)}/export`);
  return r.data;
}

export async function squashProjectDoc(projectId: string, slot: ProjectDocSlot, req: { expected_version?: number }) {
  const r = await authedApi.post(`${base(projectId, slot)}/squash`, req);
  return r.data as { version: number };
}

// ========== 章节操作 ==========

export async function getProjectDocSections(projectId: string, slot: ProjectDocSlot): Promise<SectionMeta> {
  const r = await authedApi.get<SectionMeta>(`${base(projectId, slot)}/sections`);
  return r.data;
}

export async function getProjectDocSection(projectId: string, slot: ProjectDocSlot, sectionId: string, includeChildren?: boolean): Promise<SectionContent> {
  const params = includeChildren ? { include_children: true } : {};
  const r = await authedApi.get<SectionContent>(`${base(projectId, slot)}/sections/${sectionId}`, { params });
  return r.data;
}

export async function updateProjectDocSection(
  projectId: string, 
  slot: ProjectDocSlot, 
  sectionId: string, 
  content: string, 
  expectedVersion?: number
): Promise<{ version: number }> {
  const r = await authedApi.put(`${base(projectId, slot)}/sections/${sectionId}`, { 
    content, 
    expected_version: expectedVersion 
  });
  return r.data;
}

export async function insertProjectDocSection(
  projectId: string, 
  slot: ProjectDocSlot, 
  title: string, 
  content: string, 
  afterSectionId?: string,
  expectedVersion?: number
): Promise<Section> {
  const r = await authedApi.post(`${base(projectId, slot)}/sections`, {
    title,
    content,
    after_section_id: afterSectionId,
    expected_version: expectedVersion,
  });
  return r.data;
}

export async function deleteProjectDocSection(
  projectId: string, 
  slot: ProjectDocSlot, 
  sectionId: string, 
  cascade?: boolean,
  expectedVersion?: number
): Promise<{ version: number }> {
  const params: Record<string, unknown> = {};
  if (cascade) params.cascade = true;
  if (expectedVersion !== undefined) params.expected_version = expectedVersion;
  const r = await authedApi.delete(`${base(projectId, slot)}/sections/${sectionId}`, { params });
  return r.data;
}

// ========== 便捷方法 ==========

// 全文替换
export async function replaceProjectDocFull(projectId: string, slot: ProjectDocSlot, content: string, expectedVersion?: number) {
  return appendProjectDoc(projectId, slot, { 
    content, 
    expected_version: expectedVersion, 
    op: 'replace_full', 
    source: 'ui' 
  });
}

// 获取完整内容（兼容旧接口格式）
export async function getProjectDocContent(projectId: string, slot: ProjectDocSlot): Promise<{ content: string; exists: boolean; version?: number }> {
  try {
    const result = await exportProjectDoc(projectId, slot);
    return {
      content: result.content || '',
      exists: !!result.content,
      version: result.version,
    };
  } catch (e: unknown) {
    // 如果新 API 失败，尝试旧 API
    const slotToOldPath: Record<ProjectDocSlot, string> = {
      feature_list: 'feature-list',
      architecture_design: 'architecture-design',
    };
    try {
      const r = await authedApi.get(`/projects/${projectId}/${slotToOldPath[slot]}`);
      return {
        content: r.data.content || '',
        exists: r.data.exists || !!r.data.content,
      };
    } catch {
      return { content: '', exists: false };
    }
  }
}

// 保存内容（兼容旧接口）
export async function saveProjectDocContent(projectId: string, slot: ProjectDocSlot, content: string): Promise<void> {
  try {
    await replaceProjectDocFull(projectId, slot, content);
  } catch {
    // 如果新 API 失败，尝试旧 API
    const slotToOldPath: Record<ProjectDocSlot, string> = {
      feature_list: 'feature-list',
      architecture_design: 'architecture-design',
    };
    await authedApi.put(`/projects/${projectId}/${slotToOldPath[slot]}`, { content });
  }
}
