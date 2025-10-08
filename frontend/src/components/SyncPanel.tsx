import React, { useState } from 'react';
import { Card, Space, Input, Select, Button, message, Checkbox } from 'antd';
import type { SyncMode } from '../api/sync';
import { dispatchSync } from '../api/sync';
import { adminReload, listTasks } from '../api/client';
import { listProjects } from '../api/projects';

interface Props {
  isAdmin: boolean;
}

const modes: { label: string; value: SyncMode; desc: string }[] = [
  { label: '推送覆盖 (push / client_overwrite)', value: 'client_overwrite', desc: 'Push: 以本机为权威，覆盖目标服务器同名文件' },
  { label: '拉取覆盖 (pull / pull_overwrite)', value: 'pull_overwrite', desc: 'Pull: 获取目标服务器文件并在本机写入覆盖（不会上传本地差异）' },
  { label: '服务端保持 (server_overwrite)', value: 'server_overwrite', desc: 'Server authoritative: 忽略本机文件，只取远端文件摘要（不自动写回）' },
  { label: '缺失填充 (merge_no_overwrite)', value: 'merge_no_overwrite', desc: '只写本机缺失的文件，已存在则跳过' },
];

export const SyncPanel: React.FC<Props> = ({ isAdmin }) => {
  const [target, setTarget] = useState('http://127.0.0.1:8000');
  const [mode, setMode] = useState<SyncMode>('client_overwrite');
  const [loading, setLoading] = useState(false);
  const [reloading, setReloading] = useState(false);
  const [returnFiles, setReturnFiles] = useState(false);
  const [result, setResult] = useState<any>(null);

  if(!isAdmin){ return null; }

  async function runDispatch(){
    setLoading(true);
    try {
      const resp = await dispatchSync(target, mode, returnFiles);
      setResult(resp);
      message.success('后台 HMAC dispatch 完成');
    } catch(e:any){ message.error(e?.response?.data?.error || e.message); }
    finally { setLoading(false); }
  }

  async function runReload(){
    setReloading(true);
    try {
      const resp = await adminReload();
      // Optionally trigger refetch of tasks / projects (fire and forget)
      listTasks().catch(()=>{});
      listProjects().catch(()=>{});
      message.success(resp.message || 'Reload OK');
    } catch(e:any){
      message.error(e?.response?.data?.error || e.message);
    } finally { setReloading(false); }
  }

  return (
    <Card size="small" title="跨主机同步 (Admin)" style={{ marginBottom: 16 }}>
      <Space direction="vertical" style={{ width:'100%' }}>
        <Space wrap>
          <Input style={{ width:260 }} value={target} onChange={e=>setTarget(e.target.value)} placeholder="目标服务器BaseURL" />
          <Select style={{ width:260 }} value={mode} onChange={setMode} options={modes.map(m=>({ label: m.label, value: m.value }))} />
          <Checkbox checked={returnFiles} onChange={e=>setReturnFiles(e.target.checked)}>返回本端文件摘要</Checkbox>
        </Space>
        <div style={{ fontSize:12, color:'#666' }}>{modes.find(m=>m.value===mode)?.desc}</div>
        <Space>
          <Button size="small" type="primary" onClick={runDispatch} loading={loading} danger={mode==='client_overwrite'}>执行同步 (Dispatch)</Button>
          <Button size="small" onClick={runReload} loading={reloading}>后端 Reload (重新扫描)</Button>
        </Space>
        {result && <pre style={{ maxHeight:200, overflow:'auto', fontSize:11, background:'#f5f5f5', padding:8, margin:0 }}>{JSON.stringify(result,null,2)}</pre>}
      </Space>
    </Card>
  );
};

export default SyncPanel;
