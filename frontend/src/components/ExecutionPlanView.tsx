import React, { useEffect, useRef, useState } from 'react';
import { Alert, Card, Descriptions, Empty, Spin, Tag, Timeline, Typography } from 'antd';
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import { ExecutionPlan, ExecutionPlanStep, getExecutionPlan } from '../api/tasks';
import MarkdownViewer from './MarkdownViewer';

const { Text } = Typography;

interface Props {
  projectId: string;
  taskId: string;
}

const stepIcon = (status: ExecutionPlanStep['status']) => {
  switch (status) {
    case 'succeeded': return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
    case 'in-progress': return <SyncOutlined spin style={{ color: '#1677ff' }} />;
    case 'failed': return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
    case 'cancelled': return <ExclamationCircleOutlined style={{ color: '#faad14' }} />;
    default: return <ClockCircleOutlined style={{ color: '#8c8c8c' }} />;
  }
};

const statusLabel = (status: string) => {
  const labels: Record<string, string> = {
    Draft: '草稿',
    'Pending Approval': '待审批',
    Approved: '已批准',
    Rejected: '已拒绝',
    Executing: '执行中',
    Completed: '已完成',
    Failed: '失败',
  };
  return labels[status] || status;
};

/** Historical compatibility view. It intentionally exposes no mutation path. */
const ExecutionPlanView: React.FC<Props> = ({ projectId, taskId }) => {
  const [plan, setPlan] = useState<ExecutionPlan | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const requestId = useRef(0);

  useEffect(() => {
    const current = ++requestId.current;
    setLoading(true);
    setError(null);
    setPlan(null);
    getExecutionPlan(projectId, taskId)
      .then((result) => {
        if (requestId.current === current) setPlan(result.data || null);
      })
      .catch((reason: unknown) => {
        if (requestId.current !== current) return;
        const status = (reason as { response?: { status?: number } })?.response?.status;
        if (status === 404) {
          setPlan(null);
        } else {
          setError(status === 403 ? '没有权限读取历史执行计划' : '历史执行计划读取失败');
        }
      })
      .finally(() => {
        if (requestId.current === current) setLoading(false);
      });
    return () => { requestId.current += 1; };
  }, [projectId, taskId]);

  if (loading) return <div style={{ padding: 32, textAlign: 'center' }}><Spin /></div>;

  return (
    <div className="historical-plan-view">
      <Alert
        type="info"
        showIcon
        message="历史执行计划 · 只读"
        description="此记录来自旧版本，仅供查阅。任务生命周期以任务状态、三份约束和审核结果为准。"
        style={{ marginBottom: 16 }}
      />
      {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      {!plan ? <Empty description="没有历史执行计划" /> : (
        <>
          <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }} style={{ marginBottom: 16 }}>
            <Descriptions.Item label="历史状态"><Tag>{statusLabel(plan.status)}</Tag></Descriptions.Item>
            <Descriptions.Item label="最后更新">{plan.updated_at || '—'}</Descriptions.Item>
          </Descriptions>
          {plan.steps?.length > 0 && (
            <Card size="small" title="历史步骤">
              <Timeline
                items={plan.steps.map((step) => ({
                  dot: stepIcon(step.status),
                  children: (
                    <div>
                      <Text strong>{step.description || step.id}</Text>
                      <div><Tag>{step.status}</Tag>{step.updated_at && <Text type="secondary">{step.updated_at}</Text>}</div>
                      {step.output && <MarkdownViewer>{step.output}</MarkdownViewer>}
                    </div>
                  ),
                }))}
              />
            </Card>
          )}
        </>
      )}
    </div>
  );
};

export default ExecutionPlanView;
