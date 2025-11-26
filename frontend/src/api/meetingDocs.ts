/**
 * 会议文档统一 API
 * 与项目文档 (projectDocs.ts) 保持一致的接口风格
 */
import { authedApi } from './auth';
import { Section, SectionMeta, SectionContent } from '../types/section';

// 会议文档槽位：polish（会议详情/润色记录）, summary（会议总结）, topic（话题）
export type MeetingDocSlot = 'polish' | 'summary' | 'topic';

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

function base(meetingId: string, slot: MeetingDocSlot) {
  return `/meetings/${meetingId}/docs/${slot}`;
}

// ========== 文档操作 ==========

export async function appendMeetingDoc(meetingId: string, slot: MeetingDocSlot, req: AppendRequest): Promise<AppendResponse> {
  const r = await authedApi.post<AppendResponse>(`${base(meetingId, slot)}/append`, req);
  return r.data;
}

export async function listMeetingDocChunks(meetingId: string, slot: MeetingDocSlot): Promise<ListChunksResponse> {
  const r = await authedApi.get<ListChunksResponse>(`${base(meetingId, slot)}/chunks`);
  return r.data;
}

export async function exportMeetingDoc(meetingId: string, slot: MeetingDocSlot): Promise<ExportResponse> {
  const r = await authedApi.get<ExportResponse>(`${base(meetingId, slot)}/export`);
  return r.data;
}

export async function squashMeetingDoc(meetingId: string, slot: MeetingDocSlot, req: { expected_version?: number }) {
  const r = await authedApi.post(`${base(meetingId, slot)}/squash`, req);
  return r.data as { version: number };
}

// ========== 章节操作 ==========

export async function getMeetingDocSections(meetingId: string, slot: MeetingDocSlot): Promise<SectionMeta> {
  const r = await authedApi.get<SectionMeta>(`${base(meetingId, slot)}/sections`);
  return r.data;
}

export async function getMeetingDocSection(meetingId: string, slot: MeetingDocSlot, sectionId: string, includeChildren?: boolean): Promise<SectionContent> {
  const params = includeChildren ? { include_children: true } : {};
  const r = await authedApi.get<SectionContent>(`${base(meetingId, slot)}/sections/${sectionId}`, { params });
  return r.data;
}

export async function updateMeetingDocSection(
  meetingId: string, 
  slot: MeetingDocSlot, 
  sectionId: string, 
  content: string, 
  expectedVersion?: number
): Promise<{ version: number }> {
  const r = await authedApi.put(`${base(meetingId, slot)}/sections/${sectionId}`, { 
    content, 
    expected_version: expectedVersion 
  });
  return r.data;
}

export async function insertMeetingDocSection(
  meetingId: string, 
  slot: MeetingDocSlot, 
  title: string, 
  content: string, 
  afterSectionId?: string,
  expectedVersion?: number
): Promise<Section> {
  const r = await authedApi.post(`${base(meetingId, slot)}/sections`, {
    title,
    content,
    after_section_id: afterSectionId,
    expected_version: expectedVersion,
  });
  return r.data;
}

export async function deleteMeetingDocSection(
  meetingId: string, 
  slot: MeetingDocSlot, 
  sectionId: string, 
  cascade?: boolean,
  expectedVersion?: number
): Promise<{ version: number }> {
  const params: Record<string, unknown> = {};
  if (cascade) params.cascade = true;
  if (expectedVersion !== undefined) params.expected_version = expectedVersion;
  const r = await authedApi.delete(`${base(meetingId, slot)}/sections/${sectionId}`, { params });
  return r.data;
}

// ========== 便捷方法 ==========

// 全文替换
export async function replaceMeetingDocFull(meetingId: string, slot: MeetingDocSlot, content: string, expectedVersion?: number) {
  return appendMeetingDoc(meetingId, slot, { 
    content, 
    expected_version: expectedVersion, 
    op: 'replace_full', 
    source: 'ui' 
  });
}

// 获取完整内容（兼容旧接口格式）
export async function getMeetingDocContent(meetingId: string, slot: MeetingDocSlot): Promise<{ content: string; exists: boolean; version?: number }> {
  try {
    const result = await exportMeetingDoc(meetingId, slot);
    return {
      content: result.content || '',
      exists: !!result.content,
      version: result.version,
    };
  } catch (e: unknown) {
    // 如果新 API 失败，尝试旧 API
    const slotToOldPath: Record<MeetingDocSlot, string> = {
      polish: 'polish',
      summary: 'meeting-summary',
      topic: 'topic',
    };
    try {
      const r = await authedApi.get(`/tasks/${meetingId}/${slotToOldPath[slot]}`);
      return {
        content: r.data.content || '',
        exists: !!r.data.content,
      };
    } catch {
      return { content: '', exists: false };
    }
  }
}

// 保存内容（兼容旧接口）
export async function saveMeetingDocContent(meetingId: string, slot: MeetingDocSlot, content: string): Promise<void> {
  try {
    await replaceMeetingDocFull(meetingId, slot, content);
  } catch {
    // 如果新 API 失败，尝试旧 API
    const slotToOldPath: Record<MeetingDocSlot, string> = {
      polish: 'polish',
      summary: 'meeting-summary',
      topic: 'topic',
    };
    await authedApi.put(`/tasks/${meetingId}/${slotToOldPath[slot]}`, { content });
  }
}
