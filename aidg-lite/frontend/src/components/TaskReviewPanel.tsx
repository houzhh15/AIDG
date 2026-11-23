import React, { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Empty, Form, Input, Radio, Space, Spin, Tag, Typography } from 'antd';
import { getTaskReview, requestTaskReview, ReviewDocument, updateTaskReviewResult } from '../api/taskReview';
import MarkdownViewer from './MarkdownViewer';

const { Text } = Typography;
const { TextArea } = Input;

interface Props {
  projectId: string;
  taskId: string;
  taskRevision?: number;
  readyCount: number;
  onChanged?: () => void;
}

const errorMessage = (error: any) => {
  const status = error?.response?.status;
  if (status === 401) return '登录状态已失效，请重新登录后重试。';
  if (status === 403) return '没有执行审核操作的权限。';
  if (status === 404) return '任务或审核记录不存在，请刷新任务列表。';
  if (status === 409) return '任务已被其他协作者更新。你的输入仍保留，请刷新对比后重试。';
  if (status === 422) return '审核内容或任务状态不满足约束，请检查后重试。';
  if (status >= 500) return '服务暂时不可用，你的输入仍保留，请稍后重试。';
  return '请求失败，你的输入仍保留。';
};

const TaskReviewPanel: React.FC<Props> = ({ projectId, taskId, taskRevision, readyCount, onChanged }) => {
  const [review, setReview] = useState<ReviewDocument | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [requestForm] = Form.useForm();
  const [resultForm] = Form.useForm();
  const requestId = useRef(0);

  const load = async () => {
    const current = ++requestId.current;
    setLoading(true);
    setError(null);
    try {
      const value = await getTaskReview(projectId, taskId);
      if (requestId.current === current) setReview(value);
    } catch (reason) {
      if (requestId.current === current) setError(errorMessage(reason));
    } finally {
      if (requestId.current === current) setLoading(false);
    }
  };

  useEffect(() => {
    setReview(null);
    requestForm.resetFields();
    resultForm.resetFields();
    void load();
    return () => { requestId.current += 1; };
  }, [projectId, taskId]);

  const submitRequest = async (values: { target_type: 'AIDG文档' | '文件' | '代码'; target_path: string; criteria: string }) => {
    setSaving(true);
    setError(null);
    try {
      const response = await requestTaskReview(projectId, taskId, { ...values, expected_revision: taskRevision });
      setReview(response.data.review);
      onChanged?.();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setSaving(false);
    }
  };

  const submitResult = async (values: { result: '通过' | '不通过'; issues?: string }) => {
    setSaving(true);
    setError(null);
    try {
      const response = await updateTaskReviewResult(projectId, taskId, { ...values, expected_revision: taskRevision });
      setReview(response.data.review);
      onChanged?.();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Alert
          type={readyCount === 3 ? 'success' : 'warning'}
          showIcon
          message={`任务约束 ${readyCount}/3`}
          description={readyCount === 3 ? '需求、设计、测试约束均已就绪，可以发起审核。' : '请先补齐需求、设计、测试三份约束；输入会保留但不能发起审核。'}
        />
        {error && (
          <Alert
            type="error"
            showIcon
            message={error}
            action={error.includes('其他协作者') ? <Button size="small" onClick={() => void load()}>刷新远端记录</Button> : undefined}
          />
        )}
        <Card size="small" title="审核请求" extra={<Tag>{taskRevision === undefined ? 'revision 未知' : `r${taskRevision}`}</Tag>}>
          <Form form={requestForm} layout="vertical" initialValues={{ target_type: 'AIDG文档' }} onFinish={submitRequest}>
            <Form.Item name="target_type" label="目标类型" rules={[{ required: true }]}>
              <Radio.Group options={['AIDG文档', '文件', '代码']} optionType="button" />
            </Form.Item>
            <Form.Item name="target_path" label="目标路径/引用" rules={[{ required: true, whitespace: true }]}>
              <Input placeholder="例如：task requirements/design/test" />
            </Form.Item>
            <Form.Item name="criteria" label="审核标准" rules={[{ required: true, whitespace: true }]}>
              <TextArea rows={5} placeholder="写明可验证的审核标准" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={saving} disabled={readyCount !== 3 || taskRevision === undefined}>发起审核</Button>
          </Form>
        </Card>
        <Card size="small" title="审核记录">
          {!review ? <Empty description="尚无审核记录" /> : (
            <>
              <Space style={{ marginBottom: 12 }}><Tag color="blue">v{review.version}</Tag><Text type="secondary">{review.updated_at}</Text></Space>
              <MarkdownViewer>{review.content}</MarkdownViewer>
            </>
          )}
        </Card>
        {review && (
          <Card size="small" title="更新审核结果">
            <Form form={resultForm} layout="vertical" onFinish={submitResult}>
              <Form.Item name="result" label="结论" rules={[{ required: true }]}>
                <Radio.Group options={['通过', '不通过']} optionType="button" />
              </Form.Item>
              <Form.Item name="issues" label="问题列表"><TextArea rows={4} placeholder="每行一个以 - 开头的问题；通过时可留空" /></Form.Item>
              <Button htmlType="submit" loading={saving} disabled={taskRevision === undefined}>保存审核结果</Button>
            </Form>
          </Card>
        )}
      </Space>
    </Spin>
  );
};

export default TaskReviewPanel;
