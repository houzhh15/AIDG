import React, { useState, useEffect } from 'react';
import { 
  Card, 
  Button, 
  Input, 
  message, 
  Spin, 
  Typography, 
  Space, 
  Divider,
  Modal,
  Alert 
} from 'antd';
import { 
  EditOutlined, 
  SaveOutlined, 
  EyeOutlined, 
  HistoryOutlined,
  FileTextOutlined 
} from '@ant-design/icons';
import { documentsAPI, DocMetaEntry, UpdateContentRequest } from '../api/documents';

const { Title, Text } = Typography;
const { TextArea } = Input;

interface DocumentContentViewerProps {
  projectId: string;
  nodeId: string;
  nodeName: string;
  taskId?: string; // 添加 taskId 用于防护机制
  onClose?: () => void;
}

interface DocumentContent {
  meta: DocMetaEntry;
  content: string;
}

const DocumentContentViewer: React.FC<DocumentContentViewerProps> = ({
  projectId,
  nodeId,
  nodeName,
  taskId,
  onClose
}) => {
  const [documentContent, setDocumentContent] = useState<DocumentContent | null>(null);
  const [loading, setLoading] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState('');
  const [saving, setSaving] = useState(false);

  // 加载文档内容
  const loadContent = async () => {
    if (!nodeId) return;
    
    // 防止 taskId 被错误地当作文档ID使用
    if (nodeId.startsWith('task_')) {
      console.warn('[WARN] DocumentContentViewer: 试图使用 taskId 作为文档ID，已阻止:', nodeId);
      message.warning(`任务ID ${nodeId} 不能直接作为文档编辑，请选择具体的文档节点`);
      return;
    }
    
    setLoading(true);
    try {
      const result = await documentsAPI.getContent(projectId, nodeId);
      setDocumentContent(result);
      setEditContent(result.content);
    } catch (error) {
      console.error('Failed to load document content:', error);
      message.error('加载文档内容失败');
    } finally {
      setLoading(false);
    }
  };

  // 保存文档内容
  const handleSave = async () => {
    if (!documentContent) return;

    setSaving(true);
    try {
      const updateRequest: UpdateContentRequest = {
        content: editContent,
        version: documentContent.meta.version
      };

      const result = await documentsAPI.updateContent(projectId, nodeId, updateRequest);
      
      if (result.success) {
        message.success('文档保存成功');
        setIsEditing(false);
        // 重新加载内容以获取最新版本
        await loadContent();
      } else {
        message.error('文档保存失败');
      }
    } catch (error) {
      console.error('Failed to save document:', error);
      message.error('文档保存失败');
    } finally {
      setSaving(false);
    }
  };

  // 取消编辑
  const handleCancelEdit = () => {
    Modal.confirm({
      title: '确认取消编辑?',
      content: '您的修改将会丢失，是否确认取消编辑？',
      onOk: () => {
        setIsEditing(false);
        setEditContent(documentContent?.content || '');
      }
    });
  };

  // 组件挂载时加载内容
  useEffect(() => {
    loadContent();
  }, [projectId, nodeId]);

  if (loading) {
    return (
      <Card>
        <div style={{ textAlign: 'center', padding: '50px' }}>
          <Spin size="large" />
          <div style={{ marginTop: 16 }}>正在加载文档内容...</div>
        </div>
      </Card>
    );
  }

  if (!documentContent) {
    return (
      <Card>
        <Alert 
          message="无法加载文档内容" 
          description="文档可能不存在或您没有访问权限" 
          type="warning" 
          showIcon 
        />
      </Card>
    );
  }

  return (
    <Card 
      title={
        <Space>
          <FileTextOutlined />
          <Title level={4} style={{ margin: 0 }}>
            {nodeName}
          </Title>
        </Space>
      }
      extra={
        <Space>
          {!isEditing ? (
            <>
              <Button 
                type="primary" 
                icon={<EditOutlined />}
                onClick={() => setIsEditing(true)}
              >
                编辑
              </Button>
              <Button 
                icon={<HistoryOutlined />}
                onClick={() => message.info('版本历史功能开发中...')}
              >
                版本历史
              </Button>
            </>
          ) : (
            <>
              <Button 
                type="primary" 
                icon={<SaveOutlined />}
                loading={saving}
                onClick={handleSave}
              >
                保存
              </Button>
              <Button onClick={handleCancelEdit}>
                取消
              </Button>
            </>
          )}
          {onClose && (
            <Button onClick={onClose}>
              关闭
            </Button>
          )}
        </Space>
      }
    >
      {/* 文档元信息 */}
      <div style={{ marginBottom: 16 }}>
        <Space direction="vertical" size="small">
          <Text type="secondary">
            版本: v{documentContent.meta.version} | 
            最后修改: {new Date(documentContent.meta.updated_at).toLocaleString()} |
            创建时间: {new Date(documentContent.meta.created_at).toLocaleString()}
          </Text>
          <Text type="secondary">
            文档类型: {documentContent.meta.type} | 层级: {documentContent.meta.level}
          </Text>
        </Space>
      </div>

      <Divider />

      {/* 内容区域 */}
      {isEditing ? (
        <div>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>
            编辑内容 (支持Markdown格式):
          </Text>
          <TextArea
            value={editContent}
            onChange={(e) => setEditContent(e.target.value)}
            placeholder="输入文档内容，支持Markdown格式..."
            autoSize={{ minRows: 20, maxRows: 50 }}
            style={{ fontSize: '14px', fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace' }}
          />
          <div style={{ marginTop: 8 }}>
            <Text type="secondary" style={{ fontSize: '12px' }}>
              提示: 支持Markdown语法，如 **粗体**、*斜体*、`代码`、# 标题 等
            </Text>
          </div>
        </div>
      ) : (
        <div>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>
            文档内容:
          </Text>
          {documentContent.content ? (
            <div 
              style={{ 
                backgroundColor: '#fafafa', 
                padding: '16px', 
                borderRadius: '6px',
                minHeight: '300px',
                whiteSpace: 'pre-wrap',
                fontSize: '14px',
                lineHeight: '1.6'
              }}
            >
              {documentContent.content}
            </div>
          ) : (
            <div style={{ textAlign: 'center', padding: '50px', color: '#999' }}>
              <FileTextOutlined style={{ fontSize: '48px', marginBottom: '16px' }} />
              <div>文档内容为空</div>
              <Button 
                type="link" 
                onClick={() => setIsEditing(true)}
                style={{ marginTop: '8px' }}
              >
                点击添加内容
              </Button>
            </div>
          )}
        </div>
      )}
    </Card>
  );
};

export default DocumentContentViewer;