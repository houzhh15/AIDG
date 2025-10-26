import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import dayjs from 'dayjs';
import { DocumentTreeDTO, MoveNodeRequest, Relationship as RelationshipDTO, RelationType, DependencyType, CreateRelationshipRequest, ImpactResult as ImpactAnalyzerResult } from '../api/documents';
import { addCustomResource } from '../api/resourceApi';
import { Space, Typography, Tabs, message, Spin, Modal, Button, Layout, Drawer, Tooltip, Form, Select, Input, Table, Tag, Popconfirm, Divider } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { FileTextOutlined, SearchOutlined, ShareAltOutlined, HistoryOutlined, ExclamationCircleOutlined, BarChartOutlined, FullscreenOutlined, CompressOutlined, PlusOutlined, CloseOutlined } from '@ant-design/icons';
import { 
  GlobalSearchBox, 
  SearchResultsView, 
  VersionHistoryPanel,
  DiffViewModal,
  ReferencePanel,
  RelationshipGraph,
  ConflictResolver,
  DocumentTypeSelector,
  MarkdownEditor,
  ImpactAnalysisPanel,
  EnhancedTreeView,
  SearchResult,
  Reference,
  ConflictItem,
  ImpactResult as ImpactNodeResult,
  DocumentTreeNode,
  DocumentType,
  AnalysisMode,
  AddNodePayload,
  ReferenceOptionGroup,
  ReferenceSourceValue,
  ReferenceContextOptionsMap,
  ReferenceContextSelection,
  ReferenceContextType,
  ReferenceContextOption
} from './documents';

const { Title } = Typography;
const { TabPane } = Tabs;
const { Sider, Content } = Layout;

const sidebarPanelStyle: React.CSSProperties = {
  border: '1px solid #e6ebf2',
  borderRadius: 10,
  backgroundColor: '#fff',
  padding: 16,
  display: 'flex',
  flexDirection: 'column',
  boxShadow: '0 1px 2px rgba(15, 23, 42, 0.04)'
};

const sidebarPanelHeaderStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: 12
};

const sidebarPanelTitleStyle: React.CSSProperties = {
  fontSize: 14,
  fontWeight: 600,
  color: '#1f2937'
};

const sidebarPanelBodyStyle: React.CSSProperties = {
  flex: 1,
  display: 'flex',
  flexDirection: 'column',
  minHeight: 0
};

const sidebarActionGroupStyle: React.CSSProperties = {
  display: 'flex',
  gap: 8,
  padding: '8px 12px',
  borderRadius: 8,
  backgroundColor: '#f5f7fb',
  border: '1px solid #e0e6f0',
  marginBottom: 12
};

const PLACEHOLDER_TITLES = new Set([
  '新特性列表',
  '新架构设计',
  '新技术方案',
  '新背景资料',
  '新需求文档',
  '新会议纪要',
  '新任务文档'
]);

const overlayWrapperStyle: React.CSSProperties = {
  position: 'absolute',
  inset: 0,
  background: 'rgba(247, 248, 250, 0.95)',
  display: 'flex',
  flexDirection: 'column',
  padding: 16,
  zIndex: 5
};

const overlayPanelStyle: React.CSSProperties = {
  flex: 1,
  display: 'flex',
  flexDirection: 'column',
  backgroundColor: '#fff',
  borderRadius: 12,
  boxShadow: '0 12px 40px rgba(15, 23, 42, 0.18)',
  padding: 16,
  overflow: 'hidden'
};

const overlayHeaderStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: 16
};

const overlayBodyStyle: React.CSSProperties = {
  flex: 1,
  minHeight: 0,
  overflow: 'auto'
};

type ReferenceSourceGroup = 'task' | 'project' | 'meeting';

interface ReferenceSourceMeta {
  group: ReferenceSourceGroup;
  label: string;
  description: string;
  documentType: DocumentType;
  requiresContext?: ReferenceContextType;
}

type ReferenceContentLoader = (context?: ReferenceContextSelection) => Promise<{ content: string; title?: string }>;

const REFERENCE_SOURCE_META: Record<ReferenceSourceValue, ReferenceSourceMeta> = {
  task_requirements: {
    group: 'task',
    label: '任务需求文档',
    description: '当前任务的需求说明文档 (requirements)',
    documentType: 'requirements',
    requiresContext: 'task'
  },
  task_design: {
    group: 'task',
    label: '任务设计文档',
    description: '当前任务的设计稿/方案 (design)',
    documentType: 'tech_design',
    requiresContext: 'task'
  },
  task_test: {
    group: 'task',
    label: '任务测试文档',
    description: '当前任务的测试计划/用例 (test)',
    documentType: 'task',
    requiresContext: 'task'
  },
  project_feature_list: {
    group: 'project',
    label: '项目特性列表',
    description: '项目级《特性列表》交付物',
    documentType: 'feature_list'
  },
  project_architecture: {
    group: 'project',
    label: '项目架构设计',
    description: '项目级《架构设计》交付物',
    documentType: 'architecture'
  },
  meeting_details: {
    group: 'meeting',
    label: '会议详情记录',
    description: '会议转录润色稿 (polish_all)',
    documentType: 'meeting',
    requiresContext: 'meeting'
  },
  meeting_summary: {
    group: 'meeting',
    label: '会议总结',
    description: '会议总结文档 (meeting_summary)',
    documentType: 'meeting',
    requiresContext: 'meeting'
  }
};

const REFERENCE_GROUP_LABEL: Record<ReferenceSourceGroup, string> = {
  task: '任务文档',
  project: '项目文档',
  meeting: '会议内容'
};

// 根据设计文档要求，只允许用户手动创建 reference 类型的关系
// parent_child 和 sibling 关系由系统自动维护
const RELATION_TYPE_OPTIONS: { value: RelationType; label: string }[] = [
  { value: 'reference', label: '引用/依赖关系' }
];

const DEPENDENCY_TYPE_OPTIONS: { value: DependencyType; label: string }[] = [
  { value: 'data', label: '数据依赖' },
  { value: 'interface', label: '接口依赖' },
  { value: 'config', label: '配置依赖' }
];

const relationTypeLabelMap = RELATION_TYPE_OPTIONS.reduce<Record<RelationType, string>>((acc, item) => {
  acc[item.value] = item.label;
  return acc;
}, {
  parent_child: '父子关系',
  sibling: '兄弟关系',
  reference: '引用/依赖'
});

const dependencyTypeLabelMap = DEPENDENCY_TYPE_OPTIONS.reduce<Record<DependencyType, string>>((acc, item) => {
  acc[item.value] = item.label;
  return acc;
}, {
  data: '数据依赖',
  interface: '接口依赖',
  config: '配置依赖'
});

const dependencyTypeColorMap: Record<DependencyType, string> = {
  data: 'cyan',
  interface: 'volcano',
  config: 'gold'
};

// 图形组件类型定义（与RelationshipGraph组件匹配）
interface GraphNode {
  id: string;
  label: string;
  type: 'architecture' | 'tech_design' | 'requirements' | 'task' | 'meeting';
  x?: number;
  y?: number;
  style?: any;
}

interface GraphEdge {
  id: string;
  source: string;
  target: string;
  type: 'inherits' | 'implements' | 'references' | 'depends_on' | 'related_to';
  label?: string;
  style?: any;
}

type RelationshipTableRow = RelationshipDTO & {
  key: string;
  from_label: string;
  to_label: string;
};

type ConflictResolutionPayload = {
  type: 'merge' | 'accept_current' | 'accept_incoming' | 'manual';
  mergedContent: string;
  reason?: string;
};

const mapDocumentTypeToGraphType = (docType: DocumentType): GraphNode['type'] => {
  const typeMap: Record<DocumentType, GraphNode['type']> = {
    architecture: 'architecture',
    tech_design: 'tech_design',
    requirements: 'requirements',
    feature_list: 'requirements',
    background: 'requirements',
    meeting: 'meeting',
    task: 'task'
  };
  return typeMap[docType] ?? 'requirements';
};

const mapRelationshipToEdgeType = (relationship: RelationshipDTO): GraphEdge['type'] => {
  if (relationship.dependency_type) {
    return 'depends_on';
  }
  switch (relationship.type) {
    case 'parent_child':
      return 'inherits';
    case 'sibling':
      return 'related_to';
    case 'reference':
    default:
      return 'references';
  }
};

interface DocumentManagementSystemProps {
  projectId: string;
  taskId: string;
}

// 侧边栏标签页类型
type SidebarTab = 'structure' | 'search' | 'relations';

const DocumentManagementSystem: React.FC<DocumentManagementSystemProps> = ({
  projectId,
  taskId
}) => {
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<SidebarTab>('structure');
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [viewportHeight, setViewportHeight] = useState<number>(
    typeof window !== 'undefined' ? window.innerHeight : 900
  );
  
  // 新增抽屉和模态框状态
  const [historyDrawerVisible, setHistoryDrawerVisible] = useState(false);
  const [activeOverlay, setActiveOverlay] = useState<'conflict' | 'impact' | null>(null);
  
  // 搜索相关状态
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  
  // 版本管理状态
  const [diffModalVisible, setDiffModalVisible] = useState(false);
  const [diffContext, setDiffContext] = useState<{ from: number; to: number } | null>(null);
  const [versionHistoryRefreshKey, setVersionHistoryRefreshKey] = useState(0);
  
  // 文档类型选择器状态
  const [selectedDocTypes, setSelectedDocTypes] = useState<DocumentType[]>(['architecture', 'tech_design']);
  
  // Markdown编辑器状态
  const [markdownContent, setMarkdownContent] = useState('');
  const [currentDocumentId, setCurrentDocumentId] = useState<string | null>(null);
  const [documentVersion, setDocumentVersion] = useState(1);
  const [currentDocumentTitle, setCurrentDocumentTitle] = useState<string>('');
  
  // 树视图状态
  const [treeData, setTreeData] = useState<DocumentTreeNode[]>([]);
  const [selectedTreeKeys, setSelectedTreeKeys] = useState<string[]>([]);
  const [expandedTreeKeys, setExpandedTreeKeys] = useState<string[]>([]);
  const [nodeTitleMap, setNodeTitleMap] = useState<Record<string, string>>({});
  const mergeNodeTitles = useCallback((overrides: Record<string, string>) => {
    console.log('[DEBUG] mergeNodeTitles called with:', overrides);
    if (!overrides || Object.keys(overrides).length === 0) {
      console.log('[DEBUG] mergeNodeTitles: no overrides, returning');
      return;
    }

    setNodeTitleMap((prev) => {
      let changed = false;
      const next = { ...prev };

      Object.entries(overrides).forEach(([id, title]) => {
        if (!title) {
          return;
        }
        if (prev[id] !== title) {
          next[id] = title;
          changed = true;
        }
      });

      console.log('[DEBUG] mergeNodeTitles: changed =', changed, 'returning', changed ? 'new map' : 'same map');
      return changed ? next : prev;
    });
  }, [setNodeTitleMap]);
  
  // 引用管理状态  
  const [references, setReferences] = useState<Reference[]>([]);
  const [referencesLoading, setReferencesLoading] = useState(false);
  const [referenceContextOptions, setReferenceContextOptions] = useState<ReferenceContextOptionsMap>({});
  
  // 关系图状态
  const [graphNodes, setGraphNodes] = useState<GraphNode[]>([]);
  const [graphEdges, setGraphEdges] = useState<GraphEdge[]>([]);
  const [relationshipsLoading, setRelationshipsLoading] = useState(false);
  const [relationshipManagerLoading, setRelationshipManagerLoading] = useState(false);
  const [relationships, setRelationships] = useState<RelationshipDTO[]>([]);
  const [relationshipFilterDocId, setRelationshipFilterDocId] = useState<string | undefined>(undefined);
  const [relationshipSubmitting, setRelationshipSubmitting] = useState(false);
  const [relationshipModalVisible, setRelationshipModalVisible] = useState(false);
  const [relationshipForm] = Form.useForm<{ from_id: string; to_id: string; type: RelationType; dependency_type?: DependencyType; description?: string }>();
  const relationshipTypeValue = Form.useWatch('type', relationshipForm);

  // 任务关联状态
  const [linkTaskModalVisible, setLinkTaskModalVisible] = useState(false);
  const [linkingDocumentId, setLinkingDocumentId] = useState<string | null>(null);
  const [availableTasks, setAvailableTasks] = useState<Array<{ id: string; name: string; feature_name?: string; assignee?: string }>>([]);
  const [linkTaskForm] = Form.useForm<{ task_id: string; anchor?: string; context?: string }>();

  const referenceLoaders = useMemo<Partial<Record<ReferenceSourceValue, ReferenceContentLoader>>>(() => {
    const loaders: Partial<Record<ReferenceSourceValue, ReferenceContentLoader>> = {};

    if (projectId) {
      const ensureTaskId = (context?: ReferenceContextSelection): string => {
        if (context?.type === 'task' && context.id) {
          return context.id;
        }
        if (taskId) {
          return taskId;
        }
        throw new Error('缺少任务ID，无法加载引用文档');
      };

      loaders.task_requirements = async (context) => {
        const tasksApi = await import('../api/tasks');
        const targetTaskId = ensureTaskId(context);
        const doc = await tasksApi.getTaskDocument(projectId, targetTaskId, 'requirements');
        return { content: doc?.content ?? '', title: '任务需求文档' };
      };
      loaders.task_design = async (context) => {
        const tasksApi = await import('../api/tasks');
        const targetTaskId = ensureTaskId(context);
        const doc = await tasksApi.getTaskDocument(projectId, targetTaskId, 'design');
        return { content: doc?.content ?? '', title: '任务设计文档' };
      };
      loaders.task_test = async (context) => {
        const tasksApi = await import('../api/tasks');
        const targetTaskId = ensureTaskId(context);
        const doc = await tasksApi.getTaskDocument(projectId, targetTaskId, 'test');
        return { content: doc?.content ?? '', title: '任务测试文档' };
      };

      loaders.project_feature_list = async () => {
        const projectsApi = await import('../api/projects');
        const result = await projectsApi.getProjectFeatureList(projectId);
        return { content: result?.content ?? '', title: '项目特性列表' };
      };
      loaders.project_architecture = async () => {
        const projectsApi = await import('../api/projects');
        const result = await projectsApi.getProjectArchitecture(projectId);
        return { content: result?.content ?? '', title: '项目架构设计' };
      };
    }

    const ensureMeetingTaskId = (context?: ReferenceContextSelection): string => {
      if (context?.type === 'meeting' && context.id) {
        return context.id;
      }
      if (taskId) {
        return taskId;
      }
      throw new Error('缺少会议对应的任务ID，无法加载会议文档');
    };

    loaders.meeting_details = async (context) => {
      const authModule = await import('../api/auth');
      const sourceTaskId = ensureMeetingTaskId(context);
      const response = await authModule.authedApi.get(`/tasks/${sourceTaskId}/polish`);
      return { content: response?.data?.content ?? '', title: '会议详情' };
    };
    loaders.meeting_summary = async (context) => {
      const authModule = await import('../api/auth');
      const sourceTaskId = ensureMeetingTaskId(context);
      const response = await authModule.authedApi.get(`/tasks/${sourceTaskId}/meeting-summary`);
      return { content: response?.data?.content ?? '', title: '会议总结' };
    };

    return loaders;
  }, [projectId, taskId]);

  const referenceOptionGroups = useMemo<ReferenceOptionGroup[]>(() => {
    const groups: Record<ReferenceSourceGroup, ReferenceOptionGroup> = {
      task: { label: REFERENCE_GROUP_LABEL.task, options: [] },
      project: { label: REFERENCE_GROUP_LABEL.project, options: [] },
      meeting: { label: REFERENCE_GROUP_LABEL.meeting, options: [] }
    };

    const availableKeys = new Set(Object.keys(referenceLoaders) as ReferenceSourceValue[]);

    (Object.entries(REFERENCE_SOURCE_META) as Array<[ReferenceSourceValue, ReferenceSourceMeta]>).forEach(([value, meta]) => {
      groups[meta.group].options.push({
        value,
        label: meta.label,
        description: meta.description,
        documentType: meta.documentType,
        disabled: !availableKeys.has(value),
        contextType: meta.requiresContext
      });
    });

    return Object.values(groups).filter(group => group.options.length > 0);
  }, [referenceLoaders]);

  useEffect(() => {
    let cancelled = false;

    if (!projectId) {
      setReferenceContextOptions({});
      return;
    }

    const loadContextOptions = async () => {
      try {
        const projectsApi = await import('../api/projects');
        const authModule = await import('../api/auth');

        const [projectTasks, projectInfo] = await Promise.all([
          projectsApi.getProjectTasks(projectId),
          projectsApi.getProject(projectId).catch((error) => {
            console.warn('获取项目信息失败，将跳过产品线筛选:', error);
            return undefined;
          })
        ]);

        if (cancelled) {
          return;
        }

        const productLine = projectInfo?.product_line;
        if (!productLine) {
          message.warning('当前项目未设置产品线，会议列表将展示全部任务');
        }

        let meetingTasks: Array<{ id?: string; meeting_time?: string; state?: string; product_line?: string }> = [];
        try {
          const response = await authModule.authedApi.get('/tasks', {
            params: productLine ? { product_line: productLine } : undefined
          });
          const tasks = response?.data?.tasks;
          if (Array.isArray(tasks)) {
            meetingTasks = tasks;
          }
        } catch (error) {
          console.error('按产品线加载会议任务失败:', error);
          message.error('会议选项加载失败');
        }

        const taskOptions: ReferenceContextOption[] = (projectTasks || []).map(task => {
          const descriptionParts = [task?.feature_name, task?.assignee].filter(Boolean) as string[];
          return {
            value: task.id,
            label: task.name || task.id,
            description: descriptionParts.length ? descriptionParts.join(' · ') : undefined
          };
        });

        const meetingOptions: ReferenceContextOption[] = meetingTasks
          .filter(task => task?.id)
          .map(task => {
            const formattedMeetingTime = task.meeting_time && dayjs(task.meeting_time).isValid()
              ? dayjs(task.meeting_time).format('YYYY-MM-DD HH:mm')
              : undefined;
            const descriptionParts = [
              formattedMeetingTime ? `时间: ${formattedMeetingTime}` : undefined,
              task.state ? `状态: ${task.state}` : undefined,
              task.product_line ? `产品线: ${task.product_line}` : undefined
            ].filter(Boolean) as string[];
            return {
              value: task.id!,
              label: task.id!,
              description: descriptionParts.length ? descriptionParts.join(' · ') : undefined
            };
          });

        setReferenceContextOptions({
          task: taskOptions,
          meeting: meetingOptions
        });
      } catch (error) {
        if (!cancelled) {
          console.error('加载任务/会议列表失败:', error);
          message.error('加载任务与会议选项失败');
          setReferenceContextOptions({});
        }
      }
    };

    loadContextOptions();

    return () => {
      cancelled = true;
    };
  }, [projectId]);

  // 冲突解决和影响分析状态
  const [conflicts, setConflicts] = useState<ConflictItem[]>([]);
  const [conflictContext, setConflictContext] = useState<Record<string, { nodeId: string; serverVersion: number; baseVersion: number }>>({});
  const [conflictLoading, setConflictLoading] = useState(false);
  const [impactLoading, setImpactLoading] = useState(false);
  const [impactResults, setImpactResults] = useState<ImpactNodeResult[]>([]);

  // 模拟数据
  const mockSearchResults: SearchResult[] = [
    {
      nodeId: 'doc1',
      title: '系统架构设计',
      type: 'architecture',
      content: '本文档描述了系统的整体架构设计...',
      breadcrumbs: [projectId, taskId, '架构设计'],
      relevanceScore: 0.95
    }
  ];

  const mockReferences: Reference[] = [
    {
      id: 'ref1',
      task_id: taskId,
      document_id: 'doc-1',
      anchor: '2.1.3',
      context: '系统架构设计中的数据库层设计章节',
      status: 'active',
      version: 1,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    }
  ];

  const mockImpactResults: ImpactNodeResult[] = [
    {
      affected_node_id: 'doc2',
      title: '用户管理模块',
      description: '需要更新用户认证流程',
      impact_level: 'high',
      change_probability: 0.85,
      relationship_type: 'dependency'
    },
    {
      affected_node_id: 'doc3',
      title: 'API 文档',
      description: '需要更新接口定义',
      impact_level: 'medium',
      change_probability: 0.60,
      relationship_type: 'reference'
    }
  ];

  const applyTitleOverrides = (
    nodes: DocumentTreeNode[],
    overrides: Record<string, string>
  ): DocumentTreeNode[] =>
    nodes.map(node => ({
      ...node,
      title: overrides[node.id] ?? node.title,
      children: node.children ? applyTitleOverrides(node.children, overrides) : undefined
    }));

  const collectPlaceholderNodeIds = (nodes: DocumentTreeNode[]): string[] => {
    const ids: string[] = [];

    const traverse = (list: DocumentTreeNode[]) => {
      list.forEach(node => {
        const cached = nodeTitleMap[node.id];
        const titleToCheck = cached ?? node.title;
        if (!titleToCheck || PLACEHOLDER_TITLES.has(titleToCheck.trim())) {
          ids.push(node.id);
        }
        if (node.children && node.children.length > 0) {
          traverse(node.children);
        }
      });
    };

    traverse(nodes);
    return Array.from(new Set(ids));
  };

  const collectNonPlaceholderTitles = (nodes: DocumentTreeNode[]): Record<string, string> => {
    const titles: Record<string, string> = {};

    const traverse = (list: DocumentTreeNode[]) => {
      list.forEach(node => {
        if (node.title && !PLACEHOLDER_TITLES.has(node.title.trim())) {
          titles[node.id] = node.title;
        }
        if (node.children && node.children.length > 0) {
          traverse(node.children);
        }
      });
    };

    traverse(nodes);
    return titles;
  };

  const flattenTreeNodes = (nodes: DocumentTreeNode[]): DocumentTreeNode[] => {
    const result: DocumentTreeNode[] = [];

    const traverse = (list: DocumentTreeNode[]) => {
      list.forEach(node => {
        result.push(node);
        if (node.children && node.children.length > 0) {
          traverse(node.children);
        }
      });
    };

    traverse(nodes);
    return result;
  };

  const documentOptions = React.useMemo(
    () =>
      flattenTreeNodes(treeData).map((node) => ({
        value: node.id,
        label: nodeTitleMap[node.id] ?? node.title ?? node.id,
        type: node.type
      })),
    [treeData, nodeTitleMap]
  );

  const documentNodeMap = React.useMemo(() => {
    const entries: Record<string, DocumentTreeNode> = {};
    flattenTreeNodes(treeData).forEach((node) => {
      entries[node.id] = node;
    });
    return entries;
  }, [treeData]);

  const documentNodeMapRef = React.useRef(documentNodeMap);
  useEffect(() => {
    documentNodeMapRef.current = documentNodeMap;
  }, [documentNodeMap]);

  const nodeTitleMapRef = React.useRef(nodeTitleMap);
  useEffect(() => {
    nodeTitleMapRef.current = nodeTitleMap;
  }, [nodeTitleMap]);

  const relationshipNodeCacheRef = React.useRef<Record<string, { label: string; type: GraphNode['type'] }>>({});

  const resolveDocumentTitle = useCallback(
    (docId: string) => nodeTitleMap[docId] ?? documentNodeMap[docId]?.title ?? docId,
    [documentNodeMap, nodeTitleMap]
  );

  const filteredRelationships = React.useMemo(() => {
    if (!relationshipFilterDocId) {
      return relationships;
    }
    return relationships.filter(
      (rel) => rel.from_id === relationshipFilterDocId || rel.to_id === relationshipFilterDocId
    );
  }, [relationshipFilterDocId, relationships]);

  const relationshipTableData = React.useMemo(
    () =>
      filteredRelationships.map((rel) => ({
        key: rel.id,
        ...rel,
        from_label: resolveDocumentTitle(rel.from_id),
        to_label: resolveDocumentTitle(rel.to_id)
      })),
    [filteredRelationships, resolveDocumentTitle]
  );

  const loadTreeData = async () => {
    setLoading(true);
    try {
      const { documentsAPI } = await import('../api/documents');
      const res = await documentsAPI.getTree(projectId, undefined, 10); // 获取所有层级
      // 递归转换为DocumentTreeNode[]
      const convert = (dto: DocumentTreeDTO): DocumentTreeNode => ({
        ...dto.node,
        children: dto.children ? dto.children.map(convert) : undefined
      });

      const normalizeTree = (tree: DocumentTreeDTO | DocumentTreeDTO[] | undefined): DocumentTreeDTO[] => {
        if (!tree) {
          return [];
        }
        const base = Array.isArray(tree) ? tree : [tree];
        if (base.length === 1 && base[0]?.node?.id === 'virtual_root') {
          return base[0].children ?? [];
        }
        return base;
      };

      // 处理后端返回的数据结构
      let nodes: DocumentTreeNode[] = normalizeTree(res.tree).map(convert);

      if (Object.keys(nodeTitleMap).length) {
        nodes = applyTitleOverrides(nodes, nodeTitleMap);
      }

      const existingTitles = collectNonPlaceholderTitles(nodes);
      if (Object.keys(existingTitles).length) {
        nodes = applyTitleOverrides(nodes, existingTitles);
      }

      const placeholderIds = collectPlaceholderNodeIds(nodes).filter(
        (id) => !existingTitles[id]
      );

      let fetchedOverrides: Record<string, string> = {};
      if (placeholderIds.length > 0) {
        const { documentsAPI } = await import('../api/documents');
        const results = await Promise.all(
          placeholderIds.map(async (id) => {
            // 防止使用taskId作为文档ID
            if (id.startsWith('task_')) {
              console.warn('[WARN] placeholderIds中包含taskId，已跳过:', id);
              return null;
            }
            try {
              const result = await documentsAPI.getContent(projectId, id);
              const metaTitle = result.meta?.title?.trim();
              if (metaTitle && !PLACEHOLDER_TITLES.has(metaTitle)) {
                return { id, title: metaTitle };
              }
            } catch (error) {
              console.error('获取节点标题失败:', error);
            }
            return null;
          })
        );

        fetchedOverrides = results.reduce<Record<string, string>>((acc, item) => {
          if (item && item.title) {
            acc[item.id] = item.title;
          }
          return acc;
        }, {});

        if (Object.keys(fetchedOverrides).length) {
          nodes = applyTitleOverrides(nodes, fetchedOverrides);
        }
      }

      setTreeData(nodes);
      // 默认展开前3层节点
      if (nodes.length > 0) {
        setExpandedTreeKeys((prev) => {
          if (prev.length > 0) return prev;
          
          const keysToExpand: string[] = [];
          const collectKeys = (nodeList: DocumentTreeNode[]) => {
            nodeList.forEach(node => {
              // 如果节点有子节点，且节点层级小于等于3，就将其加入展开列表
              if (node.children && node.children.length > 0 && node.level <= 3) {
                keysToExpand.push(node.id);
              }
              // 递归处理子节点
              if (node.children) {
                collectKeys(node.children);
              }
            });
          };
          
          collectKeys(nodes); // 展开所有有子节点的前3层节点
          return keysToExpand;
        });
      }

      const combinedTitles = { ...existingTitles, ...fetchedOverrides };
      if (Object.keys(combinedTitles).length) {
        mergeNodeTitles(combinedTitles);
      }
    } catch (e: any) {
      message.error('加载文档树失败: ' + (e?.message || e));
      console.error('Tree loading error:', e);
    } finally {
      setLoading(false);
    }
  };

  // 组件挂载和projectId变化时加载文档树数据
  useEffect(() => {
    loadTreeData();
  }, [projectId]);

  // 当切换到包含文档树的页面时，检查并重新加载数据
  useEffect(() => {
    if (activeTab === 'structure' && treeData.length === 0) {
      console.log('切换到structure页面，树数据为空，重新加载...');
      loadTreeData();
    }
  }, [activeTab, treeData.length]);

  // 控制全屏时禁止背景滚动
  useEffect(() => {
    const originalOverflow = document.body.style.overflow;
    if (isFullscreen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = originalOverflow;
    }

    return () => {
      document.body.style.overflow = originalOverflow;
    };
  }, [isFullscreen]);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    const handleResize = () => setViewportHeight(window.innerHeight);
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
    };
  }, []);

  const editorHeight = React.useMemo(() => {
    const safeHeight = viewportHeight - (isFullscreen ? 160 : 280);
    const maxHeight = isFullscreen ? viewportHeight - 120 : 520;
    return Math.max(320, Math.min(safeHeight, maxHeight));
  }, [viewportHeight, isFullscreen]);

  // 加载引用数据
  const loadReferences = useCallback(
    async (options?: { documentId?: string; fallbackToTask?: boolean }) => {
      const targetDocumentId = options?.documentId ?? currentDocumentId ?? undefined;
      const fallbackToTask = options?.fallbackToTask ?? true;

      if (!targetDocumentId && !fallbackToTask) {
        return;
      }

      setReferencesLoading(true);
      try {
        const { documentsAPI } = await import('../api/documents');
        let loadedReferences: Reference[] | null | undefined = [];

        if (targetDocumentId) {
          const result = await documentsAPI.getDocumentReferences(projectId, targetDocumentId);
          loadedReferences = result?.references;
        } else if (fallbackToTask) {
          const result = await documentsAPI.getTaskReferences(projectId, taskId);
          loadedReferences = result?.references;
        }

        setReferences(Array.isArray(loadedReferences) ? loadedReferences : []);
      } catch (error) {
        console.error('加载引用失败:', error);
        message.error('加载引用失败，请稍后重试');
        setReferences([]);
      } finally {
        setReferencesLoading(false);
      }
    },
    [currentDocumentId, projectId, taskId]
  );

  const handleAddReferenceEntry = async (
    referenceData: Omit<Reference, 'id' | 'created_at' | 'updated_at'>
  ) => {
    try {
      const { documentsAPI } = await import('../api/documents');
      const { task_id, document_id, anchor, context } = referenceData;
      await documentsAPI.createReference(projectId, {
        task_id,
        document_id,
        ...(anchor && anchor.trim() ? { anchor: anchor.trim() } : {}),
        ...(context && context.trim() ? { context: context.trim() } : {})
      });
      await loadReferences({ documentId: referenceData.document_id, fallbackToTask: false });
    } catch (error) {
      console.error('添加引用失败:', error);
      message.error('添加引用失败,请稍后重试');
    }
  };

  const handleDeleteReference = async (referenceId: string) => {
    try {
      const { documentsAPI } = await import('../api/documents');
      await documentsAPI.deleteReference(projectId, referenceId);
      
      // 重新加载引用列表
      if (relationshipFilterDocId) {
        await loadReferences({ documentId: relationshipFilterDocId, fallbackToTask: false });
      } else if (currentDocumentId) {
        await loadReferences({ documentId: currentDocumentId, fallbackToTask: false });
      }
      
      message.success('删除引用成功');
    } catch (error) {
      console.error('删除引用失败:', error);
      message.error('删除引用失败,请稍后重试');
    }
  };

  // 添加到MCP资源
  const handleAddToResource = async (nodeId: string) => {
    try {
      const { documentsAPI } = await import('../api/documents');
      const authModule = await import('../api/auth');
      
      // 获取当前用户信息
      const auth = authModule.loadAuth();
      if (!auth) {
        message.error('请先登录');
        return;
      }

      // 获取文档内容
      const { meta, content } = await documentsAPI.getContent(projectId, nodeId);
      
      // 创建MCP资源
      await addCustomResource(auth.username, {
        name: meta.title || `文档资源 - ${nodeId}`,
        description: `从项目文档 "${meta.title}" 创建的MCP资源`,
        content: content,
        visibility: 'private',
        projectId: projectId,
        taskId: taskId
      });

      message.success('成功添加到MCP资源');
    } catch (error) {
      console.error('添加到MCP资源失败:', error);
      message.error('添加到MCP资源失败，请稍后重试');
    }
  };

  // 关联任务
  const handleLinkToTask = async (nodeId: string) => {
    try {
      setLinkingDocumentId(nodeId);
      
      // 加载项目下的任务列表
      const projectsApi = await import('../api/projects');
      const tasks = await projectsApi.getProjectTasks(projectId);
      setAvailableTasks(tasks || []);
      
      // 打开任务选择Modal
      setLinkTaskModalVisible(true);
      linkTaskForm.resetFields();
    } catch (error) {
      console.error('加载任务列表失败:', error);
      message.error('加载任务列表失败，请稍后重试');
    }
  };

  // 提交任务关联
  const handleLinkTaskSubmit = async () => {
    if (!linkingDocumentId) {
      message.error('文档ID缺失');
      return;
    }

    try {
      const values = await linkTaskForm.validateFields();
      
      // 创建引用关联
      const { documentsAPI } = await import('../api/documents');
      await documentsAPI.createReference(projectId, {
        task_id: values.task_id,
        document_id: linkingDocumentId,
        ...(values.anchor && values.anchor.trim() ? { anchor: values.anchor.trim() } : {}),
        ...(values.context && values.context.trim() ? { context: values.context.trim() } : {})
      });

      message.success('任务关联成功');
      setLinkTaskModalVisible(false);
      linkTaskForm.resetFields();
      
      // 刷新引用列表
      await loadReferences({ documentId: linkingDocumentId, fallbackToTask: false });
    } catch (error) {
      console.error('关联任务失败:', error);
      message.error('关联任务失败，请稍后重试');
    }
  };

  // 当选中文档变化时，加载对应的引用
  useEffect(() => {
    if (
      currentDocumentId &&
      (!relationshipFilterDocId || relationshipFilterDocId === currentDocumentId)
    ) {
      loadReferences({ documentId: currentDocumentId, fallbackToTask: false });
    }
  }, [currentDocumentId, relationshipFilterDocId, loadReferences]);

  useEffect(() => {
    if (
      relationshipFilterDocId &&
      relationshipFilterDocId !== currentDocumentId
    ) {
      loadReferences({ documentId: relationshipFilterDocId, fallbackToTask: false });
    }
  }, [relationshipFilterDocId, currentDocumentId, loadReferences]);

  useEffect(() => {
    if (!currentDocumentId) {
      return;
    }
    setRelationshipFilterDocId((prev) => prev ?? currentDocumentId);
    const existingFrom = relationshipForm.getFieldValue('from_id');
    if (!existingFrom) {
      relationshipForm.setFieldsValue({ from_id: currentDocumentId });
    }
  }, [currentDocumentId, relationshipForm]);

  // 移除关系类型变化的 useEffect，因为现在只支持 reference 类型

  useEffect(() => {
    if (activeTab === 'relations') {
      const targetDocumentId = relationshipFilterDocId ?? currentDocumentId ?? undefined;
      loadReferences({ documentId: targetDocumentId, fallbackToTask: true });
    }
  }, [activeTab, currentDocumentId, relationshipFilterDocId, loadReferences]);

  const hydrateGraphFromRelationships = useCallback(
    async (relationList: RelationshipDTO[]) => {
      if (!relationList.length) {
        setGraphNodes([]);
        setGraphEdges([]);
        return;
      }

      const currentNodeMap = documentNodeMapRef.current;
      const currentTitleMap = nodeTitleMapRef.current;

      const nodeIds = new Set<string>();
      const edges: GraphEdge[] = relationList.map((rel) => {
        // 防止添加taskId到节点集合中
        if (!rel.from_id.startsWith('task_')) {
          nodeIds.add(rel.from_id);
        }
        if (!rel.to_id.startsWith('task_')) {
          nodeIds.add(rel.to_id);
        }
        return {
          id: rel.id,
          source: rel.from_id,
          target: rel.to_id,
          type: mapRelationshipToEdgeType(rel),
          label:
            rel.description ||
            (rel.dependency_type
              ? dependencyTypeLabelMap[rel.dependency_type]
              : relationTypeLabelMap[rel.type])
        };
      });

      const metadataMap = new Map<string, { label: string; type: GraphNode['type'] }>();
      Array.from(nodeIds).forEach((nodeId) => {
        const node = currentNodeMap[nodeId];
        if (node) {
          metadataMap.set(nodeId, {
            label: currentTitleMap[nodeId] ?? node.title ?? nodeId,
            type: mapDocumentTypeToGraphType(node.type)
          });
        } else {
          const cachedMeta = relationshipNodeCacheRef.current[nodeId];
          if (cachedMeta) {
            metadataMap.set(nodeId, cachedMeta);
          } else if (currentTitleMap[nodeId]) {
            metadataMap.set(nodeId, {
              label: currentTitleMap[nodeId],
              type: 'requirements'
            });
          }
        }
      });

      const missingNodeIds = Array.from(nodeIds).filter((id) => !metadataMap.has(id));
      const fetchedOverrides: Record<string, string> = {};

      if (missingNodeIds.length) {
        const { documentsAPI } = await import('../api/documents');
        await Promise.all(
          missingNodeIds.map(async (nodeId) => {
            // 防止使用taskId作为文档ID
            if (nodeId.startsWith('task_')) {
              console.warn('[WARN] missingNodeIds中包含taskId，已跳过:', nodeId);
              return;
            }
            try {
              const result = await documentsAPI.getContent(projectId, nodeId);
              const metaTitle = result.meta?.title?.trim();
              const metaType = (result.meta?.type ?? 'requirements') as DocumentType;
              const label = metaTitle || `Document ${nodeId}`;
              metadataMap.set(nodeId, {
                label,
                type: mapDocumentTypeToGraphType(metaType)
              });
              if (metaTitle) {
                fetchedOverrides[nodeId] = metaTitle;
              }
              relationshipNodeCacheRef.current[nodeId] = {
                label,
                type: mapDocumentTypeToGraphType(metaType)
              };
            } catch (error) {
              const fallbackMeta = {
                label: `Document ${nodeId}`,
                type: 'requirements'
              } as const;
              metadataMap.set(nodeId, fallbackMeta);
              relationshipNodeCacheRef.current[nodeId] = fallbackMeta;
            }
          })
        );
      }

      if (Object.keys(fetchedOverrides).length) {
        mergeNodeTitles(fetchedOverrides);
      }

      const nodes: GraphNode[] = Array.from(metadataMap.entries()).map(([id, meta]) => ({
        id,
        label: meta.label,
        type: meta.type
      }));

      setGraphNodes(nodes);
      setGraphEdges(edges);
    },
    [projectId, mergeNodeTitles]
  );

  const lastRelationshipLoadTime = useRef<number>(0);
  const loadRelationshipData = useCallback(async () => {
    const now = Date.now();
    if (now - lastRelationshipLoadTime.current < 100) { // Prevent calls within 100ms
      console.log('[DEBUG] loadRelationshipData throttled, last call was', now - lastRelationshipLoadTime.current, 'ms ago');
      return;
    }
    lastRelationshipLoadTime.current = now;
    console.log('[DEBUG] loadRelationshipData called, stack:', new Error().stack?.split('\n').slice(1, 4));
    setRelationshipsLoading(true);
    setRelationshipManagerLoading(true);
    try {
      const { documentsAPI } = await import('../api/documents');
      const result = await documentsAPI.getRelationships(projectId);
      const relationList = Array.isArray(result.relationships) ? result.relationships : [];
      setRelationships(relationList);
      await hydrateGraphFromRelationships(relationList);
    } catch (error: any) {
      console.error('加载关系数据失败:', error);
      message.error('加载关系数据失败，请稍后重试');
      setRelationships([]);
      setGraphNodes([
        { id: projectId, label: `项目 ${projectId}`, type: 'task' }
        // 移除 taskId 节点，避免尝试获取其文档内容
      ]);
      setGraphEdges([]);
    } finally {
      setRelationshipsLoading(false);
      setRelationshipManagerLoading(false);
    }
  }, [hydrateGraphFromRelationships, projectId, taskId]);

  const handleCreateRelationship = useCallback(
    async (values: { from_id: string; to_id: string; type: RelationType; dependency_type?: DependencyType; description?: string }) => {
      if (values.from_id === values.to_id) {
        message.error('起始文档与目标文档不能相同');
        return;
      }

      // 由于现在只允许创建 reference 类型的关系，确保依赖类型必须存在
      if (!values.dependency_type) {
        message.error('引用关系必须指定依赖类型（数据、接口或配置）');
        return;
      }

      setRelationshipSubmitting(true);
      try {
        const payload: CreateRelationshipRequest = {
          from_id: values.from_id,
          to_id: values.to_id,
          type: 'reference', // 强制设为 reference 类型
          dependency_type: values.dependency_type,
          description: values.description?.trim() || undefined
        };
        const { documentsAPI } = await import('../api/documents');
        await documentsAPI.createRelationship(projectId, payload);
        message.success('文档关系已创建');
        await loadRelationshipData();
        relationshipForm.resetFields();
        setRelationshipModalVisible(false);
        setRelationshipFilterDocId((prev) => prev ?? values.from_id);
      } catch (error: any) {
        const backendMessage =
          error?.response?.data?.message ||
          error?.response?.data?.error ||
          error?.message;
        message.error(backendMessage ? `创建关系失败：${backendMessage}` : '创建关系失败，请稍后重试');
      } finally {
        setRelationshipSubmitting(false);
      }
    },
    [loadRelationshipData, projectId, relationshipForm]
  );

  const handleRemoveRelationship = useCallback(
    async (relationship: RelationshipDTO) => {
      try {
        const { documentsAPI } = await import('../api/documents');
        await documentsAPI.removeRelationship(projectId, relationship.from_id, relationship.to_id);
        message.success('关系已删除');
        await loadRelationshipData();
      } catch (error: any) {
        const backendMessage =
          error?.response?.data?.message ||
          error?.response?.data?.error ||
          error?.message;
        message.error(backendMessage ? `删除关系失败：${backendMessage}` : '删除关系失败，请稍后重试');
      }
    },
    [loadRelationshipData, projectId]
  );

  // 关系Modal处理函数
  const handleOpenRelationshipModal = useCallback(() => {
    relationshipForm.setFieldsValue({
      from_id: currentDocumentId ?? relationshipFilterDocId,
      type: 'reference'
    });
    setRelationshipModalVisible(true);
  }, [currentDocumentId, relationshipFilterDocId, relationshipForm]);

  const handleCloseRelationshipModal = useCallback(() => {
    setRelationshipModalVisible(false);
    relationshipForm.resetFields();
  }, [relationshipForm]);

  const relationshipColumns = React.useMemo<ColumnsType<RelationshipTableRow>>(
    () => [
      {
        title: '起始文档',
        dataIndex: 'from_label',
        key: 'from_label',
        ellipsis: true,
        width: 160
      },
      {
        title: '目标文档',
        dataIndex: 'to_label',
        key: 'to_label',
        ellipsis: true,
        width: 180,
        render: (value: string) => (
          <span style={{ display: 'inline-block', maxWidth: '100%' }} title={value}>
            {value}
          </span>
        )
      },
      {
        title: '依赖类型',
        dataIndex: 'dependency_type',
        key: 'dependency_type',
        width: 110,
        render: (value: RelationshipTableRow['dependency_type']) =>
          value ? <Tag color={dependencyTypeColorMap[value]}>{dependencyTypeLabelMap[value]}</Tag> : '—'
      },
      {
        title: '说明',
        dataIndex: 'description',
        key: 'description',
        ellipsis: true,
        width: 220,
        render: (value: string | undefined) => value || '—'
      },
      {
        title: '操作',
        key: 'actions',
        width: 100,
        render: (_value, record) => (
          <Popconfirm
            title="确认删除该关系吗？"
            okText="删除"
            cancelText="取消"
            onConfirm={() => handleRemoveRelationship(record)}
          >
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        )
      }
    ],
    [handleRemoveRelationship]
  );


  useEffect(() => {
    console.log('[DEBUG] useEffect triggering loadRelationshipData, deps changed');
    loadRelationshipData();
  }, [loadRelationshipData]);

  // 解决冲突
  const handleResolveConflict = async (conflictId: string, resolution: ConflictResolutionPayload) => {
    const context = conflictContext[conflictId];
    if (!context) {
      message.error('无法获取冲突上下文，请重新加载后再试');
      return;
    }

    const { nodeId, serverVersion } = context;

    try {
      setConflictLoading(true);
      const { documentsAPI } = await import('../api/documents');

      const updateResult = await documentsAPI.updateContent(projectId, nodeId, {
        content: resolution.mergedContent,
        version: serverVersion
      });

      const refreshed = await documentsAPI.getContent(projectId, nodeId);
      let resolvedVersion = refreshed.meta?.version ?? updateResult.version ?? serverVersion;

      try {
        const history = await documentsAPI.getVersionHistory(projectId, nodeId);
        if (Array.isArray(history?.versions) && history.versions.length > 0) {
          resolvedVersion = Math.max(...history.versions.map((item) => item.version));
          if (nodeId === currentDocumentId) {
            setVersionHistoryRefreshKey((prev) => prev + 1);
          }
        }
      } catch (historyError) {
        console.warn('刷新版本历史失败:', historyError);
      }

      if (nodeId === currentDocumentId) {
        setMarkdownContent(refreshed.content);
        setDocumentVersion(resolvedVersion);

        const metaTitle = refreshed.meta?.title?.trim() ?? '';
        if (metaTitle) {
          setCurrentDocumentTitle(metaTitle);
          mergeNodeTitles({ [nodeId]: metaTitle });
        }
      } else {
        const metaTitle = refreshed.meta?.title?.trim();
        if (metaTitle) {
          mergeNodeTitles({ [nodeId]: metaTitle });
        }
      }

      setConflicts((prev) => {
        const updated = prev.map((conflict) =>
          conflict.id === conflictId
            ? { ...conflict, status: 'resolved' as const, updatedAt: new Date().toISOString() }
            : conflict
        );

        const unresolvedCount = updated.filter((conflict) => conflict.status === 'unresolved').length;
        if (unresolvedCount === 0) {
          setActiveOverlay((current) => (current === 'conflict' ? null : current));
        }

        return updated;
      });

      setConflictContext((prev) => {
        const next = { ...prev };
        delete next[conflictId];
        return next;
      });

      message.success('冲突已解决');
    } catch (error: any) {
      console.error('解决冲突失败:', error);
      if (error?.response?.data?.code === 'VERSION_MISMATCH' || error?.message?.includes('VERSION_MISMATCH')) {
        message.warning('冲突解决时检测到新的版本更新，已刷新冲突详情');
        await openConflictResolver(nodeId, serverVersion, resolution.mergedContent);
      } else {
        message.error('解决冲突失败，请稍后重试');
      }
    } finally {
      setConflictLoading(false);
    }
  };

  const convertImpactResponse = useCallback(
    (impact: ImpactAnalyzerResult | undefined): ImpactNodeResult[] => {
      if (!impact) {
        return [];
      }

      const seen = new Set<string>();
      const results: ImpactNodeResult[] = [];

      const determineMetrics = (
        relationship: ImpactNodeResult['relationship_type'],
        depth?: number
      ): { level: ImpactNodeResult['impact_level']; probability: number } => {
        const normalizedDepth = typeof depth === 'number' && depth >= 0 ? depth : 0;

        let level: ImpactNodeResult['impact_level'];
        switch (relationship) {
          case 'child':
            level = normalizedDepth > 1 ? 'medium' : 'high';
            break;
          case 'parent':
            level = normalizedDepth > 1 ? 'low' : 'medium';
            break;
          case 'reference':
            level = normalizedDepth > 1 ? 'low' : 'medium';
            break;
          case 'dependency':
          default:
            level = normalizedDepth > 0 ? 'medium' : 'low';
            break;
        }

        const baseProbability =
          relationship === 'child'
            ? 0.85
            : relationship === 'parent'
            ? 0.65
            : relationship === 'reference'
            ? 0.55
            : 0.5;

        const probability = Math.min(
          0.95,
          Math.max(0.2, baseProbability - normalizedDepth * 0.1)
        );

        return { level, probability };
      };

      const buildDescription = (
        relationship: ImpactNodeResult['relationship_type'],
        depth?: number
      ) => {
        const depthText = typeof depth === 'number' ? `（传播层级 ${depth + 1}）` : '';
        switch (relationship) {
          case 'child':
            return `当前文档的改动可能直接影响到下游实现${depthText}`;
          case 'parent':
            return `上游结构可能需要同步调整以保持一致${depthText}`;
          case 'reference':
            return `存在引用或交叉引用关系，建议同步核对描述${depthText}`;
          case 'dependency':
          default:
            return `文档之间存在依赖约束，建议复核接口或数据约束${depthText}`;
        }
      };

      const pushResult = (
        id: string,
        relationship: ImpactNodeResult['relationship_type']
      ) => {
        if (!id) {
          return;
        }
        if (id.startsWith('task_')) {
          console.warn('[WARN] 影响分析结果包含任务ID，已跳过:', id);
          return;
        }

        const key = `${relationship}:${id}`;
        if (seen.has(key)) {
          return;
        }

        const depth = impact.depth?.[id];
        const { level, probability } = determineMetrics(relationship, depth);
        const description = buildDescription(relationship, depth);

        results.push({
          affected_node_id: id,
          title: resolveDocumentTitle(id),
          description,
          impact_level: level,
          change_probability: Number(probability.toFixed(2)),
          relationship_type: relationship
        });

        seen.add(key);
      };

      impact.parents?.forEach((parentId) => pushResult(parentId, 'parent'));
      impact.children?.forEach((childId) => pushResult(childId, 'child'));
      impact.references?.forEach((refId) => pushResult(refId, 'reference'));
      impact.dependencies?.forEach((depId) => pushResult(depId, 'dependency'));

      return results.sort(
        (a, b) => (b.change_probability ?? 0) - (a.change_probability ?? 0)
      );
    },
    [resolveDocumentTitle]
  );

  // 执行影响分析
  const handleImpactAnalysis = async (nodeId: string, mode?: AnalysisMode) => {
    if (!nodeId) {
      message.warning('请选择需要分析的文档');
      return;
    }

    let modes: string[] | undefined;
    if (mode) {
      switch (mode) {
        case 'upstream':
          modes = ['parents', 'references'];
          break;
        case 'downstream':
          modes = ['children', 'dependencies'];
          break;
        case 'bidirectional':
        default:
          modes = ['parents', 'children', 'references', 'dependencies'];
          break;
      }
    }

    setImpactLoading(true);
    try {
      const { default: documentsAPI } = await import('../api/documents');
      const result = await documentsAPI.analyzeImpact(projectId, nodeId, modes);
      const normalized = convertImpactResponse(result?.impact);
      setImpactResults(normalized);
      if (!normalized.length) {
        console.info('[INFO] 影响分析未发现其他受影响节点');
      }
    } catch (error) {
      console.error('影响分析失败:', error);
      message.error('影响分析失败，请稍后重试');
      setImpactResults(mockImpactResults);
    } finally {
      setImpactLoading(false);
    }
  };

  const openConflictResolver = useCallback(
    async (nodeId: string, baseVersion: number, incomingContent: string) => {
      setConflictLoading(true);
      try {
        const { documentsAPI } = await import('../api/documents');
        const [serverResult, baseSnapshot] = await Promise.all([
          documentsAPI.getContent(projectId, nodeId),
          documentsAPI
            .getVersionContent(projectId, nodeId, baseVersion)
            .catch(() => ({ version: baseVersion, content: '' }))
        ]);

        const serverVersion = serverResult.meta?.version ?? baseVersion;
        const baseContent = baseSnapshot?.content || serverResult.content;
        const conflictId = `${nodeId}-v${baseVersion}-to-v${serverVersion}`;
        const title = resolveDocumentTitle(nodeId) ?? serverResult.meta?.title ?? nodeId;

        const conflictItem: ConflictItem = {
          id: conflictId,
          type: 'version',
          nodeId,
          title,
          description: `检测到版本冲突：服务器最新版本为 v${serverVersion}，当前编辑基于 v${baseVersion}。请选择合适的处理方式。`,
          severity: 'high',
          status: 'unresolved',
          conflictData: {
            baseVersion,
            branchVersions: [serverVersion],
            conflictContent: {
              base: baseContent,
              current: serverResult.content,
              incoming: incomingContent
            }
          },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString()
        };

        setConflicts([conflictItem]);
        setConflictContext((prev) => ({
          ...prev,
          [conflictId]: {
            nodeId,
            serverVersion,
            baseVersion
          }
        }));
        setActiveOverlay('conflict');
      } catch (error) {
        console.error('加载冲突详情失败:', error);
        message.error('加载冲突详情失败，请稍后重试');
      } finally {
        setConflictLoading(false);
      }
    },
    [projectId, resolveDocumentTitle]
  );

  const renderDocumentOverlay = () => {
    if (!activeOverlay) {
      return null;
    }

    const closeOverlay = () => setActiveOverlay(null);

    if (activeOverlay === 'conflict') {
      const unresolvedCount = conflicts.filter((conflict) => conflict.status === 'unresolved').length;
      return (
        <div style={overlayWrapperStyle}>
          <div style={overlayPanelStyle}>
            <div style={overlayHeaderStyle}>
              <Space size={8} align="center">
                <ExclamationCircleOutlined style={{ color: '#fa541c' }} />
                <Title level={5} style={{ margin: 0 }}>
                  冲突解决器
                </Title>
                <Tag color={unresolvedCount ? 'red' : 'green'}>
                  {unresolvedCount ? `待解决 ${unresolvedCount}` : '全部解决'}
                </Tag>
              </Space>
              <Space>
                <Button
                  size="small"
                  icon={<CloseOutlined />}
                  onClick={closeOverlay}
                >
                  返回编辑
                </Button>
              </Space>
            </div>
            <div style={overlayBodyStyle}>
              <ConflictResolver
                projectId={projectId}
                conflicts={conflicts}
                loading={conflictLoading}
                onViewConflictDetail={handleNodeSelect}
                onResolveConflict={handleResolveConflict}
              />
            </div>
          </div>
        </div>
      );
    }

    const handleManualAnalyze = () => {
      if (currentDocumentId) {
        handleImpactAnalysis(currentDocumentId);
      }
    };

    return (
      <div style={overlayWrapperStyle}>
        <div style={overlayPanelStyle}>
          <div style={overlayHeaderStyle}>
            <Space size={8} align="center">
              <BarChartOutlined style={{ color: '#2f54eb' }} />
              <Title level={5} style={{ margin: 0 }}>
                影响分析
              </Title>
              {(currentDocumentTitle || currentDocumentId) && (
                <Tag color="blue">
                  {currentDocumentTitle || resolveDocumentTitle(currentDocumentId || '')}
                </Tag>
              )}
            </Space>
            <Space>
              <Button
                size="small"
                onClick={handleManualAnalyze}
                loading={impactLoading}
              >
                重新分析
              </Button>
              <Button
                size="small"
                icon={<CloseOutlined />}
                onClick={closeOverlay}
              >
                返回编辑
              </Button>
            </Space>
          </div>
          <div style={overlayBodyStyle}>
            <ImpactAnalysisPanel
              projectId={projectId}
              nodeId={currentDocumentId || ''}
              analysisMode="bidirectional"
              impactResults={impactResults}
              loading={impactLoading}
              onAnalyze={handleImpactAnalysis}
              onNodeSelect={handleNodeSelect}
            />
          </div>
        </div>
      </div>
    );
  };

  useEffect(() => {
    setActiveOverlay(null);
  }, [currentDocumentId]);

  // 初始化时执行影响分析（如果有选中的文档）
  useEffect(() => {
    if (currentDocumentId) {
      handleImpactAnalysis(currentDocumentId);
    }
  }, [currentDocumentId]);

  // 事件处理函数
  const handleSearch = async (query: string) => {
    console.log('搜索查询:', query);
    setSearchLoading(true);
    
    try {
      const { default: documentsAPI } = await import('../api/documents');
      const searchResponse = await documentsAPI.searchDocuments(projectId, {
        query,
        max_results: 50,
        context_chars: 150
      });
      
      // 将API结果转换为前端期望的格式
      const convertedResults: SearchResult[] = searchResponse.results.map(result => ({
        nodeId: result.document_id,
        title: result.title,
        type: result.metadata?.type || 'feature_list' as DocumentType,
        content: result.content,
        breadcrumbs: result.metadata?.breadcrumbs || [projectId, taskId],
        relevanceScore: result.score
      }));
      
      setSearchResults(convertedResults);
      message.success(`找到 ${convertedResults.length} 个相关文档`);
    } catch (error) {
      console.error('搜索失败:', error);
      setSearchResults([]);
      message.error('搜索失败，请稍后重试');
    } finally {
      setSearchLoading(false);
    }
  };

  const findNodePath = (
    nodes: DocumentTreeNode[],
    targetId: string,
    path: string[] = []
  ): string[] | null => {
    for (const node of nodes) {
      const currentPath = [...path, node.id];
      if (node.id === targetId) {
        return currentPath;
      }
      if (node.children && node.children.length > 0) {
        const childPath = findNodePath(node.children, targetId, currentPath);
        if (childPath) {
          return childPath;
        }
      }
    }
    return null;
  };

  const handleNodeSelect = (nodeId: string) => {
    if (!nodeId) {
      return;
    }
    
    // 防止 taskId 被错误地当作文档ID使用
    if (nodeId.startsWith('task_')) {
      console.warn('[WARN] 试图选择 taskId 作为文档节点，已阻止:', nodeId);
      message.warning(`任务ID ${nodeId} 不能直接作为文档编辑，请选择具体的文档节点`);
      return;
    }
    
    setActiveTab('structure');

    const path = findNodePath(treeData, nodeId);
    if (path && path.length > 1) {
      const ancestors = path.slice(0, -1);
      setExpandedTreeKeys((prev) => {
        const next = new Set(prev);
        ancestors.forEach((id) => next.add(id));
        return Array.from(next);
      });
    }

    console.log('选择文档节点:', nodeId);
    setSelectedTreeKeys([nodeId]);
    setRelationshipFilterDocId(nodeId);
    relationshipForm.setFieldsValue({ from_id: nodeId });
    loadDocumentContent(nodeId);
  };

  // 添加节点
  const handleAddNode = async (parentId: string, payload: AddNodePayload): Promise<void> => {
    try {
  const { default: documentsAPI } = await import('../api/documents');
  const { title, type, referenceSource, referenceContext } = payload;
      
      // 为新节点生成默认标题和内容
      const defaultTitles: Record<DocumentType, string> = {
        'feature_list': '新特性列表',
        'architecture': '新架构设计',
        'tech_design': '新技术方案',
        'background': '新背景资料',
        'requirements': '新需求文档',
        'meeting': '新会议纪要',
        'task': '新任务文档'
      };

      const defaultContents: Record<DocumentType, string> = {
        'feature_list': '# 特性列表\n\n## 核心特性\n\n- [ ] 特性1\n- [ ] 特性2\n- [ ] 特性3',
        'architecture': '# 架构设计\n\n## 系统概述\n\n## 核心组件\n\n## 技术栈',
        'tech_design': '# 技术方案\n\n## 技术选型\n\n## 实现方案\n\n## 风险评估',
        'background': '# 背景资料\n\n## 项目背景\n\n## 相关资源\n\n## 参考文档',
        'requirements': '# 需求文档\n\n## 功能需求\n\n## 非功能需求\n\n## 验收标准',
        'meeting': '# 会议纪要\n\n## 会议信息\n- 时间：\n- 参与者：\n\n## 讨论要点\n\n## 行动项',
        'task': '# 任务文档\n\n## 任务描述\n\n## 实现计划\n\n## 验收标准'
      };

      const referenceMeta = referenceSource ? REFERENCE_SOURCE_META[referenceSource] : undefined;
      const effectiveType = referenceMeta?.documentType ?? type;

      const trimmedTitle = title?.trim() || '';
      let finalTitle = trimmedTitle || defaultTitles[effectiveType] || '新文档';
      let finalContent = defaultContents[effectiveType];

      if (referenceSource) {
        const loader = referenceLoaders[referenceSource];
        if (loader) {
          try {
            const meta = REFERENCE_SOURCE_META[referenceSource];
            if (meta?.requiresContext) {
              if (!referenceContext || referenceContext.type !== meta.requiresContext || !referenceContext.id) {
                message.error(`请选择${meta.requiresContext === 'task' ? '任务' : '会议'}作为引用来源`);
                return;
              }
            }

            const { content: referencedContent, title: referencedTitle } = await loader(referenceContext);
            if (referencedContent) {
              finalContent = referencedContent;
            } else {
              message.warning('引用文档暂无内容，已使用默认模板');
            }
            if (!trimmedTitle && referencedTitle) {
              finalTitle = referencedTitle;
            }
          } catch (error) {
            console.error('加载引用内容失败:', error);
            message.error('引用内容加载失败，已使用默认模板');
          }
        } else {
          message.warning('所选引用暂不可用，将使用默认模板');
        }
      }

      const result = await documentsAPI.createNode(projectId, {
        parent_id: (parentId === 'root' || parentId === '') ? undefined : parentId,
        title: finalTitle,
        type: effectiveType as any, // 临时解决类型不匹配问题
        content: finalContent
      });

      // 重新加载树数据以反映新节点
      await loadTreeData();
      const newNodeId = result?.node?.id;
      if (newNodeId) {
        mergeNodeTitles({ [newNodeId]: finalTitle });
      }
      message.success(`成功添加 ${finalTitle}${referenceSource ? '（已引用）' : ''}`);
      
    } catch (error) {
      console.error('添加节点失败:', error);
      message.error('添加节点失败，请稍后重试');
    }
  };

  const handleMoveNode = async (
    dragNodeId: string,
    targetNodeId: string,
    position: 'before' | 'after' | 'inside'
  ) => {
    if (!treeData.length) {
      return;
    }

    const ROOT_KEY = '__root__';
    const parentMap: Record<string, string | undefined> = {};
    const childrenMap: Record<string, string[]> = {};

    const buildMaps = (nodes: DocumentTreeNode[], parentId?: string) => {
      if (!nodes.length) {
        if (parentId) {
          childrenMap[parentId] = childrenMap[parentId] ?? [];
        }
        return;
      }

      const key = parentId ?? ROOT_KEY;
      childrenMap[key] = nodes.map((node) => node.id);

      nodes.forEach((node) => {
        parentMap[node.id] = parentId;
        if (node.children && node.children.length > 0) {
          buildMaps(node.children, node.id);
        } else {
          childrenMap[node.id] = childrenMap[node.id] ?? [];
        }
      });
    };

    buildMaps(treeData);

    const sourceParentId = parentMap[dragNodeId];
    const sourceKey = sourceParentId ?? ROOT_KEY;
    const currentSiblings = [...(childrenMap[sourceKey] ?? [])];
    const currentIndex = currentSiblings.indexOf(dragNodeId);

    if (currentIndex === -1) {
      return;
    }

    currentSiblings.splice(currentIndex, 1);
    childrenMap[sourceKey] = currentSiblings;

  let newParentId: string | undefined;
  let destinationKey: string;

    if (position === 'inside') {
  newParentId = targetNodeId;
  destinationKey = targetNodeId;
  const destinationChildren = newParentId === sourceParentId ? currentSiblings : [...(childrenMap[destinationKey] ?? [])];
      const filtered = destinationChildren.filter((id) => id !== dragNodeId);
      filtered.push(dragNodeId);
      childrenMap[destinationKey] = filtered;
    } else {
      newParentId = parentMap[targetNodeId];
      destinationKey = newParentId ?? ROOT_KEY;
      const destinationChildren = destinationKey === sourceKey ? currentSiblings : [...(childrenMap[destinationKey] ?? [])];
      const filtered = destinationChildren.filter((id) => id !== dragNodeId);
      const targetIndex = filtered.indexOf(targetNodeId);

      if (targetIndex === -1) {
        return;
      }

      const insertIndex = position === 'before' ? targetIndex : targetIndex + 1;
      filtered.splice(insertIndex, 0, dragNodeId);
      childrenMap[destinationKey] = filtered;
    }

    const finalKey = destinationKey;
    const finalSiblings = childrenMap[finalKey] ?? [];
    const newPosition = finalSiblings.indexOf(dragNodeId);

    if (newPosition === -1) {
      return;
    }

    if (newParentId === sourceParentId && newPosition === currentIndex) {
      return;
    }

    const request: MoveNodeRequest = { position: newPosition };
    if (newParentId) {
      request.new_parent_id = newParentId;
    }

    setLoading(true);
    try {
      const { documentsAPI } = await import('../api/documents');
      await documentsAPI.moveNode(projectId, dragNodeId, request);
      await loadTreeData();
      setExpandedTreeKeys((prev) => {
        if (!newParentId) {
          return prev;
        }
        const next = new Set(prev);
        next.add(newParentId);
        return Array.from(next);
      });
      setSelectedTreeKeys([dragNodeId]);
      message.success('节点移动成功');
    } catch (error) {
      console.error('移动节点失败:', error);
      message.error('节点移动失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  const handleRenameNode = async (nodeId: string, title: string) => {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      message.error('文档标题不能为空');
      return Promise.reject(new Error('EMPTY_TITLE'));
    }

    try {
      const { documentsAPI } = await import('../api/documents');
      await documentsAPI.updateNode(projectId, nodeId, { title: trimmedTitle });
  mergeNodeTitles({ [nodeId]: trimmedTitle });
      setTreeData((prev) => (prev.length ? applyTitleOverrides(prev, { [nodeId]: trimmedTitle }) : prev));
      if (currentDocumentId === nodeId) {
        setCurrentDocumentTitle(trimmedTitle);
      }
      message.success('标题已更新');
    } catch (error) {
      console.error('重命名节点失败:', error);
      message.error('重命名失败，请稍后重试');
      throw error;
    }
  };

  // 删除节点
  const handleDeleteNode = async (nodeId: string) => {
    try {
      const { default: documentsAPI } = await import('../api/documents');
      
      // 确认删除
      Modal.confirm({
        title: '确认删除',
        content: '确定要删除这个节点吗？此操作不可撤销。',
        okText: '删除',
        okType: 'danger',
        cancelText: '取消',
        onOk: async () => {
          try {
            await documentsAPI.deleteNode(projectId, nodeId);

            // 重新加载树数据以反映删除
            await loadTreeData();

            // 如果删除的是当前选中的节点，清空相关状态
            if (currentDocumentId === nodeId) {
              setCurrentDocumentId(null);
              setMarkdownContent('');
            }

            message.success('节点删除成功');
          } catch (error) {
            console.error('删除节点失败:', error);
            message.error('删除节点失败，请稍后重试');
          }
        }
      });

    } catch (error) {
      console.error('删除操作失败:', error);
      message.error('删除操作失败，请稍后重试');
    }
  };

  const handleVersionSelect = (version: number) => {
    console.log('选择版本:', version);
    message.success(`已选择历史版本 v${version}`);
  };

  const handleCompareWithCurrent = (version: number) => {
    if (!currentDocumentId) {
      message.warning('请先选择文档以进行版本对比');
      return;
    }
    if (version === documentVersion) {
      message.info('该版本已是当前版本，无需对比。');
      return;
    }

    const from = Math.min(version, documentVersion);
    const to = Math.max(version, documentVersion);
    setDiffContext({ from, to });
    setDiffModalVisible(true);
  };

  const handleCompareSelectedVersions = (versions: [number, number]) => {
    if (!currentDocumentId) {
      message.warning('请先选择文档以进行版本对比');
      return;
    }
    const [v1, v2] = versions;
    const from = Math.min(v1, v2);
    const to = Math.max(v1, v2);
    setDiffContext({ from, to });
    setDiffModalVisible(true);
  };

  const handleMarkdownSave = async (content: string) => {
    if (!currentDocumentId) {
      message.error('请先选择要编辑的文档');
      return;
    }
    
    // 检查当前文档ID是否在文档树中存在
    const documentPath = findNodePath(treeData, currentDocumentId);
    if (!documentPath || documentPath.length === 0) {
      message.error(`文档 ${currentDocumentId} 不存在于当前项目中，请选择其他文档`);
      // 重置状态
      setCurrentDocumentId(null);
      setCurrentDocumentTitle('');
      setMarkdownContent('');
      return;
    }
    
    console.log('开始保存文档:', currentDocumentId, '版本:', documentVersion);
    
    try {
      setLoading(true);
      
      // 使用动态导入确保 API 正确加载
      const { documentsAPI } = await import('../api/documents');
      
      // 添加超时处理
      const savePromise = documentsAPI.updateContent(projectId, currentDocumentId, {
        content,
        version: documentVersion
      });
      
      const timeoutPromise = new Promise((_, reject) => {
        setTimeout(() => reject(new Error('保存超时，请检查网络连接')), 30000);
      });
      
      const result = await Promise.race([savePromise, timeoutPromise]) as { version: number; success: boolean };
      
      console.log('保存成功，新版本:', result.version);
      
      try {
        const refreshed = await documentsAPI.getContent(projectId, currentDocumentId);
        let latestVersion = refreshed.meta?.version ?? result.version ?? 1;
        try {
          const history = await documentsAPI.getVersionHistory(projectId, currentDocumentId);
          if (Array.isArray(history?.versions) && history.versions.length > 0) {
            latestVersion = Math.max(...history.versions.map((item) => item.version));
            setVersionHistoryRefreshKey((prev) => prev + 1);
          }
        } catch (historyError) {
          console.warn('刷新版本历史失败:', historyError);
        }

        setMarkdownContent(refreshed.content);
        setDocumentVersion(latestVersion);

        const metaTitle = refreshed.meta?.title?.trim() ?? '';
        if (metaTitle) {
          setCurrentDocumentTitle(metaTitle);
          setNodeTitleMap((prev) => {
            const shouldUpdate = prev[currentDocumentId] !== metaTitle;
            return shouldUpdate ? { ...prev, [currentDocumentId]: metaTitle } : prev;
          });
          setTreeData((prev) => (prev.length ? applyTitleOverrides(prev, { [currentDocumentId]: metaTitle }) : prev));
        } else {
          setCurrentDocumentTitle('');
        }
      } catch (refreshError) {
        console.error('刷新文档版本信息失败:', refreshError);
        setDocumentVersion(result.version ?? documentVersion);
      }
      message.success('文档保存成功');
    } catch (e: any) {
      console.error('保存失败:', e);
      
      if (e?.response?.data?.code === 'VERSION_MISMATCH' || e?.message?.includes('VERSION_MISMATCH')) {
        message.warning('文档已被他人修改，正在加载冲突解决器...');
        await openConflictResolver(currentDocumentId, documentVersion, content);
        return;
      } else if (e?.message?.includes('超时')) {
        message.error('保存超时，请检查网络连接后重试');
      } else if (e?.response?.status === 404) {
        message.error(`文档 ${currentDocumentId} 不存在，无法保存。请选择其他文档进行编辑。`);
        // 当文档不存在时，重置相关状态
        setCurrentDocumentId(null);
        setCurrentDocumentTitle('');
        setMarkdownContent('');
        setDocumentVersion(1);
      } else if (e?.response?.status === 401) {
        message.error('认证失效，请重新登录');
      } else if (e?.response?.status >= 500) {
        message.error('服务器错误，请稍后重试');
      } else {
        message.error('保存失败: ' + (e?.message || e?.response?.data?.error || '未知错误'));
      }
    } finally {
      console.log('保存操作结束，关闭loading状态');
      setLoading(false);
    }
  };

  const lastDocumentLoadTime = useRef<number>(0);
  const lastDocumentId = useRef<string>('');
  const loadDocumentContent = async (documentId: string) => {
    const now = Date.now();
    if (documentId === lastDocumentId.current && now - lastDocumentLoadTime.current < 100) {
      console.log('[DEBUG] loadDocumentContent throttled for:', documentId, 'last call was', now - lastDocumentLoadTime.current, 'ms ago');
      return;
    }
    lastDocumentLoadTime.current = now;
    lastDocumentId.current = documentId;
    console.log('[DEBUG] loadDocumentContent called for:', documentId, 'stack:', new Error().stack?.split('\n').slice(1, 4));
    if (!documentId) return;
    
    // 防止 taskId 被错误地当作文档ID使用
    if (documentId.startsWith('task_')) {
      console.warn('[WARN] 试图使用 taskId 作为文档ID，已阻止:', documentId);
      message.warning(`任务ID ${documentId} 不能直接作为文档编辑，请选择具体的文档节点`);
      return;
    }
    setSelectedTreeKeys([documentId]);
    try {
      setLoading(true);
      const { documentsAPI } = await import('../api/documents');
      const result = await documentsAPI.getContent(projectId, documentId);
      let resolvedVersion = result.meta?.version ?? 1;
      try {
        const history = await documentsAPI.getVersionHistory(projectId, documentId);
        if (Array.isArray(history?.versions) && history.versions.length > 0) {
          resolvedVersion = Math.max(...history.versions.map((item) => item.version));
          setVersionHistoryRefreshKey((prev) => prev + 1);
        }
      } catch (historyError) {
        console.warn('加载版本历史失败:', historyError);
      }
      setMarkdownContent(result.content);
      setDocumentVersion(resolvedVersion);
      setCurrentDocumentId(documentId);
      const metaTitle = result.meta?.title?.trim() ?? '';
      if (metaTitle) {
        console.log('[DEBUG] Setting document title for:', documentId, 'title:', metaTitle);
        setCurrentDocumentTitle(metaTitle);
        setNodeTitleMap((prev) => {
          const shouldUpdate = prev[documentId] !== metaTitle;
          console.log('[DEBUG] NodeTitleMap update needed:', shouldUpdate, 'prev:', prev[documentId], 'new:', metaTitle);
          return shouldUpdate ? { ...prev, [documentId]: metaTitle } : prev;
        });
        setTreeData((prev) => (prev.length ? applyTitleOverrides(prev, { [documentId]: metaTitle }) : prev));
      } else {
        setCurrentDocumentTitle('');
      }
    } catch (e: any) {
      console.error('加载文档失败:', e);
      
      // 如果是404错误，说明文档不存在，需要重置状态
      if (e?.response?.status === 404) {
        message.error(`文档 ${documentId} 不存在，请选择其他文档`);
        // 重置相关状态
        setCurrentDocumentId(null);
        setCurrentDocumentTitle('');
        setMarkdownContent('');
        setDocumentVersion(1);
        setSelectedTreeKeys([]);
      } else {
        message.error('加载文档内容失败: ' + (e?.message || e?.response?.data?.error || '未知错误'));
      }
    } finally {
      setLoading(false);
    }
  };





  // 渲染左侧边栏内容
  const sidebarTabs = [
    {
      key: 'structure' as SidebarTab,
      label: (
        <Space size={4}>
          <FileTextOutlined />
          <span>文档结构</span>
        </Space>
      )
    },
    {
      key: 'search' as SidebarTab,
      label: (
        <Space size={4}>
          <SearchOutlined />
          <span>全局搜索</span>
        </Space>
      )
    },
    {
      key: 'relations' as SidebarTab,
      label: (
        <Space size={4}>
          <ShareAltOutlined />
          <span>关系管理</span>
        </Space>
      )
    }
  ];

  const renderSidebarContent = () => {
    switch (activeTab) {
      case 'structure':
        return (
          <div style={{ ...sidebarPanelStyle, flex: 1, padding: 0 }}>
            <EnhancedTreeView
              treeData={treeData}
              selectedKeys={selectedTreeKeys}
              expandedKeys={expandedTreeKeys}
              loading={loading}
              searchable
              draggable
              showContextMenu
              showToolbar
              onSelect={(keys: string[]) => {
                if (keys.length > 0) {
                  handleNodeSelect(keys[0]);
                } else {
                  setSelectedTreeKeys([]);
                  setCurrentDocumentId(null);
                  setCurrentDocumentTitle('');
                  setMarkdownContent('');
                }
              }}
              onExpand={(keys: string[]) => setExpandedTreeKeys(keys)}
              onAdd={handleAddNode}
              referenceOptions={referenceOptionGroups}
              referenceContextOptions={referenceContextOptions}
              onRename={handleRenameNode}
              onDelete={handleDeleteNode}
              onMove={handleMoveNode}
              onLinkToTask={handleLinkToTask}
              onAddToResource={handleAddToResource}
            />
          </div>
        );
        
      case 'search':
        return (
          <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
            <section style={sidebarPanelStyle}>
              <div style={sidebarPanelHeaderStyle}>
                <span style={sidebarPanelTitleStyle}>全局搜索</span>
              </div>
              <div>
                <GlobalSearchBox
                  projectId={projectId}
                  onSearch={handleSearch}
                />
              </div>
            </section>
            <section style={{ ...sidebarPanelStyle, flex: 1 }}>
              <div style={sidebarPanelHeaderStyle}>
                <span style={sidebarPanelTitleStyle}>搜索结果</span>
              </div>
              <div style={{ ...sidebarPanelBodyStyle, overflow: 'auto' }}>
                <SearchResultsView
                  results={searchResults}
                  loading={searchLoading}
                  onNodeSelect={handleNodeSelect}
                />
              </div>
            </section>
          </div>
        );
        
      case 'relations':
        return (
          <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
            <section style={{ ...sidebarPanelStyle, flex: 1 }}>
              <div style={sidebarPanelHeaderStyle}>
                <span style={sidebarPanelTitleStyle}>文档关系管理</span>
                <Space size={8}>
                  <Button 
                    type="primary" 
                    size="small" 
                    icon={<PlusOutlined />}
                    onClick={handleOpenRelationshipModal}
                  >
                    创建关系
                  </Button>
                  <Button
                    size="small"
                    onClick={loadRelationshipData}
                    loading={relationshipManagerLoading}
                    title="刷新关系数据"
                  >
                    🔄 刷新
                  </Button>
                </Space>
              </div>
              <Spin spinning={relationshipManagerLoading}>
                <div style={{ ...sidebarPanelBodyStyle, gap: 16 }}>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 8,
                      flexWrap: 'wrap'
                    }}
                  >
                    <span style={{ fontSize: 12, color: '#6b7280' }}>聚焦文档</span>
                    <Select
                      allowClear
                      showSearch
                      placeholder="选择文档以聚焦关系"
                      value={relationshipFilterDocId}
                      onChange={(value) => setRelationshipFilterDocId(value || undefined)}
                      options={documentOptions}
                      style={{ flex: 1, minWidth: 200 }}
                      optionFilterProp="label"
                    />
                    {currentDocumentId && (
                      <Button
                        size="small"
                        onClick={() => setRelationshipFilterDocId(currentDocumentId)}
                      >
                        聚焦当前文档
                      </Button>
                    )}
                    <Button size="small" onClick={() => setRelationshipFilterDocId(undefined)}>
                      查看全部
                    </Button>
                  </div>

                  <Divider style={{ margin: '8px 0' }} />

                  <div style={{ flex: 1, minHeight: 0, overflowX: 'hidden' }}>
                    <Table
                      size="small"
                      columns={relationshipColumns}
                      dataSource={relationshipTableData}
                      loading={relationshipsLoading}
                      pagination={{ pageSize: 8, showTotal: (total) => `共 ${total} 条关系` }}
                      rowKey="key"
                      scroll={{ y: 220 }}
                      tableLayout="fixed"
                      style={{ width: '100%' }}
                    />
                  </div>
                </div>
              </Spin>
            </section>
            <section style={{ ...sidebarPanelStyle }}>
              <div style={sidebarPanelHeaderStyle}>
                <span style={sidebarPanelTitleStyle}>引用记录</span>
              </div>
              <div style={{ ...sidebarPanelBodyStyle, overflow: 'auto' }}>
                <ReferencePanel
                  projectId={projectId}
                  nodeId={relationshipFilterDocId ?? currentDocumentId ?? ''}
                  references={references}
                  loading={referencesLoading}
                  onAddReference={handleAddReferenceEntry}
                  onDeleteReference={handleDeleteReference}
                  onReferenceClick={(documentId) => handleNodeSelect(documentId)}
                />
              </div>
            </section>
          </div>
        );
        
      default:
        return null;
    }
  };

  // 渲染右侧内容区域
  const renderContentArea = () => {
    switch (activeTab) {
      case 'structure':
      case 'search':
        return (
          <div
            style={{
              flex: 1,
              minHeight: 0,
              display: 'flex',
              flexDirection: 'column',
              backgroundColor: '#fff',
              borderRadius: 8,
              padding: isFullscreen ? 16 : 16,
              boxShadow: '0 1px 3px rgba(0, 0, 0, 0.08)'
            }}
          >
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: 12
              }}
            >
              <span style={{ ...sidebarPanelTitleStyle, fontSize: 16 }}>文档查看</span>
              <Space size={8}>
                <Button
                  icon={<ExclamationCircleOutlined />}
                  size="small"
                  style={{ fontSize: 12, padding: '0 12px', height: 28 }}
                  onClick={() => {
                    if (!conflicts.length) {
                      message.info('暂无待解决的冲突');
                      return;
                    }
                    setActiveOverlay('conflict');
                  }}
                >
                  冲突解决
                </Button>
                <Button
                  icon={<BarChartOutlined />}
                  size="small"
                  style={{ fontSize: 12, padding: '0 12px', height: 28 }}
                  onClick={() => {
                    if (!currentDocumentId) {
                      message.warning('请先选择文档再查看影响分析');
                      return;
                    }
                    setActiveOverlay('impact');
                  }}
                >
                  影响分析
                </Button>
                <Button
                  type="primary"
                  size="small"
                  icon={<HistoryOutlined />}
                  style={{ fontSize: 12, padding: '0 12px', height: 28 }}
                  onClick={() => setHistoryDrawerVisible(true)}
                >
                  历史
                </Button>
              </Space>
            </div>

            <div
              style={{
                position: 'relative',
                flex: 1,
                minHeight: 0,
                display: 'flex',
                flexDirection: 'column'
              }}
            >
              <MarkdownEditor
                value={markdownContent}
                onChange={setMarkdownContent}
                onSave={handleMarkdownSave}
                loading={loading}
                height={editorHeight}
                showPreview
                showToolbar
                placeholder={currentDocumentId ? `编辑文档 ${currentDocumentTitle || currentDocumentId}...` : '请先选择要编辑的文档'}
                readOnly={!currentDocumentId}
              />
              {renderDocumentOverlay()}
            </div>
          </div>
        );
        
      case 'relations':
        return (
          <div style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
            <RelationshipGraph
              projectId={projectId}
              nodes={graphNodes}
              edges={graphEdges}
              loading={relationshipsLoading}
              onNodeSelect={handleNodeSelect}
            />
          </div>
        );
        
      default:
        return null;
    }
  };

  const containerStyle: React.CSSProperties = isFullscreen
    ? {
        position: 'fixed',
        inset: 0,
        zIndex: 1000,
        backgroundColor: '#fff',
        display: 'flex',
        flexDirection: 'column'
      }
    : {
        display: 'flex',
        flexDirection: 'column',
        minHeight: 'calc(100vh - 120px)'
      };

  const layoutStyle: React.CSSProperties = {
    flex: 1,
    display: 'flex',
    minHeight: 0,
    backgroundColor: '#fff'
  };

  return (
    <Spin spinning={loading}>
      <div style={containerStyle}>
        {/* 页面头部 */}
        <div style={{ padding: '16px', borderBottom: '1px solid #f0f0f0', backgroundColor: '#fafafa' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <Title level={4} style={{ margin: 0 }}>
              {projectId} - {taskId} 文档管理系统
            </Title>
            <Space>
              <Tooltip title={isFullscreen ? '退出全屏' : '全屏显示'}>
                <Button
                  icon={isFullscreen ? <CompressOutlined /> : <FullscreenOutlined />}
                  onClick={() => setIsFullscreen(!isFullscreen)}
                />
              </Tooltip>
            </Space>
          </div>
        </div>

        {/* 主要布局 */}
        <Layout style={layoutStyle}>
          {/* 左侧边栏 */}
          <Sider
            width={350}
            style={{
              backgroundColor: '#fff',
              borderRight: '1px solid #f0f0f0',
              display: 'flex',
              flexDirection: 'column'
            }}
          >
            <Tabs
              activeKey={activeTab}
              onChange={(key) => setActiveTab(key as SidebarTab)}
              items={sidebarTabs}
              size="small"
              tabBarStyle={{ margin: 0, padding: '8px 16px' }}
            />
            <div style={{ flex: 1, padding: '0 0 16px', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
              {renderSidebarContent()}
            </div>
          </Sider>

          {/* 右侧内容区域 */}
          <Content
            style={{
              padding: '16px',
              backgroundColor: '#f7f8fa',
              display: 'flex',
              flexDirection: 'column',
              minHeight: 0,
              position: 'relative',
              overflow: 'hidden'
            }}
          >
            <div style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
              {renderContentArea()}
            </div>

            <Drawer
              title="版本历史"
              placement="right"
              width={360}
              visible={historyDrawerVisible}
              onClose={() => setHistoryDrawerVisible(false)}
              mask={false}
              getContainer={false}
              rootStyle={{ position: 'absolute' }}
              bodyStyle={{ padding: 0, height: '100%' }}
              headerStyle={{ padding: '12px 16px' }}
            >
              <div style={{ height: '100%', overflow: 'auto', padding: 16 }}>
                <VersionHistoryPanel
                  projectId={projectId}
                  nodeId={currentDocumentId || ''}
                  currentVersion={currentDocumentId ? documentVersion : undefined}
                  currentTitle={currentDocumentTitle}
                  onVersionSelect={handleVersionSelect}
                  onCompareWithCurrent={handleCompareWithCurrent}
                  onCompareSelected={handleCompareSelectedVersions}
                  refreshKey={versionHistoryRefreshKey}
                />
              </div>
            </Drawer>
          </Content>
        </Layout>
        {/* 创建关系模态框 */}
        <Modal
          title="创建文档关系"
          open={relationshipModalVisible}
          onOk={relationshipForm.submit}
          onCancel={handleCloseRelationshipModal}
          confirmLoading={relationshipSubmitting}
          okText="创建关系"
          cancelText="取消"
          width={600}
        >
          <Form
            form={relationshipForm}
            layout="vertical"
            onFinish={handleCreateRelationship}
            requiredMark={false}
          >
            <Form.Item
              label="起始文档"
              name="from_id"
              rules={[{ required: true, message: '请选择起始文档' }]}
            >
              <Select
                showSearch
                placeholder="选择起始文档"
                options={documentOptions}
                optionFilterProp="label"
              />
            </Form.Item>

            <Form.Item
              label="目标文档"
              name="to_id"
              rules={[{ required: true, message: '请选择目标文档' }]}
            >
              <Select
                showSearch
                placeholder="选择目标文档"
                options={documentOptions}
                optionFilterProp="label"
              />
            </Form.Item>

            {/* 隐藏的关系类型字段，固定为 reference */}
            <Form.Item
              name="type"
              initialValue="reference"
              style={{ display: 'none' }}
            >
              <Input type="hidden" />
            </Form.Item>

            <Form.Item
              label="依赖类型"
              name="dependency_type"
              rules={[{ required: true, message: '请选择依赖类型' }]}
              extra="引用关系必须指定依赖类型：数据依赖、接口依赖或配置依赖"
            >
              <Select 
                options={DEPENDENCY_TYPE_OPTIONS} 
                placeholder="请选择依赖类型"
              />
            </Form.Item>

            <Form.Item 
              label="关系说明" 
              name="description"
            >
              <Input.TextArea 
                rows={3} 
                placeholder="可选，描述该关系的背景或备注"
                maxLength={500}
                showCount
              />
            </Form.Item>
          </Form>
        </Modal>

        {/* 任务关联模态框 */}
        <Modal
          title="关联任务"
          open={linkTaskModalVisible}
          onOk={handleLinkTaskSubmit}
          onCancel={() => {
            setLinkTaskModalVisible(false);
            linkTaskForm.resetFields();
          }}
          okText="创建关联"
          cancelText="取消"
          width={600}
        >
          <Form
            form={linkTaskForm}
            layout="vertical"
            requiredMark={false}
          >
            <Form.Item
              label="选择任务"
              name="task_id"
              rules={[{ required: true, message: '请选择要关联的任务' }]}
            >
              <Select
                showSearch
                placeholder="选择任务"
                options={availableTasks.map(task => ({
                  value: task.id,
                  label: task.name,
                  description: [task.feature_name, task.assignee].filter(Boolean).join(' · ')
                }))}
                optionFilterProp="label"
                optionRender={(option) => (
                  <div>
                    <div>{option.label}</div>
                    {option.data.description && (
                      <div style={{ fontSize: '12px', color: '#999' }}>
                        {option.data.description}
                      </div>
                    )}
                  </div>
                )}
              />
            </Form.Item>

            <Form.Item
              label="锚点"
              name="anchor"
              extra="可选，用于标识文档中的具体位置，如章节号"
            >
              <Input placeholder="例如：2.1.3" maxLength={50} />
            </Form.Item>

            <Form.Item
              label="上下文说明"
              name="context"
              extra="可选，描述引用的上下文或用途"
            >
              <Input.TextArea
                rows={3}
                placeholder="例如：该文档的数据库设计部分被当前任务引用"
                maxLength={500}
                showCount
              />
            </Form.Item>
          </Form>
        </Modal>

        {/* 版本差异对比模态框 */}
        {diffContext && currentDocumentId && (
          <DiffViewModal
            projectId={projectId}
            nodeId={currentDocumentId}
            fromVersion={diffContext.from}
            toVersion={diffContext.to}
            currentVersion={documentVersion}
            visible={diffModalVisible}
            onClose={() => {
              setDiffModalVisible(false);
              setDiffContext(null);
            }}
          />
        )}
      </div>
    </Spin>
  );
};

export default DocumentManagementSystem;