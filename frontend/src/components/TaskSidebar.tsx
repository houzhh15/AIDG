import React from 'react';
import { Button, Layout, List, Space, Tag, Typography, Popconfirm, message, Tooltip } from 'antd';
import { SettingOutlined } from '@ant-design/icons';
import TaskDetailSettings from './TaskDetailSettings';
import { PlayCircleOutlined, StopOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { TaskSummary, AvDevice } from '../types';
import { listAvfoundationDevices, updateTaskDiarization, updateTaskEmbeddingScript, reprocessTask } from '../api/client';
import { authedApi } from '../api/auth';

// 简单缓存设备列表, 减少重复 ffmpeg 调用
let avCache: { ts: number; devices: AvDevice[] } | null = null;
const CACHE_TTL = 60_000; // 60s

interface Props {
  tasks: TaskSummary[];
  current?: string;
  onSelect: (id: string) => void;
  onCreate: () => void;
  onDelete: (id: string) => Promise<void>;
  onStart: (id: string) => Promise<void>;
  onStop: (id: string) => Promise<void>;
  onChangeDevice: (id: string, dev: string) => Promise<void>;
  scopes?: string[]; // 可选：传入当前用户权限
}

export const TaskSidebar: React.FC<Props> = ({ tasks, current, onSelect, onCreate, onDelete, onStart, onStop, onChangeDevice, scopes }) => {
  const canWriteMeeting = scopes ? scopes.includes('meeting.write') : true;
  const currentTask = tasks.find(t=>t.id===current);
  // moved device/backend settings into TaskDetailSettings drawer
  const [settingsOpen,setSettingsOpen] = React.useState(false);
  const [configSnapshot,setConfigSnapshot] = React.useState<any>(null);
  async function refreshConfig(){
    if(!currentTask) return;
    try {
  // Prefer dedicated config endpoint to populate settings drawer
  const r = await authedApi.get(`/tasks/${currentTask.id}/config`);
      setConfigSnapshot(r.data);
    } catch(e:any){ message.error(e.message); }
  }
  React.useEffect(()=>{ if(settingsOpen){ refreshConfig(); } }, [settingsOpen, currentTask?.id]);
  React.useEffect(()=>{}, [currentTask?.ffmpeg_device, currentTask?.diarization_backend, current]);

  async function loadDevices(force=false){}
  return (
    <Layout.Sider width={280} style={{ background: '#fff', borderRight: '1px solid #eee', height: '100%', position: 'relative' }}>
      <div className="scroll-region" style={{ 
        height: 'calc(100% - 140px)', 
        paddingBottom: '8px' 
      }}>
        <List
          size="small"
          dataSource={tasks}
          renderItem={t => (
            <List.Item
              onClick={() => onSelect(t.id)}
              style={{ cursor: 'pointer', background: t.id === current ? '#f0f5ff' : undefined, padding: '8px 8px' }}
            >
              <div style={{ display:'flex', width:'100%', alignItems:'stretch' }}>
                <div style={{ flex:1, minWidth:0 }}>
                  <Typography.Text strong title={t.id} style={{ display:'block', maxWidth:160, overflow:'hidden', textOverflow:'ellipsis' }}>{t.id}</Typography.Text>
                  {t.product_line && (
                    <div style={{ marginTop: 2 }}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>产品线: {t.product_line}</Typography.Text>
                    </div>
                  )}
                  {t.meeting_time && (
                    <div style={{ marginTop: 2 }}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        会议: {new Date(t.meeting_time).toLocaleString('zh-CN', { 
                          month: 'numeric', 
                          day: 'numeric', 
                          hour: '2-digit', 
                          minute: '2-digit' 
                        })}
                      </Typography.Text>
                    </div>
                  )}
                  <Tag style={{ marginTop:4 }} color={t.state === 'running' ? 'green' : t.state === 'stopping' ? 'orange' : 'default'}>{t.state}</Tag>
                </div>
                <div style={{ display:'flex', flexDirection:'column', gap:4 }} onClick={(e)=>e.stopPropagation()}>
                  <Button 
                    size="small" 
                    type="primary" 
                    icon={<PlayCircleOutlined />} 
                    disabled={!canWriteMeeting || t.state === 'running'} 
                    style={{ background: '#0266B3', borderColor: '#0266B3' }}
                    onClick={() => canWriteMeeting && onStart(t.id).catch(err=>message.error(err.message))}
                  >开始</Button>
                  <Button size="small" danger icon={<StopOutlined />} disabled={!canWriteMeeting || t.state !== 'running'} onClick={() => canWriteMeeting && onStop(t.id).catch(err=>message.error(err.message))}>停止</Button>
                  <Button size="small" disabled={!canWriteMeeting} onClick={() => canWriteMeeting && reprocessTask(t.id).then(()=>message.success('已开始重处理')).catch(err=>message.error(err.message))}>重处理</Button>
                </div>
              </div>
            </List.Item>
          )}
        />
      </div>
      <div style={{ 
        position: 'absolute',
        bottom: 0,
        left: 0,
        right: 0,
        height: '140px',
        padding: 8, 
        borderTop: '1px solid #eee', 
        background: '#fff'
      }}>
        <Space style={{ width: '100%' }}>
          <Button style={{ flex:1 }} icon={<PlusOutlined />} type="dashed" disabled={!canWriteMeeting} onClick={onCreate}>新建任务</Button>
          <Tooltip title={current ? (canWriteMeeting ? '编辑当前任务设置' : '无写权限') : '请选择一个任务'}>
            <Button icon={<SettingOutlined />} disabled={!current || !canWriteMeeting} onClick={()=> setSettingsOpen(true)}>编辑任务</Button>
          </Tooltip>
          {current && (
            <Popconfirm title="确认删除?" onConfirm={() => onDelete(current)}>
              <Tooltip title={canWriteMeeting ? '删除当前任务' : '无写权限'}>
                <Button 
                  size="small" 
                  danger 
                  icon={<DeleteOutlined />} 
                  disabled={!current || !canWriteMeeting}
                />
              </Tooltip>
            </Popconfirm>
          )}
        </Space>
      </div>
  <TaskDetailSettings open={!!currentTask && settingsOpen} onClose={()=>setSettingsOpen(false)} taskId={currentTask?.id||''} initial={configSnapshot} refresh={refreshConfig} />
    </Layout.Sider>
  );
};
