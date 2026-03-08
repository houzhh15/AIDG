import { useCallback, useEffect, useState } from 'react';
import { message } from 'antd';
import {
  getProjectFeatureList,
  saveProjectFeatureList,
  getProjectFeatureListHistory,
  deleteProjectFeatureListHistory,
  getProjectArchitecture,
  saveProjectArchitecture,
  getProjectArchitectureHistory,
  deleteProjectArchitectureHistory,
  getProjectTechDesign,
  saveProjectTechDesign,
  getProjectTechDesignHistory,
  deleteProjectTechDesignHistory,
  copyDeliverablesFromTask
} from '../api/projects';

// 类型
export type DeliverableKind = 'feature-list' | 'architecture-design' | 'tech-design';

interface HistoryItem { timestamp: string; content: string; version: number; }

interface BaseReturn {
  content: string;
  exists: boolean;
  loading: boolean;
  isEditing: boolean;
  editContent: string;
  saving: boolean;
  history: HistoryItem[];
  loadingHistory: boolean;
  setEditContent: (v: string)=>void;
  setIsEditing: (v: boolean)=>void;
  load: ()=>Promise<void>;
  loadHistory: ()=>Promise<void>;
  save: ()=>Promise<void>;
  deleteHistoryVersion: (version: number)=>Promise<void>;
  performCopyFromTask: (taskId: string, kinds: DeliverableKind[], fetchSourceForDiff?: boolean)=>Promise<{sourceContent: string}>;
}

export function useProjectDeliverable(projectId: string, kind: DeliverableKind): BaseReturn {
  const [content, setContent] = useState('');
  const [exists, setExists] = useState(false);
  const [loading, setLoading] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState('');
  const [saving, setSaving] = useState(false);
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  const load = useCallback(async () => {
    if(!projectId) return;
    setLoading(true);
    try {
      let r:any;
      if(kind==='feature-list') r = await getProjectFeatureList(projectId);
      else if(kind==='architecture-design') r = await getProjectArchitecture(projectId);
      else r = await getProjectTechDesign(projectId);
      setContent(r.content||''); setExists(r.exists||false);
    } finally { setLoading(false); }
  }, [projectId, kind]);

  const loadHistory = useCallback(async () => {
    if(!projectId) return; setLoadingHistory(true);
    try {
      let h:any[] = [];
      if(kind==='feature-list') h = await getProjectFeatureListHistory(projectId);
      else if(kind==='architecture-design') h = await getProjectArchitectureHistory(projectId);
      else h = await getProjectTechDesignHistory(projectId);
      setHistory(h);
    } catch { message.error('历史加载失败'); } finally { setLoadingHistory(false); }
  }, [projectId, kind]);

  const save = useCallback(async () => {
    if(!projectId) return; setSaving(true);
    try {
      if(kind==='feature-list') await saveProjectFeatureList(projectId, editContent);
      else if(kind==='architecture-design') await saveProjectArchitecture(projectId, editContent);
      else await saveProjectTechDesign(projectId, editContent);
      setContent(editContent); setExists(true); setIsEditing(false); message.success('已保存');
      if(history.length>0) loadHistory();
    } catch { message.error('保存失败'); } finally { setSaving(false); }
  }, [projectId, kind, editContent, history.length, loadHistory]);

  const deleteHistoryVersion = useCallback(async (version: number) => {
    try {
      if(kind==='feature-list') await deleteProjectFeatureListHistory(projectId, version);
      else if(kind==='architecture-design') await deleteProjectArchitectureHistory(projectId, version);
      else await deleteProjectTechDesignHistory(projectId, version);
      message.success('已删除'); loadHistory();
    } catch { message.error('删除失败'); }
  }, [projectId, kind, loadHistory]);

  const performCopyFromTask = useCallback(async (taskId: string, kinds: DeliverableKind[], fetchSourceForDiff?: boolean)=>{
    if(!projectId || !taskId) return { sourceContent: '' }; 
    let sourceContent = '';
    if(fetchSourceForDiff){
      try {
        const resp = await fetch(`/api/v1/tasks/${taskId}/${kind}`);
        if(resp.ok){ const data = await resp.json(); sourceContent = data.content || ''; }
      } catch { /* ignore */ }
    }
    await copyDeliverablesFromTask(projectId, taskId, kinds);
    return { sourceContent };
  }, [projectId, kind]);

  useEffect(()=>{ load(); setIsEditing(false); }, [load]);

  return { content, exists, loading, isEditing, editContent, saving, history, loadingHistory, setEditContent, setIsEditing, load, loadHistory, save, deleteHistoryVersion, performCopyFromTask };
}
