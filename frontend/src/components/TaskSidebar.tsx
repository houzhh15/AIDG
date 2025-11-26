import React from 'react';
import { Button, Layout, List, Space, Tag, Typography, Popconfirm, message, Tooltip, Input } from 'antd';
import { SettingOutlined, MenuFoldOutlined, MenuUnfoldOutlined, SearchOutlined } from '@ant-design/icons';
import TaskDetailSettings from './TaskDetailSettings';
import { PlayCircleOutlined, StopOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { TaskSummary, AvDevice } from '../types';
import { listAvfoundationDevices, updateTaskDiarization, updateTaskEmbeddingScript, reprocessTask } from '../api/client';
import { authedApi } from '../api/auth';

// 模块加载时立即执行的日志 - 用于验证新代码是否被加载
console.log('[DEBUG] ========== TaskSidebar.tsx MODULE LOADED - BUILD TIME: 2025-10-19 13:32 ==========');

// 简单缓存设备列表, 减少重复 ffmpeg 调用
const avCache: { ts: number; devices: AvDevice[] } | null = null;
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
  onRefreshTasks?: () => void; // 刷新任务列表的回调
  scopes?: string[]; // 可选：传入当前用户权限
  collapsed?: boolean; // 是否折叠
  onCollapse?: (collapsed: boolean) => void; // 折叠状态改变回调
}

export const TaskSidebar: React.FC<Props> = ({ tasks, current, onSelect, onCreate, onDelete, onStart, onStop, onChangeDevice, onRefreshTasks, scopes, collapsed = false, onCollapse }) => {
  const canWriteMeeting = scopes ? scopes.includes('meeting.write') : true;
  const currentTask = tasks.find(t=>t.id===current);
  // 搜索关键词
  const [searchKeyword, setSearchKeyword] = React.useState('');
  // moved device/backend settings into TaskDetailSettings drawer
  const [settingsOpen,setSettingsOpen] = React.useState(false);
  const [configSnapshot,setConfigSnapshot] = React.useState<any>(null);
  async function refreshConfig(){
    if(!currentTask) return;
    try {
  // Prefer dedicated config endpoint to populate settings drawer
  const r = await authedApi.get(`/tasks/${encodeURIComponent(currentTask.id)}/config`);
      console.log('[DEBUG] TaskSidebar: Config API response:', r.data);
      console.log('[DEBUG] TaskSidebar: whisper_model in config:', r.data?.whisper_model);
      setConfigSnapshot(r.data);
    } catch(e:any){ message.error(e.message); }
  }
  React.useEffect(()=>{ 
    console.log('[DEBUG] settingsOpen changed to:', settingsOpen, 'currentTask:', currentTask);
    if(settingsOpen){ 
      console.log('[DEBUG] Calling refreshConfig...');
      refreshConfig(); 
    } 
  }, [settingsOpen, currentTask?.id]);
  
  React.useEffect(()=>{
    console.log('[DEBUG] currentTask changed:', currentTask, 'settingsOpen:', settingsOpen);
    console.log('[DEBUG] open prop will be:', !!currentTask && settingsOpen);
  }, [currentTask, settingsOpen]);
  
  React.useEffect(()=>{}, [currentTask?.ffmpeg_device, currentTask?.diarization_backend, current]);
  
  // 处理任务重命名后的刷新
  const handleAfterRename = () => {
    setSettingsOpen(false);
    if (onRefreshTasks) {
      onRefreshTasks();
    }
  };

  async function loadDevices(force=false){}
  
  // 过滤后的任务列表 - 必须在所有条件分支之前调用 Hook
  const filteredTasks = React.useMemo(() => {
    if (!searchKeyword.trim()) return tasks;
    const keyword = searchKeyword.toLowerCase();
    return tasks.filter(t => 
      t.id.toLowerCase().includes(keyword) ||
      t.product_line?.toLowerCase().includes(keyword)
    );
  }, [tasks, searchKeyword]);
  
  // 折叠状态下的简化视图
  if (collapsed) {
    return (
      <Layout.Sider 
        width={48}
        style={{ 
          background: '#fff', 
          borderRight: '1px solid #eee', 
          height: '100%',
        }}
      >
        <div style={{
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          paddingTop: 8,
        }}>
          <Tooltip title="展开会议列表" placement="right">
            <Button
              type="text"
              icon={<MenuUnfoldOutlined />}
              onClick={() => onCollapse?.(false)}
              style={{ marginBottom: 8 }}
            />
          </Tooltip>
          <div style={{ 
            writingMode: 'vertical-rl', 
            textOrientation: 'mixed',
            color: '#666',
            fontSize: 12,
            marginTop: 8
          }}>
            会议列表 ({tasks.length})
          </div>
        </div>
      </Layout.Sider>
    );
  }
  
  return (
    <Layout.Sider width={280} style={{ background: '#fff', borderRight: '1px solid #eee', height: '100%', position: 'relative' }}>
      {/* 标题栏：显示标题和折叠按钮 */}
      <div style={{ 
        padding: '8px 12px', 
        borderBottom: '1px solid #f0f0f0',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}>
        <span style={{ fontWeight: 500, fontSize: 14 }}>会议列表 ({filteredTasks.length}/{tasks.length})</span>
        <Tooltip title="收起会议列表">
          <Button
            type="text"
            icon={<MenuFoldOutlined />}
            onClick={() => onCollapse?.(true)}
            size="small"
          />
        </Tooltip>
      </div>
      {/* 搜索框 */}
      <div style={{ padding: '8px 12px', borderBottom: '1px solid #f0f0f0' }}>
        <Input
          placeholder="搜索会议..."
          prefix={<SearchOutlined style={{ color: '#bbb' }} />}
          value={searchKeyword}
          onChange={e => setSearchKeyword(e.target.value)}
          allowClear
          size="small"
        />
      </div>
      <div className="scroll-region" style={{ 
        height: 'calc(100% - 140px)', 
        paddingBottom: '8px' 
      }}>
        <List
          size="small"
          dataSource={filteredTasks}
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
        padding: 8, 
        borderTop: '1px solid #eee', 
        background: '#fff'
      }}>
        <Space style={{ width: '100%' }}>
          <Button style={{ flex:1 }} icon={<PlusOutlined />} type="dashed" disabled={!canWriteMeeting} onClick={onCreate}>新建任务</Button>
          <Tooltip title={current ? (canWriteMeeting ? '编辑当前任务设置' : '无写权限') : '请选择一个任务'}>
            <Button icon={<SettingOutlined />} disabled={!current || !canWriteMeeting} onClick={()=> {
              console.log('[DEBUG] Edit button clicked - currentTask:', currentTask, 'settingsOpen will be set to:', true);
              setSettingsOpen(true);
            }}>编辑任务</Button>
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
  <TaskDetailSettings 
    open={!!currentTask && settingsOpen} 
    onClose={()=>setSettingsOpen(false)} 
    taskId={currentTask?.id||''} 
    initial={configSnapshot} 
    refresh={refreshConfig}
    onAfterRename={handleAfterRename}
  />
    </Layout.Sider>
  );
};
