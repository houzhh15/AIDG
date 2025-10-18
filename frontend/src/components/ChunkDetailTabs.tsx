import React, { useEffect, useState } from 'react';
import { Tabs, Spin, Typography, Button, message, Space, Input, InputNumber } from 'antd';
import MarkdownViewer from './MarkdownViewer';
import { getChunkFileRaw, updateSegments, mergeOnly, mergeChunk, debugChunk, redoSpeakers, redoEmbeddings, redoMapped, asrOnce } from '../api/client';
import { SegmentsFile } from '../types';
import { authedApi } from '../api/auth';

interface Props {
  taskId: string;
  chunkId?: string;
  canWriteMeeting?: boolean; // 权限：是否具备 meeting.write
  canReadMeeting?: boolean; // 权限：是否具备 meeting.read（write 包含）
}

export const ChunkDetailTabs: React.FC<Props> = ({ taskId, chunkId, canWriteMeeting, canReadMeeting }) => {
  const [active, setActive] = useState('segments');
  const [loading, setLoading] = useState(false);
  const [raw, setRaw] = useState('');
  const [editable, setEditable] = useState('');
  const [wrap, setWrap] = useState(true);
  const [markdownMode, setMarkdownMode] = useState<{[k:string]:boolean}>({ polish: true });
  const [asrModel,setAsrModel] = useState('');
  const [asrSeg,setAsrSeg] = useState<number>(20);
  const [asrLoading,setAsrLoading] = useState(false);
  const isSegments = active === 'segments';
  const allowWrite = !!canWriteMeeting;
  const allowRead = canReadMeeting ?? allowWrite;
  const isReadOnly = allowRead && !allowWrite;

  // 适配右侧面板的高度，减少尺寸避免滚动条
  const viewportHeight = typeof window !== 'undefined' ? Math.max(window.innerHeight - 300, 300) : 400;

  const baseKinds = ['segments','speakers','embeddings','mapped','merged','merged_all','polish'] as const;
  type KindType = typeof baseKinds[number];
  const kinds: KindType[] = [...baseKinds];

  useEffect(()=>{
    if(!allowRead) return;
    if(active === 'merged_all') { loadMergedAll(); return; }
    if(active === 'polish') { loadPolish(); return; }    if(!chunkId) return;
    load();
    // eslint-disable-next-line
  }, [chunkId, active, taskId, allowRead]);

  // lazy fetch task config to seed defaults for ASR controls
  useEffect(()=>{
    if(!allowRead) return;
    if(taskId){
      authedApi.get(`/tasks/${encodeURIComponent(taskId)}/config`).then(r=>{
        if(!asrModel) setAsrModel(r.data.whisper_model||'');
      }).catch(()=>{});
    }
  },[taskId, allowRead, asrModel]);

  async function runAsrOnce(){
    if(!allowWrite || !taskId || !chunkId) return;
    setAsrLoading(true);
    try {
  // asrSeg 为 0 时，发送字符串 "0"，后端将跳过 --segments 参数
  const segStr = asrSeg === 0 ? '0' : `${asrSeg}s`;
      await asrOnce(taskId, chunkId, asrModel, segStr);
      message.success('ASR完成, 重新加载segments');
      load(true);
    } catch(e:any){ message.error(e.message); }
    finally { setAsrLoading(false); }
  }

  async function loadMergedAll(){
    if(!allowRead) return;
    setLoading(true);
    try {
      const r = await authedApi.get(`/tasks/${encodeURIComponent(taskId)}/merged_all`);
      setRaw(r.data.content || '');
    } catch(e:any){ message.error(e.message); }
    finally { setLoading(false); }
  }

  async function loadPolish(){
    if(!allowRead) return;
    setLoading(true);
    try {
      const r = await authedApi.get(`/tasks/${encodeURIComponent(taskId)}/polish`);
      setRaw(r.data.content || '');
    } catch(e:any){ message.error(e.message); }
    finally { setLoading(false); }
  }

  async function generatePolish(){
    if(!allowWrite) return;
    setLoading(true);
    try {
      message.info('正在生成Polish文件，请稍候...');
      const r = await authedApi.post(`/tasks/${encodeURIComponent(taskId)}/generate_polish`);
      
      if(r.data.error) {
        message.error(`生成失败: ${r.data.error}`);
      } else {
        message.success('Polish文件生成成功！');
        // 生成成功后自动加载内容
        await loadPolish();
      }
    } catch(e:any){ 
      message.error(`生成Polish失败: ${e.message}`); 
    }
    finally { setLoading(false); }
  }

  function toggleMarkdown(tab:string){
    setMarkdownMode(m=>({...m, [tab]: !m[tab]}));
  }

  function copyCurrent(){
    if(!raw) return; navigator.clipboard.writeText(raw).then(()=>message.success('已复制')); }

  function downloadCurrent(filename:string){
    const blob = new Blob([raw], {type:'text/plain'});
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    a.click();
    URL.revokeObjectURL(a.href);
  }

  async function load(noCache:boolean=false){
    if(!allowRead || !chunkId) return;
    setLoading(true);
    try {
      const needsNoCache = ['merged','speakers','embeddings','mapped'].includes(active);
      const txt = await getChunkFileRaw(taskId, chunkId, active, needsNoCache ? true : noCache);
      setRaw(txt);
      if(isSegments){ setEditable(txt); }
    } catch(e:any){ message.error(e.message); }
    finally { setLoading(false); }
  }

  async function save(){
    if(!allowWrite || !chunkId) return;
    try {
      const parsed: SegmentsFile = JSON.parse(editable);
      await updateSegments(taskId, chunkId, parsed);
      message.success('保存成功');
      load();
    } catch(e:any){ message.error(e.message); }
  }

  async function doMerge(){
    if(!allowWrite || !taskId) return;
    try {
      await mergeOnly(taskId);
      message.success('已执行合并');
      if(active==='merged' && chunkId){
        // 重新加载当前 merged 内容
        const txt = await getChunkFileRaw(taskId, chunkId, 'merged');
        setRaw(txt);
      }
  } catch(e:any){ message.error('合并全部失败: '+ e.message); }
  }

  async function doMergeChunk(){
    if(!allowWrite || !taskId || !chunkId) return;
    try {
  await mergeChunk(taskId, chunkId);
      message.success('分块已合并');
  const txt = await getChunkFileRaw(taskId, chunkId, 'merged', true);
      setRaw(txt);
  } catch(e:any){ message.error('单块合并失败: '+ e.message); }
  }

  async function showDebug(){
    if(!allowRead || !taskId || !chunkId) return;
    try {
      const info = await debugChunk(taskId, chunkId);
      message.info('调试信息已在控制台');
      // 简单展示在控制台，避免占用 UI
      // 也可后续改成弹窗
      // eslint-disable-next-line no-console
      console.log('Chunk Debug', info);
    } catch(e:any){ message.error(e.message); }
  }

  const items = kinds.map(k => ({ key: k, label: k.toUpperCase(), children: loading ? <Spin /> : (
    k==='segments' ? (
      <div style={{ display:'flex', flexDirection:'column', gap:8, height: '500px' }}>
        <Space size={4} wrap>
          <Input
            size="small"
            style={{ width:200 }}
            placeholder="Whisper Model"
            value={asrModel}
            onChange={e=>setAsrModel(e.target.value)}
            disabled={!allowWrite}
          />
          <InputNumber
            size="small"
            style={{ width:100 }}
            min={0}
            value={asrSeg}
            onChange={v=>{
              if(v === null || v === undefined) { setAsrSeg(20); return; }
              setAsrSeg(v);
            }}
            addonAfter="s"
            disabled={!allowWrite}
          />
          <Button
            size="small"
            type="primary"
            loading={asrLoading}
            disabled={!allowWrite || !chunkId}
            onClick={runAsrOnce}
            style={{ background:'#0266B3', borderColor:'#0266B3' }}
          >执行ASR</Button>
        </Space>
        {isReadOnly && (
          <Typography.Text type="secondary" style={{ fontSize:12 }}>只读模式：需要 meeting.write 权限才能编辑或重新生成。</Typography.Text>
        )}
        <textarea
          style={{ flex:1, width:'100%', resize:'vertical', minHeight: '380px', fontFamily:'monospace', lineHeight: '1.4', fontSize: 12 }}
          value={editable}
          onChange={e=>setEditable(e.target.value)}
          readOnly={!allowWrite}
        />
        <div style={{ display:'flex', gap:8 }}>
          <Button
            type="primary"
            onClick={save}
            disabled={!allowWrite || !chunkId}
            style={{ background: '#0266B3', borderColor: '#0266B3' }}
          >保存</Button>
          <Button onClick={()=>setEditable(raw)}>重置</Button>
        </div>
      </div>
    ) : k==='merged' ? (
      <div style={{ display:'flex', flexDirection:'column', gap:8, height: '500px' }}>
        <Space wrap>
          <Button size="small" onClick={doMergeChunk} disabled={!allowWrite || !chunkId}>合并当前分块</Button>
          <Button size="small" onClick={doMerge} disabled={!allowWrite}>合并全部(MERGE ALL)</Button>
          <Button size="small" onClick={showDebug} disabled={!allowRead || !chunkId}>调试(DEBUG)</Button>
          <Button size="small" onClick={()=>load(true)} disabled={!allowRead || !chunkId}>刷新(REFRESH)</Button>
          <Typography.Text type="secondary">生成/刷新 merged / merged_all.txt</Typography.Text>
        </Space>
        <pre style={{ flex:1, whiteSpace:'pre-wrap', fontFamily:'monospace', background:'#fafafa', padding:8, overflow:'auto', margin:0 }}>{raw}</pre>
      </div>
    ) : k==='speakers' ? (
      <div style={{ display:'flex', flexDirection:'column', gap:8, height: '500px' }}>
        <Space wrap>
          <Button size="small" disabled={!allowWrite || !chunkId} onClick={async()=>{ if(!allowWrite || !chunkId) return; try { await redoSpeakers(taskId, chunkId); message.success('已重新生成 SPEAKERS'); load(true); } catch(e:any){ message.error(e.message);} }}>重新生成(SPEAKERS)</Button>
        </Space>
        <pre style={{ flex:1, whiteSpace:'pre-wrap', fontFamily:'monospace', background:'#fafafa', padding:8, overflow:'auto', margin:0 }}>{raw}</pre>
      </div>
    ) : k==='embeddings' ? (
      <div style={{ display:'flex', flexDirection:'column', gap:8, height: '500px' }}>
        <Space wrap>
          <Button size="small" disabled={!allowWrite || !chunkId} onClick={async()=>{ if(!allowWrite || !chunkId) return; try { await redoEmbeddings(taskId, chunkId); message.success('已重新生成 EMBEDDINGS'); load(true); } catch(e:any){ message.error(e.message);} }}>重新生成(EMBEDDINGS)</Button>
        </Space>
        <pre style={{ flex:1, whiteSpace:'pre-wrap', fontFamily:'monospace', background:'#fafafa', padding:8, overflow:'auto', margin:0 }}>{raw}</pre>
      </div>
    ) : k==='mapped' ? (
      <div style={{ display:'flex', flexDirection:'column', gap:8, height: '500px' }}>
        <Space wrap>
          <Button size="small" disabled={!allowWrite || !chunkId} onClick={async()=>{ if(!allowWrite || !chunkId) return; try { await redoMapped(taskId, chunkId); message.success('已重新生成 MAPPED'); load(true); } catch(e:any){ message.error(e.message);} }}>重新生成(MAPPED)</Button>
        </Space>
        <pre style={{ flex:1, whiteSpace:'pre-wrap', fontFamily:'monospace', background:'#fafafa', padding:8, overflow:'auto', margin:0 }}>{raw}</pre>
      </div>
    ) : k==='merged_all' ? (
      <div style={{ display:'flex', flexDirection:'column', gap:8, height: '500px' }}>
        <Space wrap>
          <Button size="small" onClick={loadMergedAll} disabled={!allowRead}>刷新(REFRESH)</Button>
          <Button size="small" onClick={()=>setWrap(w=>!w)}>{wrap?'不换行(NOWRAP)':'换行(WRAP)'}</Button>
          <Button size="small" onClick={()=>toggleMarkdown('merged_all')}>{markdownMode['merged_all']?'纯文本(TEXT)':'Markdown(MD)'}</Button>
          <Button size="small" onClick={()=>copyCurrent()} disabled={!allowRead}>复制(COPY)</Button>
          <Button size="small" onClick={()=>downloadCurrent('merged_all.txt')} disabled={!allowRead}>下载(DL)</Button>
          <Typography.Text type="secondary">完整合并结果 merged_all.txt</Typography.Text>
        </Space>
        { markdownMode['merged_all'] ? (
          <div style={{ flex:1, overflow:'auto', background:'#fafafa', padding:8 }}>
            <MarkdownViewer>{raw}</MarkdownViewer>
          </div>
        ) : (
          <pre style={{ flex:1, whiteSpace: wrap?'pre-wrap':'pre', fontFamily:'monospace', background:'#fafafa', padding:8, overflow:'auto', margin:0 }}>{raw}</pre>
        )}
      </div>
    ) : k==='polish' ? (
      <div style={{ display:'flex', flexDirection:'column', gap:8, height: '500px' }}>
        <Space wrap>
          <Button size="small" onClick={generatePolish} type="primary" loading={loading} disabled={!allowWrite}>生成(GENERATE)</Button>
          <Button size="small" onClick={loadPolish} disabled={!allowRead}>刷新(REFRESH)</Button>
          <Button size="small" onClick={()=>setWrap(w=>!w)}>{wrap?'不换行(NOWRAP)':'换行(WRAP)'}</Button>
          <Button size="small" onClick={()=>toggleMarkdown('polish')}>{markdownMode['polish']?'纯文本(TEXT)':'Markdown(MD)'}</Button>
          <Button size="small" onClick={()=>copyCurrent()} disabled={!allowRead}>复制(COPY)</Button>
          <Button size="small" onClick={()=>downloadCurrent('polish_all.md')} disabled={!allowRead}>下载(DL)</Button>
          <Typography.Text type="secondary">润色合成结果 polish_all.md</Typography.Text>
        </Space>
        { markdownMode['polish'] ? (
          <div style={{ flex:1, overflow:'auto', background:'#fafafa', padding:8 }}>
            <MarkdownViewer>{raw}</MarkdownViewer>
          </div>
        ) : (
          <pre style={{ flex:1, whiteSpace: wrap?'pre-wrap':'pre', fontFamily:'monospace', background:'#fafafa', padding:8, overflow:'auto', margin:0 }}>{raw}</pre>
        )}
      </div>
    ) : (
      <pre style={{ whiteSpace:'pre-wrap', fontFamily:'monospace', background:'#fafafa', padding:8, maxHeight: '500px', height: '500px', overflow:'auto', margin:0 }}>{raw}</pre>
    )
  ) }));

  return <Tabs size="small" activeKey={active} onChange={setActive} items={items} />;
};
