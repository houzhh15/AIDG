import React, { useEffect, useState } from 'react';
import { Button, Dropdown, Empty, Input, MenuProps, message, Modal, Spin, Tabs, Table } from 'antd';
import { EditOutlined, SaveOutlined, CloseOutlined, CopyOutlined, HistoryOutlined, DeleteOutlined, CheckCircleOutlined, FileTextOutlined, TableOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import type { Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import '../../markdown.css';
import { getProjectFeatureList, saveProjectFeatureList, getProjectFeatureListHistory, deleteProjectFeatureListHistory, copyDeliverablesFromTask, getProjectFeatureListJson } from '../../api/projects';
import { TaskSelector } from '../TaskSelector';
import { DiffModal } from '../DiffModal';
import { authedApi } from '../../api/auth';

const { TextArea } = Input;

interface Props { projectId: string; }

// 特性表格组件
interface FeatureTableProps {
  data: any;
}

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

const FeatureTable: React.FC<FeatureTableProps> = ({ data }) => {
  if (!data || !Array.isArray(data)) {
    return <div style={{ textAlign: 'center', padding: 40, color: '#666' }}>暂无表格数据</div>;
  }

  // 构建单个组件的表格数据
  const buildTableData = (features: any[]) => {
    const tableData: any[] = [];
    features.forEach((feature: any) => {
      if (feature.sub_features && feature.sub_features.length > 0) {
        // 有子特性的情况
        feature.sub_features.forEach((subFeature: any, index: number) => {
          tableData.push({
            key: `${feature.id}-${subFeature.id || index}`,
            l1Feature: index === 0 ? feature.name : '',
            l2Feature: subFeature.name,
            featureId: subFeature.id || `${feature.id}-${index}`,
            description: subFeature.description,
            priority: subFeature.priority,
            source: subFeature.source,
            rowSpan: index === 0 ? feature.sub_features.length : 0,
          });
        });
      } else {
        // 没有子特性的情况
        tableData.push({
          key: feature.id,
          l1Feature: feature.name,
          l2Feature: '',
          featureId: feature.id,
          description: feature.description,
          priority: feature.priority,
          source: feature.source,
          rowSpan: 1,
        });
      }
    });
    return tableData;
  };

  const columns = [
    {
      title: 'L1特性',
      dataIndex: 'l1Feature',
      key: 'l1Feature',
      width: 200,
      render: (text: string, record: any) => ({
        children: <div style={{ fontWeight: 500, fontSize: 13 }}>{text}</div>,
        props: {
          rowSpan: record.rowSpan,
        },
      }),
    },
    {
      title: 'L2特性',
      dataIndex: 'l2Feature',
      key: 'l2Feature',
      width: 200,
      render: (text: string) => (
        <div style={{ fontSize: 12, color: '#333' }}>{text}</div>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: false,
      render: (text: string | string[]) => {
        if (Array.isArray(text)) {
          return (
            <div style={{ fontSize: 12, lineHeight: '1.4' }}>
              {text.map((line, index) => (
                <div key={index} style={{ marginBottom: index < text.length - 1 ? 4 : 0 }}>
                  {line}
                </div>
              ))}
            </div>
          );
        }
        return <div style={{ fontSize: 12, lineHeight: '1.4' }}>{text}</div>;
      },
    },
    {
      title: '特性ID',
      dataIndex: 'featureId',
      key: 'featureId',
      width: 120,
      render: (text: string) => (
        <div style={{ fontSize: 11, fontFamily: 'monospace', color: '#1890ff', fontWeight: 500 }}>
          {text}
        </div>
      ),
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      key: 'priority',
      width: 100,
      render: (priority: string) => {
        const colors: Record<string, string> = {
          'P0': '#ff4d4f',
          'P1': '#faad14',
          'P2': '#52c41a',
          'High': '#ff4d4f',
          'Medium': '#faad14',
          'Low': '#52c41a',
          'Planning': '#1890ff'
        };
        return (
          <span style={{ color: colors[priority] || '#666', fontWeight: 500, fontSize: 12 }}>
            {priority}
          </span>
        );
      },
    },
    {
      title: '来源',
      dataIndex: 'source',
      key: 'source',
      width: 200,
      ellipsis: true,
      render: (text: string) => (
        <div style={{ fontSize: 11, color: '#666' }}>{text}</div>
      ),
    },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      {data.map((componentData: any, index: number) => (
        <div key={index}>
          <div style={{ 
            fontSize: 16, 
            fontWeight: 600, 
            marginBottom: 12, 
            color: '#1890ff',
            padding: '8px 12px',
            backgroundColor: '#f0f9ff',
            border: '1px solid #91d5ff',
            borderRadius: 6
          }}>
            {componentData.component || `组件 ${index + 1}`}
          </div>
          <Table
            columns={columns}
            dataSource={buildTableData(componentData.features || [])}
            pagination={false}
            size="small"
            bordered
            scroll={{ y: 400 }}
            style={{ fontSize: 12 }}
          />
        </div>
      ))}
    </div>
  );
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
  const [activeSubTab, setActiveSubTab] = useState<string>('document');
  const [featureData, setFeatureData] = useState<any>(null);

  // 解析 JSON 数据
  const parseFeatureData = (content: string) => {
    try {
      const data = JSON.parse(content);
      setFeatureData(data);
    } catch (e) {
      setFeatureData(null);
    }
  };

  async function load(){
    if(!projectId) return;
    setLoading(true);
    try {
      // 加载 markdown 文档
      const r = await getProjectFeatureList(projectId);
      const contentStr = r.content || '';
      setContent(contentStr); 
      setExists(r.exists || false);
      
      // 尝试加载 JSON 数据
      try {
        const jsonData = await getProjectFeatureListJson(projectId);
        setFeatureData(jsonData);
      } catch (e) {
        // 如果 JSON 不存在，尝试解析 markdown 内容
        parseFeatureData(contentStr);
      }
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
      parseFeatureData(editContent);
      if(history.length>0) loadHistoryFn(); 
    }
    catch(e:any){ message.error('保存失败'); } finally { setSaving(false); }
  }

  async function deleteHistoryVersion(v:number){
    try { await deleteProjectFeatureListHistory(projectId, v); message.success('已删除'); loadHistoryFn(); } catch(e:any){ message.error('删除失败'); }
  }

  async function performCopy(){
    if(!projectId || !sourceTaskId) return;
    setCopying(true);
    try { await copyDeliverablesFromTask(projectId, sourceTaskId, selectedKinds); message.success('拷贝成功'); setShowDiffModal(false); load(); }
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

  useEffect(()=>{ load(); setIsEditing(false); }, [projectId]);

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
          <Tabs
            activeKey={activeSubTab}
            onChange={setActiveSubTab}
            size="small"
            style={{ flex:1, display:'flex', flexDirection:'column', minHeight:0 }}
            tabBarStyle={{ paddingLeft:16, paddingRight:16, marginBottom:0, flexShrink:0 }}
            items={[
              {
                key: 'document',
                label: <span><FileTextOutlined />文档</span>,
                children: (
                  <div className="scroll-region" style={{ height:'100%', padding:16 }}>
                    <div className="markdown-body project-markdown">
                      <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>{content}</ReactMarkdown>
                    </div>
                  </div>
                )
              },
              {
                key: 'table',
                label: <span><TableOutlined />表格</span>,
                children: (
                  <div className="scroll-region" style={{ height:'100%', padding:16 }}>
                    <FeatureTable data={featureData} />
                  </div>
                )
              }
            ]}
          />
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
