import React, { useEffect, useState } from 'react';
import { Button, Dropdown, Empty, Input, MenuProps, message, Modal, Spin } from 'antd';
import { EditOutlined, SaveOutlined, CloseOutlined, CopyOutlined, HistoryOutlined, DeleteOutlined, CheckCircleOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import type { Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import '../../markdown.css';
import { getProjectFeatureList, saveProjectFeatureList, getProjectFeatureListHistory, deleteProjectFeatureListHistory, copyDeliverablesFromTask } from '../../api/projects';
import { TaskSelector } from '../TaskSelector';
import { DiffModal } from '../DiffModal';
import { authedApi } from '../../api/auth';
import { useRefreshTrigger, useTaskRefresh } from '../../contexts/TaskRefreshContext';

const { TextArea } = Input;

interface Props { projectId: string; }

const markdownComponents: Components = {
  table({ children, ...props }) {
    return (
      <div style={{ overflowX: 'auto', margin: '16px 0' }}>
        <table {...props}>
          {children}
        </table>
      </div>
    );
  },
};

export const ProjectFeatureList: React.FC<Props> = ({ projectId }) => {
  const [content, setContent] = useState('');
  const [exists, setExists] = useState(false);
  const [loading, setLoading] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState('');
  const [saving, setSaving] = useState(false);
  const [history, setHistory] = useState<Array<{timestamp:string, content:string, version:number}>>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // copy
  const [showCopyModal, setShowCopyModal] = useState(false);
  const [sourceTaskId, setSourceTaskId] = useState('');
  const [sourceContent, setSourceContent] = useState('');
  const [showDiffModal, setShowDiffModal] = useState(false);
  const [copying, setCopying] = useState(false);
  const [selectedKinds, setSelectedKinds] = useState<string[]>(['feature-list']);
  
  // 监听项目文档刷新事件
  const projectDocRefresh = useRefreshTrigger('project-document');
  const { triggerRefreshFor } = useTaskRefresh();

  async function load(){
    if(!projectId) return;
    setLoading(true);
    try {
      // 加载 markdown 文档
      const r = await getProjectFeatureList(projectId);
      const contentStr = r.content || '';
      setContent(contentStr); 
      setExists(r.exists || false);
    } catch(e:any){ /* ignore */ } finally { setLoading(false); }
  }

  async function loadHistoryFn(){
    if(!projectId) return; setLoadingHistory(true);
    try { const h = await getProjectFeatureListHistory(projectId); setHistory(h); } catch(e:any){ message.error('历史加载失败'); } finally { setLoadingHistory(false); }
  }

  async function save(){
    if(!projectId) return; setSaving(true);
    try { 
      await saveProjectFeatureList(projectId, editContent); 
      message.success('已保存'); 
      setContent(editContent); 
      setExists(true); 
      setIsEditing(false); 
      if(history.length>0) loadHistoryFn();
      
      // 触发项目文档刷新
      triggerRefreshFor('project-document');
    }
    catch(e:any){ message.error('保存失败'); } finally { setSaving(false); }
  }

  async function deleteHistoryVersion(v:number){
    try { await deleteProjectFeatureListHistory(projectId, v); message.success('已删除'); loadHistoryFn(); } catch(e:any){ message.error('删除失败'); }
  }

  async function performCopy(){
    if(!projectId || !sourceTaskId) return;
    setCopying(true);
    try { 
      await copyDeliverablesFromTask(projectId, sourceTaskId, selectedKinds); 
      message.success('拷贝成功'); 
      setShowDiffModal(false); 
      load();
      
      // 触发项目文档刷新
      triggerRefreshFor('project-document');
    }
    catch(e:any){ message.error('拷贝失败'); }
    finally { setCopying(false); }
  }

  const handleCopy = async ()=>{
    if(!sourceTaskId){ message.error('请选择源任务'); return; }
    if(selectedKinds.length===0){ message.error('请选择至少一个交付物'); return; }
    // 如果已有内容且不是空 => 先获取源内容再 diff
    if(exists && content){
      try {
        // 仅获取本组件对应的 deliverable 内容用于 Diff
        const resp = await authedApi.get(`/tasks/${sourceTaskId}/feature-list`);
        setSourceContent(resp.data.content || '');
      } catch { setSourceContent(''); }
      setShowDiffModal(true);
      setShowCopyModal(false);
    } else {
      setShowCopyModal(false);
      performCopy();
    }
  };

  useEffect(()=>{ load(); setIsEditing(false); }, [projectId, projectDocRefresh]);

  const historyMenu: MenuProps['items'] = history.map((h,i)=>({
    key: String(h.version||i+1),
    label: (
      <div style={{ display:'flex', justifyContent:'space-between', minWidth:300 }}>
        <div style={{ flex:1, cursor:'pointer' }} onClick={()=>{ setEditContent(h.content); setIsEditing(true); message.success('已载入历史版本'); }}>
          <div>{new Date(h.timestamp).toLocaleString()}</div>
          <div style={{ fontSize:12, color:'#666' }}>{h.content.slice(0,50)}{h.content.length>50?'...':''}</div>
        </div>
        <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={(e)=>{ e.stopPropagation(); deleteHistoryVersion(h.version||i+1); }} />
      </div>
    )
  }));

  return (
    <div style={{ height:'100%', display:'flex', flexDirection:'column', gap:12 }}>
      <div style={{ display:'flex', justifyContent:'space-between', alignItems:'center' }}>
        <div style={{ display:'flex', alignItems:'center', gap:8 }}>
          <CheckCircleOutlined style={{ color:'#52c41a' }} />
          <span style={{ fontWeight:600, color:'#52c41a' }}>项目特性列表</span>
        </div>
        <div style={{ display:'flex', gap:8 }}>
          {!exists && !isEditing && !loading && (
            <>
              <Button size="small" type="primary" icon={<EditOutlined />} style={{ background:'#52c41a', borderColor:'#52c41a' }} onClick={()=>{ setEditContent('# 项目特性列表'); setIsEditing(true); }}>创建</Button>
              <Button size="small" icon={<CopyOutlined />} style={{ color:'#52c41a' }} onClick={()=> setShowCopyModal(true)}>拷贝</Button>
            </>
          )}
          {exists && !isEditing && (
            <>
              <Dropdown menu={{ items: historyMenu }} trigger={['click']} onOpenChange={(o)=>{ if(o && history.length===0) loadHistoryFn(); }}>
                <Button size="small" type="text" icon={<HistoryOutlined />} style={{ color:'#52c41a' }}>历史</Button>
              </Dropdown>
              <Button size="small" type="text" icon={<EditOutlined />} style={{ color:'#52c41a' }} onClick={()=>{ setEditContent(content); setIsEditing(true); }}>编辑</Button>
              <Button size="small" type="text" icon={<CopyOutlined />} style={{ color:'#52c41a' }} onClick={()=> setShowCopyModal(true)}>拷贝</Button>
            </>
          )}
          {isEditing && (
            <>
              <Button size="small" type="primary" icon={<SaveOutlined />} loading={saving} style={{ background:'#52c41a', borderColor:'#52c41a' }} onClick={save}>保存</Button>
              <Button size="small" icon={<CloseOutlined />} onClick={()=>{ setIsEditing(false); }}>取消</Button>
            </>
          )}
        </div>
      </div>
      <div style={{ flex:1, background:'#f6ffed', border:'1px solid #b7eb8f', borderRadius:8, minHeight:0, display:'flex', flexDirection:'column' }}>
        {loading ? (
          <div style={{ display:'flex', alignItems:'center', justifyContent:'center', height:160, gap:12 }}><Spin /><span>加载中...</span></div>
        ) : isEditing ? (
          <div className="scroll-region" style={{ flex:1, padding:16 }}>
            <TextArea value={editContent} onChange={e=>setEditContent(e.target.value)} autoSize={{ minRows:20, maxRows:40 }} />
          </div>
        ) : !exists ? (
          <div style={{ display:'flex', alignItems:'center', justifyContent:'center', height:160 }}>
            <Empty description={<span style={{ color:'#999' }}>暂无特性列表</span>} />
          </div>
        ) : (
          <div className="scroll-region" style={{ flex:1, padding:16 }}>
            <div className="markdown-body project-markdown">
              <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>{content}</ReactMarkdown>
            </div>
          </div>
        )}
      </div>
      <div style={{ fontSize:11, textAlign:'center', color:'#999' }}>📋 项目级特性清单</div>

      <Modal 
        title="从任务拷贝" 
        open={showCopyModal} 
        onCancel={()=>{ setShowCopyModal(false); }} 
        onOk={handleCopy} 
        okText="拷贝"
        okButtonProps={{ disabled: !sourceTaskId || selectedKinds.length===0 }}
      >
        <p>选择源任务：</p>
        <TaskSelector currentTaskId={''} placeholder="选择任务" onChange={setSourceTaskId} />
        <p style={{ marginTop:12 }}>选择要拷贝的交付物：</p>
        <div style={{ display:'flex', flexDirection:'column', gap:6 }}>
          {['feature-list','architecture-design','tech-design'].map(k=> (
            <label key={k} style={{ userSelect:'none' }}>
              <input type="checkbox" checked={selectedKinds.includes(k)} onChange={(e)=>{
                if(e.target.checked) setSelectedKinds(prev=>[...prev,k]); else setSelectedKinds(prev=>prev.filter(x=>x!==k));
              }} style={{ marginRight:6 }} /> {k}
            </label>
          ))}
        </div>
      </Modal>

      <DiffModal visible={showDiffModal} title="拷贝差异对比 (源 vs 当前)" currentContent={content} sourceContent={sourceContent} onConfirm={performCopy} onCancel={()=>{ setShowDiffModal(false); setSourceTaskId(''); }} loading={copying} />
    </div>
  );
};
