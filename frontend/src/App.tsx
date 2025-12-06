import React, { useEffect, useState } from 'react';
import { Layout, message, Select, DatePicker, Space, Button, Segmented, Tabs, Tooltip } from 'antd';
import { ProjectSidebar } from './components/ProjectSidebar';
import { Deliverables } from './components/Deliverables';
import { TaskSidebar } from './components/TaskSidebar';
import { ChunkList } from './components/ChunkList';
import UserManagement from './components/UserManagement';
import ProjectTaskSidebar from './components/ProjectTaskSidebar';
import TaskDocuments from './components/TaskDocuments';
import { RightPanel } from './components/RightPanel';
import ProjectTaskSelector from './components/ProjectTaskSelector';
import ContextManagerDropdown from './components/project/ContextManagerDropdown';
import { AudioModeSelectModal, AudioMode } from './components/AudioModeSelectModal';
import { UploadModal } from './components/UploadModal';
import { useAudioService } from './services/audioService';
import { listTasks, createTask, deleteTask, startTask, stopTask, listChunks, updateTaskDevice, getServicesStatus, ServicesStatus } from './api/client';
import { authedApi } from './api/auth';
import { smartLogin, loadAuth, clearAuth, onAuthChange, refreshToken } from './api/auth';
import { TaskSummary, ChunkFlag } from './types';
import { ClearOutlined, KeyOutlined, SafetyOutlined, UserOutlined, MenuFoldOutlined, MenuUnfoldOutlined, ApiOutlined } from '@ant-design/icons';
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
// 身份源相关组件
import { IdentityProviderList } from './components/identity-provider';
import LoginSuccessPage from './components/LoginSuccessPage';

const { RangePicker } = DatePicker;

const { Content, Header } = Layout;

type ViewMode = 'meeting' | 'project' | 'user' | 'profile' | 'idp';

const MeetingView: React.FC<{ 
  tasks: TaskSummary[]; currentTask?: string; onSelectTask:(id:string)=>void; refreshTasks:()=>void;
  chunks: ChunkFlag[]; currentChunk?: string; onSelectChunk:(id:string)=>void; onPlay:(id:string)=>void;
  handleCreate: ()=>Promise<void> | void; handleDelete:(id:string)=>Promise<void>; handleStart:(id:string)=>Promise<void>; handleStop:(id:string)=>Promise<void>; handleChangeDevice:(id:string, dev:string)=>Promise<void>;
  filterProductLine?: string; setFilterProductLine:(v: string|undefined)=>void;
  filterMeetingDateRange: [dayjs.Dayjs, dayjs.Dayjs] | null; setFilterMeetingDateRange: (v: [dayjs.Dayjs, dayjs.Dayjs] | null)=>void;
  scopes: string[];
  servicesStatus: ServicesStatus | null;
}> = ({
  tasks, currentTask, onSelectTask, refreshTasks,
  chunks, currentChunk, onSelectChunk, onPlay,
  handleCreate, handleDelete, handleStart, handleStop, handleChangeDevice,
  filterProductLine, setFilterProductLine,
  filterMeetingDateRange, setFilterMeetingDateRange,
  scopes,
  servicesStatus
}) => {
  const canWriteMeeting = scopes.includes('meeting.write');
  const canReadMeeting = canWriteMeeting || scopes.includes('meeting.read');
  
  // 折叠状态
  const [taskSidebarCollapsed, setTaskSidebarCollapsed] = useState(false);
  const [chunkListCollapsed, setChunkListCollapsed] = useState(true); // 默认收起
  
  // 检查是否应该显示 chunk 相关功能
  // 只有当 whisper 和 deps-service 都可用时才显示
  const showChunkFeatures = servicesStatus?.whisper_available && servicesStatus?.deps_service_available;
  
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
        onRefreshTasks={refreshTasks}
        scopes={scopes}
        collapsed={taskSidebarCollapsed}
        onCollapse={setTaskSidebarCollapsed}
      />
      <Content style={{ display:'flex', height: '100%', minHeight: 0 }}>
        {canReadMeeting && showChunkFeatures && (
          <div style={{ 
            width: chunkListCollapsed ? 48 : 280, 
            borderRight:'1px solid #f0f0f0', 
            height: '100%',
            transition: 'width 0.2s ease',
            display: 'flex',
            flexDirection: 'column',
            background: '#fff'
          }}>
            {chunkListCollapsed ? (
              // 折叠状态
              <div style={{ 
                display: 'flex', 
                flexDirection: 'column', 
                alignItems: 'center',
                paddingTop: 8
              }}>
                <Tooltip title="展开音频片段" placement="right">
                  <Button
                    type="text"
                    icon={<MenuUnfoldOutlined />}
                    onClick={() => setChunkListCollapsed(false)}
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
                  音频片段 ({chunks.length})
                </div>
              </div>
            ) : (
              // 展开状态
              <>
                <div style={{ 
                  padding: '8px 12px', 
                  borderBottom: '1px solid #f0f0f0',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center'
                }}>
                  <span style={{ fontWeight: 500, fontSize: 14 }}>音频片段 ({chunks.length})</span>
                  <Tooltip title="收起音频片段">
                    <Button
                      type="text"
                      icon={<MenuFoldOutlined />}
                      onClick={() => setChunkListCollapsed(true)}
                      size="small"
                    />
                  </Tooltip>
                </div>
                <div className="scroll-region" style={{ flex: 1, minHeight: 0 }}>
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
              </>
            )}
          </div>
        )}
        <div className="scroll-region" style={{ flex:1, padding:12, minWidth:0, height: '100%' }}>
          <RightPanel 
            taskId={currentTask||''} 
            chunkId={canReadMeeting && showChunkFeatures ? currentChunk : undefined} 
            canWriteMeeting={canWriteMeeting} 
            canReadMeeting={canReadMeeting}
            showChunkDetails={showChunkFeatures}
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
  const [projectTaskSidebarCollapsed, setProjectTaskSidebarCollapsed] = useState(false);
  const isAdmin = !!auth?.scopes?.includes('user.manage');
  const [showSync, setShowSync] = useState(false);
  const [svnRunning, setSvnRunning] = useState(false);
  const [currentToken, setCurrentToken] = useState<string | null>(null);
  const [servicesStatus, setServicesStatus] = useState<ServicesStatus | null>(null);
  
  // 身份源权限检查
  const { hasPermission } = usePermission();
  const hasIdpReadPermission = hasPermission('idp.read');
  
  // 音频录制模式选择
  const [audioModalOpen, setAudioModalOpen] = useState(false);
  const [uploadModalOpen, setUploadModalOpen] = useState(false);
  const [selectedUploadMode, setSelectedUploadMode] = useState<AudioMode>('file_upload');
  const [pendingTaskId, setPendingTaskId] = useState<string | null>(null);
  
  // 音频服务（使用 pendingTaskId 或 currentTask，优先使用 pendingTaskId）
  const audioService = useAudioService({
    taskId: pendingTaskId || currentTask || '',
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
  
  async function refreshServicesStatus(){
    if(!auth) return;
    try {
      const status = await getServicesStatus();
      setServicesStatus(status);
    } catch(e:any){ 
      console.error('Failed to get services status:', e);
      // 如果获取失败，设置默认值（假设服务不可用）
      setServicesStatus({
        whisper_available: false,
        deps_service_available: false
      });
    }
  }
  
  useEffect(()=>{ if(auth) refreshTasks(); },[auth]);
  useEffect(()=>{ if(auth) refreshChunks(); },[currentTask, auth]);
  useEffect(()=>{ if(auth) refreshTasks(); },[filterProductLine, filterMeetingDateRange, auth]);
  useEffect(()=>{ if(auth) refreshServicesStatus(); },[auth]);
  
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
  async function handleAudioModeSelect(mode: AudioMode, deviceId?: string) {
    console.log('[Debug] handleAudioModeSelect called', { mode, deviceId, pendingTaskId, currentTask });
    setAudioModalOpen(false);
    
    if (!pendingTaskId) {
      console.log('[Debug] No pending task ID, returning');
      return;
    }
    
    try {
      // 设置当前任务（如果还未设置）
      if (currentTask !== pendingTaskId) {
        console.log('[Debug] Setting current task to:', pendingTaskId);
        setCurrentTask(pendingTaskId);
      }
      
      // 根据选择的模式执行相应操作
      if (mode === 'browser_record') {
        // 浏览器录音模式
        console.log('[Debug] Calling audioService.startBrowserRecording() with deviceId:', deviceId);
        await audioService.startBrowserRecording(deviceId);
        console.log('[Debug] startBrowserRecording() completed');
        
        // 同时调用原有的startTask（兼容后端状态管理）
        console.log('[Debug] Calling startTask API for:', pendingTaskId);
        await startTask(pendingTaskId); 
        refreshTasks();
      } else if (mode === 'file_upload' || mode === 'text_upload') {
        // 音频或文本文件上传模式 - 打开上传 Modal
        console.log('[Debug] Opening upload modal for mode:', mode);
        setSelectedUploadMode(mode);
        setUploadModalOpen(true);
        
        // 注意：不在这里调用 startTask，等待文件上传成功后再调用
      }
    } catch(e:any){ 
      console.error('[Debug] Error in handleAudioModeSelect:', e);
      
      // 处理依赖缺失错误（503状态码）
      if (e.response?.status === 503 && e.response?.data) {
        const errorData = e.response.data;
        let errorMsg = errorData.error || '服务不可用';
        
        // 如果有详细的依赖指导信息，展示给用户
        if (errorData.details && errorData.details.length > 0) {
          errorMsg += '\n\n' + errorData.details.join('\n');
        }
        
        message.error({
          content: errorMsg,
          duration: 10, // 显示10秒，给用户充分时间阅读
          style: { whiteSpace: 'pre-line' } // 支持换行显示
        });
      } else {
        message.error(e.message); 
      }
    } finally {
      // 只在浏览器录音模式下清除 pendingTaskId
      // 文件上传模式需要保留 pendingTaskId 直到上传完成
      if (mode === 'browser_record') {
        setPendingTaskId(null);
      }
    }
  }
  
  /**
   * 上传成功后的处理
   */
  async function handleUploadSuccess() {
    console.log('[Debug] Upload success for:', pendingTaskId, 'mode:', selectedUploadMode);
    if (pendingTaskId) {
      try {
        // 根据上传模式决定提示信息
        if (selectedUploadMode === 'file_upload') {
          // 音频文件上传：后端已自动创建 orchestrator 并开始处理
          console.log('[Debug] Audio file uploaded, processing started automatically');
          message.success('音频文件上传成功，正在处理');
        } else if (selectedUploadMode === 'text_upload') {
          // 文本文件上传：不触发处理流程
          console.log('[Debug] Text file uploaded, no processing needed');
          message.success('文本文件上传成功');
        }
        
        refreshTasks();
      } catch (e: any) {
        console.error('[Debug] Error after upload:', e);
        message.error(e.message);
      } finally {
        setPendingTaskId(null);
      }
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
    const url = `/api/v1/tasks/${encodeURIComponent(currentTask)}/audio/chunk_${chunkId}.wav`;
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
    // 检查是否是 OIDC 登录成功回调
    if (window.location.pathname === '/login/success' || window.location.search.includes('token=')) {
      return (
        <LoginSuccessPage 
          onLoginSuccess={(authInfo) => {
            setAuth(authInfo);
            message.success('登录成功');
            // 清理 URL 并重定向到首页
            window.history.replaceState({}, document.title, '/');
          }}
          onNavigateHome={() => {
            window.location.href = '/';
          }}
        />
      );
    }
    
    const doLogin = async (e: React.FormEvent)=>{
      e.preventDefault();
      const form = e.target as HTMLFormElement;
      const fd = new FormData(form);
      const username = String(fd.get('username')||'').trim();
      const password = String(fd.get('password')||'');
      if(!username || !password){ setLoginError('请输入用户名与密码'); return; }
      setLoggingIn(true); setLoginError(null);
      try { 
        // 使用智能登录：先尝试 LDAP，失败后尝试本地认证
        const a = await smartLogin(username, password); 
        setAuth(a); 
        message.success('登录成功'); 
      }
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
        <div style={{fontSize:12,color:'#666',textAlign:'center'}}>支持本地账号或 LDAP 统一登录</div>
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
            options={[
              {label:'会议', value:'meeting'}, 
              {label:'项目', value:'project'}, 
              {label:'用户', value:'user'},
              // 仅当有身份源读取权限时显示
              ...(hasIdpReadPermission ? [{label:'身份源', value:'idp'}] : [])
            ]}
          />
          {viewMode === 'project' && (
            <div style={{display:'flex',alignItems:'center',gap:12}}>
              <ProjectTaskSelector />
              <ContextManagerDropdown
                username={auth?.username || ''}
                projectId={currentProject}
                taskId={currentProjectTask}
              />
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
            servicesStatus={servicesStatus}
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
                collapsed={projectTaskSidebarCollapsed}
                onCollapse={setProjectTaskSidebarCollapsed}
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
        {viewMode === 'idp' && hasIdpReadPermission && (
          <Content style={{ display:'flex', height:'100%', minHeight:0 }}>
            <div className="scroll-region" style={{ flex:1, padding:12, minWidth:0, height:'100%' }}>
              <IdentityProviderList />
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
    
    {/* 文件上传对话框 */}
    <UploadModal
      open={uploadModalOpen}
      mode={selectedUploadMode}
      taskId={pendingTaskId || ''}
      onCancel={() => {
        setUploadModalOpen(false);
        setPendingTaskId(null);
      }}
      onSuccess={handleUploadSuccess}
    />
  </TaskRefreshProvider>
  );
};

export default App;
