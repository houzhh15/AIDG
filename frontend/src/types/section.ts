// 章节管理相关的 TypeScript 类型定义

export interface Section {
  id: string
  title: string
  level: number
  order: number
  parent_id: string | null
  file: string
  children: string[]
  hash: string
}

export interface SectionMeta {
  version: number
  updated_at: string
  root_level: number
  sections: Section[]
  etag: string
}

export interface SectionContent {
  id: string
  title: string
  level: number
  order: number
  parent_id: string | null
  file: string
  children: string[]
  hash: string
  content: string
  children_content?: SectionContent[]
}

export interface UpdateSectionRequest {
  content: string
  expected_version?: number
}

export interface InsertSectionRequest {
  title: string
  content: string
  after_section_id?: string
  expected_version?: number
}

export interface DeleteSectionRequest {
  expected_version?: number
}

export interface ReorderSectionRequest {
  section_id: string
  after_section_id?: string
  expected_version?: number
}

export interface SyncSectionsRequest {
  direction: 'from_compiled' | 'to_compiled'
}
