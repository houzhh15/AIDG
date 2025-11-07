import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Tabs, Button, Space, Input, message, Table, Tag, Modal, Typography, Divider, Switch } from 'antd';
import { ReloadOutlined, PlusOutlined, DeleteOutlined, EditOutlined, FileTextOutlined, ExclamationCircleOutlined, CloudUploadOutlined, ThunderboltOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { appendDoc, deleteDocChunk, exportDoc, listDocChunks, replaceFull, DocChunk, DocType, AppendResponse, toggleDocChunk, squashDoc } from '../api/taskDocs';
import { MermaidChart } from './MermaidChart';
import { useTaskRefresh } from '../contexts/TaskRefreshContext';

const { TextArea } = Input;
const { Paragraph } = Typography;

interface Props { projectId: string; taskId: string; defaultDoc?: DocType; }

interface DocState {
  loading: boolean;
  chunks: DocChunk[];
  compiled: string;
  metaVersion?: number;
  etag?: string;
  duplicate?: boolean;
  appendLoading: boolean;
  deleteLoadingSeq?: number;
  conflict?: boolean;
  replaceVisible: boolean;
  replaceContent: string;
  expectedVersion?: number; // local optimistic
  lastUpdated?: number;
}

const emptyState: DocState = { loading: false, chunks: [], compiled: '', appendLoading: false, replaceVisible:false, replaceContent:'', expectedVersion: undefined };

const DOCS: DocType[] = ['requirements','design','test'];

export const TaskDocIncremental: React.FC<Props> = ({ projectId, taskId, defaultDoc='requirements' }) => {
  const [active, setActive] = useState<DocType>(defaultDoc);
  const [states, setStates] = useState<Record<DocType, DocState>>({ requirements: {...emptyState}, design:{...emptyState}, test:{...emptyState} });
  const [appendContent, setAppendContent] = useState('');
  const [appendVisible, setAppendVisible] = useState(false);

  const { triggerRefresh } = useTaskRefresh();

  const current = states[active];

  const load = useCallback(async (doc: DocType) => {
    setStates(s => ({...s, [doc]: { ...s[doc], loading:true }}));
    try {
      const [listResp, exportResp] = await Promise.all([
        listDocChunks(projectId, taskId, doc),
        exportDoc(projectId, taskId, doc)
      ]);
      setStates(s => ({...s, [doc]: { ...s[doc], loading:false, chunks:listResp.chunks, compiled:exportResp.content, metaVersion:listResp.meta.version, etag:listResp.meta.etag, expectedVersion:listResp.meta.version, conflict:false }}));
    } catch(e:any){
      message.error(`加载 ${doc} 失败: ${e.message||e}`);
      setStates(s => ({...s, [doc]: { ...s[doc], loading:false }}));
    }
  },[projectId, taskId]);

  useEffect(()=>{ if(projectId && taskId) DOCS.forEach(d=> load(d)); },[projectId, taskId, load]);

  const doAppend = async (op: 'add_full' | 'replace_full' = 'add_full', customContent?: string) => {
    const content = (op==='replace_full'? customContent: appendContent) || '';
    if(!content.trim()) { message.warning('内容不能为空'); return; }
    setStates(s => ({...s, [active]: { ...s[active], appendLoading:true }}));
    try {
  const resp: AppendResponse = await appendDoc(projectId, taskId, active, { content, op, expected_version: current.expectedVersion, source:'ui' });
      if(resp.duplicate){
        message.info('重复内容，未写入（duplicate）');
      } else {
        message.success(op==='replace_full'?'全文替换成功':'追加成功');
      }
      // reload minimal: push if not dup
      await load(active);
      if(op==='add_full') setAppendContent('');
      if(op==='replace_full') setStates(s => ({...s, [active]: { ...s[active], replaceVisible:false, replaceContent:'' }}));
    } catch(e:any){
      if(e?.response?.status === 409){
        message.error('版本冲突，请刷新后重试');
        setStates(s => ({...s, [active]: { ...s[active], conflict:true }}));
      } else {
        message.error(`操作失败: ${e.message||e}`);
      }
    } finally {
      setStates(s => ({...s, [active]: { ...s[active], appendLoading:false }}));
    }
  };

  const doToggle = async (seq:number) => {
    setStates(s=> ({...s, [active]: { ...s[active], deleteLoadingSeq: seq }}));
    try { await toggleDocChunk(projectId, taskId, active, seq); await load(active); message.success('已切换状态'); triggerRefresh(); }
    catch(e:any){ message.error(`切换失败: ${e.message||e}`); }
    finally { setStates(s=> ({...s, [active]: { ...s[active], deleteLoadingSeq: undefined }})); }
  };

  const doSquash = async () => {
    Modal.confirm({
      title:'合并并压缩历史？',
      content:'该操作会将当前所有 Active 内容合并为单一最新 chunk，旧 chunks 归档备份。确认继续？',
      onOk: async ()=>{
        try { await squashDoc(projectId, taskId, active, { expected_version: current.expectedVersion }); await load(active); message.success('合并完成'); }
        catch(e:any){ if(e?.response?.status===409){ message.error('版本冲突，请刷新'); } else { message.error(`合并失败: ${e.message||e}`);} }
      }
    });
  };

  const columns = useMemo(()=>[
    { title:'Seq', dataIndex:'sequence', width:70 },
    { title:'操作类型', dataIndex:'op', width:90, render:(v:string)=> <Tag color={v==='replace_full'||v==='section_full_no_parse'?'volcano':'blue'}>{v==='replace_full'||v==='section_full_no_parse'?'全文替换':'追加'}</Tag> },
    { title:'激活', dataIndex:'active', width:110, fixed:'right' as const, render:(_:any, row:DocChunk)=> (
        <Switch
          checked={row.active}
          checkedChildren="激活"
          unCheckedChildren="停用"
          loading={current.deleteLoadingSeq===row.sequence}
          size='small'
          onChange={()=> doToggle(row.sequence)}
        />
      ) }
  ],[current.deleteLoadingSeq, active]);

  const migrationNotice = useMemo(()=>{
    if(current.chunks.length===1 && current.chunks[0].source==='migration') return <Tag color='purple'>迁移初始版本</Tag>;
    return null;
  },[current.chunks]);

  const labelMap: Record<DocType,string> = { requirements:'需求历史', design:'设计历史', test:'测试历史' };
  const tabItems = DOCS.map(d => ({ key:d, label: labelMap[d], children: (
    <div style={{ display:'flex', gap:16, height:'100%', alignItems:'stretch' }}>
      <div style={{ flex:1, display:'flex', flexDirection:'column', minHeight:0 }}>
        <Space style={{ marginBottom:8 }} wrap>
          <Button icon={<ReloadOutlined/>} onClick={()=>load(d)} size='small'>刷新</Button>
          <Button type='primary' icon={<PlusOutlined/>} size='small' onClick={()=> setAppendVisible(true)}>追加内容</Button>
          <Button icon={<ThunderboltOutlined/>} size='small' onClick={()=> setStates(s=> ({...s, [d]: { ...s[d], replaceVisible:true, replaceContent: current.compiled }}))}>全文替换</Button>
          <Button icon={<CloudUploadOutlined/>} size='small' onClick={doSquash}>合并(压缩)</Button>
          {migrationNotice}
          {current.conflict && <Tag color='red'>版本冲突，刷新后重试</Tag>}
        </Space>
        <div style={{ display:'flex', gap:16, flex:1, minHeight:0 }}>
          <div style={{ flex:1, overflow:'auto', border:'1px solid #eee', padding:12, borderRadius:6, background:'#fafafa' }}>
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                code({ node, className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '');
                  
                  if (match && match[1] === 'mermaid') {
                    const chartContent = String(children).replace(/\n$/, '');
                    return <MermaidChart chart={chartContent} />;
                  }

                  return match ? (
                    <SyntaxHighlighter
                      style={vscDarkPlus}
                      language={match[1]}
                      PreTag="div"
                    >
                      {String(children).replace(/\n$/, '')}
                    </SyntaxHighlighter>
                  ) : (
                    <code className={className} {...props}>
                      {children}
                    </code>
                  );
                },
                table({ children, ...props }) {
                  return (
                    <table 
                      style={{
                        borderCollapse: 'collapse',
                        width: '100%',
                        marginBottom: '16px',
                        border: '1px solid #d0d7de'
                      }} 
                      {...props}
                    >
                      {children}
                    </table>
                  );
                },
                th({ children, ...props }) {
                  return (
                    <th 
                      style={{
                        border: '1px solid #d0d7de',
                        padding: '8px 12px',
                        backgroundColor: '#f6f8fa',
                        fontWeight: '600',
                        textAlign: 'left'
                      }} 
                      {...props}
                    >
                      {children}
                    </th>
                  );
                },
                td({ children, ...props }) {
                  return (
                    <td 
                      style={{
                        border: '1px solid #d0d7de',
                        padding: '8px 12px'
                      }} 
                      {...props}
                    >
                      {children}
                    </td>
                  );
                },
              }}
            >
              {current.compiled || '*（暂无内容）*'}
            </ReactMarkdown>
          </div>
          <div style={{ width:300, display:'flex', flexDirection:'column' }}>
            <Paragraph type='secondary' style={{ fontSize:12, marginBottom:4 }}>版本: {current.metaVersion ?? '-'} | 段数: {current.chunks.length} | ETag: {current.etag?.slice(0,8)}</Paragraph>
            <div style={{ flex:1, minHeight:0 }}>
              <Table size='small' dataSource={current.chunks} columns={columns} rowKey='sequence' pagination={false} scroll={{ y:280 }} />
            </div>
          </div>
        </div>
      </div>
    </div>
  ) }));

  return (
    <div style={{ height:'100%', display:'flex', flexDirection:'column' }}>
      <Tabs activeKey={active} onChange={k=> setActive(k as DocType)} items={tabItems} />
      <Modal
        open={current.replaceVisible}
        title={`全文替换 - ${active}`}
        onCancel={()=> setStates(s=> ({...s, [active]: { ...s[active], replaceVisible:false }}))}
        width={800}
        destroyOnClose
        onOk={()=> doAppend('replace_full', states[active].replaceContent)}
        okText='提交替换'
      >
        <TextArea value={states[active].replaceContent} onChange={e=> setStates(s=> ({...s, [active]: { ...s[active], replaceContent:e.target.value }}))} autoSize={{minRows:12}} placeholder='新的全文内容...' />
        <Divider />
        <Paragraph type='secondary' style={{ fontSize:12 }}>此操作将以 replace_full 方式写入新的编译内容（仍保留历史 chunk）。</Paragraph>
      </Modal>
      <Modal
        open={appendVisible}
        title={`追加内容 - ${active}`}
        onCancel={()=> { setAppendVisible(false); setAppendContent(''); }}
        width={700}
        destroyOnClose
        onOk={()=> { doAppend('add_full', appendContent); setAppendVisible(false); }}
        okButtonProps={{ loading: current.appendLoading }}
        okText='提交追加'
      >
        <TextArea value={appendContent} onChange={e=> setAppendContent(e.target.value)} autoSize={{minRows:10}} placeholder='请输入要追加的内容，不会覆盖已有历史。' />
        <Divider />
        <Paragraph type='secondary' style={{ fontSize:12 }}>本操作将以追加模式写入新的增量内容；若重复最近窗口内容将被判定 duplicate 而不写入。</Paragraph>
      </Modal>
    </div>
  );
};

export default TaskDocIncremental;