import React, { useState, useEffect } from 'react';
import { Modal, Select, Radio, Checkbox, Space, message, Input, Typography, Divider, Alert } from 'antd';
import type { RemoteSafe } from '../api/remotes';
import { listRemotes, testRemoteURL } from '../api/remotes';
import type { CopyMode, CopyResource, CopyPushResponse } from '../api/copy';
import { copyPush } from '../api/copy';

const { Text } = Typography;

export interface CopyDialogProps {
  open: boolean;
  onClose: () => void;
  resourceType: 'meeting' | 'project' | 'task';
  resourceId: string;
  resourceName?: string;
  projectId?: string; // required when resourceType === 'task'
}

export const CopyResourceDialog: React.FC<CopyDialogProps> = ({
  open,
  onClose,
  resourceType,
  resourceId,
  resourceName,
  projectId,
}) => {
  const [remotes, setRemotes] = useState<RemoteSafe[]>([]);
  const [selectedRemoteId, setSelectedRemoteId] = useState<string | undefined>();
  const [customURL, setCustomURL] = useState('');
  const [useCustomURL, setUseCustomURL] = useState(false);
  const [mode, setMode] = useState<CopyMode>('overwrite');
  const [includeAudio, setIncludeAudio] = useState(false);
  const [includeSubTasks, setIncludeSubTasks] = useState(true);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<CopyPushResponse | null>(null);

  useEffect(() => {
    if (open) {
      setResult(null);
      listRemotes().then(data => {
        setRemotes(data || []);
        if (data && data.length > 0 && !selectedRemoteId) {
          setSelectedRemoteId(data[0].id);
        }
      }).catch(() => {});
    }
  }, [open]);

  const typeLabel = { meeting: '会议', project: '项目', task: '任务' }[resourceType];
  const displayName = resourceName || resourceId;

  const handleCopy = async () => {
    const resource: CopyResource = {
      type: resourceType,
      id: resourceId,
    };
    if (resourceType === 'task' && projectId) {
      resource.project_id = projectId;
    }

    const req: any = {
      resources: [resource],
      mode,
      options: {
        include_audio: includeAudio,
        include_sub_tasks: includeSubTasks,
      },
    };

    if (useCustomURL && customURL) {
      req.remote_url = customURL;
    } else if (selectedRemoteId) {
      req.remote_id = selectedRemoteId;
    } else {
      message.warning('请选择目标系统或输入地址');
      return;
    }

    setLoading(true);
    try {
      const resp = await copyPush(req);
      setResult(resp);
      if (resp.remote_response?.success) {
        const s = resp.remote_response.summary;
        message.success(`拷贝完成: ${s.created} 新建, ${s.updated} 更新, ${s.skipped} 跳过`);
      } else {
        message.warning('拷贝完成，但部分资源有错误');
      }
    } catch (e: any) {
      message.error('拷贝失败: ' + (e?.response?.data?.error || e.message));
    } finally {
      setLoading(false);
    }
  };

  const selectedRemote = remotes.find(r => r.id === selectedRemoteId);

  return (
    <Modal
      title={`拷贝${typeLabel}到远端 AIDG`}
      open={open}
      onOk={handleCopy}
      onCancel={onClose}
      okText={loading ? '拷贝中...' : '开始拷贝'}
      cancelText="关闭"
      confirmLoading={loading}
      width={520}
      destroyOnClose
    >
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        {/* Resource info */}
        <div>
          <Text type="secondary">资源:</Text>
          <Text strong style={{ marginLeft: 8 }}>{typeLabel}「{displayName}」</Text>
        </div>

        <Divider style={{ margin: '8px 0' }} />

        {/* Target selection */}
        <div>
          <Text type="secondary" style={{ display: 'block', marginBottom: 4 }}>目标系统:</Text>
          {!useCustomURL ? (
            <Select
              style={{ width: '100%' }}
              value={selectedRemoteId}
              onChange={setSelectedRemoteId}
              placeholder="选择目标 AIDG 系统"
              options={remotes.map(r => ({
                label: `${r.name} (${r.url})`,
                value: r.id,
              }))}
              notFoundContent="暂无远端系统，请先在管理页面添加"
            />
          ) : (
            <Input
              value={customURL}
              onChange={e => setCustomURL(e.target.value)}
              placeholder="http://192.168.1.100:8000"
            />
          )}
          <Checkbox
            checked={useCustomURL}
            onChange={e => setUseCustomURL(e.target.checked)}
            style={{ marginTop: 4 }}
          >
            手动输入地址
          </Checkbox>
        </div>

        <Divider style={{ margin: '8px 0' }} />

        {/* Mode */}
        <div>
          <Text type="secondary" style={{ display: 'block', marginBottom: 4 }}>拷贝模式:</Text>
          <Radio.Group value={mode} onChange={e => setMode(e.target.value)}>
            <Radio value="overwrite">覆盖 (目标已有则替换)</Radio>
            <Radio value="skip_existing">跳过已有 (仅拷贝新文件)</Radio>
          </Radio.Group>
        </div>

        {/* Options */}
        <div>
          <Text type="secondary" style={{ display: 'block', marginBottom: 4 }}>选项:</Text>
          <Space direction="vertical">
            {resourceType === 'meeting' && (
              <Checkbox checked={includeAudio} onChange={e => setIncludeAudio(e.target.checked)}>
                包含音频文件 (.wav)
              </Checkbox>
            )}
            {resourceType === 'project' && (
              <Checkbox checked={includeSubTasks} onChange={e => setIncludeSubTasks(e.target.checked)}>
                包含所有子任务
              </Checkbox>
            )}
          </Space>
        </div>

        {/* Result */}
        {result && (
          <>
            <Divider style={{ margin: '8px 0' }} />
            {result.remote_response?.success ? (
              <Alert
                type="success"
                showIcon
                message="拷贝成功"
                description={`新建 ${result.remote_response.summary.created} 项，更新 ${result.remote_response.summary.updated} 项，跳过 ${result.remote_response.summary.skipped} 项`}
              />
            ) : (
              <Alert
                type="warning"
                showIcon
                message="拷贝部分完成"
                description={
                  <pre style={{ fontSize: 11, maxHeight: 120, overflow: 'auto', margin: 0 }}>
                    {JSON.stringify(result.remote_response, null, 2)}
                  </pre>
                }
              />
            )}
          </>
        )}
      </Space>
    </Modal>
  );
};

export default CopyResourceDialog;
