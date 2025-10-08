// Documents API - 多层级文档管理API封装
import { api as apiClient } from './client'

// 文档节点类型
export type DocumentType =
  | 'feature_list'
  | 'architecture'
  | 'tech_design'
  | 'background'
  | 'requirements'
  | 'meeting'
  | 'task'

// 文档元数据
export interface DocMetaEntry {
  id: string
  parent_id?: string
  title: string
  type: DocumentType
  level: number
  position: number
  version: number
  updated_at: string
  created_at: string
}

// 文档树节点
export interface DocumentTreeDTO {
  node: DocMetaEntry
  children?: DocumentTreeDTO[]
}

// 关系类型
// 根据设计文档要求，关系分为两类：
// 1. 显式关系（系统自动维护）：parent_child, sibling - 基于文档树结构自动生成和维护
// 2. 隐式关系（用户手动创建）：reference - 用户可以手动创建的依赖关系
export type RelationType = 'parent_child' | 'sibling' | 'reference'

// 依赖类型 - 仅适用于 reference 类型的关系，用于细化依赖关系的语义
export type DependencyType = 'data' | 'interface' | 'config'

// 关系数据
export interface Relationship {
  id: string
  from_id: string
  to_id: string
  type: RelationType
  dependency_type?: DependencyType
  description?: string
  created_at: string
  updated_at: string
}

// 引用状态
export type ReferenceStatus = 'active' | 'outdated' | 'broken'

// 引用数据
export interface Reference {
  id: string
  task_id: string
  document_id: string
  anchor?: string | null
  context?: string | null
  status: ReferenceStatus
  version: number
  created_at: string
  updated_at: string
}

// 创建节点请求
export interface CreateNodeRequest {
  parent_id?: string
  title: string
  type: DocumentType
  content: string
}

// 移动节点请求
export interface MoveNodeRequest {
  new_parent_id?: string
  position: number
}

export interface UpdateNodeRequest {
  title?: string
  type?: DocumentType
}

// 创建关系请求
export interface CreateRelationshipRequest {
  from_id: string
  to_id: string
  type: RelationType
  dependency_type?: DependencyType
  description?: string
}

// 创建引用请求
export interface CreateReferenceRequest {
  task_id: string
  document_id: string
  anchor?: string
  context?: string
}

// 更新内容请求
export interface UpdateContentRequest {
  content: string
  version: number
}

// Documents API
export const documentsAPI = {
  // 节点管理
  async createNode(projectId: string, request: CreateNodeRequest): Promise<{ node: DocMetaEntry }> {
    const response = await apiClient.post(`/projects/${projectId}/documents/nodes`, request)
    return response.data
  },

  async getTree(projectId: string, nodeId?: string, depth?: number): Promise<{ tree: DocumentTreeDTO }> {
    const params = new URLSearchParams()
    if (nodeId) params.append('node_id', nodeId)
    if (depth) params.append('depth', depth.toString())
    
    const response = await apiClient.get(`/projects/${projectId}/documents/tree?${params}`)
    return response.data
  },

  async moveNode(projectId: string, nodeId: string, request: MoveNodeRequest): Promise<{ success: boolean }> {
    const response = await apiClient.put(`/projects/${projectId}/documents/nodes/${nodeId}/move`, request)
    return response.data
  },

  async updateNode(projectId: string, nodeId: string, request: UpdateNodeRequest): Promise<{ node: DocMetaEntry }> {
    const response = await apiClient.patch(`/projects/${projectId}/documents/nodes/${nodeId}`, request)
    return response.data
  },

  async deleteNode(projectId: string, nodeId: string): Promise<{ success: boolean }> {
    const response = await apiClient.delete(`/projects/${projectId}/documents/nodes/${nodeId}`)
    return response.data
  },

  // 内容管理
  async getContent(projectId: string, nodeId: string): Promise<{ meta: DocMetaEntry; content: string }> {
    console.log('[API] documentsAPI.getContent called with:', { projectId, nodeId });
    if (nodeId.startsWith('task_')) {
      console.error('[API] WARNING: Attempting to call getContent with taskId:', nodeId);
      console.error('[API] Current Error stack:', new Error().stack);
      
      // 创建一个详细的错误并立即返回，避免实际的API调用
      return Promise.reject(new Error(`Blocked API call with taskId: ${nodeId}. Check the call stack above.`));
    }
    const response = await apiClient.get(`/projects/${projectId}/documents/${nodeId}/content`)
    return response.data
  },

  async updateContent(projectId: string, nodeId: string, request: UpdateContentRequest): Promise<{ version: number; success: boolean }> {
    const response = await apiClient.put(`/projects/${projectId}/documents/${nodeId}/content`, request)
    return response.data
  },

  // 关系管理
  async createRelationship(projectId: string, request: CreateRelationshipRequest): Promise<{ relationship: Relationship }> {
    const response = await apiClient.post(`/projects/${projectId}/documents/relationships`, request)
    return response.data
  },

  async getRelationships(projectId: string, nodeId?: string): Promise<{ relationships: Relationship[] }> {
    const params = nodeId ? `?node_id=${nodeId}` : ''
    const response = await apiClient.get(`/projects/${projectId}/documents/relationships${params}`)
    return response.data
  },

  async removeRelationship(projectId: string, fromId: string, toId: string): Promise<{ success: boolean }> {
    const response = await apiClient.delete(`/projects/${projectId}/documents/relationships/${fromId}/${toId}`)
    return response.data
  },

  // 引用管理
  async createReference(projectId: string, request: CreateReferenceRequest): Promise<{ reference: Reference }> {
    const response = await apiClient.post(`/projects/${projectId}/documents/references`, request)
    return response.data
  },

  async getTaskReferences(projectId: string, taskId: string): Promise<{ references: Reference[] }> {
    try {
      const response = await apiClient.get(`/projects/${projectId}/tasks/${taskId}/references`)
      return response.data
    } catch (error: any) {
      const status = error?.response?.status
      if (status === 404 || status === 400) {
        const fallbackResponse = await apiClient.get(`/tasks/${taskId}/references`)
        return fallbackResponse.data
      }
      throw error
    }
  },

  async getDocumentReferences(projectId: string, docId: string): Promise<{ references: Reference[] }> {
    const response = await apiClient.get(`/projects/${projectId}/documents/${docId}/references`)
    return response.data
  },

  async updateReferenceStatus(projectId: string, referenceId: string, status: ReferenceStatus): Promise<{ success: boolean }> {
    const response = await apiClient.put(`/projects/${projectId}/references/${referenceId}/status`, { status })
    return response.data
  },

  // 版本管理
  async getVersionHistory(projectId: string, docId: string, limit?: number): Promise<{ versions: SnapshotMeta[]; total: number }> {
    const params = limit ? `?limit=${limit}` : ''
    const response = await apiClient.get(`/projects/${projectId}/documents/${docId}/versions${params}`)
    return response.data
  },

  async getVersionContent(projectId: string, docId: string, version: number): Promise<{ version: number; content: string }> {
    const response = await apiClient.get(`/projects/${projectId}/documents/${docId}/versions/${version}`)
    return response.data
  },

  // 内容分析
  async compareVersions(projectId: string, docId: string, fromVersion: number, toVersion: number): Promise<{ diff: DiffResult }> {
    const response = await apiClient.get(`/projects/${projectId}/documents/${docId}/diff?from=${fromVersion}&to=${toVersion}`)
    return response.data
  },

  async analyzeImpact(projectId: string, docId: string, modes?: string[]): Promise<{ impact: ImpactResult }> {
    const modesParam = modes ? modes.join(',') : 'all'
    const response = await apiClient.get(`/projects/${projectId}/documents/${docId}/impact?modes=${modesParam}`)
    return response.data
  },

  // 搜索相关方法
  async searchDocuments(projectId: string, options: SearchOptions): Promise<SearchResponse> {
    const response = await apiClient.post(`/projects/${projectId}/documents/search`, options)
    return response.data
  },

  async getSearchSuggestions(projectId: string, query: string, limit?: number): Promise<{ suggestions: string[] }> {
    const params = new URLSearchParams({ q: query })
    if (limit) {
      params.append('limit', limit.toString())
    }
    const response = await apiClient.get(`/projects/${projectId}/documents/search/suggestions?${params}`)
    return response.data
  }
}

// 快照元数据接口
export interface SnapshotMeta {
  version: number
  created_at: string
  path: string
  size: number
}

// 差异类型
export type DiffType = 'add' | 'delete' | 'modify' | 'equal'

// 行级差异
export interface DiffLine {
  type: DiffType
  line_num: number
  content: string
  old_line?: number
  new_line?: number
}

// 差异摘要
export interface DiffSummary {
  added: number
  deleted: number
  modified: number
  total: number
}

// 差异结果
export interface DiffResult {
  from_version: number
  to_version: number
  lines: DiffLine[]
  summary: DiffSummary
}

// 分析模式
export type AnalysisMode = 'parents' | 'children' | 'references' | 'dependencies' | 'all'

// 影响分析结果
export interface ImpactResult {
  node_id: string
  parents: string[]
  children: string[]
  references: string[]
  dependencies: string[]
  depth: { [key: string]: number }
  paths: { [key: string]: string[] }
}

// 搜索选项
export interface SearchOptions {
  query: string
  case_sensitive?: boolean
  whole_word?: boolean
  use_regex?: boolean
  max_results?: number
  document_types?: string[]
  context_chars?: number
}

// 匹配高亮信息
export interface MatchHighlight {
  start: number
  end: number
  text: string
  before: string
  after: string
}

// 搜索结果
export interface SearchResult {
  document_id: string
  title: string
  content: string
  score: number
  title_matches: MatchHighlight[]
  content_matches: MatchHighlight[]
  metadata: { [key: string]: any }
  created_at: string
  updated_at: string
}

// 搜索响应
export interface SearchResponse {
  results: SearchResult[]
  count: number
  query: string
}

export default documentsAPI