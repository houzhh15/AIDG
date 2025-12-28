/**
 * 会议文档组件 - 统一样式
 * 与项目文档 (ProjectDocument.tsx) 保持相同的 UI 风格
 */
import React, { useState, useEffect, useCallback } from 'react';
import { Spin, Button, message, Space, Empty, Modal } from 'antd';
import {
  FileTextOutlined,
  EditOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { 
  exportMeetingDoc, 
  type MeetingDocSlot,
} from '../../api/meetingDocs';
import MarkdownViewer from '../MarkdownViewer';
import DocumentTOC from '../DocumentTOC';
import MeetingDocSectionEditor from './MeetingDocSectionEditor';
import { useTaskRefresh, useRefreshTrigger } from '../../contexts/TaskRefreshContext';

interface Props {
  meetingId: string;
  slot: MeetingDocSlot;
  title: string;
  color?: string; // 主题色
}

const MeetingDocument: React.FC<Props> = ({ 
  meetingId, 
  slot, 
  title,
  color = '#1890ff' 
}) => {
  const [content, setContent] = useState('');
  const [exists, setExists] = useState(false);
  const [version, setVersion] = useState(0);
  const [loading, setLoading] = useState(false);
  const [isEditMode, setIsEditMode] = useState(false);
  const [editorHasUnsavedChanges, setEditorHasUnsavedChanges] = useState(false);
  
  const { triggerRefreshFor } = useTaskRefresh();
  const meetingDocRefresh = useRefreshTrigger('meeting-document');

  // 加载文档内容
  const loadDocument = useCallback(async () => {
    if (!meetingId) return;
    setLoading(true);
    try {
      const result = await exportMeetingDoc(meetingId, slot);
      setContent(result.content || '');
      setExists(!!result.content);
      setVersion(result.version || 0);
    } catch {
      // 如果新API失败，文档可能不存在
      setContent('');
      setExists(false);
    } finally {
      setLoading(false);
    }
  }, [meetingId, slot]);

  useEffect(() => {
    loadDocument();
    setIsEditMode(false);
  }, [loadDocument, meetingDocRefresh]);

  // 处理编辑章节
  const handleEditSection = useCallback(() => {
    setIsEditMode(true);
  }, []);

  // 处理复制章节名
  const handleCopySectionName = useCallback((sectionTitle: string) => {
    navigator.clipboard.writeText(sectionTitle);
    message.success('已复制章节名');
  }, []);

  // 编辑模式
  if (isEditMode) {
    return (
      <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
        <MeetingDocSectionEditor
          meetingId={meetingId}
          slot={slot}
          onCancel={() => {
            // 如果有未保存的更改，弹出确认对话框
            if (editorHasUnsavedChanges) {
              Modal.confirm({
                title: '未保存的更改',
                content: '当前有未保存的更改，关闭将丢失这些更改。确认关闭吗？',
                okText: '确认关闭',
                cancelText: '继续编辑',
                okType: 'danger',
                onOk: () => {
                  setIsEditMode(false);
                  setEditorHasUnsavedChanges(false);
                }
              });
            } else {
              setIsEditMode(false);
            }
          }}
          onSave={() => {
            loadDocument();
            triggerRefreshFor('meeting-document');
            setEditorHasUnsavedChanges(false);
          }}
          onUnsavedChanges={setEditorHasUnsavedChanges}
        />
      </div>
    );
  }

  // 加载中
  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 200, gap: 12 }}>
        <Spin />
        <span>加载中...</span>
      </div>
    );
  }

  // 空文档
  if (!exists && !content) {
    return (
      <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
        <div style={{ marginBottom: 12 }}>
          <Space size="middle">
            <Button
              type="primary"
              icon={<EditOutlined />}
              onClick={() => setIsEditMode(true)}
              size="small"
              style={{ backgroundColor: color, borderColor: color }}
            >
              编辑
            </Button>
          </Space>
        </div>
        <div style={{ 
          flex: 1, 
          display: 'flex', 
          alignItems: 'center', 
          justifyContent: 'center',
          flexDirection: 'column',
          background: '#fafafa',
          borderRadius: 8,
          border: '1px solid #f0f0f0'
        }}>
          <FileTextOutlined style={{ fontSize: 48, marginBottom: 16, color: '#d9d9d9' }} />
          <div style={{ marginBottom: 16, color: '#999' }}>暂无{title}</div>
          <div style={{ color: '#bbb' }}>点击上方「编辑」按钮创建文档</div>
        </div>
      </div>
    );
  }

  // 预览模式 - 显示全文
  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
      <div style={{ marginBottom: 12, flexShrink: 0, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Space size="middle">
          <Button
            type="primary"
            icon={<EditOutlined />}
            onClick={() => setIsEditMode(true)}
            size="small"
            style={{ backgroundColor: color, borderColor: color }}
          >
            编辑
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={loadDocument}
            size="small"
          >
            刷新
          </Button>
        </Space>
        {version > 0 && (
          <span style={{ color: '#999', fontSize: 12 }}>版本: {version}</span>
        )}
      </div>
      
      <div style={{ flex: 1, minHeight: 0, display: 'flex', gap: 12 }}>
        {/* 左侧目录导航 - 固定高度，内部滚动 */}
        <div style={{
          width: 260,
          flexShrink: 0,
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          border: '1px solid #f0f0f0',
          borderRadius: 6,
          background: '#fff'
        }}>
          <div style={{
            padding: '8px 12px',
            borderBottom: '1px solid #f5f5f5',
            fontWeight: 500,
            fontSize: 12,
            background: '#fafafa',
            flexShrink: 0
          }}>目录</div>
          <div style={{
            flex: 1,
            overflowY: 'auto',
            overflowX: 'hidden',
            padding: '8px 12px',
            minHeight: 0
          }}>
            <DocumentTOC 
              content={content} 
              onEditSection={handleEditSection}
            />
          </div>
        </div>

        {/* 右侧文档内容 - 独立滚动 */}
        <div style={{
          flex: 1,
          minHeight: 0,
          display: 'flex',
          flexDirection: 'column',
          border: '1px solid #f0f0f0',
          borderRadius: 6,
          backgroundColor: '#fafafa'
        }}>
          <div 
            className="scroll-region"
            style={{
              flex: 1,
              overflowY: 'auto',
              overflowX: 'hidden',
              padding: '16px',
              minHeight: 0
            }}
          >
            <MarkdownViewer
              showFullscreenButton={true}
              onEditSection={handleEditSection}
              onCopySectionName={handleCopySectionName}
            >
              {content}
            </MarkdownViewer>
          </div>
        </div>
      </div>
    </div>
  );
};

export default MeetingDocument;
