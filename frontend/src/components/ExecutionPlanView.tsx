import React, { useState, useEffect } from 'react';
import { Card, Spin, Alert, Empty, Descriptions, Tag, Timeline, Space, Button, message, Modal, Popconfirm, Tooltip } from 'antd';
import { 
  CheckCircleOutlined, 
  ClockCircleOutlined, 
  CloseCircleOutlined, 
  SyncOutlined, 
  ExclamationCircleOutlined,
  CheckOutlined,
  CloseOutlined,
  SendOutlined,
  RollbackOutlined,
  EditOutlined,
  SaveOutlined,
  PlusOutlined,
  DeleteOutlined,
  InsertRowBelowOutlined
} from '@ant-design/icons';
import { 
  getExecutionPlan, 
  submitExecutionPlan,
  approveExecutionPlan, 
  rejectExecutionPlan,
  restoreApproval,
  updateExecutionPlanContent,
  resetExecutionPlan,
  ExecutionPlan, 
  ExecutionPlanStep 
} from '../api/tasks';
import MarkdownViewer from './MarkdownViewer';
import PlanEditor from './ExecutionPlan/PlanEditor';
import StepEditorModal, { StepFormData, StepEditMode } from './StepEditorModal';
import { 
  buildMarkdown, 
  renumberSteps, 
  parseStepsFromMarkdown,
  mergeFrontmatter,
  ExecutionPlanFrontmatter,
  ExecutionPlanStep as BuilderStep
} from '../utils/planMarkdownBuilder';
import { TagButton, TagVersionSelect, TagConfirmModal } from './TagManagement';
import { tagService } from '../services/tagService';

interface Props {
  projectId: string;
  taskId: string;
}

const ExecutionPlanView: React.FC<Props> = ({ projectId, taskId }) => {
  const [plan, setPlan] = useState<ExecutionPlan | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [approving, setApproving] = useState(false);
  const [rejecting, setRejecting] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [selectedStep, setSelectedStep] = useState<ExecutionPlanStep | null>(null);
  const [detailModalVisible, setDetailModalVisible] = useState(false);
  const [isEditMode, setIsEditMode] = useState(false);
  const [editorContent, setEditorContent] = useState<string>('');
  const [saving, setSaving] = useState(false);
  const [taskAssignee, setTaskAssignee] = useState<string>('');
  const [currentUser, setCurrentUser] = useState<string>('');
  
  // 步骤编辑器状态
  const [stepModalVisible, setStepModalVisible] = useState(false);
  const [stepModalMode, setStepModalMode] = useState<StepEditMode>('create');
  const [editingStep, setEditingStep] = useState<ExecutionPlanStep | null>(null);
  const [insertPosition, setInsertPosition] = useState<number | undefined>(undefined);
  const [resetting, setResetting] = useState(false);

  // Tag版本管理状态
  const [selectedTag, setSelectedTag] = useState<string>('当前版本');
  const [tagRefreshKey, setTagRefreshKey] = useState<number>(0);
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const [pendingSwitchTag, setPendingSwitchTag] = useState<{
    tagName: string;
    currentMd5?: string;
  } | null>(null);

  // 辅助函数: 将字面量转义字符转换为实际字符
  const unescapeText = (text: string): string => {
    if (!text) return '';
    return text
      .replace(/\\n/g, '\n')    // 换行符
      .replace(/\\t/g, '\t')    // 制表符
      .replace(/\\r/g, '\r')    // 回车符
      .replace(/\\\\/g, '\\');  // 反斜杠本身
  };

  useEffect(() => {
    if (projectId && taskId) {
      loadExecutionPlan();
    }
  }, [projectId, taskId]);

  const loadExecutionPlan = async () => {
    if (!projectId || !taskId) return;
    
    setLoading(true);
    setError(null);
    
    try {
      const result = await getExecutionPlan(projectId, taskId);
      setPlan(result.data || null);
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error || '加载执行计划失败';
      setError(errorMsg);
      setPlan(null);
    } finally {
      setLoading(false);
    }
  };

  const handleApprove = async () => {
    if (!projectId || !taskId) return;
    
    setApproving(true);
    try {
      await approveExecutionPlan(projectId, taskId, { comment: '批准执行计划' });
      message.success('执行计划已批准');
      await loadExecutionPlan(); // 重新加载以获取最新状态
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error || '批准失败';
      message.error(errorMsg);
    } finally {
      setApproving(false);
    }
  };

  const handleReject = async () => {
    if (!projectId || !taskId) return;
    
    Modal.confirm({
      title: '确认拒绝执行计划',
      content: '请输入拒绝原因',
      okText: '确认拒绝',
      cancelText: '取消',
      onOk: async () => {
        setRejecting(true);
        try {
          await rejectExecutionPlan(projectId, taskId, { 
            comment: '拒绝执行计划',
            reason: '需要进一步修改'
          });
          message.success('执行计划已拒绝');
          await loadExecutionPlan();
        } catch (err: any) {
          const errorMsg = err?.response?.data?.error || '拒绝失败';
          message.error(errorMsg);
        } finally {
          setRejecting(false);
        }
      }
    });
  };

  const handleSubmit = async () => {
    if (!projectId || !taskId) return;
    
    setSubmitting(true);
    try {
      await submitExecutionPlan(projectId, taskId, { comment: '提交执行计划审核' });
      message.success('执行计划已提交审核');
      await loadExecutionPlan(); // 重新加载以获取最新状态
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error || '提交失败';
      message.error(errorMsg);
    } finally {
      setSubmitting(false);
    }
  };

  const handleRestore = async () => {
    if (!projectId || !taskId) return;
    
    Modal.confirm({
      title: '确认恢复审核',
      content: '确定要将执行计划恢复到待审批状态吗？',
      okText: '确认恢复',
      cancelText: '取消',
      onOk: async () => {
        setRestoring(true);
        try {
          await restoreApproval(projectId, taskId, { comment: '恢复执行计划审核' });
          message.success('执行计划已恢复到待审批状态');
          await loadExecutionPlan(); // 重新加载以获取最新状态
        } catch (err: any) {
          const errorMsg = err?.response?.data?.error || '恢复失败';
          message.error(errorMsg);
        } finally {
          setRestoring(false);
        }
      }
    });
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'succeeded':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'in-progress':
        return <SyncOutlined spin style={{ color: '#1890ff' }} />;
      case 'failed':
        return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
      case 'cancelled':
        return <ExclamationCircleOutlined style={{ color: '#faad14' }} />;
      case 'pending':
      default:
        return <ClockCircleOutlined style={{ color: '#d9d9d9' }} />;
    }
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      'Pending Approval': { color: 'gold', text: '待审批' },
      'Approved': { color: 'green', text: '已批准' },
      'Rejected': { color: 'red', text: '已拒绝' },
      'Executing': { color: 'blue', text: '执行中' },
      'Completed': { color: 'success', text: '已完成' },
      'Failed': { color: 'error', text: '失败' }
    };
    
    const config = statusMap[status] || { color: 'default', text: status };
    return <Tag color={config.color}>{config.text}</Tag>;
  };

  const getStepStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      'pending': { color: 'default', text: '待执行' },
      'in-progress': { color: 'processing', text: '执行中' },
      'succeeded': { color: 'success', text: '成功' },
      'failed': { color: 'error', text: '失败' },
      'cancelled': { color: 'warning', text: '已取消' }
    };
    
    const config = statusMap[status] || { color: 'default', text: status };
    return <Tag color={config.color}>{config.text}</Tag>;
  };

  const showStepDetail = (step: ExecutionPlanStep) => {
    setSelectedStep(step);
    setDetailModalVisible(true);
  };

  // 检查计划状态是否可编辑
  const isEditableState = (status: string): boolean => {
    // 允许编辑已完成的计划（如需要存档管理，请移除 'Completed'）
    return ['Draft', 'Pending Approval', 'Rejected', 'Approved', 'Executing', 'Completed'].includes(status);
  };

  // ========== Tag版本管理功能 ==========
  
  // 创建Tag
  const handleCreateTag = async (tagName: string) => {
    try {
      await tagService.createExecutionPlanTag(projectId, taskId, tagName);
      message.success(`标签 "${tagName}" 创建成功`);
      // 刷新tag列表
      setTagRefreshKey(prev => prev + 1);
    } catch (error: any) {
      const errorMsg = error?.response?.data?.error || error.message || '创建标签失败';
      message.error(errorMsg);
      throw error;
    }
  };

  // 切换Tag
  const handleSwitchTag = async (tagName: string) => {
    try {
      const result = await tagService.switchExecutionPlanTag(projectId, taskId, tagName, false);
      
      if (result.needConfirm) {
        // 需要用户确认
        setPendingSwitchTag({
          tagName,
          currentMd5: result.currentMd5
        });
        setShowConfirmModal(true);
      } else {
        // 直接切换成功
        setSelectedTag(tagName);
        message.success(`已切换到标签: ${tagName}`);
        await loadExecutionPlan();
      }
    } catch (error: any) {
      const errorMsg = error?.response?.data?.error || error.message || '切换标签失败';
      message.error(errorMsg);
    }
  };

  // 确认强制切换Tag
  const handleConfirmSwitch = async () => {
    if (!pendingSwitchTag) return;
    
    try {
      await tagService.switchExecutionPlanTag(projectId, taskId, pendingSwitchTag.tagName, true);
      setSelectedTag(pendingSwitchTag.tagName);
      message.success(`已切换到标签: ${pendingSwitchTag.tagName}`);
      await loadExecutionPlan();
    } catch (error: any) {
      const errorMsg = error?.response?.data?.error || error.message || '切换标签失败';
      message.error(errorMsg);
    } finally {
      setShowConfirmModal(false);
      setPendingSwitchTag(null);
    }
  };

  // 取消切换Tag
  const handleCancelSwitch = () => {
    setShowConfirmModal(false);
    setPendingSwitchTag(null);
  };

  // 检查是否可以进行步骤级别的编辑（添加、插入、删除步骤）
  const canEditSteps = (): boolean => {
    if (!plan) return false;
    // 在 Approved、Executing、Completed 状态下不允许步骤级别的编辑
    const restrictedStates = ['Approved', 'Executing', 'Completed'];
    if (restrictedStates.includes(plan.status)) return false;
    // 检查状态
    if (!isEditableState(plan.status)) return false;
    // TODO: 检查用户权限（当前用户是否为任务负责人）
    // 这需要从任务信息中获取 assignee 并与当前用户对比
    return true;
  };

  // 检查是否可以编辑步骤状态（在任何可编辑状态下都允许）
  const canEditStepStatus = (): boolean => {
    if (!plan) return false;
    // 检查状态
    if (!isEditableState(plan.status)) return false;
    // TODO: 检查用户权限（当前用户是否为任务负责人）
    // 这需要从任务信息中获取 assignee 并与当前用户对比
    return true;
  };

  // 检查用户是否有编辑权限
  const canEdit = (): boolean => {
    if (!plan) return false;
    // 检查状态
    if (!isEditableState(plan.status)) return false;
    // TODO: 检查用户权限（当前用户是否为任务负责人）
    // 这需要从任务信息中获取 assignee 并与当前用户对比
    return true;
  };

  const handleEdit = () => {
    if (!plan || !canEdit()) {
      message.warning('当前计划状态不允许编辑');
      return;
    }
    setEditorContent(plan.content || '');
    setIsEditMode(true);
  };

  const handleCancelEdit = () => {
    Modal.confirm({
      title: '确认取消编辑',
      content: '未保存的修改将丢失，是否继续？',
      okText: '确认',
      cancelText: '取消',
      onOk: () => {
        setIsEditMode(false);
        setEditorContent('');
      }
    });
  };

  const handleSave = async () => {
    if (!projectId || !taskId || !editorContent.trim()) {
      message.error('内容不能为空');
      return;
    }

    setSaving(true);
    try {
      await updateExecutionPlanContent(projectId, taskId, editorContent);
      message.success('执行计划已更新');
      setIsEditMode(false);
      setEditorContent('');
      await loadExecutionPlan(); // 重新加载以获取最新内容
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error || '保存失败';
      message.error(errorMsg);
    } finally {
      setSaving(false);
    }
  };

  // 生成默认执行计划模板
  const handleResetPlan = async (force: boolean = false) => {
    if (!projectId || !taskId) return;
    
    setResetting(true);
    try {
      await resetExecutionPlan(projectId, taskId, force);
      message.success('执行计划模板已生成');
      await loadExecutionPlan();
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error || '生成模板失败';
      message.error(errorMsg);
    } finally {
      setResetting(false);
    }
  };

  // 打开步骤编辑器 - 创建模式
  const handleAddStep = () => {
    setStepModalMode('create');
    setEditingStep(null);
    setInsertPosition(undefined);
    setStepModalVisible(true);
  };

  // 打开步骤编辑器 - 编辑模式
  const handleEditStep = (step: ExecutionPlanStep) => {
    setStepModalMode('edit');
    setEditingStep(step);
    setInsertPosition(undefined);
    setStepModalVisible(true);
  };

  // 打开步骤编辑器 - 插入模式
  const handleInsertStep = (afterStepIndex: number) => {
    setStepModalMode('insert');
    setEditingStep(null);
    setInsertPosition(afterStepIndex + 1);
    setStepModalVisible(true);
  };

  // 删除步骤
  const handleDeleteStep = async (stepId: string) => {
    if (!plan || !projectId || !taskId) return;
    
    setSaving(true);
    try {
      // 过滤掉被删除的步骤
      const newSteps = plan.steps.filter(s => s.id !== stepId);
      
      // 重新编号
      const renumbered = renumberSteps(newSteps.map(s => ({
        id: s.id,
        description: s.description,
        status: s.status,
        priority: s.priority as any,
        dependencies: plan.dependencies
          .filter(d => d.target === s.id)
          .map(d => d.source)
      })));

      // 重建 frontmatter
      const frontmatter: ExecutionPlanFrontmatter = {
        plan_id: plan.plan_id,
        task_id: plan.task_id,
        status: plan.status,
        created_at: plan.created_at,
        updated_at: new Date().toISOString(),
        dependencies: [] // 将从 renumbered 中提取
      };

      // 从步骤中提取新的依赖关系
      const newDeps: any[] = [];
      renumbered.forEach(step => {
        step.dependencies?.forEach(depId => {
          newDeps.push({ source: depId, target: step.id });
        });
      });
      frontmatter.dependencies = newDeps;

      // 重建 Markdown
      const newContent = buildMarkdown(frontmatter, renumbered);
      
      await updateExecutionPlanContent(projectId, taskId, newContent);
      message.success('步骤已删除');
      await loadExecutionPlan();
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error || '删除步骤失败';
      message.error(errorMsg);
    } finally {
      setSaving(false);
    }
  };

  // 步骤表单提交
  const handleStepSubmit = async (formData: StepFormData) => {
    if (!plan || !projectId || !taskId) return;
    
    setSaving(true);
    try {
      let newSteps: BuilderStep[];
      
      if (stepModalMode === 'create') {
        // 创建新步骤 - 追加到末尾
        const newStep: BuilderStep = {
          id: `step-${String(plan.steps.length + 1).padStart(2, '0')}`,
          description: formData.description,
          status: formData.status,
          priority: formData.priority,
          dependencies: formData.dependencies
        };
        newSteps = [...plan.steps.map(s => ({
          id: s.id,
          description: s.description,
          status: s.status,
          priority: s.priority as any,
          dependencies: plan.dependencies
            .filter(d => d.target === s.id)
            .map(d => d.source)
        })), newStep];
      } else if (stepModalMode === 'insert') {
        // 插入步骤
        const allSteps: BuilderStep[] = plan.steps.map(s => ({
          id: s.id,
          description: s.description,
          status: s.status,
          priority: s.priority as any,
          dependencies: plan.dependencies
            .filter(d => d.target === s.id)
            .map(d => d.source)
        }));
        
        const newStep: BuilderStep = {
          id: 'temp-id',
          description: formData.description,
          status: formData.status,
          priority: formData.priority,
          dependencies: formData.dependencies
        };
        
        allSteps.splice(insertPosition || 0, 0, newStep);
        newSteps = renumberSteps(allSteps);
      } else {
        // 编辑现有步骤
        newSteps = plan.steps.map(s => {
          if (s.id === editingStep?.id) {
            return {
              id: s.id,
              description: formData.description,
              status: formData.status,
              priority: formData.priority,
              dependencies: formData.dependencies
            };
          }
          return {
            id: s.id,
            description: s.description,
            status: s.status,
            priority: s.priority as any,
            dependencies: plan.dependencies
              .filter(d => d.target === s.id)
              .map(d => d.source)
          };
        });
      }

      // 重建 frontmatter
      const frontmatter: ExecutionPlanFrontmatter = {
        plan_id: plan.plan_id,
        task_id: plan.task_id,
        status: plan.status,
        created_at: plan.created_at,
        updated_at: new Date().toISOString(),
        dependencies: []
      };

      // 从步骤中提取依赖关系
      const newDeps: any[] = [];
      newSteps.forEach(step => {
        step.dependencies?.forEach(depId => {
          newDeps.push({ source: depId, target: step.id });
        });
      });
      frontmatter.dependencies = newDeps;

      // 重建 Markdown
      const newContent = buildMarkdown(frontmatter, newSteps);
      
      await updateExecutionPlanContent(projectId, taskId, newContent);
      message.success('步骤已保存');
      setStepModalVisible(false);
      await loadExecutionPlan();
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error || '保存步骤失败';
      message.error(errorMsg);
    } finally {
      setSaving(false);
    }
  };

  // 获取指定步骤的依赖项
  const getStepDependencies = (stepId: string): string[] => {
    if (!plan?.dependencies) return [];
    return plan.dependencies
      .filter(dep => dep.target === stepId)
      .map(dep => dep.source);
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '50px' }}>
        <Spin size="large" tip="加载执行计划..." />
      </div>
    );
  }

  if (error) {
    return (
      <Alert
        message="加载失败"
        description={error}
        type="error"
        showIcon
        action={
          <Button size="small" onClick={loadExecutionPlan}>
            重试
          </Button>
        }
      />
    );
  }

  if (!plan) {
    return (
      <Empty 
        description="暂无执行计划" 
        image={Empty.PRESENTED_IMAGE_SIMPLE}
      >
        <Button 
          type="primary" 
          icon={<PlusOutlined />}
          loading={resetting}
          onClick={() => handleResetPlan(false)}
        >
          生成默认模板
        </Button>
      </Empty>
    );
  }

  const showApprovalToolbar = plan.status === 'Pending Approval';
  const showSubmitToolbar = plan.status === 'Draft' || plan.status === 'Rejected';
  const showRestoreToolbar = plan.status === 'Approved' || plan.status === 'Executing' || plan.status === 'Completed';
  const showEditButton = canEdit() && !isEditMode;

  return (
    <div style={{ padding: '0' }}>
      {isEditMode ? (
        // 编辑模式
        <Card 
          title="编辑执行计划" 
          style={{ marginBottom: 16 }}
          extra={
            <Space>
              <Button
                onClick={handleCancelEdit}
                disabled={saving}
              >
                取消
              </Button>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={saving}
                onClick={handleSave}
              >
                保存
              </Button>
            </Space>
          }
        >
          <div style={{ marginBottom: 12, color: '#666' }}>
            提示：您正在编辑执行计划的完整内容（YAML + Markdown）。请确保格式正确。
          </div>
          <PlanEditor
            value={editorContent}
            onChange={(value) => setEditorContent(value || '')}
            height="70vh"
          />
        </Card>
      ) : (
        // 查看模式
        <>
          {/* 计划头部信息 */}
          <Card 
            title="执行计划概览" 
            style={{ marginBottom: 16 }}
            extra={
              <Space>
                {showEditButton && (
                  <Button
                    type="default"
                    icon={<EditOutlined />}
                    onClick={handleEdit}
                    size="small"
                  >
                    编辑
                  </Button>
                )}
                <TagButton
                  onCreateTag={handleCreateTag}
                  docType="execution_plan"
                  size="small"
                />
                <TagVersionSelect
                  key={`tag-select-execution-plan-${projectId}-${taskId}`}
                  projectId={projectId}
                  taskId={taskId}
                  docType="execution-plan"
                  currentVersion={selectedTag}
                  onSwitchTag={handleSwitchTag}
                  refreshKey={tagRefreshKey}
                  size="small"
                />
                {showSubmitToolbar && (
                  <Button
                    type="primary"
                    icon={<SendOutlined />}
                    loading={submitting}
                    onClick={handleSubmit}
                  >
                    提交审核
                  </Button>
                )}
                {showApprovalToolbar && (
                  <>
                    <Button
                      type="primary"
                      icon={<CheckOutlined />}
                      loading={approving}
                      onClick={handleApprove}
                    >
                      批准
                    </Button>
                    <Button
                      danger
                      icon={<CloseOutlined />}
                      loading={rejecting}
                      onClick={handleReject}
                    >
                      拒绝
                    </Button>
                  </>
                )}
                {showRestoreToolbar && (
                  <Button
                    type="default"
                    icon={<RollbackOutlined />}
                    loading={restoring}
                    onClick={handleRestore}
                  >
                    恢复审核
                  </Button>
                )}
              </Space>
            }
          >
        <Descriptions column={2} size="small">
          <Descriptions.Item label="计划ID">{plan.plan_id}</Descriptions.Item>
          <Descriptions.Item label="状态">{getStatusTag(plan.status)}</Descriptions.Item>
          <Descriptions.Item label="创建时间">{new Date(plan.created_at).toLocaleString('zh-CN')}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{new Date(plan.updated_at).toLocaleString('zh-CN')}</Descriptions.Item>
          <Descriptions.Item label="步骤总数">{plan.steps?.length || 0}</Descriptions.Item>
          <Descriptions.Item label="依赖关系">{plan.dependencies?.length || 0} 条</Descriptions.Item>
        </Descriptions>
      </Card>

      {/* 步骤时间线 */}
      <Card 
        title="执行步骤" 
        style={{ marginBottom: 16 }}
        extra={
          canEditSteps() && (
            <Button 
              type="primary" 
              icon={<PlusOutlined />}
              onClick={handleAddStep}
              size="small"
            >
              添加步骤
            </Button>
          )
        }
      >
        {plan.steps && plan.steps.length > 0 ? (
          <Timeline>
            {plan.steps.map((step) => {
              const dependencies = getStepDependencies(step.id);
              return (
                <Timeline.Item
                  key={step.id}
                  dot={getStatusIcon(step.status)}
                >
                  <div>
                    <Space>
                      <strong>{step.id}</strong>
                      {getStepStatusTag(step.status)}
                      {step.priority && <Tag color="blue">{step.priority}</Tag>}
                    </Space>
                    <div style={{ marginTop: 8, color: '#666' }}>
                      {step.description}
                    </div>
                    {dependencies.length > 0 && (
                      <div style={{ marginTop: 4, fontSize: 12, color: '#999' }}>
                        依赖: {dependencies.map(dep => (
                          <Tag key={dep} color="default" style={{ marginRight: 4, fontSize: 12 }}>
                            {dep}
                          </Tag>
                        ))}
                      </div>
                    )}
                    {step.output && (
                      <Button 
                        type="link" 
                        size="small"
                        onClick={() => showStepDetail(step)}
                      >
                        查看输出
                      </Button>
                    )}
                    {step.updated_at && (
                      <div style={{ fontSize: 12, color: '#999', marginTop: 4 }}>
                        更新于: {new Date(step.updated_at).toLocaleString('zh-CN')}
                      </div>
                    )}
                    {canEditSteps() && (
                      <div style={{ marginTop: 8 }}>
                        <Space size="small">
                          <Tooltip title="编辑步骤">
                            <Button 
                              type="link" 
                              size="small"
                              icon={<EditOutlined />}
                              onClick={() => handleEditStep(step)}
                            >
                              编辑
                            </Button>
                          </Tooltip>
                          <Tooltip title="在此步骤后插入新步骤">
                            <Button 
                              type="link" 
                              size="small"
                              icon={<InsertRowBelowOutlined />}
                              onClick={() => handleInsertStep(plan.steps.indexOf(step))}
                            >
                              插入
                            </Button>
                          </Tooltip>
                          <Popconfirm
                            title="确认删除此步骤？"
                            description="删除后将重新编号所有步骤"
                            onConfirm={() => handleDeleteStep(step.id)}
                            okText="确认"
                            cancelText="取消"
                          >
                            <Button 
                              type="link" 
                              size="small"
                              danger
                              icon={<DeleteOutlined />}
                            >
                              删除
                            </Button>
                          </Popconfirm>
                        </Space>
                      </div>
                    )}
                    {canEditStepStatus() && !canEditSteps() && (
                      <div style={{ marginTop: 8 }}>
                        <Space size="small">
                          <Tooltip title="编辑步骤状态">
                            <Button 
                              type="link" 
                              size="small"
                              icon={<EditOutlined />}
                              onClick={() => handleEditStep(step)}
                            >
                              编辑状态
                            </Button>
                          </Tooltip>
                        </Space>
                      </div>
                    )}
                  </div>
                </Timeline.Item>
              );
            })}
          </Timeline>
        ) : (
          <Empty description="暂无步骤" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        )}
      </Card>

      {/* 步骤详情模态框 */}
      <Modal
        title={selectedStep ? `步骤详情: ${selectedStep.id}` : '步骤详情'}
        open={detailModalVisible}
        onCancel={() => setDetailModalVisible(false)}
        footer={null}
        width={800}
      >
        {selectedStep && (
          <div>
            <Descriptions column={1} size="small" style={{ marginBottom: 16 }}>
              <Descriptions.Item label="状态">{getStepStatusTag(selectedStep.status)}</Descriptions.Item>
              <Descriptions.Item label="优先级">{selectedStep.priority || 'N/A'}</Descriptions.Item>
              <Descriptions.Item label="描述">{selectedStep.description}</Descriptions.Item>
              {selectedStep.started_at && (
                <Descriptions.Item label="开始时间">
                  {new Date(selectedStep.started_at).toLocaleString('zh-CN')}
                </Descriptions.Item>
              )}
              {selectedStep.completed_at && (
                <Descriptions.Item label="完成时间">
                  {new Date(selectedStep.completed_at).toLocaleString('zh-CN')}
                </Descriptions.Item>
              )}
            </Descriptions>
            {selectedStep.output && (
              <div>
                <h4>执行输出：</h4>
                <div style={{ 
                  background: '#f5f5f5', 
                  padding: 12, 
                  borderRadius: 4,
                  maxHeight: 400,
                  overflow: 'auto'
                }}>
                  <MarkdownViewer>
                    {unescapeText(selectedStep.output)}
                  </MarkdownViewer>
                </div>
              </div>
            )}
          </div>
        )}
      </Modal>
      
      {/* 步骤编辑器模态框 */}
      <StepEditorModal
        mode={stepModalMode}
        visible={stepModalVisible}
        initialStep={editingStep ? {
          id: editingStep.id,
          description: editingStep.description,
          status: editingStep.status,
          priority: editingStep.priority as any,
          dependencies: plan?.dependencies
            .filter(d => d.target === editingStep.id)
            .map(d => d.source)
        } : undefined}
        availableSteps={plan?.steps.map(s => ({
          id: s.id,
          description: s.description,
          status: s.status,
          priority: s.priority as any,
          dependencies: plan?.dependencies
            .filter(d => d.target === s.id)
            .map(d => d.source)
        })) || []}
        insertPosition={insertPosition}
        planStatus={plan?.status}
        onSubmit={handleStepSubmit}
        onCancel={() => setStepModalVisible(false)}
      />

      {/* Tag切换确认对话框 */}
      <TagConfirmModal
        visible={showConfirmModal}
        currentMd5={pendingSwitchTag?.currentMd5}
        targetTag={pendingSwitchTag?.tagName}
        onConfirm={handleConfirmSwitch}
        onCancel={handleCancelSwitch}
      />
        </>
      )}
    </div>
  );
};

export default ExecutionPlanView;
