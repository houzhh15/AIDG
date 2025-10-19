import React from 'react';
import { Drawer, Form, Input, Button, Divider, message, Select, Table, Progress, Space } from 'antd';
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
  const [modelDownloadDrawerOpen, setModelDownloadDrawerOpen] = React.useState(false);

  React.useEffect(() => {
    const fetchWhisperModels = async () => {
      try {
        setWhisperModelsLoading(true);
        const response = await fetch('/api/v1/services/whisper/models');
        if (response.ok) {
          const data = await response.json();
          if (data.success && data.data.models) {
            const modelIds = data.data.models.map((m: any) => m.id);
            
            // 确保当前设置的模型在列表中
            const currentModel = initial?.whisper_model || 'ggml-large-v3';
            if (currentModel && !modelIds.includes(currentModel)) {
              modelIds.push(currentModel);
            }
            
            setWhisperModels(modelIds);
          }
        }
      } catch (error) {
        console.warn('Failed to fetch Whisper models:', error);
        // 如果API失败，至少包含当前设置的模型
        const currentModel = initial?.whisper_model || 'ggml-large-v3';
        setWhisperModels([currentModel]);
      } finally {
        setWhisperModelsLoading(false);
      }
    };
    fetchWhisperModels();
  }, [initial?.whisper_model]);

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
    <>
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
            <div style={{ display: 'flex', gap: '8px' }}>
              <Select
                placeholder="选择 Whisper 模型"
                loading={whisperModelsLoading}
                options={whisperModels.map(model => ({ label: model, value: model }))}
                showSearch
                allowClear
                style={{ flex: 1 }}
              />
              <Button type="default" onClick={() => setModelDownloadDrawerOpen(true)}>
                下载模型
              </Button>
            </div>
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

      {/* 模型下载抽屉 */}
      <ModelDownloadDrawer
        open={modelDownloadDrawerOpen}
        onClose={() => setModelDownloadDrawerOpen(false)}
        onModelDownloaded={() => {
          // 刷新模型列表
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
        }}
      />
    </>
  );
};

export default TaskDetailSettings;

// 模型下载抽屉组件
interface ModelDownloadDrawerProps {
  open: boolean;
  onClose: () => void;
  onModelDownloaded: () => void;
}

const ModelDownloadDrawer: React.FC<ModelDownloadDrawerProps> = ({ open, onClose, onModelDownloaded }) => {
  const [models, setModels] = React.useState<any[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [downloadingModels, setDownloadingModels] = React.useState<Set<string>>(new Set());
  const [downloadProgress, setDownloadProgress] = React.useState<Map<string, { status: string; total?: number; completed?: number }>>(new Map());

  React.useEffect(() => {
    if (open) {
      fetchModels();
    }
  }, [open]);

  const fetchModels = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/v1/services/whisper/models-extended');
      if (response.ok) {
        const data = await response.json();
        if (data.success && data.data.models) {
          setModels(data.data.models);
        }
      }
    } catch (error) {
      message.error('获取模型列表失败');
    } finally {
      setLoading(false);
    }
  };

  const downloadModel = async (modelPath: string) => {
    if (downloadingModels.has(modelPath)) return;

    setDownloadingModels(prev => new Set(prev).add(modelPath));
    setDownloadProgress(prev => new Map(prev).set(modelPath, { status: '准备下载...' }));

    try {
      const response = await fetch('/api/v1/services/whisper/models/download', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ path: modelPath }),
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const reader = response.body?.getReader();
      if (!reader) throw new Error('无法读取响应流');

      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6));
              if (data.status && data.status.includes('downloading')) {
                // 更新下载进度
                setDownloadProgress(prev => new Map(prev).set(modelPath, {
                  status: data.status,
                  total: data.total,
                  completed: data.completed
                }));
              } else if (data.id) {
                // 下载完成
                setDownloadProgress(prev => new Map(prev).set(modelPath, { status: '已完成' }));
                message.success(`模型 ${data.id} 下载完成`);
                onModelDownloaded();
                fetchModels(); // 刷新列表
                break;
              }
            } catch (e) {
              // 忽略解析错误
            }
          }
        }
      }
    } catch (error) {
      message.error(`下载模型失败: ${error}`);
      setDownloadProgress(prev => new Map(prev).set(modelPath, { status: '下载失败' }));
    } finally {
      setDownloadingModels(prev => {
        const newSet = new Set(prev);
        newSet.delete(modelPath);
        return newSet;
      });
    }
  };

  const columns = [
    {
      title: 'Model',
      dataIndex: 'id',
      key: 'id',
    },
    {
      title: 'Disk',
      dataIndex: 'size_mb',
      key: 'size_mb',
    },
    {
      title: 'Download',
      key: 'download',
      render: (_: any, record: any) => {
        const isDownloading = downloadingModels.has(record.path);
        const isDownloaded = record.exists;
        const progress = downloadProgress.get(record.path);

        if (isDownloaded) {
          return <span style={{ color: 'green' }}>已下载</span>;
        }

        if (isDownloading && progress) {
          const percent = progress.total && progress.completed
            ? Math.round((progress.completed / progress.total) * 100)
            : 0;

          return (
            <div style={{ width: 120 }}>
              <Progress percent={percent} size="small" status={progress.status === '下载失败' ? 'exception' : 'active'} />
              <div style={{ fontSize: '12px', color: '#666', marginTop: 4 }}>
                {progress.status}
              </div>
            </div>
          );
        }

        return (
          <Button
            type="primary"
            size="small"
            loading={isDownloading}
            onClick={() => downloadModel(record.path)}
            disabled={isDownloading}
          >
            {isDownloading ? '下载中...' : '下载'}
          </Button>
        );
      },
    },
  ];

  return (
    <Drawer
      title="下载 Whisper 模型"
      width={600}
      open={open}
      onClose={onClose}
      destroyOnClose
    >
      <Table
        columns={columns}
        dataSource={models}
        loading={loading}
        rowKey="id"
        pagination={false}
        size="small"
      />
    </Drawer>
  );
};
