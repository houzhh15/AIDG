import React, { useEffect, useState } from 'react';
import { Button, Dropdown, Input, MenuProps, message, Modal, Spin } from 'antd';
import { EditOutlined, SaveOutlined, CloseOutlined, CopyOutlined, HistoryOutlined, DeleteOutlined, CodeOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { getProjectTechDesign, saveProjectTechDesign, getProjectTechDesignHistory, deleteProjectTechDesignHistory, copyDeliverablesFromTask } from '../../api/projects';
import { TaskSelector } from '../TaskSelector';
import { DiffModal } from '../DiffModal';
import { authedApi } from '../../api/auth';

const { TextArea } = Input;

interface Props { projectId: string; }

export const ProjectTechDesign: React.FC<Props> = ({ projectId }) => {
  const [content, setContent] = useState('');
  const [exists, setExists] = useState(false);
  const [loading, setLoading] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState('');
  const [saving, setSaving] = useState(false);
  const [history, setHistory] = useState<Array<{timestamp:string, content:string, version:number}>>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  const [showCopyModal, setShowCopyModal] = useState(false);
  const [sourceTaskId, setSourceTaskId] = useState('');
  const [sourceContent, setSourceContent] = useState('');
  const [showDiffModal, setShowDiffModal] = useState(false);
  const [copying, setCopying] = useState(false);
  const [selectedKinds, setSelectedKinds] = useState<string[]>(['tech-design']);

  async function load(){ if(!projectId) return; setLoading(true); try { const r = await getProjectTechDesign(projectId); setContent(r.content||''); setExists(r.exists||false); } finally { setLoading(false); } }
  async function loadHistoryFn(){ if(!projectId) return; setLoadingHistory(true); try { const h = await getProjectTechDesignHistory(projectId); setHistory(h); } catch { message.error('历史加载失败'); } finally { setLoadingHistory(false); } }
  async function save(){ if(!projectId) return; setSaving(true); try { await saveProjectTechDesign(projectId, editContent); message.success('已保存'); setContent(editContent); setExists(true); setIsEditing(false); if(history.length>0) loadHistoryFn(); } catch { message.error('保存失败'); } finally { setSaving(false); } }
  async function deleteHistoryVersion(v:number){ try { await deleteProjectTechDesignHistory(projectId, v); message.success('已删除'); loadHistoryFn(); } catch { message.error('删除失败'); } }

  async function performCopy(){ if(!projectId || !sourceTaskId) return; setCopying(true); try { await copyDeliverablesFromTask(projectId, sourceTaskId, selectedKinds); message.success('拷贝成功'); setShowDiffModal(false); load(); } catch { message.error('拷贝失败'); } finally { setCopying(false); } }
  const handleCopy = async ()=>{ 
    if(!sourceTaskId){ message.error('请选择源任务'); return; }
    if(selectedKinds.length===0){ message.error('请选择至少一个交付物'); return; }
    if(exists && content){
      try {
        const resp = await authedApi.get(`/tasks/${sourceTaskId}/tech-design`);
        setSourceContent(resp.data.content || '');
      } catch { setSourceContent(''); }
      setShowDiffModal(true); setShowCopyModal(false);
    } else { setShowCopyModal(false); performCopy(); }
  };

  useEffect(()=>{ load(); setIsEditing(false); }, [projectId]);

  const historyMenu: MenuProps['items'] = history.map((h,i)=>({ key:String(h.version||i+1), label:(
    <div style={{ display:'flex', justifyContent:'space-between', minWidth:300 }}>
      <div style={{ flex:1, cursor:'pointer' }} onClick={()=>{ setEditContent(h.content); setIsEditing(true); message.success('已载入历史版本'); }}>
        <div>{new Date(h.timestamp).toLocaleString()}</div>
        <div style={{ fontSize:12, color:'#666' }}>{h.content.slice(0,50)}{h.content.length>50?'...':''}</div>
      </div>
      <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={(e)=>{ e.stopPropagation(); deleteHistoryVersion(h.version||i+1); }} />
    </div>
  ) }));

  return (
    <div style={{ height:'100%', display:'flex', flexDirection:'column', gap:12 }}>
      <div style={{ display:'flex', justifyContent:'space-between', alignItems:'center' }}>
        <div style={{ display:'flex', alignItems:'center', gap:8 }}>
          <CodeOutlined style={{ color:'#1890ff' }} />
          <span style={{ fontWeight:600, color:'#1890ff' }}>项目方案设计</span>
        </div>
        <div style={{ display:'flex', gap:8 }}>
          {!exists && !isEditing && !loading && (
            <>
              <Button size="small" type="primary" icon={<EditOutlined />} style={{ background:'#1890ff', borderColor:'#1890ff' }} onClick={()=>{ setEditContent('# 项目方案设计'); setIsEditing(true); }}>创建</Button>
              <Button size="small" icon={<CopyOutlined />} style={{ color:'#1890ff' }} onClick={()=> setShowCopyModal(true)}>拷贝</Button>
            </>
          )}
          {exists && !isEditing && (
            <>
              <Dropdown menu={{ items: historyMenu }} trigger={['click']} onOpenChange={(o)=>{ if(o && history.length===0) loadHistoryFn(); }}>
                <Button size="small" type="text" icon={<HistoryOutlined />} style={{ color:'#1890ff' }}>历史</Button>
              </Dropdown>
              <Button size="small" type="text" icon={<EditOutlined />} style={{ color:'#1890ff' }} onClick={()=>{ setEditContent(content); setIsEditing(true); }}>编辑</Button>
              <Button size="small" type="text" icon={<CopyOutlined />} style={{ color:'#1890ff' }} onClick={()=> setShowCopyModal(true)}>拷贝</Button>
            </>
          )}
          {isEditing && (
            <>
              <Button size="small" type="primary" icon={<SaveOutlined />} loading={saving} style={{ background:'#1890ff', borderColor:'#1890ff' }} onClick={save}>保存</Button>
              <Button size="small" icon={<CloseOutlined />} onClick={()=>{ setIsEditing(false); }}>取消</Button>
            </>
          )}
        </div>
      </div>
      <div className="scroll-region" style={{ flex:1, background:'#f0f8ff', border:'1px solid #91d5ff', borderRadius:8, padding:16, minHeight:0 }}>
        {loading ? (<div style={{ display:'flex', alignItems:'center', justifyContent:'center', height:160, gap:12 }}><Spin /><span>加载中...</span></div>) : isEditing ? (
          <TextArea value={editContent} onChange={e=>setEditContent(e.target.value)} autoSize={{ minRows:20, maxRows:40 }} />
        ) : !exists ? (
          <div style={{ color:'#999', textAlign:'center', marginTop:40 }}>暂无方案设计</div>
        ) : (
          <ReactMarkdown remarkPlugins={[remarkGfm] as any}>{content}</ReactMarkdown>
        )}
      </div>
      <div style={{ fontSize:11, textAlign:'center', color:'#999' }}>🔧 项目级技术方案</div>

      <Modal 
        title="从任务拷贝" 
        open={showCopyModal} 
        onCancel={()=> setShowCopyModal(false)} 
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
              <input type="checkbox" checked={selectedKinds.includes(k)} onChange={(e)=>{ if(e.target.checked) setSelectedKinds(prev=>[...prev,k]); else setSelectedKinds(prev=>prev.filter(x=>x!==k)); }} style={{ marginRight:6 }} /> {k}
            </label>
          ))}
        </div>
      </Modal>

      <DiffModal visible={showDiffModal} title="拷贝差异对比 (源 vs 当前)" currentContent={content} sourceContent={sourceContent} onConfirm={performCopy} onCancel={()=>{ setShowDiffModal(false); setSourceTaskId(''); }} loading={copying} />
    </div>
  );
};
