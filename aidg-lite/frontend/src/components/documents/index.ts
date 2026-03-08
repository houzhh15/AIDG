// 文档管理组件库 (Lite)

// Markdown编辑器
export { default as MarkdownEditor } from './MarkdownEditor';

// 增强树视图
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

// 文件上传区域
export { default as FileUploadArea } from './FileUploadArea';

// 导出类型定义
export type {
  DocumentType,
  DocMetaEntry,
  DocumentTreeNode,
} from '../../types/documents';