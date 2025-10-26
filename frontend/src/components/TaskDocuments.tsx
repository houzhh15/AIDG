import React, { useState, useEffect } from 'react';
import { Tabs, Spin, Button, message, Descriptions, Tag, Space, Typography, List, Card } from 'antd';
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
} from '@ant-design/icons';
import { getTaskDocument, saveTaskDocument, getProjectTask, getTaskPrompts, getExecutionPlan, ProjectTask, TaskPrompt } from '../api/tasks';
import MarkdownViewer from './MarkdownViewer';
import ExecutionPlanView from './ExecutionPlanView';
import TaskDocIncremental from './TaskDocIncremental';
import TaskLinkedDocuments from './TaskLinkedDocuments';
import SectionEditor from './SectionEditor';
import DocumentTOC from './DocumentTOC';
import TaskSummaryPanel from './TaskSummaryPanel';

interface Props {
  projectId: string;
  taskId: string;
}

const TaskDocuments: React.FC<Props> = ({ projectId, taskId }) => {
  const [activeTab, setActiveTab] = useState<'info' | 'requirements' | 'design' | 'test' | 'prompts' | 'incremental' | 'documents' | 'execution-plan'>('info');
  const [documents, setDocuments] = useState<Record<string, { content: string; exists: boolean }>>({
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

  useEffect(() => {
    if (projectId && taskId) {
      // 并行加载所有数据，提升页面加载速度
      loadAllData();
    }
  }, [projectId, taskId]);

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
        message.error('加载文档失败');
        console.error(documentsResult.reason);
      }

      // 处理任务信息
      if (taskInfoResult.status === 'fulfilled') {
        setTaskInfo(taskInfoResult.value.data || null);
      } else {
        message.error('加载任务信息失败');
        console.error(taskInfoResult.reason);
        setTaskInfo(null);
      }

      // 处理提示词
      if (promptsResult.status === 'fulfilled') {
        setPrompts(promptsResult.value.data || []);
      } else {
        message.error('加载提示词失败');
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
      message.error('加载数据失败');
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
        const doc = await getTaskDocument(projectId, taskId, docType);
        return { docType, doc };
      } catch (error) {
        return { docType, doc: { content: '', exists: false } };
      }
    });

    const results = await Promise.all(promises);
    const newDocuments = { ...documents };
    
    results.forEach(({ docType, doc }) => {
      newDocuments[docType] = {
        ...newDocuments[docType],
        content: doc.content,
        exists: doc.exists,
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
    } catch (error) {
      message.error('加载任务信息失败');
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
    } catch (error) {
      message.error('加载提示词失败');
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

  const renderDocument = (docType: 'requirements' | 'design' | 'test') => {
    const doc = documents[docType];
    const isEditMode = editMode[docType];

    // 章节编辑模式
    if (isEditMode) {
      return (
        <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
          <SectionEditor
            projectId={projectId}
            taskId={taskId}
            docType={docType}
            onCancel={() => setEditMode(prev => ({ ...prev, [docType]: false }))}
            onSave={() => reloadDocument(docType)}
          />
        </div>
      );
    }

    // 预览模式 - 空文档
    if (!doc.exists && !doc.content) {
      return (
        <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
          <div style={{ marginBottom: 12 }}>
            <Button
              type="primary"
              icon={<EditOutlined />}
              onClick={() => setEditMode(prev => ({ ...prev, [docType]: true }))}
              size="small"
            >
              编辑
            </Button>
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
        <div style={{ marginBottom: 12, flexShrink: 0 }}>
          <Button
            type="primary"
            icon={<EditOutlined />}
            onClick={() => setEditMode(prev => ({ ...prev, [docType]: true }))}
            size="small"
          >
            编辑
          </Button>
        </div>
        <div style={{ flex: 1, minHeight: 0, display: 'flex', gap: 12 }}>
            {/* 固定左侧目录导航 */}
            <div style={{
              width: 260,
              flexShrink: 0,
              position: 'sticky',
              top: 0,
              alignSelf: 'flex-start',
              maxHeight: '100vh',
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
                  content={doc.content} 
                  projectId={projectId}
                  taskId={taskId}
                  docType={docType as 'requirements' | 'design' | 'test'}
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
                <MarkdownViewer>{doc.content}</MarkdownViewer>
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
                whiteSpace: 'pre-wrap',
                backgroundColor: '#fafafa',
                padding: 12,
                borderRadius: 6,
                border: '1px solid #f0f0f0'
              }}>
                {taskInfo.description}
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
      children: renderDocument('requirements'),
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
      children: renderDocument('design'),
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
      children: renderDocument('test'),
    },
    {
      key: 'prompts',
      label: (
        <span>
          <MessageOutlined />
          提示词
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
  ];

  if (!taskId) {
    return (
      <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
        请选择一个任务查看文档
      </div>
    );
  }

  return (
    <Spin spinning={loading}>
      <div style={{ height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        <Tabs
          activeKey={activeTab}
          onChange={(key) => setActiveTab(key as any)}
          items={tabItems}
          style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}
          tabBarStyle={{ margin: 0, paddingLeft: 16 }}
        />
      </div>
    </Spin>
  );
};

export default TaskDocuments;