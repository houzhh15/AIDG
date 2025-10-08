// 文档管理组件库

// 搜索相关组件 (T-M3-07a)
export { default as GlobalSearchBox } from './GlobalSearchBox';
export { default as SearchResultsView } from './SearchResultsView';

// 版本管理组件 (T-M3-07b) 
export { default as VersionHistoryPanel } from './VersionHistoryPanel';
export { default as DiffViewModal } from './DiffViewModal';

// 引用管理组件 (T-M3-07c)
export { default as ReferencePanel } from './ReferencePanel';

// 关系图组件 (T-M3-07d)
export { default as RelationshipGraph } from './RelationshipGraph';

// 冲突解决组件 (T-M3-07e)
export { default as ConflictResolver } from './ConflictResolver';
export type { ConflictItem, ConflictType } from './ConflictResolver';

// 文档类型选择器 (T-M1-08)
export { 
  DocumentTypeSelector, 
  DocumentTypeFilter, 
  typeConfigs 
} from './DocumentTypeSelector';
export type { 
  DocumentTypeFilterProps, 
  DocumentTypeConfig 
} from './DocumentTypeSelector';

// Markdown编辑器 (T-M1-09)
export { default as MarkdownEditor } from './MarkdownEditor';

// 影响分析面板 (T-M3-08)
export { default as ImpactAnalysisPanel } from './ImpactAnalysisPanel';

// 增强树视图 (T-M1-07增强)
export { default as EnhancedTreeView } from './EnhancedTreeView';
export type { 
  ReferenceOptionGroup, 
  ReferenceOption, 
  ReferenceSourceValue, 
  AddNodePayload,
  ReferenceContextType,
  ReferenceContextOption,
  ReferenceContextOptionsMap,
  ReferenceContextSelection
} from './EnhancedTreeView';

// 导出类型定义
export type {
  DocumentType,
  DocMetaEntry,
  DocumentTreeNode,
  SearchResult,
  GlobalSearchBoxProps,
  SearchResultsViewProps,
  SnapshotMeta,
  VersionHistoryPanelProps,
  DiffViewModalProps,
  Reference,
  ReferenceStatus,
  ReferencePanelProps,
  TypeConfig,
  DocumentTypeSelectorProps,
  AnalysisMode,
  ImpactResult,
  ImpactAnalysisPanelProps
} from '../../types/documents';