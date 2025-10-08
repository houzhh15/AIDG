import React from 'react';
import { Drawer, Form, InputNumber, Switch, Input, Button, Space, Divider, Typography, message, Select, Tag } from 'antd';
import { updateTaskDiarization, updateTaskEmbeddingScript } from '../api/client';
import { authedApi } from '../api/auth';

interface Props {
  open: boolean;
  onClose: () => void;
  taskId: string;
  initial: any; // config object
  refresh: () => void;
}

const numOrUndefined = (v: any) => (typeof v === 'number' && !isNaN(v) ? v : undefined);

export const TaskDetailSettings: React.FC<Props> = ({ open, onClose, taskId, initial, refresh }) => {
  const [loading, setLoading] = React.useState(false);
  const [renameLoading, setRenameLoading] = React.useState(false);
  const [deviceLoading, setDeviceLoading] = React.useState(false);
  const [devices, setDevices] = React.useState<{index:string;name:string;kind:string}[]>([]);
  const [deviceSelectValue, setDeviceSelectValue] = React.useState<string | undefined>(undefined);
  const [renameId, setRenameId] = React.useState(taskId);
  const [form] = Form.useForm();
  const [fixedSpeakerMode, setFixedSpeakerMode] = React.useState(false);
  React.useEffect(()=>{ if(open){ form.setFieldsValue({
    record_chunk_seconds: numOrUndefined(initial?.record_chunk_seconds),
    sb_overcluster_factor: numOrUndefined(initial?.sb_overcluster_factor),
    sb_merge_threshold: numOrUndefined(initial?.sb_merge_threshold),
    sb_min_segment_merge: numOrUndefined(initial?.sb_min_segment_merge),
  sb_num_speakers: numOrUndefined(initial?.sb_num_speakers),
  sb_min_speakers: numOrUndefined(initial?.sb_min_speakers) ?? 1,
  sb_max_speakers: numOrUndefined(initial?.sb_max_speakers) ?? 8,
    sb_energy_vad: initial?.sb_energy_vad,
    sb_energy_vad_thr: numOrUndefined(initial?.sb_energy_vad_thr),
    sb_reassign_after_merge: initial?.sb_reassign_after_merge,
    embedding_threshold: initial?.embedding_threshold,
    embedding_auto_lower_min: initial?.embedding_auto_lower_min,
    embedding_auto_lower_step: initial?.embedding_auto_lower_step,
    initial_embeddings_path: initial?.initial_embeddings_path,
  whisper_model: initial?.whisper_model,
  whisper_segments: initial?.whisper_segments,
    ffmpeg_device: initial?.ffmpeg_device,
  diarization_backend: initial?.diarization_backend,
  product_line: initial?.product_line,
  meeting_time: initial?.meeting_time ? new Date(initial.meeting_time).toISOString().slice(0, 16) : undefined,
  }); } }, [open, initial, form]);
  React.useEffect(()=>{ setRenameId(taskId); }, [taskId]);
  React.useEffect(()=>{ if(open){ setFixedSpeakerMode( (initial?.sb_num_speakers||0) > 0 ); } }, [open, initial]);

  function deriveSelectValue(ffmpegDevice: string | undefined, list: {index:string;name:string;kind:string}[]): string | undefined {
    if(!ffmpegDevice) return undefined;
    // 支持两种格式: "<index>:<name>" 或仅 ":<name>" 或 直接名称
    const m = ffmpegDevice.match(/^(\d+):(.+)$/);
    if(m){
      const idx = m[1];
      if(list.find(d=>d.index===idx)) return idx; // 用 index 作为 option value
    }
    // 处理 ":Name" 或 纯名称
    let name = ffmpegDevice;
    if(ffmpegDevice.startsWith(':')) name = ffmpegDevice.slice(1);
    const byName = list.find(d=>d.name===name);
    if(byName) return byName.index;
    return undefined;
  }

  async function loadDevices(){
    if(!open) return;
    setDeviceLoading(true);
    try {
      const r = await authedApi.get('/devices/avfoundation');
      // 仅保留音频设备，避免与视频设备 index 冲突
      const audioOnly = (r.data.devices||[]).filter((d:any)=>d.kind==='audio');
      setDevices(audioOnly);
      // 根据当前表单或 initial 的 ffmpeg_device 尝试匹配
      const currentVal = form.getFieldValue('ffmpeg_device') || initial?.ffmpeg_device;
      const sel = deriveSelectValue(currentVal, audioOnly);
      setDeviceSelectValue(sel);
    } catch(e:any){ message.error(e.message); } finally { setDeviceLoading(false); }
  }
  React.useEffect(()=>{ if(open) loadDevices(); }, [open]);

  async function submit(){
    try {
      const values = await form.validateFields();
      // Convert datetime-local format to RFC3339 for meeting_time
      if (values.meeting_time) {
        values.meeting_time = new Date(values.meeting_time).toISOString();
      }
      setLoading(true);
      await authedApi.patch(`/tasks/${taskId}/config`, values);
      message.success('已保存');
      refresh();
      onClose();
    } catch(e:any){ if(e?.errorFields) return; message.error(e.message); } finally { setLoading(false); }
  }

  async function doRename(){
    if(!renameId || renameId===taskId) return;
    setRenameLoading(true);
    try {
      await authedApi.patch(`/tasks/${taskId}/rename`, { new_id: renameId });
      message.success('已重命名, 请重新选择任务');
      // 简单策略: 强制刷新页面或触发父级刷新 (父级目前没有回调, 提示用户)
    } catch(e:any){ message.error(e.message); } finally { setRenameLoading(false); }
  }

  async function onBackendChange(v:string){
    try {
      await updateTaskDiarization(taskId, v);
      message.success('已更新分离后端');
      if(v==='speechbrain') {
        await updateTaskEmbeddingScript(taskId, 'speechbrain/generate_speaker_embeddings_sb.py');
        message.success('已切换 SpeechBrain Embedding 脚本');
      } else if(v==='pyannote') {
        await updateTaskEmbeddingScript(taskId, 'pyannote/generate_speaker_embeddings.py');
        message.success('已切换 Pyannote Embedding 脚本');
      }
      refresh();
    } catch(e:any){ message.error(e.message); }
  }

  async function applyDevice(idx:string){
    try {
      // devices 现在只包含音频设备
      const dev = devices.find(d=>d.index===idx);
      if(!dev){ message.error('设备不存在'); return; }
      const value = `${dev.index}:${dev.name}`;
      // 立即更新本地 form、下拉选中，减少“不同步”体验
      form.setFieldsValue({ ffmpeg_device: value });
      setDeviceSelectValue(dev.index);
      await authedApi.patch(`/tasks/${taskId}/config`, { ffmpeg_device: value });
      message.success('已更新设备: '+ value);
      refresh();
    } catch(e:any){ message.error(e.message); }
  }

  return (
    <Drawer title={`任务设置: ${taskId}`} width={480} open={open} onClose={onClose} destroyOnClose extra={<Space><Button onClick={loadDevices} loading={deviceLoading}>刷新设备</Button><Button type="primary" onClick={submit} loading={loading}>保存</Button></Space>}>
      <Space direction="vertical" style={{width:'100%'}} size="small">
        <Typography.Text strong>重命名任务</Typography.Text>
        <Space>
          <Input size="small" style={{width:260}} value={renameId} onChange={e=>setRenameId(e.target.value.trim())} />
          <Button size="small" type="dashed" disabled={!renameId || renameId===taskId} loading={renameLoading} onClick={doRename}>重命名</Button>
        </Space>
        <Divider style={{margin:'8px 0'}} />
      </Space>
      <Form layout="vertical" form={form}>
        <Divider plain>基础信息</Divider>
        <Form.Item label="产品线" name="product_line">
          <Input placeholder="输入产品线名称" />
        </Form.Item>
        <Form.Item label="会议时间" name="meeting_time">
          <Input type="datetime-local" placeholder="选择会议时间" />
        </Form.Item>
        <Form.Item label="输入设备(FFmpeg)" name="ffmpeg_device" extra={
          deviceSelectValue ? null : (form.getFieldValue('ffmpeg_device')? <Tag color="red">当前值未匹配检测设备</Tag>: null)
        }>
          <Input placeholder=":BlackHole 2ch" onBlur={()=>{
            // 用户手填后尝试匹配
            const v = form.getFieldValue('ffmpeg_device');
            const sel = deriveSelectValue(v, devices);
            setDeviceSelectValue(sel);
          }} />
        </Form.Item>
        <Form.Item label="检测到的音频设备">
          <Select
            placeholder="选择设备后自动保存"
            loading={deviceLoading}
            value={deviceSelectValue}
            onDropdownVisibleChange={(v)=>{ if(v) loadDevices(); }}
            onSelect={(val)=>{ applyDevice(val as string); }}
            options={devices.map(d=>({value:d.index,label:`[${d.index}] ${d.name}`}))}
            showSearch
            optionFilterProp="label"
            allowClear
            onClear={()=>{ setDeviceSelectValue(undefined); }}
          />
        </Form.Item>
        <Form.Item label="分离后端" name="diarization_backend">
          <Select
            options={[{value:'pyannote',label:'pyannote'},{value:'speechbrain',label:'speechbrain'}]}
            onChange={onBackendChange}
          />
        </Form.Item>
        <Form.Item label="分段长度(秒)" name="record_chunk_seconds" rules={[{ type:'number', min:10, message:'>=10'}]}>
          <InputNumber style={{width:'100%'}} min={10} step={5} />
        </Form.Item>
        <Divider plain>SpeechBrain Diarization</Divider>
        <Form.Item label="固定说话人模式" tooltip="开启: 仅使用固定说话人数; 关闭: 使用最小/最大范围" style={{marginBottom:4}}>
          <Switch size="small" checked={fixedSpeakerMode} onChange={(v)=>{
            setFixedSpeakerMode(v);
            if(v){
              const cur = form.getFieldValue('sb_num_speakers');
              if(!cur || cur<=0){ form.setFieldsValue({ sb_num_speakers: 2 }); }
            } else {
              form.setFieldsValue({ sb_num_speakers: 0, sb_min_speakers: form.getFieldValue('sb_min_speakers')||1, sb_max_speakers: form.getFieldValue('sb_max_speakers')||8 });
            }
          }} />
        </Form.Item>
        <Space wrap>
          <Form.Item label="固定说话人" name="sb_num_speakers" tooltip=">0 不启用; 开启模式时生效">
            <InputNumber min={0} max={50} disabled={!fixedSpeakerMode} />
          </Form.Item>
          <Form.Item label="最少说话人" name="sb_min_speakers" tooltip="范围下界" >
            <InputNumber min={1} max={50} disabled={fixedSpeakerMode} />
          </Form.Item>
          <Form.Item label="最多说话人" name="sb_max_speakers" tooltip="范围上界" >
            <InputNumber min={1} max={50} disabled={fixedSpeakerMode} />
          </Form.Item>
          <Form.Item label="过聚类因子" name="sb_overcluster_factor" tooltip=">1 启用过聚类">
            <InputNumber min={1} max={5} step={0.1} />
          </Form.Item>
          <Form.Item label="合并阈值" name="sb_merge_threshold" tooltip="簇中心余弦相似度阈值">
            <InputNumber min={0.5} max={0.99} step={0.01} />
          </Form.Item>
          <Form.Item label="短段最小合并秒" name="sb_min_segment_merge" tooltip="小于此秒尝试合并">
            <InputNumber min={0} max={5} step={0.1} />
          </Form.Item>
        </Space>
        <Space wrap>
          <Form.Item label="能量VAD" name="sb_energy_vad" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item label="VAD阈值" name="sb_energy_vad_thr">
            <InputNumber min={0.1} max={2} step={0.05} />
          </Form.Item>
          <Form.Item label="合并后重贴标签" name="sb_reassign_after_merge" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Space>
        <Divider plain>Embedding 参数</Divider>
        <Space wrap>
          <Form.Item label="初始阈值" name="embedding_threshold">
            <Input />
          </Form.Item>
          <Form.Item label="AutoLower最小" name="embedding_auto_lower_min">
            <Input />
          </Form.Item>
          <Form.Item label="AutoLower步长" name="embedding_auto_lower_step">
            <Input />
          </Form.Item>
        </Space>
        <Form.Item label="初始 Embeddings 路径" name="initial_embeddings_path">
          <Input placeholder="meeting_embeddings.json" />
        </Form.Item>
        <Divider plain>其他</Divider>
        <Space.Compact style={{width:'100%'}}>
          <Form.Item label="Whisper 模型" name="whisper_model" style={{flex:1}}>
            <Input placeholder="ggml-large-v3" />
          </Form.Item>
          <Form.Item label="Segments" name="whisper_segments" tooltip="如 20s; 为空或0表示不加 --segments" style={{width:150}}>
            <Input placeholder="20s" allowClear />
          </Form.Item>
        </Space.Compact>
        <Typography.Paragraph type="secondary" style={{fontSize:12}}>
          修改仅在任务未运行时生效; 运行中请先停止。SpeechBrain 参数会影响后续新 chunk 的说话人分离质量。
        </Typography.Paragraph>
      </Form>
    </Drawer>
  );
};

export default TaskDetailSettings;
