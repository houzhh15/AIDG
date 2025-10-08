import React from 'react';
import { List, Tag, Button, Space, Typography } from 'antd';
import { ChunkFlag } from '../types';
import { PlayCircleOutlined, FileTextOutlined } from '@ant-design/icons';
import { authedApi } from '../api/auth';

interface Props {
  taskId?: string;
  chunks: ChunkFlag[];
  current?: string;
  onSelect: (id: string) => void;
  onPlay: (id: string) => void;
  chunkDuration?: number; // seconds per chunk (may be undefined, we then fetch config)
}

const color = (ok: boolean) => (ok ? 'green' : 'default');

function formatTime(total: number): string {
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = Math.floor(total % 60);
  const pad = (n:number)=> n.toString().padStart(2,'0');
  return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

export const ChunkList: React.FC<Props> = ({ taskId, chunks, current, onSelect, onPlay, chunkDuration }) => {
  const [localDuration,setLocalDuration] = React.useState<number|undefined>(chunkDuration);
  React.useEffect(()=>{ setLocalDuration(chunkDuration); },[chunkDuration]);
  React.useEffect(()=>{
    if(!taskId) return;
    if(localDuration!=null) return; // already have
    (async()=>{
      try {
        const r = await authedApi.get(`/tasks/${taskId}/config`);
        if(typeof r.data.record_chunk_seconds === 'number') setLocalDuration(r.data.record_chunk_seconds);
      } catch { /* ignore */ }
    })();
  },[taskId, localDuration]);
  return (
    <List
      size="small"
      dataSource={chunks}
      style={{ height: '100%', overflow: 'auto', background: '#fff', borderRight: '1px solid #eee' }}
      renderItem={c => (
        <List.Item
          onClick={() => onSelect(c.id)}
          style={{ cursor: 'pointer', background: c.id === current ? '#e6f7ff' : undefined }}
        >
          <Space direction="vertical" style={{ width: '100%' }}>
            <Space style={{ justifyContent: 'space-between', width: '100%' }}>
              <Typography.Text>
                <span style={{ fontWeight:600 }}>{c.id.slice(0,4)}</span>
                {typeof localDuration === 'number' && !isNaN(parseInt(c.id,10)) && parseInt(c.id,10) >=0 && (
                  (()=>{
                    const idx = parseInt(c.id,10);
                    const start = idx * localDuration;
                    const end = start + localDuration;
                    return <span style={{ marginLeft:4, fontWeight:400, color:'#888', fontSize:10 }}>{`(${formatTime(start)} - ${formatTime(end)})`}</span>;
                  })()
                )}
              </Typography.Text>
              <Button size="small" icon={<PlayCircleOutlined />} onClick={(e)=>{ e.stopPropagation(); onPlay(c.id);} } />
            </Space>
            <Space size={1} style={{ display:'flex', flexWrap:'nowrap' }}>
              <Tag style={{ marginBottom:0, fontSize:10, lineHeight:'14px', padding:'0 4px' }} color={color(c.wav)}>WAV</Tag>
              <Tag style={{ marginBottom:0, fontSize:10, lineHeight:'14px', padding:'0 4px' }} color={color(c.segments)}>SEG</Tag>
              <Tag style={{ marginBottom:0, fontSize:10, lineHeight:'14px', padding:'0 4px' }} color={color(c.speakers)}>SPK</Tag>
              <Tag style={{ marginBottom:0, fontSize:10, lineHeight:'14px', padding:'0 4px' }} color={color(c.embeddings)}>EMB</Tag>
              <Tag style={{ marginBottom:0, fontSize:10, lineHeight:'14px', padding:'0 4px' }} color={color(c.mapped)}>MAP</Tag>
              <Tag style={{ marginBottom:0, fontSize:10, lineHeight:'14px', padding:'0 4px' }} color={color(c.merged)}>MRG</Tag>
            </Space>
          </Space>
        </List.Item>
      )}
    />
  );
};
