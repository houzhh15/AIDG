import React, { useEffect, useState } from 'react';
import { Layout, message, Select, DatePicker, Space, Button, Segmented, Tabs } from 'antd';
import { ProjectSidebar } from './components/ProjectSidebar';
import { Deliverables } from './components/Deliverables';
import { TaskSidebar } from './components/TaskSidebar';
import { ChunkList } from './components/ChunkList';
import UserManagement from './components/UserManagement';
import ProjectTaskSidebar from './components/ProjectTaskSidebar';
import TaskDocuments from './components/TaskDocuments';
import { RightPanel } from './components/RightPanel';
import ProjectTaskSelector from './components/ProjectTaskSelector';
import { AudioModeSelectModal, AudioMode } from './components/AudioModeSelectModal';
import { useAudioService } from './services/audioService';
import { listTasks, createTask, deleteTask, startTask, stopTask, listChunks, updateTaskDevice } from './api/client';
import { authedApi } from './api/auth';
import { login, loadAuth, clearAuth, onAuthChange, refreshToken } from './api/auth';
import { TaskSummary, ChunkFlag } from './types';
import { ClearOutlined, KeyOutlined, SafetyOutlined, UserOutlined } from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import { SyncPanel } from './components/SyncPanel';
import { svnSync } from './api/svn';
import { RoleManagement } from './components/role/RoleManagement';
import { usePermission } from './hooks/usePermission';
import { useProjectPermission } from './hooks/useProjectPermission';
import { ScopeUserManage } from './constants/permissions';
import { UserProfile } from './components/UserProfile';
import NoPermissionPage from './components/NoPermissionPage';
import { TaskRefreshProvider } from './contexts/TaskRefreshContext';

const { RangePicker } = DatePicker;

const { Content, Header } = Layout;

type ViewMode = 'meeting' | 'project' | 'user' | 'profile';

const MeetingView: React.FC<{ 
  tasks: TaskSummary[]; currentTask?: string; onSelectTask:(id:string)=>void; refreshTasks:()=>void;
  chunks: ChunkFlag[]; currentChunk?: string; onSelectChunk:(id:string)=>void; onPlay:(id:string)=>void;
  handleCreate: ()=>Promise<void> | void; handleDelete:(id:string)=>Promise<void>; handleStart:(id:string)=>Promise<void>; handleStop:(id:string)=>Promise<void>; handleChangeDevice:(id:string, dev:string)=>Promise<void>;
  filterProductLine?: string; setFilterProductLine:(v: string|undefined)=>void;
  filterMeetingDateRange: [dayjs.Dayjs, dayjs.Dayjs] | null; setFilterMeetingDateRange: (v: [dayjs.Dayjs, dayjs.Dayjs] | null)=>void;
  scopes: string[];
}> = ({
  tasks, currentTask, onSelectTask, refreshTasks,
  chunks, currentChunk, onSelectChunk, onPlay,
  handleCreate, handleDelete, handleStart, handleStop, handleChangeDevice,
  filterProductLine, setFilterProductLine,
  filterMeetingDateRange, setFilterMeetingDateRange,
  scopes
}) => {
  const canWriteMeeting = scopes.includes('meeting.write');
  const canReadMeeting = canWriteMeeting || scopes.includes('meeting.read');
  return (
    <>
      <TaskSidebar
        tasks={tasks}
        current={currentTask}
        onSelect={onSelectTask}
        // 如果没有写入权限则禁用相关操作
        onCreate={canWriteMeeting ? (async ()=>{ await handleCreate(); }) : (()=>{})}
        onDelete={canWriteMeeting ? (async (id)=>{ await handleDelete(id); }) : (async()=>{})}
        onStart={canWriteMeeting ? (async (id)=>{ await handleStart(id); }) : (async()=>{})}
        onStop={canWriteMeeting ? (async (id)=>{ await handleStop(id); }) : (async()=>{})}
        onChangeDevice={canWriteMeeting ? (async (id, dev)=>{ await handleChangeDevice(id, dev); }) : (async()=>{})}
        scopes={scopes}
      />
      <Content style={{ display:'flex', height: '100%', minHeight: 0 }}>
        {canReadMeeting && (
          <div className="scroll-region" style={{ width:280, borderRight:'1px solid #f0f0f0', height: '100%' }}>
            {(() => { const taskObj = tasks.find(t=>t.id===currentTask); return (
              <ChunkList 
                taskId={currentTask}
                chunks={chunks} 
                current={currentChunk} 
                onSelect={onSelectChunk} 
                onPlay={onPlay} 
                chunkDuration={taskObj?.record_chunk_seconds}
              />); })()}
          </div>
        )}
        <div className="scroll-region" style={{ flex:1, padding:12, minWidth:0, height: '100%' }}>
          <RightPanel 
            taskId={currentTask||''} 
            chunkId={canReadMeeting ? currentChunk : undefined} 
            canWriteMeeting={canWriteMeeting} 
            canReadMeeting={canReadMeeting}
          />
        </div>
      </Content>
    </>
  );
};

const App: React.FC = () => {
  const [tasks, setTasks] = useState<TaskSummary[]>([]);
  const [auth, setAuth] = useState(loadAuth());
  const [loggingIn, setLoggingIn] = useState(false);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [currentTask, setCurrentTask] = useState<string | undefined>();
  const [chunks, setChunks] = useState<ChunkFlag[]>([]);
  const [currentChunk, setCurrentChunk] = useState<string | undefined>();
  const [filterProductLine, setFilterProductLine] = useState<string | undefined>();
  const [filterMeetingDateRange, setFilterMeetingDateRange] = useState<[Dayjs, Dayjs] | null>(null);
  const [viewMode, setViewMode] = useState<ViewMode>('project');
  const [currentProject, setCurrentProject] = useState<string | undefined>();
  const [currentProjectTask, setCurrentProjectTask] = useState<string | undefined>();
  const isAdmin = !!auth?.scopes?.includes('user.manage');
  const [showSync, setShowSync] = useState(false);
  const [svnRunning, setSvnRunning] = useState(false);
  const [currentToken, setCurrentToken] = useState<string | null>(null);
  
  // 音频录制模式选择
  const [audioModalOpen, setAudioModalOpen] = useState(false);
  const [pendingTaskId, setPendingTaskId] = useState<string | null>(null);
  
  // 音频服务（始终初始化，但使用空字符串作为默认值）
  const audioService = useAudioService({
    taskId: currentTask || '',
    onSuccess: () => refreshTasks(),
    onError: (err) => message.error(err.message)
  });
  
  // 项目权限检查
  const { 
    hasPermission: hasProjectPermission, 
    loading: permissionLoading, 
    checkPermission 
  } = useProjectPermission();
  const handleSVNSync = async ()=>{
    if(svnRunning) return;
    setSvnRunning(true);
    try{
      const res = await svnSync();
      message.success('SVN同步完成');
      console.log('[SVN output]', res.output);
    }catch(err:any){
      message.error('SVN同步失败: ' + (err?.response?.data?.error || err.message));
      console.error('SVN sync error', err);
    }finally{
      setSvnRunning(false);
    }
  };

  async function refreshTasks(){
    if(!auth) return;
    try {
      const params: any = {};
      if(filterProductLine) params.product_line = filterProductLine;
      if(filterMeetingDateRange){
        const [startDate, endDate] = filterMeetingDateRange;
        params.meeting_time_start = startDate.format('YYYY-MM-DD');
        params.meeting_time_end = endDate.format('YYYY-MM-DD');
      }
      const r = await authedApi.get('/tasks', { params });
      const t: TaskSummary[] = r.data.tasks || [];
      setTasks(t);
      if(!currentTask && t.length>0) setCurrentTask(t[0].id);
    } catch(e:any){ message.error(e?.response?.data?.error || e.message); }
  }
  async function refreshChunks(){
    if(!auth) return;
    if(!currentTask) return; 
    try { 
      const ch = await listChunks(currentTask); 
      setChunks(ch); 
      if(!currentChunk && ch.length>0) setCurrentChunk(ch[0].id); 
    } catch(e:any){ /* ignore */ }
  }
  useEffect(()=>{ if(auth) refreshTasks(); },[auth]);
  useEffect(()=>{ if(auth) refreshChunks(); },[currentTask, auth]);
  useEffect(()=>{ if(auth) refreshTasks(); },[filterProductLine, filterMeetingDateRange, auth]);
  
  // 当选择项目时检查权限
  useEffect(() => {
    if (currentProject && auth) {
      checkPermission(currentProject);
    }
  }, [currentProject, auth, checkPermission]);
  
  // Listen for auth changes (e.g., when 401 triggers clearAuth)
  useEffect(() => {
    const unsubscribe = onAuthChange((newAuth) => {
      setAuth(newAuth);
      if (!newAuth) {
        // Clear current selections when logged out
        setCurrentTask(undefined);
        setCurrentProject(undefined);
        setTasks([]);
        setChunks([]);
      }
    });
    return unsubscribe;
  }, []);

  async function handleCreate(){
    try { const t = await createTask(); message.success('已创建'); setCurrentTask(t.id); refreshTasks(); }
    catch(e:any){ message.error(e.message); }
  }
  async function handleDelete(id:string){
    try { await deleteTask(id); if(currentTask===id) { setCurrentTask(undefined); setChunks([]); } refreshTasks(); }
    catch(e:any){ message.error(e.message); }
  }
  
  /**
   * 处理"开始"按钮点击
   * 弹出模态框让用户选择录音方式
   */
  async function handleStart(id:string){ 
    try { 
      // 保存待处理的任务ID
      setPendingTaskId(id);
      // 打开录音模式选择对话框
      setAudioModalOpen(true);
    } catch(e:any){ 
      message.error(e.message); 
    } 
  }
  
  /**
   * 用户选择录音模式后的处理
   */
  async function handleAudioModeSelect(mode: AudioMode) {
    setAudioModalOpen(false);
    
    if (!pendingTaskId) return;
    
    try {
      // 设置当前任务（如果还未设置）
      if (currentTask !== pendingTaskId) {
        setCurrentTask(pendingTaskId);
      }
      
      // 根据选择的模式执行相应操作
      if (mode === 'browser_record') {
        // 浏览器录音模式
        await audioService.startBrowserRecording();
      } else if (mode === 'file_upload') {
        // 文件上传模式
        audioService.triggerFileUpload();
      }
      
      // 同时调用原有的startTask（兼容后端状态管理）
      await startTask(pendingTaskId); 
      refreshTasks();
    } catch(e:any){ 
      message.error(e.message); 
    } finally {
      setPendingTaskId(null);
    }
  }
  
  async function handleStop(id:string){ 
    try { 
      // 如果正在录音，停止录音
      if (audioService.isRecording) {
        await audioService.stopRecording();
      }
      
      // 调用后端停止
      await stopTask(id); 
      refreshTasks(); 
    } catch(e:any){ 
      message.error(e.message); 
    } 
  }
  
  async function handleChangeDevice(id:string, dev:string){ try { await updateTaskDevice(id, dev); message.success('设备已更新'); refreshTasks(); } catch(e:any){ message.error(e.message); } }

  function onPlay(chunkId: string){
    if(!currentTask) return;
    const url = `/api/v1/tasks/${currentTask}/audio/chunk_${chunkId}.wav`;
    const audio = new Audio(url); audio.play();
  }

  async function handleGetToken() {
    try {
      const authInfo = await refreshToken();
      // 直接显示token在界面上
      setCurrentToken(authInfo.token);
      message.success('新Token已生成');
    } catch (e: any) {
      message.error(`获取Token失败: ${e.response?.data?.error || e.message}`);
    }
  }

  if(!auth){
    const doLogin = async (e: React.FormEvent)=>{
      e.preventDefault();
      const form = e.target as HTMLFormElement;
      const fd = new FormData(form);
      const username = String(fd.get('username')||'').trim();
      const password = String(fd.get('password')||'');
      if(!username || !password){ setLoginError('请输入用户名与密码'); return; }
      setLoggingIn(true); setLoginError(null);
      try { const a = await login(username, password); setAuth(a); message.success('登录成功'); }
      catch(err:any){ setLoginError(err?.response?.data?.error || err.message); }
      finally { setLoggingIn(false); }
    };
    return <div style={{display:'flex',alignItems:'center',justifyContent:'center',height:'100dvh',background:'#f5f7fa'}}>
      <form onSubmit={doLogin} style={{width:340,padding:32,background:'#fff',borderRadius:8,boxShadow:'0 4px 12px rgba(0,0,0,0.08)',display:'flex',flexDirection:'column',gap:16}}>
        <h2 style={{margin:0,textAlign:'center'}}>登录</h2>
        <div style={{display:'flex',flexDirection:'column',gap:4}}>
          <label>用户名</label>
          <input name="username" placeholder="请输入用户名" style={{padding:8,border:'1px solid #ccc',borderRadius:4}} />
        </div>
        <div style={{display:'flex',flexDirection:'column',gap:4}}>
          <label>密码</label>
          <input name="password" type="password" placeholder="请输入密码" style={{padding:8,border:'1px solid #ccc',borderRadius:4}} />
        </div>
        {loginError && <div style={{color:'#c00',fontSize:12}}>{loginError}</div>}
        <button disabled={loggingIn} style={{padding:'10px 12px',background:'#0266B3',color:'#fff',border:'none',borderRadius:4,cursor:'pointer'}}>{loggingIn?'登录中...':'登录'}</button>
  <div style={{fontSize:12,color:'#666',textAlign:'center'}}>请输入已分配的账号与密码</div>
      </form>
    </div>;
  }

  return (
  <TaskRefreshProvider>
  <Layout className="app-root-layout" style={{ height: '100dvh', overflow: 'hidden' }}>
  <Header 
    onWheel={(e)=>{ e.preventDefault(); e.stopPropagation(); }}
    style={{ height:64, lineHeight:'64px', display:'flex', alignItems:'center', justifyContent:'space-between', padding:'0 16px', background:'#0266B3' }}>
        <div style={{ display:'flex', flex:1, alignItems:'center', gap:20, minWidth:0 }}>
          <div style={{ display:'flex', flexDirection:'column', justifyContent:'center' }}>
            <div style={{ color:'#fff', fontWeight:600, letterSpacing:0.5, lineHeight:1 }}>Meeting & Project Copilot</div>
            <div 
              style={{color:'#d0e6f9', fontSize:11, lineHeight:1.2, cursor:'pointer', userSelect:'none'}} 
              onClick={() => setViewMode('profile')}
              title="点击查看个人中心"
            >
              用户: {auth?.username}
            </div>
          </div>
          <Segmented
            size="small"
            value={viewMode}
            onChange={(v)=> setViewMode(v as ViewMode)}
            options={[{label:'会议', value:'meeting'}, {label:'项目', value:'project'}, {label:'用户', value:'user'}]}
          />
          {viewMode === 'project' && (
            <div style={{display:'flex',alignItems:'center',gap:12}}>
              <ProjectTaskSelector />
            </div>
          )}
          {viewMode === 'meeting' && (
            <div style={{display:'flex',alignItems:'center',gap:12}}>
              <Select
                placeholder="筛选产品线"
                style={{ width: 140 }}
                allowClear
                value={filterProductLine}
                onChange={setFilterProductLine}
                options={[
                  ...new Set(tasks.map(t => t.product_line).filter(Boolean))
                ].map(line => ({ label: line, value: line }))}
              />
              <RangePicker
                placeholder={['开始日期', '结束日期']}
                style={{ width: 220 }}
                onChange={(dates) => setFilterMeetingDateRange(dates as [Dayjs, Dayjs] | null)}
                allowClear
              />
              <Button 
                icon={<ClearOutlined />} 
                onClick={() => {
                  setFilterProductLine(undefined);
                  setFilterMeetingDateRange(null);
                }}
                style={{ color: '#fff', borderColor: '#fff' }}
                ghost
                size="small"
              >
                清空筛选
              </Button>
            </div>
          )}
        </div>
        <div style={{ display:'flex', alignItems:'center', gap:12 }}>
          <Button 
            size="small" 
            ghost 
            style={{color:'#fff',borderColor:'#fff'}} 
            icon={<KeyOutlined />}
            onClick={handleGetToken}
            title="生成新Token并在界面上显示"
          >
            获取Token
          </Button>
          <Button 
            size="small" 
            ghost 
            style={{color:'#fff',borderColor:'#fff'}} 
            icon={<UserOutlined />}
            onClick={() => setViewMode('profile')}
            title="查看个人中心"
          >
            个人中心
          </Button>
          <Button size="small" ghost style={{color:'#fff',borderColor:'#fff'}} onClick={()=>{ clearAuth(false); }}>登出</Button>
        </div>
      </Header>
      {currentToken && (
        <div style={{ background:'#e6f7ff', padding:'8px 12px', borderBottom:'1px solid #91d5ff', display:'flex', alignItems:'center', gap:8 }}>
          <span style={{ fontWeight:'bold', color:'#1890ff' }}>Token:</span>
          <code style={{ background:'#f6f8fa', padding:'2px 4px', borderRadius:2, fontSize:'12px', wordBreak:'break-all' }}>{currentToken}</code>
          <Button size="small" type="link" onClick={() => setCurrentToken(null)} style={{ padding:0, height:'auto' }}>关闭</Button>
        </div>
      )}
      {isAdmin && showSync && (
        <div style={{ background:'#f6f8fa', padding:'8px 12px', borderBottom:'1px solid #ddd' }}>
          <SyncPanel isAdmin={isAdmin} />
        </div>
      )}
      <Layout style={{ flex:1, minHeight:0, overflow: 'hidden' }}>
        {viewMode === 'meeting' && auth && (
          <MeetingView
            tasks={tasks}
            currentTask={currentTask}
            onSelectTask={setCurrentTask}
            refreshTasks={refreshTasks}
            chunks={chunks}
            currentChunk={currentChunk}
            onSelectChunk={setCurrentChunk}
            onPlay={onPlay}
            handleCreate={handleCreate}
            handleDelete={handleDelete}
            handleStart={handleStart}
            handleStop={handleStop}
            handleChangeDevice={handleChangeDevice}
            filterProductLine={filterProductLine}
            setFilterProductLine={setFilterProductLine}
            filterMeetingDateRange={filterMeetingDateRange}
            setFilterMeetingDateRange={setFilterMeetingDateRange}
            scopes={auth.scopes}
          />
        )}
        {viewMode === 'project' && (
          <>
            <ProjectSidebar current={currentProject} onSelect={(projectId) => {
              setCurrentProject(projectId);
              setCurrentProjectTask(undefined); // 重置任务选择
            }} />
            {currentProject && hasProjectPermission && !permissionLoading && (
              <ProjectTaskSidebar 
                projectId={currentProject}
                currentTask={currentProjectTask}
                onTaskSelect={setCurrentProjectTask}
              />
            )}
            <Content style={{ display:'flex', height:'100%', minHeight:0 }}>
              <div className="scroll-region" style={{ flex:1, padding:12, minWidth:0, height:'100%' }}>
                {!currentProject ? (
                  <div style={{ height:'100%', display:'flex', alignItems:'center', justifyContent:'center', color:'#999' }}>请选择或创建一个项目</div>
                ) : permissionLoading ? (
                  <div style={{ height:'100%', display:'flex', alignItems:'center', justifyContent:'center' }}>
                    <div>检查权限中...</div>
                  </div>
                ) : !hasProjectPermission ? (
                  <NoPermissionPage 
                    projectId={currentProject}
                    onBack={() => setCurrentProject(undefined)}
                  />
                ) : !currentProjectTask ? (
                  <div style={{ height:'100%', display:'flex', flexDirection:'column', gap:16 }}>
                    
                    <Deliverables mode="project" targetId={currentProject} />
                  </div>
                ) : (
                  <TaskDocuments projectId={currentProject} taskId={currentProjectTask} />
                )}
              </div>
            </Content>
          </>
        )}
        {viewMode === 'user' && (
          <Content style={{ display:'flex', height:'100%', minHeight:0 }}>
            <div className="scroll-region" style={{ flex:1, padding:12, minWidth:0, height:'100%' }}>
              <Tabs
                defaultActiveKey="users"
                items={[
                  {
                    key: 'users',
                    label: (
                      <span>
                        <UserOutlined /> 用户管理
                      </span>
                    ),
                    children: <UserManagement />,
                  },
                  {
                    key: 'roles',
                    label: (
                      <span>
                        <SafetyOutlined /> 角色管理
                      </span>
                    ),
                    children: <RoleManagement />,
                  },
                ]}
                style={{ height: '100%' }}
              />
            </div>
          </Content>
        )}
        {viewMode === 'profile' && (
          <Content style={{ display:'flex', height:'100%', minHeight:0 }}>
            <div className="scroll-region" style={{ flex:1, padding:12, minWidth:0, height:'100%' }}>
              <UserProfile />
            </div>
          </Content>
        )}
      </Layout>
    </Layout>
    
    {/* 音频录制模式选择对话框 */}
    <AudioModeSelectModal
      open={audioModalOpen}
      onCancel={() => {
        setAudioModalOpen(false);
        setPendingTaskId(null);
      }}
      onConfirm={handleAudioModeSelect}
    />
  </TaskRefreshProvider>
  );
};

export default App;
