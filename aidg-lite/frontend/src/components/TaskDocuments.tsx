import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Tabs, Spin, Button, message, Descriptions, Tag, Space, Typography, List, Card, Drawer, Badge, Modal, Form, Input } from 'antd';
import {
  FileTextOutlined,
  EditOutlined,
  SaveOutlined,
  InfoCircleOutlined,
  ProjectOutlined,
  MessageOutlined,
  FolderOutlined,
  UserOutlined,
  AppstoreOutlined,
  ClockCircleOutlined,
  ReloadOutlined,
  BulbOutlined,
  VerticalAlignTopOutlined,
} from '@ant-design/icons';
import { getTaskDocument, saveTaskDocument, getProjectTask, getTaskPrompts, getExecutionPlan, getTaskSection, ProjectTask, TaskPrompt } from '../api/tasks';
import { addCustomResource } from '../api/resourceApi';
import { loadAuth } from '../api/auth';
import MarkdownViewer from './MarkdownViewer';
import ExecutionPlanView from './ExecutionPlanView';
import TaskDocIncremental from './TaskDocIncremental';
import TaskLinkedDocuments from './TaskLinkedDocuments';
import SectionEditor from './SectionEditor';
import DocumentTOC from './DocumentTOC';
import TaskSummaryPanel from './TaskSummaryPanel';
import RecommendationPanel from './RecommendationPanel';
import { useTaskRefresh } from '../contexts/TaskRefreshContext';
import { TagButton, TagVersionSelect, TagConfirmModal } from './TagManagement';
import { tagService } from '../services/tagService';

const { Text } = Typography;

interface Props {
  projectId: string;
  taskId: string;
  liteMode?: boolean; // lite 模式下隐藏提示词记录、历史记录、关联文档
}

const TaskDocuments: React.FC<Props> = ({ projectId, taskId, liteMode = false }) => {
  const [activeTab, setActiveTab] = useState<'info' | 'requirements' | 'design' | 'test' | 'prompts' | 'incremental' | 'documents' | 'execution-plan'>('info');
  const [showBackTop, setShowBackTop] = useState(false);
  const lastScrollElementRef = useRef<HTMLElement | null>(null);
  const [documents, setDocuments] = useState<Record<string, { 
    content: string; 
    exists: boolean;
    recommendations?: Array<{
      task_id: string;
      doc_type: string;
      section_id: string;
      title: string;
      similarity: number;
      snippet: string;
    }>;
  }>>({
    requirements: { content: '', exists: false },
    design: { content: '', exists: false },
    test: { content: '', exists: false },
  });
  const [taskInfo, setTaskInfo] = useState<ProjectTask | null>(null);
  const [prompts, setPrompts] = useState<TaskPrompt[]>([]);
  const [loading, setLoading] = useState(false);
  const [promptsLoading, setPromptsLoading] = useState(false);
  const [executionPlanExists, setExecutionPlanExists] = useState(false);
  
  // 编辑模式状态：每个文档类型单独控制是否进入章节编辑模式
  const [editMode, setEditMode] = useState<Record<string, boolean>>({
    requirements: false,
    design: false,
    test: false,
  });

  // 章节编辑弹窗状态（用于从TOC触发编辑）
  const [sectionEditorModal, setSectionEditorModal] = useState<{
    visible: boolean;
    docType: 'requirements' | 'design' | 'test' | null;
    sectionTitle: string | null;
  }>({
    visible: false,
    docType: null,
    sectionTitle: null,
  });
  // 追踪章节编辑器是否有未保存的更改
  const [sectionEditorHasUnsavedChanges, setSectionEditorHasUnsavedChanges] = useState(false);

  // MCP资源添加弹窗状态
  const [mcpResourceModal, setMcpResourceModal] = useState<{
    visible: boolean;
    docType: 'requirements' | 'design' | 'test' | null;
    sectionTitle: string | null;
  }>({
    visible: false,
    docType: null,
    sectionTitle: null,
  });
  const [mcpForm] = Form.useForm();
  const [mcpSaving, setMcpSaving] = useState(false);

  // Tag版本管理状态
  const [selectedTag, setSelectedTag] = useState<Record<string, string>>({
    requirements: '当前版本',
    design: '当前版本',
    test: '当前版本',
  });
  const [tagRefreshKey, setTagRefreshKey] = useState<Record<string, number>>({
    requirements: 0,
    design: 0,
    test: 0,
  });
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const [pendingSwitchTag, setPendingSwitchTag] = useState<{
    docType: 'requirements' | 'design' | 'test';
    tagName: string;
    currentMd5?: string;
  } | null>(null);

  // 推荐抽屉状态
  const [recommendationDrawerOpen, setRecommendationDrawerOpen] = useState(false);
  const [currentRecommendations, setCurrentRecommendations] = useState<Array<{
    task_id: string;
    doc_type: string;
    section_id: string;
    title: string;
    similarity: number;
    snippet: string;
    source_section_id?: string;  // 源章节ID
    source_title?: string;        // 源章节标题
  }>>([]);
  
  // 选中的推荐文档详情(包含源章节和目标章节,便于对比)
  const [selectedRecommendation, setSelectedRecommendation] = useState<{
    // 源章节(当前任务中匹配的章节)
    sourceSection: {
      taskId: string;
      taskName: string;
      sectionId: string;
      title: string;
      content: string;
    };
    // 目标章节(推荐的相似章节)
    targetSection: {
      taskId: string;
      taskName: string;
      docType: string;
      sectionId: string;
      title: string;
      content: string;
    };
    similarity: number;
    loading: boolean;
  } | null>(null);

  const { refreshTrigger } = useTaskRefresh();

  // 使用全局滚动监听 - 监听页面上所有的滚动事件
  useEffect(() => {
    const handleScroll = (e: Event) => {
      const target = e.target as HTMLElement;
      if (target && target.scrollTop !== undefined) {
        const scrollTop = target.scrollTop;
        
        // 记录最后滚动的元素
        lastScrollElementRef.current = target;
        
        // 只在状态真正需要改变时才更新，避免不必要的重新渲染
        const shouldShowBackTop = scrollTop > 100;
        setShowBackTop(prev => prev !== shouldShowBackTop ? shouldShowBackTop : prev);
      }
    };

    // 使用捕获阶段监听所有滚动事件
    document.addEventListener('scroll', handleScroll, { passive: true, capture: true });

    return () => {
      document.removeEventListener('scroll', handleScroll, true);
    };
  }, []);

  // 返回顶部函数
  const scrollToTop = () => {
    // 滚动最后记录的元素
    if (lastScrollElementRef.current) {
      lastScrollElementRef.current.scrollTo({ top: 0, behavior: 'smooth' });
    } else {
      // 回退: 尝试滚动 .scroll-region
      const scrollRegion = document.querySelector('.scroll-region');
      if (scrollRegion) {
        scrollRegion.scrollTo({ top: 0, behavior: 'smooth' });
      }
    }
  };

  // 当切换任务时，重置tag选择状态
  useEffect(() => {
    setSelectedTag({
      requirements: '当前版本',
      design: '当前版本',
      test: '当前版本',
    });
    setTagRefreshKey({
      requirements: 0,
      design: 0,
      test: 0,
    });
  }, [projectId, taskId]);

  useEffect(() => {
    if (projectId && taskId) {
      // 并行加载所有数据，提升页面加载速度
      loadAllData();
    }
  }, [projectId, taskId, refreshTrigger]);

  // 优化：并行加载所有数据
  const loadAllData = async () => {
    if (!projectId || !taskId) return;
    
    setLoading(true);
    setPromptsLoading(true);
    
    try {
      // 并行请求所有数据
      const [documentsResult, taskInfoResult, promptsResult, executionPlanResult] = await Promise.allSettled([
        // 1. 加载三个文档（requirements, design, test）
        loadDocumentsData(),
        // 2. 加载任务信息
        getProjectTask(projectId, taskId),
        // 3. 加载提示词
        getTaskPrompts(projectId, taskId),
        // 4. 加载执行计划状态
        getExecutionPlan(projectId, taskId),
      ]);

      // 处理文档数据
      if (documentsResult.status === 'fulfilled') {
        setDocuments(documentsResult.value);
      } else {
        // 对403/500等权限/服务器错误不显示提示，避免影响无权限用户体验
        const error = documentsResult.reason;
        if (error?.response?.status !== 403 && error?.response?.status !== 500) {
          message.error('加载文档失败');
        }
        console.error(documentsResult.reason);
      }

      // 处理任务信息
      if (taskInfoResult.status === 'fulfilled') {
        setTaskInfo(taskInfoResult.value.data || null);
      } else {
        // 对403/500等权限/服务器错误不显示提示
        const error = taskInfoResult.reason;
        if (error?.response?.status !== 403 && error?.response?.status !== 500) {
          message.error('加载任务信息失败');
        }
        console.error(taskInfoResult.reason);
        setTaskInfo(null);
      }

      // 处理提示词
      if (promptsResult.status === 'fulfilled') {
        setPrompts(promptsResult.value.data || []);
      } else {
        // 对403/500等权限/服务器错误不显示提示
        const error = promptsResult.reason;
        if (error?.response?.status !== 403 && error?.response?.status !== 500) {
          message.error('加载提示词失败');
        }
        console.error(promptsResult.reason);
        setPrompts([]);
      }

      // 处理执行计划状态
      if (executionPlanResult.status === 'fulfilled') {
        const executionPlan = executionPlanResult.value.data;
        // 如果执行计划存在且状态不是 Draft，则认为存在
        setExecutionPlanExists(!!executionPlan && executionPlan.status !== 'Draft');
      } else {
        setExecutionPlanExists(false);
      }
    } catch (error) {
      // 对403/500等权限/服务器错误不显示提示
      const err = error as any;
      if (err?.response?.status !== 403 && err?.response?.status !== 500) {
        message.error('加载数据失败');
      }
      console.error(error);
    } finally {
      setLoading(false);
      setPromptsLoading(false);
    }
  };

  // 提取文档加载逻辑为独立函数
  const loadDocumentsData = async () => {
    const docTypes: Array<'requirements' | 'design' | 'test'> = ['requirements', 'design', 'test'];
    const promises = docTypes.map(async (docType) => {
      try {
        const doc = await getTaskDocument(projectId, taskId, docType, true); // 启用推荐功能
        return { docType, doc };
      } catch (error) {
        return { docType, doc: { content: '', exists: false } };
      }
    });

    const results = await Promise.all(promises);
    const newDocuments = { ...documents };
    
    results.forEach(({ docType, doc }) => {
      newDocuments[docType] = {
        content: doc.content,
        exists: doc.exists,
        recommendations: doc.recommendations, // 保存推荐数据
      };
    });
    
    return newDocuments;
  };

  // 保留单独的加载函数，用于后续刷新
  const loadTaskInfo = async () => {
    if (!projectId || !taskId) return;
    
    try {
      const result = await getProjectTask(projectId, taskId);
      setTaskInfo(result.data || null);
    } catch (error: any) {
      // 对403/500等权限/服务器错误不显示提示
      if (error?.response?.status !== 403 && error?.response?.status !== 500) {
        message.error('加载任务信息失败');
      }
      console.error(error);
      setTaskInfo(null);
    }
  };

  const loadPrompts = async () => {
    if (!projectId || !taskId) return;
    setPromptsLoading(true);
    
    try {
      const result = await getTaskPrompts(projectId, taskId);
      setPrompts(result.data || []);
    } catch (error: any) {
      // 对403/500等权限/服务器错误不显示提示
      if (error?.response?.status !== 403 && error?.response?.status !== 500) {
        message.error('加载提示词失败');
      }
      console.error(error);
      setPrompts([]);
    } finally {
      setPromptsLoading(false);
    }
  };

  const loadExecutionPlanStatus = async () => {
    if (!projectId || !taskId) return;
    
    try {
      const result = await getExecutionPlan(projectId, taskId);
      // 如果执行计划存在且状态不是 Draft，则认为存在
      setExecutionPlanExists(!!result.data && result.data.status !== 'Draft');
    } catch (error) {
      setExecutionPlanExists(false);
    }
  };

  // 加载推荐文档的内容(加载源章节和目标章节,便于对比)
  const loadRecommendationContent = async (
    recommendedTaskId: string, 
    docType: string, 
    sectionId: string,
    similarity: number,
    sourceSectionId?: string,
    sourceTitle?: string
  ) => {
    setSelectedRecommendation({
      sourceSection: {
        taskId: taskId,
        taskName: '',
        sectionId: sourceSectionId || '',
        title: sourceTitle || '',
        content: ''
      },
      targetSection: {
        taskId: recommendedTaskId,
        taskName: '',
        docType,
        sectionId,
        title: '',
        content: ''
      },
      similarity,
      loading: true
    });

    try {
      // 并行加载所有需要的数据
      const promises: Promise<any>[] = [
        getProjectTask(projectId, taskId),  // 当前任务信息
        getProjectTask(projectId, recommendedTaskId),  // 推荐任务信息
        getTaskSection(projectId, recommendedTaskId, docType as 'requirements' | 'design' | 'test', sectionId, false)  // 目标章节
      ];

      // 如果有源章节ID,也加载源章节内容
      if (sourceSectionId) {
        promises.push(getTaskSection(projectId, taskId, docType as 'requirements' | 'design' | 'test', sourceSectionId, false));
      }

      const results = await Promise.all(promises);
      const [currentTaskResult, targetTaskResult, targetSectionResult, sourceSectionResult] = results;

      setSelectedRecommendation({
        sourceSection: {
          taskId: taskId,
          taskName: currentTaskResult.data?.name || taskId,
          sectionId: sourceSectionId || '',
          title: sourceTitle || '',
          content: sourceSectionResult?.content || ''
        },
        targetSection: {
          taskId: recommendedTaskId,
          taskName: targetTaskResult.data?.name || recommendedTaskId,
          docType,
          sectionId,
          title: targetSectionResult.title || '',
          content: targetSectionResult.content || ''
        },
        similarity,
        loading: false
      });
    } catch (error: any) {
      // 对403/500等权限/服务器错误不显示提示
      if (error?.response?.status !== 403 && error?.response?.status !== 500) {
        message.error('加载推荐文档失败');
      }
      console.error('加载推荐文档失败:', error);
      setSelectedRecommendation(null);
    }
  };

  // 重新加载单个文档
  const reloadDocument = async (docType: 'requirements' | 'design' | 'test') => {
    try {
      const doc = await getTaskDocument(projectId, taskId, docType);
      setDocuments(prev => ({
        ...prev,
        [docType]: {
          content: doc.content,
          exists: doc.exists,
        }
      }));
    } catch (error) {
      console.error(`重新加载${docType}文档失败:`, error);
    }
  };

  // 为每个文档类型创建稳定的编辑回调
  const handleEditRequirements = useCallback((sectionTitle: string) => {
    setSectionEditorModal({
      visible: true,
      docType: 'requirements',
      sectionTitle,
    });
  }, []);

  const handleEditDesign = useCallback((sectionTitle: string) => {
    setSectionEditorModal({
      visible: true,
      docType: 'design',
      sectionTitle,
    });
  }, []);

  const handleEditTest = useCallback((sectionTitle: string) => {
    setSectionEditorModal({
      visible: true,
      docType: 'test',
      sectionTitle,
    });
  }, []);

  // 为每个文档类型创建稳定的复制回调
  const handleCopyRequirements = useCallback((sectionTitle: string) => {
    const copyText = `${taskId}::需求文档::${sectionTitle}`;
    navigator.clipboard.writeText(copyText).then(() => {
      message.success(`已复制: ${copyText}`);
    }).catch(err => {
      console.error('复制失败:', err);
      message.error('复制失败');
    });
  }, [taskId]);

  const handleCopyDesign = useCallback((sectionTitle: string) => {
    const copyText = `${taskId}::设计文档::${sectionTitle}`;
    navigator.clipboard.writeText(copyText).then(() => {
      message.success(`已复制: ${copyText}`);
    }).catch(err => {
      console.error('复制失败:', err);
      message.error('复制失败');
    });
  }, [taskId]);

  const handleCopyTest = useCallback((sectionTitle: string) => {
    const copyText = `${taskId}::测试文档::${sectionTitle}`;
    navigator.clipboard.writeText(copyText).then(() => {
      message.success(`已复制: ${copyText}`);
    }).catch(err => {
      console.error('复制失败:', err);
      message.error('复制失败');
    });
  }, [taskId]);

  // 为每个文档类型创建稳定的MCP资源添加回调
  const handleAddRequirementsToMCP = useCallback((sectionTitle: string) => {
    setMcpResourceModal({
      visible: true,
      docType: 'requirements',
      sectionTitle,
    });
    mcpForm.setFieldsValue({
      name: `${sectionTitle} - ${taskId}`,
      description: `来自任务 ${taskId} 的章节内容`,
    });
  }, [taskId, mcpForm]);

  const handleAddDesignToMCP = useCallback((sectionTitle: string) => {
    setMcpResourceModal({
      visible: true,
      docType: 'design',
      sectionTitle,
    });
    mcpForm.setFieldsValue({
      name: `${sectionTitle} - ${taskId}`,
      description: `来自任务 ${taskId} 的章节内容`,
    });
  }, [taskId, mcpForm]);

  const handleAddTestToMCP = useCallback((sectionTitle: string) => {
    setMcpResourceModal({
      visible: true,
      docType: 'test',
      sectionTitle,
    });
    mcpForm.setFieldsValue({
      name: `${sectionTitle} - ${taskId}`,
      description: `来自任务 ${taskId} 的章节内容`,
    });
  }, [taskId, mcpForm]);

  // 从文档内容中提取章节内容
  const getSectionContent = (content: string, sectionTitle: string): string => {
    const lines = content.split('\n');
    const headingRegex = /^(#{1,6})\s+(.+?)\s*$/;
    
    let startIndex = -1;
    let endIndex = lines.length;
    let currentLevel = 0;

    // 找到当前章节的起始位置
    for (let i = 0; i < lines.length; i++) {
      const m = headingRegex.exec(lines[i]);
      if (m && m[2].trim() === sectionTitle) {
        startIndex = i;
        currentLevel = m[1].length;
        break;
      }
    }

    if (startIndex === -1) return '';

    // 找到下一个同级或更高级标题的位置
    for (let i = startIndex + 1; i < lines.length; i++) {
      const m = headingRegex.exec(lines[i]);
      if (m && m[1].length <= currentLevel) {
        endIndex = i;
        break;
      }
    }

    // 提取内容
    return lines.slice(startIndex, endIndex).join('\n');
  };

  // 提交MCP资源
  const handleSubmitMCPResource = async () => {
    if (!mcpResourceModal.docType || !mcpResourceModal.sectionTitle) return;

    try {
      const values = await mcpForm.validateFields();
      const auth = loadAuth();
      if (!auth) {
        message.error('请先登录');
        return;
      }

      setMcpSaving(true);

      // 获取对应文档类型的内容
      const docContent = documents[mcpResourceModal.docType]?.content || '';
      
      // 获取章节及其子章节的内容
      const sectionContent = getSectionContent(docContent, mcpResourceModal.sectionTitle);

      await addCustomResource(auth.username, {
        name: values.name,
        description: values.description,
        content: sectionContent,
        visibility: 'private',
        projectId: projectId,
        taskId: taskId,
      });

      message.success('已添加到MCP资源');
      setMcpResourceModal({
        visible: false,
        docType: null,
        sectionTitle: null,
      });
      mcpForm.resetFields();
    } catch (error: any) {
      console.error('添加MCP资源失败:', error);
      message.error('添加失败: ' + (error.message || '未知错误'));
    } finally {
      setMcpSaving(false);
    }
  };

  // 刷新整个页面数据
  const refreshPage = async () => {
    if (!projectId || !taskId) return;

    setLoading(true);
    setPromptsLoading(true);

    try {
      // 并行请求所有数据
      const [documentsResult, taskInfoResult, promptsResult, executionPlanResult] = await Promise.allSettled([
        // 1. 加载三个文档（requirements, design, test）
        loadDocumentsData(),
        // 2. 加载任务信息
        getProjectTask(projectId, taskId),
        // 3. 加载提示词
        getTaskPrompts(projectId, taskId),
        // 4. 加载执行计划状态
        getExecutionPlan(projectId, taskId),
      ]);

      // 处理文档数据
      if (documentsResult.status === 'fulfilled') {
        setDocuments(documentsResult.value);
      } else {
        // 对403/500等权限/服务器错误不显示提示，避免影响无权限用户体验
        const error = documentsResult.reason;
        if (error?.response?.status !== 403 && error?.response?.status !== 500) {
          message.error('加载文档失败');
        }
        console.error(documentsResult.reason);
      }

      // 处理任务信息
      if (taskInfoResult.status === 'fulfilled') {
        setTaskInfo(taskInfoResult.value.data || null);
      } else {
        // 对403/500等权限/服务器错误不显示提示
        const error = taskInfoResult.reason;
        if (error?.response?.status !== 403 && error?.response?.status !== 500) {
          message.error('加载任务信息失败');
        }
        console.error(taskInfoResult.reason);
        setTaskInfo(null);
      }

      // 处理提示词
      if (promptsResult.status === 'fulfilled') {
        setPrompts(promptsResult.value.data || []);
      } else {
        // 对403/500等权限/服务器错误不显示提示
        const error = promptsResult.reason;
        if (error?.response?.status !== 403 && error?.response?.status !== 500) {
          message.error('加载提示词失败');
        }
        console.error(promptsResult.reason);
        setPrompts([]);
      }

      // 处理执行计划状态
      if (executionPlanResult.status === 'fulfilled') {
        const executionPlan = executionPlanResult.value.data;
        // 如果执行计划存在且状态不是 Draft，则认为存在
        setExecutionPlanExists(!!executionPlan && executionPlan.status !== 'Draft');
      } else {
        setExecutionPlanExists(false);
      }

      message.success('页面数据已刷新');
    } catch (error) {
      // 对403/500等权限/服务器错误不显示提示
      const err = error as any;
      if (err?.response?.status !== 403 && err?.response?.status !== 500) {
        message.error('刷新页面数据失败');
      }
      console.error(error);
    } finally {
      setLoading(false);
      setPromptsLoading(false);
    }
  };

  // Tag相关回调函数
  const handleCreateTag = async (docType: 'requirements' | 'design' | 'test', tagName: string) => {
    try {
      await tagService.createTag(projectId, taskId, docType, tagName);
      // 成功提示由TagButton组件显示
      setTagRefreshKey(prev => ({ ...prev, [docType]: prev[docType] + 1 })); // 增加刷新键
      // 刷新文档内容
      await reloadDocument(docType);
    } catch (error: any) {
      // 错误会被TagButton组件处理，这里直接抛出
      throw new Error(error.response?.data?.error || error.message || '创建标签失败');
    }
  };

  const handleSwitchTag = async (docType: 'requirements' | 'design' | 'test', tagName: string) => {
    try {
      const response = await tagService.switchTag(projectId, taskId, docType, tagName, false);
      
      if (response.needConfirm) {
        // 需要用户确认
        setPendingSwitchTag({
          docType,
          tagName,
          currentMd5: response.currentMd5,
        });
        setShowConfirmModal(true);
      } else {
        // 直接切换成功
        setSelectedTag(prev => ({ ...prev, [docType]: tagName }));
        setTagRefreshKey(prev => ({ ...prev, [docType]: prev[docType] + 1 })); // 增加刷新键
        await reloadDocument(docType);
        message.success(`已切换到标签版本: ${tagName}`);
      }
    } catch (error: any) {
      message.error(`切换标签失败: ${error.response?.data?.error || error.message || '未知错误'}`);
    }
  };

  const handleConfirmSwitch = async (action: 'create' | 'discard') => {
    if (!pendingSwitchTag) return;

    const { docType, tagName } = pendingSwitchTag;

    try {
      if (action === 'discard') {
        // 放弃修改，强制切换
        const response = await tagService.switchTag(projectId, taskId, docType, tagName, true);
        setSelectedTag(prev => ({ ...prev, [docType]: tagName }));
        setTagRefreshKey(prev => ({ ...prev, [docType]: prev[docType] + 1 })); // 增加刷新键
        await reloadDocument(docType);
        message.success(`已切换到标签版本: ${tagName}`);
      } else if (action === 'create') {
        // 用户选择创建新Tag，关闭确认对话框，让用户通过TagButton创建
        message.info('请使用"创建标签"按钮为当前版本创建新标签');
      }
      
      setShowConfirmModal(false);
      setPendingSwitchTag(null);
    } catch (error: any) {
      message.error(`操作失败: ${error.response?.data?.error || error.message || '未知错误'}`);
    }
  };

  const handleCancelSwitch = () => {
    setShowConfirmModal(false);
    setPendingSwitchTag(null);
  };

  const renderDocument = (docType: 'requirements' | 'design' | 'test') => {
    const doc = documents[docType];
    const isEditMode = editMode[docType];

    // 章节编辑模式
    if (isEditMode) {
      return (
        <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
          <SectionEditor
            key={`${taskId}-${docType}`}
            projectId={projectId}
            taskId={taskId}
            docType={docType}
            onCancel={() => {
              setEditMode(prev => ({ ...prev, [docType]: false }));
            }}
            onSave={() => {
              reloadDocument(docType);
            }}
          />
        </div>
      );
    }

    // 预览模式 - 空文档
    if (!doc.exists && !doc.content) {
      return (
        <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
          <div style={{ marginBottom: 12 }}>
            <Space size="middle">
              <Button
                type="primary"
                icon={<EditOutlined />}
                onClick={() => setEditMode(prev => ({ ...prev, [docType]: true }))}
                size="small"
              >
                编辑
              </Button>
              <TagButton
                onCreateTag={(tagName) => handleCreateTag(docType, tagName)}
                docType={docType}
                size="small"
              />
              <TagVersionSelect
                key={`tag-select-empty-${projectId}-${taskId}-${docType}`}
                projectId={projectId}
                taskId={taskId}
                docType={docType}
                currentVersion={selectedTag[docType]}
                onSwitchTag={(tagName) => handleSwitchTag(docType, tagName)}
                onTagDeleted={() => setTagRefreshKey(prev => ({ ...prev, [docType]: prev[docType] + 1 }))}
                refreshKey={tagRefreshKey[docType]}
                size="small"
              />
            </Space>
          </div>
          <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
            <FileTextOutlined style={{ fontSize: 48, marginBottom: 16, color: '#d9d9d9' }} />
            <div style={{ marginBottom: 16 }}>暂无{getDocumentTitle(docType)}</div>
            <div style={{ color: '#bbb' }}>点击上方「编辑」按钮创建文档</div>
          </div>
        </div>
      );
    }

    // 预览模式 - 显示全文预览
    return (
      <div style={{ height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        <div style={{ marginBottom: 12, flexShrink: 0, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Space size="middle">
            <Button
              type="primary"
              icon={<EditOutlined />}
              onClick={() => setEditMode(prev => ({ ...prev, [docType]: true }))}
              size="small"
            >
              编辑
            </Button>
            <TagButton
              onCreateTag={(tagName) => handleCreateTag(docType, tagName)}
              docType={docType}
              size="small"
            />
            <TagVersionSelect
              key={`tag-select-${projectId}-${taskId}-${docType}`}
              projectId={projectId}
              taskId={taskId}
              docType={docType}
              currentVersion={selectedTag[docType]}
              onSwitchTag={(tagName) => handleSwitchTag(docType, tagName)}
              onTagDeleted={() => setTagRefreshKey(prev => ({ ...prev, [docType]: prev[docType] + 1 }))}
              refreshKey={tagRefreshKey[docType]}
              size="small"
            />
          </Space>
          
          {/* 推荐按钮（仅在有推荐时显示） */}
          {doc.recommendations && doc.recommendations.length > 0 && (
            <Badge count={doc.recommendations.length} offset={[-5, 5]} color="#52c41a">
              <Button
                type="default"
                icon={<BulbOutlined style={{ color: '#faad14' }} />}
                onClick={() => {
                  setCurrentRecommendations(doc.recommendations || []);
                  setRecommendationDrawerOpen(true);
                }}
                size="small"
                style={{ borderColor: '#faad14', color: '#faad14' }}
              >
                查看相似推荐
              </Button>
            </Badge>
          )}
        </div>
        <div style={{ flex: 1, minHeight: 0, display: 'flex', gap: 12 }}>
            {/* 固定左侧目录导航 */}
            <div style={{
              width: 260,
              flexShrink: 0,
              position: 'sticky',
              top: 0,
              alignSelf: 'flex-start',
              maxHeight: 'calc(100vh - 150px)',
              display: 'flex',
              flexDirection: 'column',
              border: '1px solid #f0f0f0',
              borderRadius: 6,
              background: '#fff'
            }}>
              <div style={{
                padding: '8px 12px',
                borderBottom: '1px solid #f5f5f5',
                fontWeight: 500,
                fontSize: 12,
                background: '#fafafa',
                flexShrink: 0
              }}>目录</div>
              <div style={{
                flex: 1,
                overflowY: 'auto',
                overflowX: 'hidden',
                padding: '8px 12px',
                minHeight: 0
              }}>
                <DocumentTOC 
                  key={`toc-${docType}-${projectId}-${taskId}`}
                  content={doc.content} 
                  projectId={projectId}
                  taskId={taskId}
                  docType={docType as 'requirements' | 'design' | 'test'}
                  onEditSection={
                    docType === 'requirements' ? handleEditRequirements :
                    docType === 'design' ? handleEditDesign :
                    handleEditTest
                  }
                />
              </div>
            </div>
            <div style={{
              flex: 1,
              minHeight: 0,
              display: 'flex',
              flexDirection: 'column',
              border: '1px solid #f0f0f0',
              borderRadius: 6,
              backgroundColor: '#fafafa'
            }}>
              <div style={{
                flex: 1,
                overflowY: 'auto',
                overflowX: 'hidden',
                padding: '16px'
              }}>
                <MarkdownViewer 
                  key={`markdown-${docType}-${projectId}-${taskId}`}
                  showFullscreenButton={docType === 'requirements' || docType === 'design'}
                  onEditSection={
                    docType === 'requirements' ? handleEditRequirements :
                    docType === 'design' ? handleEditDesign :
                    handleEditTest
                  }
                  onCopySectionName={
                    docType === 'requirements' ? handleCopyRequirements :
                    docType === 'design' ? handleCopyDesign :
                    handleCopyTest
                  }
                  onAddToMCP={
                    docType === 'requirements' ? handleAddRequirementsToMCP :
                    docType === 'design' ? handleAddDesignToMCP :
                    handleAddTestToMCP
                  }
                >
                  {doc.content}
                </MarkdownViewer>
              </div>
            </div>
        </div>
      </div>
    );
  };

  const getDocumentTitle = (docType: string) => {
    switch (docType) {
      case 'requirements': return '需求文档';
      case 'design': return '设计文档';
      case 'test': return '测试文档';
      default: return '文档';
    }
  };

  const getStatusColor = (status?: string) => {
    switch (status) {
      case 'todo': return 'default';
      case 'in-progress': return 'processing';
      case 'completed': return 'success';
      case 'cancelled': return 'error';
      default: return 'default';
    }
  };

  const getStatusText = (status?: string) => {
    switch (status) {
      case 'todo': return '待开始';
      case 'in-progress': return '进行中';
      case 'completed': return '已完成';
      case 'cancelled': return '已取消';
      default: return status || '未设置';
    }
  };

  const renderTaskInfo = () => {
    if (!taskInfo) {
      return (
        <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
          <InfoCircleOutlined style={{ fontSize: 48, marginBottom: 16, color: '#d9d9d9' }} />
          <div>加载任务信息中...</div>
        </div>
      );
    }

    return (
      <div style={{ padding: '16px 0' }}>
        <Descriptions
          title={
            <Space>
              <InfoCircleOutlined />
              任务基本信息
            </Space>
          }
          bordered
          column={1}
          size="middle"
        >
          <Descriptions.Item label="任务名称">
            <strong style={{ fontSize: 16 }}>{taskInfo.name}({taskInfo.id})</strong>
          </Descriptions.Item>
          
          <Descriptions.Item label="任务状态">
            <Tag color={getStatusColor(taskInfo.status)}>
              {getStatusText(taskInfo.status)}
            </Tag>
          </Descriptions.Item>
          
          {taskInfo.assignee && (
            <Descriptions.Item label="负责人">
              <Space>
                <UserOutlined />
                {taskInfo.assignee}
              </Space>
            </Descriptions.Item>
          )}
          
          {taskInfo.module && (
            <Descriptions.Item label="所属模块">
              <Space>
                <AppstoreOutlined />
                {taskInfo.module}
              </Space>
            </Descriptions.Item>
          )}
          
          {taskInfo.feature_id && (
            <Descriptions.Item label="关联特性ID">
              <Tag>{taskInfo.feature_id}</Tag>
            </Descriptions.Item>
          )}
          
          {taskInfo.feature_name && (
            <Descriptions.Item label="特性名称">
              {taskInfo.feature_name}
            </Descriptions.Item>
          )}
          
          <Descriptions.Item label="创建时间">
            <Space>
              <ClockCircleOutlined />
              {new Date(taskInfo.created_at).toLocaleString('zh-CN')}
            </Space>
          </Descriptions.Item>
          
          <Descriptions.Item label="更新时间">
            <Space>
              <ClockCircleOutlined />
              {new Date(taskInfo.updated_at).toLocaleString('zh-CN')}
            </Space>
          </Descriptions.Item>
          
          {taskInfo.description && (
            <Descriptions.Item label="任务描述">
              <div style={{ 
                backgroundColor: '#fafafa',
                padding: 12,
                borderRadius: 6,
                border: '1px solid #f0f0f0'
              }}>
                <MarkdownViewer>{taskInfo.description}</MarkdownViewer>
              </div>
            </Descriptions.Item>
          )}
        </Descriptions>
      </div>
    );
  };

  const renderPrompts = () => {
    return (
      <div style={{ padding: '16px' }}>
        <Spin spinning={promptsLoading}>
          {prompts.length === 0 ? (
            <div style={{ textAlign: 'center', color: '#999', padding: '40px 0' }}>
              <MessageOutlined style={{ fontSize: '48px', marginBottom: '16px' }} />
              <div>暂无提示词记录</div>
            </div>
          ) : (
            <List
              dataSource={prompts}
              renderItem={(prompt) => (
                <List.Item>
                  <Card
                    style={{ width: '100%' }}
                    size="small"
                    title={
                      <Space>
                        <UserOutlined />
                        <Typography.Text strong>{prompt.username}</Typography.Text>
                        <Typography.Text type="secondary">
                          {new Date(prompt.created_at).toLocaleString('zh-CN')}
                        </Typography.Text>
                      </Space>
                    }
                  >
                    <Typography.Paragraph
                      style={{
                        margin: 0,
                        whiteSpace: 'pre-wrap',
                        backgroundColor: '#fafafa',
                        padding: '12px',
                        borderRadius: '6px',
                        border: '1px solid #f0f0f0'
                      }}
                    >
                      {prompt.content}
                    </Typography.Paragraph>
                  </Card>
                </List.Item>
              )}
            />
          )}
        </Spin>
      </div>
    );
  };

  const tabItems = [
    {
      key: 'info',
      label: (
        <span>
          <InfoCircleOutlined />
          基本信息
        </span>
      ),
      children: (
        <div>
          {renderTaskInfo()}
          {/* 任务总结区域 */}
          <div style={{ marginTop: 24, paddingLeft: 16, paddingRight: 16, paddingBottom: 16 }}>
            <TaskSummaryPanel projectId={projectId} taskId={taskId} />
          </div>
        </div>
      ),
    },
    {
      key: 'requirements',
      label: (
        <span>
          <FileTextOutlined />
          需求文档
          {documents.requirements.exists && <span style={{ color: '#52c41a', marginLeft: 4 }}>●</span>}
        </span>
      ),
      children: <div key={`requirements-${refreshTrigger}`}>{renderDocument('requirements')}</div>,
    },
    {
      key: 'design',
      label: (
        <span>
          <FileTextOutlined />
          设计文档
          {documents.design.exists && <span style={{ color: '#52c41a', marginLeft: 4 }}>●</span>}
        </span>
      ),
      children: <div key={`design-${refreshTrigger}`}>{renderDocument('design')}</div>,
    },
    {
      key: 'execution-plan',
      label: (
        <span>
          <ProjectOutlined />
          执行计划
          {executionPlanExists && <span style={{ color: '#52c41a', marginLeft: 4 }}>●</span>}
        </span>
      ),
      children: (
        <div style={{ padding: '16px' }}>
          <ExecutionPlanView
            projectId={projectId}
            taskId={taskId}
          />
        </div>
      )
    },
    {
      key: 'test',
      label: (
        <span>
          <FileTextOutlined />
          测试文档
          {documents.test.exists && <span style={{ color: '#52c41a', marginLeft: 4 }}>●</span>}
        </span>
      ),
      children: <div key={`test-${refreshTrigger}`}>{renderDocument('test')}</div>,
    },
    // lite 模式下隐藏提示词记录、历史记录、关联文档
    ...(!liteMode ? [
      {
        key: 'prompts',
        label: (
          <span>
            <MessageOutlined />
            提示词记录
            {prompts.length > 0 && <span style={{ color: '#1890ff', marginLeft: 4 }}>({prompts.length})</span>}
          </span>
        ),
        children: renderPrompts(),
      },
      {
        key: 'incremental',
        label: (
          <span>
            <FileTextOutlined />
            历史记录
          </span>
        ),
        children: <div style={{height:'70vh'}}><TaskDocIncremental projectId={projectId} taskId={taskId} /></div>
      },
      {
        key: 'documents',
        label: (
          <span>
            <FolderOutlined />
            关联文档
          </span>
        ),
        children: (
          <div style={{ height: '80vh', padding: '16px 0' }}>
            <TaskLinkedDocuments
              projectId={projectId}
              taskId={taskId}
            />
          </div>
        )
      }
    ] : [])
  ];

  if (!taskId) {
    return (
      <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
        请选择一个任务查看文档
      </div>
    );
  }

  return (
    <>
      <Spin spinning={loading}>
        <div style={{ height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
          <Tabs
            activeKey={activeTab}
            onChange={(key) => setActiveTab(key as any)}
            items={tabItems}
            destroyInactiveTabPane={true}
            style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}
            tabBarStyle={{ margin: 0, paddingLeft: 16, marginBottom: 5 }}
            tabBarExtraContent={{
              right: (
                <Button
                  type="primary"
                  icon={<ReloadOutlined />}
                  onClick={refreshPage}
                  loading={loading}
                  size="small"
                >
                  刷新页面
                </Button>
              )
            }}
          />
        </div>

        {/* 推荐抽屉 */}
      <Drawer
        title={
          selectedRecommendation ? (
            <Space>
              <Button 
                type="text" 
                icon={<BulbOutlined />} 
                size="small"
                onClick={() => setSelectedRecommendation(null)}
              >
                返回推荐列表
              </Button>
            </Space>
          ) : (
            <Space>
              <BulbOutlined style={{ color: '#1890ff' }} />
              <span>相似历史参考（基于语义检索）</span>
              <Tag color="blue">{currentRecommendations.length}条推荐</Tag>
            </Space>
          )
        }
        placement="right"
        width={650}
        open={recommendationDrawerOpen}
        onClose={() => {
          setRecommendationDrawerOpen(false);
          setSelectedRecommendation(null);
        }}
        styles={{
          body: { padding: selectedRecommendation ? '0' : '16px' }
        }}
        extra={
          <Button size="small" onClick={() => {
            setRecommendationDrawerOpen(false);
            setSelectedRecommendation(null);
          }}>
            关闭
          </Button>
        }
      >
        {selectedRecommendation ? (
          // 显示选中的推荐文档内容(源章节 vs 目标章节对比)
          <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
            {/* 相似度信息 */}
            <div style={{ 
              padding: '12px 16px', 
              background: '#e6f7ff',
              borderBottom: '1px solid #91d5ff',
              textAlign: 'center'
            }}>
              <Text strong style={{ fontSize: 14, color: '#1890ff' }}>
                相似度: {(selectedRecommendation.similarity * 100).toFixed(1)}%
              </Text>
            </div>

            {selectedRecommendation.loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>
                <Spin tip="加载文档内容..." />
              </div>
            ) : (
              <div style={{ 
                flex: 1, 
                overflow: 'auto',
                display: 'grid',
                gridTemplateColumns: '1fr 1fr',
                gap: '1px',
                background: '#f0f0f0'
              }}>
                {/* 源章节(当前任务) */}
                <div style={{ background: '#fff', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
                  <div style={{ 
                    padding: '12px 16px', 
                    borderBottom: '1px solid #f0f0f0',
                    background: '#fafafa',
                    flexShrink: 0
                  }}>
                    <div style={{ marginBottom: 6 }}>
                      <Tag color="blue">当前任务</Tag>
                      <Text strong style={{ fontSize: 14 }}>
                        {selectedRecommendation.sourceSection.taskName}
                      </Text>
                    </div>
                    <div>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {selectedRecommendation.sourceSection.title}
                      </Text>
                    </div>
                  </div>
                  <div style={{ flex: 1, overflow: 'auto', padding: '16px' }}>
                    {selectedRecommendation.sourceSection.content ? (
                      <MarkdownViewer>{selectedRecommendation.sourceSection.content}</MarkdownViewer>
                    ) : (
                      <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>
                        暂无源章节内容
                      </div>
                    )}
                  </div>
                </div>

                {/* 目标章节(推荐任务) */}
                <div style={{ background: '#fff', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
                  <div style={{ 
                    padding: '12px 16px', 
                    borderBottom: '1px solid #f0f0f0',
                    background: '#fafafa',
                    flexShrink: 0
                  }}>
                    <div style={{ marginBottom: 6 }}>
                      <Tag color="green">推荐任务</Tag>
                      <Text strong style={{ fontSize: 14 }}>
                        {selectedRecommendation.targetSection.taskName}
                      </Text>
                    </div>
                    <div>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {selectedRecommendation.targetSection.title}
                      </Text>
                    </div>
                    <div style={{ marginTop: 4 }}>
                      <Tag color="orange" style={{ fontSize: 11 }}>
                        {selectedRecommendation.targetSection.docType === 'requirements' ? '需求文档' : 
                         selectedRecommendation.targetSection.docType === 'design' ? '设计文档' : '测试文档'}
                      </Tag>
                      <Tag color="purple" style={{ fontSize: 11 }}>
                        {selectedRecommendation.targetSection.sectionId}
                      </Tag>
                    </div>
                  </div>
                  <div style={{ flex: 1, overflow: 'auto', padding: '16px' }}>
                    <MarkdownViewer>{selectedRecommendation.targetSection.content}</MarkdownViewer>
                  </div>
                </div>
              </div>
            )}
          </div>
        ) : (
          // 显示推荐列表
          currentRecommendations.length > 0 ? (
            <>
              <div style={{ marginBottom: 12, padding: '8px 12px', background: '#f0f7ff', borderRadius: 4, fontSize: 12, color: '#666' }}>
                💡 点击标题可查看推荐文档的详细内容
              </div>
              <RecommendationPanel
                recommendations={currentRecommendations}
                projectId={projectId}
                inDrawer={true}
                onRecommendationClick={(recommendedTaskId, sectionId) => {
                  const recommendation = currentRecommendations.find(
                    r => r.task_id === recommendedTaskId && r.section_id === sectionId
                  );
                  if (recommendation) {
                    loadRecommendationContent(
                      recommendedTaskId, 
                      recommendation.doc_type, 
                      sectionId,
                      recommendation.similarity,
                      recommendation.source_section_id,
                      recommendation.source_title
                    );
                  }
                }}
              />
            </>
          ) : (
            <div style={{ textAlign: 'center', padding: '40px 20px', color: '#999' }}>
              <BulbOutlined style={{ fontSize: 48, marginBottom: 16, opacity: 0.3 }} />
              <div>暂无相似推荐</div>
            </div>
          )
        )}
      </Drawer>

      {/* Tag切换确认对话框 */}
      <TagConfirmModal
        visible={showConfirmModal}
        currentMd5={pendingSwitchTag?.currentMd5}
        targetTag={pendingSwitchTag?.tagName}
        onConfirm={handleConfirmSwitch}
        onCancel={handleCancelSwitch}
      />
      </Spin>

      {/* 章节编辑弹窗 */}
      <Modal
        title={`编辑章节 - ${sectionEditorModal.docType === 'requirements' ? '需求文档' : sectionEditorModal.docType === 'design' ? '设计文档' : '测试文档'}`}
        open={sectionEditorModal.visible}
        onCancel={() => {
          // 如果有未保存的更改，弹出确认对话框
          if (sectionEditorHasUnsavedChanges) {
            Modal.confirm({
              title: '未保存的更改',
              content: '当前有未保存的更改，关闭将丢失这些更改。确认关闭吗？',
              okText: '确认关闭',
              cancelText: '继续编辑',
              okType: 'danger',
              onOk: () => {
                setSectionEditorModal({ visible: false, docType: null, sectionTitle: null });
                setSectionEditorHasUnsavedChanges(false);
              }
            });
          } else {
            setSectionEditorModal({ visible: false, docType: null, sectionTitle: null });
          }
        }}
        width="90%"
        style={{ top: 20 }}
        footer={null}
        destroyOnClose
      >
        {sectionEditorModal.visible && sectionEditorModal.docType && (
          <div style={{ height: 'calc(100vh - 150px)', display: 'flex', flexDirection: 'column' }}>
            <SectionEditor
              key={`modal-${taskId}-${sectionEditorModal.docType}-${sectionEditorModal.sectionTitle || ''}`}
              projectId={projectId}
              taskId={taskId}
              docType={sectionEditorModal.docType}
              initialSectionTitle={sectionEditorModal.sectionTitle || undefined}
              onCancel={() => {
                setSectionEditorModal({ visible: false, docType: null, sectionTitle: null });
                setSectionEditorHasUnsavedChanges(false);
              }}
              onSave={() => {
                if (sectionEditorModal.docType) {
                  reloadDocument(sectionEditorModal.docType);
                }
                setSectionEditorModal({ visible: false, docType: null, sectionTitle: null });
                setSectionEditorHasUnsavedChanges(false);
              }}
              onUnsavedChanges={setSectionEditorHasUnsavedChanges}
            />
          </div>
        )}
      </Modal>

      {/* MCP资源添加弹窗 */}
      <Modal
        title="添加到MCP资源"
        open={mcpResourceModal.visible}
        onCancel={() => {
          setMcpResourceModal({ visible: false, docType: null, sectionTitle: null });
          mcpForm.resetFields();
        }}
        onOk={handleSubmitMCPResource}
        confirmLoading={mcpSaving}
        okText="添加"
        cancelText="取消"
      >
        <Form
          form={mcpForm}
          layout="vertical"
        >
          <Form.Item
            name="name"
            label="资源名称"
            rules={[{ required: true, message: '请输入资源名称' }]}
          >
            <Input placeholder="请输入资源名称" />
          </Form.Item>
          <Form.Item
            name="description"
            label="资源描述"
          >
            <Input.TextArea
              rows={3}
              placeholder="请输入资源描述（可选）"
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 返回顶部浮动按钮 */}
      {showBackTop && (
        <Button
          type="primary"
          shape="circle"
          icon={<VerticalAlignTopOutlined />}
          size="large"
          onClick={scrollToTop}
          style={{
            position: 'fixed',
            right: 24,
            bottom: 24,
            zIndex: 9999,
            width: 40,
            height: 40,
            boxShadow: '0 2px 8px rgba(0, 0, 0, 0.15)',
          }}
          title="返回顶部"
        />
      )}
    </>
  );
};

export default TaskDocuments;