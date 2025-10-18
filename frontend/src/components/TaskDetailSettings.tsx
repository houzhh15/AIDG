import React from 'react';
import { Drawer, Form, Input, Button, Divider, message, Select } from 'antd';
import { authedApi } from '../api/auth';

interface Props {
  open: boolean;
  onClose: () => void;
  taskId: string;
  initial: any;
  refresh: () => void;
  onAfterRename?: () => void; // 重命名后的回调
}

export const TaskDetailSettings: React.FC<Props> = ({ open, onClose, taskId, initial, refresh, onAfterRename }) => {
  const [loading, setLoading] = React.useState(false);
  const [form] = Form.useForm();
  const [whisperModels, setWhisperModels] = React.useState<string[]>(['ggml-base']);
  const [whisperModelsLoading, setWhisperModelsLoading] = React.useState(false);

  React.useEffect(() => {
    const fetchWhisperModels = async () => {
      try {
        setWhisperModelsLoading(true);
        const response = await fetch('/api/v1/services/whisper/models');
        if (response.ok) {
          const data = await response.json();
          if (data.success && data.data.models) {
            const modelIds = data.data.models.map((m: any) => m.id);
            setWhisperModels(modelIds);
          }
        }
      } catch (error) {
        console.warn('Failed to fetch Whisper models:', error);
      } finally {
        setWhisperModelsLoading(false);
      }
    };
    fetchWhisperModels();
  }, []);

  React.useEffect(() => {
    if (open) {
      form.setFieldsValue({
        whisper_model: initial?.whisper_model || 'ggml-large-v3',
        whisper_segments: initial?.whisper_segments || '15s',
        product_line: initial?.product_line,
        meeting_time: initial?.meeting_time ? new Date(initial.meeting_time).toISOString().slice(0, 16) : undefined,
        embedding_threshold: initial?.embedding_threshold || '0.55',
        embedding_auto_lower_min: initial?.embedding_auto_lower_min || '0.35',
        embedding_auto_lower_step: initial?.embedding_auto_lower_step || '0.02',
        task_name: taskId,
      });
    }
  }, [open, initial, form, taskId]);

  async function submit() {
    try {
      const values = await form.validateFields();
      if (values.meeting_time) {
        values.meeting_time = new Date(values.meeting_time).toISOString();
      }

      // 保存新的任务名称用于重命名（如果需要）
      const newTaskName = values.task_name?.trim();
      
      // 移除 task_name，不发送到后端配置更新
      delete values.task_name;
      
      // 固定设置：分段长度为 300 秒，使用 pyannote 后端
      values.record_chunk_seconds = 300;
      values.diarization_backend = 'pyannote';

      setLoading(true);
      
      // 先保存配置（URL 编码任务 ID）
      await authedApi.patch(`/tasks/${encodeURIComponent(taskId)}/config`, values);
      message.success('配置已保存');
      
      // 如果需要重命名，在配置保存成功后再执行
      if (newTaskName && newTaskName !== taskId) {
        try {
          await authedApi.patch(`/tasks/${encodeURIComponent(taskId)}/rename`, { new_id: newTaskName });
          message.success('任务已重命名');
          // 重命名成功后，调用回调刷新任务列表并关闭窗口
          if (onAfterRename) {
            onAfterRename();
          } else {
            refresh();
            onClose();
          }
        } catch (e: any) {
          message.error('重命名失败: ' + e.message);
          // 即使重命名失败，配置也已保存成功
          refresh();
        }
      } else {
        refresh();
        onClose();
      }
    } catch (e: any) {
      if (e?.errorFields) return;
      message.error(e.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <Drawer
      title={`任务设置: ${taskId}`}
      width={480}
      open={open}
      onClose={onClose}
      destroyOnClose
      extra={<Button type="primary" onClick={submit} loading={loading}>保存</Button>}
    >
      <Form layout="vertical" form={form}>
        <Divider plain>基础信息</Divider>
        <Form.Item label="任务名称" name="task_name">
          <Input placeholder="输入任务名称" />
        </Form.Item>
        <Form.Item label="产品线" name="product_line">
          <Input placeholder="输入产品线名称" />
        </Form.Item>
        <Form.Item label="会议时间" name="meeting_time">
          <Input type="datetime-local" placeholder="选择会议时间" />
        </Form.Item>

        <Divider plain>转录设置</Divider>
        <Form.Item label="Whisper 模型" name="whisper_model">
          <Select
            placeholder="选择 Whisper 模型"
            loading={whisperModelsLoading}
            options={whisperModels.map(model => ({ label: model, value: model }))}
            showSearch
            allowClear
          />
        </Form.Item>
        <Form.Item label="Segments" name="whisper_segments" tooltip="如 20s; 为空或0表示不加 --segments">
          <Input placeholder="20s" allowClear />
        </Form.Item>

        <Divider plain>Embedding 参数</Divider>
        <Form.Item label="初始阈值" name="embedding_threshold">
          <Input placeholder="0.55" />
        </Form.Item>
        <Form.Item label="AutoLower最小" name="embedding_auto_lower_min">
          <Input placeholder="0.35" />
        </Form.Item>
        <Form.Item label="AutoLower步长" name="embedding_auto_lower_step">
          <Input placeholder="0.02" />
        </Form.Item>
      </Form>
    </Drawer>
  );
};

export default TaskDetailSettings;
