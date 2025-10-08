import React, { useState, useEffect } from 'react';
import { Card, Spin, Alert, Empty, Descriptions, Tag, Timeline, Space, Button, message, Modal } from 'antd';
import { 
  CheckCircleOutlined, 
  ClockCircleOutlined, 
  CloseCircleOutlined, 
  SyncOutlined, 
  ExclamationCircleOutlined,
  CheckOutlined,
  CloseOutlined,
  EditOutlined,
  SaveOutlined
} from '@ant-design/icons';
import { 
  getExecutionPlan, 
  approveExecutionPlan, 
  rejectExecutionPlan,
  updateExecutionPlanContent,
  ExecutionPlan, 
  ExecutionPlanStep 
} from '../api/tasks';
import MarkdownViewer from './MarkdownViewer';
import PlanEditor from './ExecutionPlan/PlanEditor';

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
  const [selectedStep, setSelectedStep] = useState<ExecutionPlanStep | null>(null);
  const [detailModalVisible, setDetailModalVisible] = useState(false);
  const [isEditMode, setIsEditMode] = useState(false);
  const [editorContent, setEditorContent] = useState<string>('');
  const [saving, setSaving] = useState(false);
  const [taskAssignee, setTaskAssignee] = useState<string>('');
  const [currentUser, setCurrentUser] = useState<string>('');

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
    return ['Pending Approval', 'Rejected', 'Approved', 'Executing', 'Completed'].includes(status);
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
      />
    );
  }

  const showApprovalToolbar = plan.status === 'Pending Approval';
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
      <Card title="执行步骤" style={{ marginBottom: 16 }}>
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
        </>
      )}
    </div>
  );
};

export default ExecutionPlanView;
