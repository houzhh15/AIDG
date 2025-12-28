// 文档管理系统类型定义
// 基于设计文档3.3节数据模型

export type DocumentType = 'feature_list' | 'architecture' | 'tech_design' | 'background' | 'requirements' | 'meeting' | 'task';

// 文件导入元数据
export interface ImportMeta {
  source_type: 'file_import';
  original_filename: string;
  file_size: number;
  content_type: 'markdown' | 'svg';
}

export interface DocMetaEntry {
  id: string;
  parent_id?: string;
  title: string;
  type: DocumentType;
  level: number;
  position: number;
  version: number;
  updated_at: string;
  import_meta?: ImportMeta;  // 文件导入元数据
}

export interface DocumentTreeNode extends DocMetaEntry {
  children?: DocumentTreeNode[];
  breadcrumbs?: string[];
}

// 搜索相关类型
export interface SearchResult {
  nodeId: string;
  title: string;
  type: DocumentType;
  content: string;
  breadcrumbs: string[];
  relevanceScore: number;
}

export interface GlobalSearchBoxProps {
  projectId: string;
  onSearch: (query: string) => void;
  placeholder?: string;
}

export interface SearchResultsViewProps {
  results: SearchResult[];
  loading: boolean;
  onNodeSelect: (nodeId: string) => void;
}

// 版本管理相关类型
export interface SnapshotMeta {
  version: number;
  created_at: string;
  path: string;
}

export interface VersionHistoryPanelProps {
  projectId: string;
  nodeId: string;
  onVersionSelect: (version: number) => void;
  currentVersion?: number;
  currentTitle?: string;
  onCompareWithCurrent?: (version: number) => void;
  onCompareSelected?: (versions: [number, number]) => void;
  refreshKey?: number;
}

export interface DiffViewModalProps {
  projectId: string;
  nodeId: string;
  fromVersion: number;
  toVersion: number;
  visible: boolean;
  onClose: () => void;
  currentVersion?: number;
}

// 引用管理相关类型
export type ReferenceStatus = 'active' | 'outdated' | 'broken';

export interface Reference {
  id: string;
  task_id: string;
  document_id: string;
  anchor?: string | null;
  context?: string | null;
  status: ReferenceStatus;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface ReferencePanelProps {
  projectId: string;
  nodeId: string;
  references: Reference[];
  onReferenceClick: (referenceId: string) => void;
}

// 文档类型配置
export interface TypeConfig {
  type: DocumentType;
  label: string;
  icon: React.ReactNode;
  color: string;
}

export interface DocumentTypeSelectorProps {
  value?: DocumentType;
  onChange: (type: DocumentType) => void;
  disabled?: boolean;
}

// 影响分析相关类型  
export type AnalysisMode = 'upstream' | 'downstream' | 'bidirectional';

export interface ImpactResult {
  affected_node_id: string;
  title: string;
  description: string;
  impact_level: 'high' | 'medium' | 'low';
  change_probability?: number;
  relationship_type: 'parent' | 'child' | 'reference' | 'dependency';
}

export interface ImpactAnalysisPanelProps {
  projectId: string;
  nodeId: string;
  visible: boolean;
  onClose: () => void;
}