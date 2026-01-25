import React from 'react';
import { Drawer, Form, Input, Button, Divider, message, Select, Table } from 'antd';
import { authedApi } from '../api/auth';

// 模块加载时立即执行的日志 - 用于验证新代码是否被加载
console.log('[DEBUG] ========== TaskDetailSettings.tsx MODULE LOADED - BUILD TIME: 2025-10-19 13:32 ==========');

interface TaskInitial {
  name?: string;
  description?: string;
  whisper_model?: string;
  meeting_time?: string | number | Date;
  [key: string]: unknown;
}

interface WhisperModel {
  id: string;
  path?: string;
  exists?: boolean;
  size_mb?: number;
  [key: string]: unknown;
}

interface Props {
  open: boolean;
  onClose: () => void;
  taskId: string;
  initial: TaskInitial;
  refresh: () => void;
  onAfterRename?: () => void; // 重命名后的回调
}

export const TaskDetailSettings: React.FC<Props> = ({ open, onClose, taskId, initial, refresh, onAfterRename }) => {
  console.log('[DEBUG] TaskDetailSettings rendered - open:', open, 'taskId:', taskId, 'initial:', initial);
  
  const [loading, setLoading] = React.useState(false);
  const [form] = Form.useForm();
  const [whisperModels, setWhisperModels] = React.useState<string[]>(['ggml-base']);
  const [whisperModelsLoading, setWhisperModelsLoading] = React.useState(false);
  const [modelDownloadDrawerOpen, setModelDownloadDrawerOpen] = React.useState(false);

  // 获取whisper模型列表 - 组件挂载时立即获取
  React.useEffect(() => {
    const fetchWhisperModels = async () => {
      try {
        setWhisperModelsLoading(true);
        console.log('[DEBUG] Fetching whisper models from /api/v1/services/whisper/models');
        const response = await fetch('/api/v1/services/whisper/models');
        console.log('[DEBUG] Response status:', response.status, response.ok);
        
        if (response.ok) {
          const data = await response.json();
          console.log('[DEBUG] Response data:', data);
          
          if (data.success && data.data.models) {
            const modelIds = data.data.models.map((m: WhisperModel) => m.id);
            console.log('[DEBUG] Extracted model IDs:', modelIds);
            setWhisperModels(modelIds);
          }
        } else {
          console.error('[ERROR] API request failed with status:', response.status);
        }
      } catch (error) {
        console.warn('[WARN] Failed to fetch Whisper models:', error);
      } finally {
        setWhisperModelsLoading(false);
      }
    };
    
    fetchWhisperModels();
  }, []); // 空依赖数组,只在组件挂载时执行一次

  // 设置表单初始值
  React.useEffect(() => {
    if (open && initial) {
      console.log('[DEBUG] ========== Form Initialization ==========');
      console.log('[DEBUG] Full initial data:', initial);
      console.log('[DEBUG] initial.whisper_model:', initial.whisper_model);
      console.log('[DEBUG] whisperModels array:', whisperModels);
      
      const currentModel = initial?.whisper_model || 'ggml-large-v3';
      console.log('[DEBUG] currentModel (computed):', currentModel);
      
      // 确保当前模型在选项列表中（如果还没有）
      if (currentModel && !whisperModels.includes(currentModel)) {
        console.log('[DEBUG] Adding current model to whisperModels list');
        setWhisperModels(prev => [...prev, currentModel]);
      }
      
      // 处理日期 - 如果是无效日期（如 0000 年），则不设置
      let meetingTime = undefined;
      if (initial?.meeting_time) {
        const date = new Date(initial.meeting_time);
        // 检查日期是否有效且不是 0000 年
        if (!isNaN(date.getTime()) && date.getFullYear() > 1900) {
          meetingTime = date.toISOString().slice(0, 16);
        }
      }
      
      const formValues = {
        whisper_model: currentModel,
        whisper_temperature: initial?.whisper_temperature !== undefined ? initial.whisper_temperature : 0.0,
        whisper_segments: initial?.whisper_segments || '15s',
        product_line: initial?.product_line,
        meeting_time: meetingTime,
        embedding_threshold: initial?.embedding_threshold || '0.55',
        embedding_auto_lower_min: initial?.embedding_auto_lower_min || '0.35',
        embedding_auto_lower_step: initial?.embedding_auto_lower_step || '0.02',
        task_name: taskId,
      };
      
      console.log('[DEBUG] Setting form values to:', formValues);
      form.setFieldsValue(formValues);
      
      // 验证表单值是否设置成功
      setTimeout(() => {
        const actualValues = form.getFieldsValue();
        console.log('[DEBUG] Form values after setFieldsValue:', actualValues);
        console.log('[DEBUG] whisper_model field value:', actualValues.whisper_model);
      }, 100);
    }
  }, [open, initial, form, taskId, whisperModels]); // 添加 whisperModels 依赖

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
        } catch (e: unknown) {
          const error = e as Error;
          message.error('重命名失败: ' + error.message);
          // 即使重命名失败，配置也已保存成功
          refresh();
        }
      } else {
        refresh();
        onClose();
      }
    } catch (e: unknown) {
      const error = e as { errorFields?: unknown; message?: string };
      if (error.errorFields) return;
      message.error(error.message || '保存失败');
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
          <div style={{ display: 'flex', gap: '8px', alignItems: 'flex-start', marginBottom: '24px' }}>
            <Form.Item 
              label="Whisper 模型" 
              name="whisper_model"
              initialValue={initial?.whisper_model || 'ggml-large-v3'}
              style={{ flex: 1, marginBottom: 0 }}
            >
              <Select
                placeholder="选择 Whisper 模型"
                loading={whisperModelsLoading}
                options={whisperModels.map(model => ({ label: model, value: model }))}
                showSearch
              />
            </Form.Item>
            <Button 
              type="default" 
              onClick={() => setModelDownloadDrawerOpen(true)}
              style={{ marginTop: '30px' }}
            >
              下载模型
            </Button>
          </div>
          <Form.Item 
            label="Temperature" 
            name="whisper_temperature" 
            tooltip="控制采样随机性(0.0-1.0)。较低值(如0.0)更确定，可减少重复和幻觉；较高值增加随机性。推荐0.0以获得最稳定的转录结果"
            initialValue={0.0}
          >
            <Input 
              type="number" 
              placeholder="0.0" 
              min={0} 
              max={1} 
              step={0.1}
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
                  const modelIds = data.data.models.map((m: WhisperModel) => m.id);
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
  const [models, setModels] = React.useState<WhisperModel[]>([]);
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
      console.log('[DEBUG] ModelDownloadDrawer: Fetching models from /api/v1/services/whisper/models-extended');
      
      const response = await fetch('/api/v1/services/whisper/models-extended');
      console.log('[DEBUG] ModelDownloadDrawer: Response status:', response.status, response.ok);
      
      if (response.ok) {
        const data = await response.json();
        console.log('[DEBUG] ModelDownloadDrawer: Response data:', data);
        
        if (data.success && data.data.models) {
          console.log('[DEBUG] ModelDownloadDrawer: Setting models:', data.data.models);
          setModels(data.data.models);
        } else {
          console.warn('[WARN] ModelDownloadDrawer: API response missing expected structure');
        }
      } else {
        console.error('[ERROR] ModelDownloadDrawer: API request failed');
      }
    } catch (error) {
      console.error('[ERROR] ModelDownloadDrawer: Exception:', error);
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

      // eslint-disable-next-line no-constant-condition
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
              // go-whisper 返回格式: {"current": bytes, "total": bytes, "percent": float}
              // 或完成时: {"id": "model-id", "object": "model", "path": "filename", "created": timestamp}
              if (data.percent !== undefined) {
                // 下载进度更新
                setDownloadProgress(prev => new Map(prev).set(modelPath, {
                  status: `下载中 ${data.percent.toFixed(1)}%`,
                  total: data.total,
                  completed: data.current
                }));
              } else if (data.status && data.status.includes('downloading')) {
                // 兼容旧格式
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
      render: (_: unknown, record: WhisperModel) => {
        const isDownloading = downloadingModels.has(record.path || '');
        const isDownloaded = record.exists;
        const _progress = downloadProgress.get(record.path || '');

        if (isDownloaded) {
          return <span style={{ color: 'green' }}>已下载</span>;
        }

        if (isDownloading) {
          return <span style={{ color: '#1890ff' }}>下载中...</span>;
        }

        return (
          <Button
            type="primary"
            size="small"
            loading={isDownloading}
            onClick={() => record.path && downloadModel(record.path)}
            disabled={isDownloading || !record.path}
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
