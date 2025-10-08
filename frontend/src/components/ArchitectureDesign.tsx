import React, { useState, useEffect } from 'react';
import { Spin, Alert, Button, Input, message, Typography, Modal, Dropdown, MenuProps } from 'antd';
import { FileSearchOutlined, EditOutlined, SaveOutlined, CloseOutlined, CopyOutlined, HistoryOutlined, DeleteOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { TaskSelector } from './TaskSelector';
import { DiffModal } from './DiffModal';
import { MermaidChart } from './MermaidChart';
import { authedApi } from '../api/auth';

const { Title } = Typography;
const { TextArea } = Input;

const containerStyle: React.CSSProperties = {
  height: '100%',
  overflowY: 'auto',
  padding: 16,
  background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
  borderRadius: 8,
};

const contentStyle: React.CSSProperties = {
  background: 'white',
  borderRadius: 8,
  padding: 24,
  boxShadow: '0 4px 12px rgba(102, 126, 234, 0.15)',
  minHeight: 'calc(100% - 32px)',
};

const markdownStyle: React.CSSProperties = {
  lineHeight: 1.7,
  color: '#4a5568',
};

const emptyStateStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  height: 200,
  color: '#718096',
};

interface ArchitectureDesignProps {
  taskId: string;
}

export const ArchitectureDesign: React.FC<ArchitectureDesignProps> = ({ taskId }) => {
  const [content, setContent] = useState<string>('');
  const [exists, setExists] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState('');
  const [saving, setSaving] = useState(false);
  
  // 拷贝相关状态
  const [showCopyModal, setShowCopyModal] = useState(false);
  const [sourceTaskId, setSourceTaskId] = useState<string>('');
  const [sourceContent, setSourceContent] = useState<string>('');
  const [showDiffModal, setShowDiffModal] = useState(false);
  const [copying, setCopying] = useState(false);

  // 历史记录相关状态
  const [history, setHistory] = useState<Array<{content: string, timestamp: string}>>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // 加载架构设计内容
  const loadArchitectureContent = async () => {
    if (!taskId) return;
    setLoading(true);
    try {
      const response = await authedApi.get(`/tasks/${taskId}/architecture-design`);
      setContent(response.data.content || '');
      setExists(response.data.exists || false);
    } catch (error) {
      console.error('Failed to load architecture design:', error);
      setExists(false);
    } finally {
      setLoading(false);
    }
  };

  // 保存架构设计内容
  const saveArchitectureContent = async () => {
    setSaving(true);
    try {
      await authedApi.put(`/tasks/${taskId}/architecture-design`, {
        content: editContent,
      });

      setContent(editContent);
      setExists(true);
      setIsEditing(false);
      message.success('架构设计保存成功！');
      // 重新加载历史记录以反映最新状态
      if (history.length > 0) {
        loadHistory();
      }
    } catch (error) {
      console.error('Failed to save architecture design:', error);
      message.error('保存失败，请重试');
    } finally {
      setSaving(false);
    }
  };

  // 加载历史记录
  const loadHistory = async () => {
    if (!taskId) return;
    setLoadingHistory(true);
    try {
      const response = await authedApi.get(`/tasks/${taskId}/architecture-design/history`);
      setHistory(response.data.history || []);
    } catch (error) {
      console.error('Failed to load history:', error);
      message.error('加载历史记录失败');
    } finally {
      setLoadingHistory(false);
    }
  };

  // 恢复历史版本
  const restoreFromHistory = (content: string) => {
    setEditContent(content);
    setIsEditing(true);
    message.success('已恢复历史版本，请确认后保存');
  };

  // 删除历史版本
  const deleteHistory = async (version: number) => {
    if (!taskId) return;
    try {
      await authedApi.delete(`/tasks/${taskId}/architecture-design/history/${version}`);
      message.success('历史版本已删除');
      // 重新加载历史记录
      loadHistory();
    } catch (error) {
      console.error('Failed to delete history:', error);
      message.error('删除失败');
    }
  };

  // 开始编辑
  const handleEdit = () => {
    setEditContent(content);
    setIsEditing(true);
  };

  // 取消编辑
  const handleCancel = () => {
    setIsEditing(false);
    setEditContent('');
  };

  // 获取源文件内容
  const fetchSourceContent = async (sourceId: string) => {
    try {
      const response = await authedApi.get(`/tasks/${sourceId}/architecture-design`);
      return response.data.content || '';
    } catch (error) {
      console.error('Failed to fetch source content:', error);
    }
    return '';
  };

  // 处理拷贝操作
  const handleCopy = async () => {
    if (!sourceTaskId) {
      message.error('请选择源任务');
      return;
    }

    const sourceContent = await fetchSourceContent(sourceTaskId);
    if (!sourceContent) {
      message.error('源任务中没有找到架构设计文件');
      return;
    }

    setSourceContent(sourceContent);
    setShowCopyModal(false);

    // 如果当前任务已有内容，显示差异对比
    if (exists && content) {
      setShowDiffModal(true);
    } else {
      // 直接复制
      performCopy();
    }
  };

  // 执行实际的拷贝操作
  const performCopy = async () => {
    setCopying(true);
    try {
      await authedApi.post(`/tasks/${taskId}/copy-architecture-design`, {
        sourceTaskId: sourceTaskId,
      });

      message.success('架构设计复制成功！');
      setShowDiffModal(false);
      loadArchitectureContent(); // 重新加载内容
    } catch (error) {
      console.error('Failed to copy architecture design:', error);
      message.error('复制失败，请重试');
    } finally {
      setCopying(false);
    }
  };

  // 取消拷贝操作
  const handleCopyCancel = () => {
    setShowCopyModal(false);
    setShowDiffModal(false);
    setSourceTaskId('');
    setSourceContent('');
  };

  useEffect(() => {
    loadArchitectureContent();
    setIsEditing(false);
  }, [taskId]);

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexShrink: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <FileSearchOutlined style={{ color: '#fa8c16', fontSize: '16px' }} />
          <Title level={4} style={{ margin: 0, color: '#fa8c16' }}>架构设计</Title>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          {!taskId && (
            <div style={{ color: '#fa8c16', fontSize: 12 }}>未选择任务</div>
          )}
          {taskId && !exists && !loading && !isEditing && (
            <>
              <Button
                type="primary"
                size="small"
                icon={<EditOutlined />}
                onClick={() => {
                  setEditContent('# 系统架构设计\n\n## 1. 整体架构\n\n### 1.1 系统概述\n\n### 1.2 技术栈\n\n## 2. 模块设计\n\n### 2.1 核心模块\n\n### 2.2 接口设计\n\n## 3. 部署架构\n\n### 3.1 环境配置\n\n### 3.2 扩展方案');
                  setIsEditing(true);
                }}
                style={{ backgroundColor: '#fa8c16', borderColor: '#fa8c16' }}
              >
                创建
              </Button>
              <Button
                size="small"
                icon={<CopyOutlined />}
                onClick={() => {
                  console.log('架构设计拷贝按钮被点击 - 空白页面');
                  setShowCopyModal(true);
                }}
                style={{ color: '#fa8c16', borderColor: '#fa8c16' }}
              >
                拷贝
              </Button>
            </>
          )}
          {taskId && exists && !isEditing && (
            <>
              <Dropdown
                menu={{
                  items: history.map((item, index) => ({
                    key: index,
                    label: (
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', minWidth: '300px' }}>
                        <div style={{ flex: 1, cursor: 'pointer' }} onClick={() => restoreFromHistory(item.content)}>
                          <div>{new Date(item.timestamp).toLocaleString()}</div>
                          <div style={{ fontSize: '12px', color: '#666' }}>
                            {item.content.length > 50 ? `${item.content.substring(0, 50)}...` : item.content}
                          </div>
                        </div>
                        <Button
                          type="text"
                          size="small"
                          icon={<DeleteOutlined />}
                          onClick={(e) => {
                            e.stopPropagation();
                            deleteHistory(index + 1);
                          }}
                          style={{ color: '#ff4d4f', marginLeft: '8px' }}
                          title="删除此版本"
                        />
                      </div>
                    ),
                    onClick: (e) => e.domEvent.stopPropagation()
                  })),
                  onClick: (e) => e.domEvent.stopPropagation()
                } as MenuProps}
                onOpenChange={(open) => {
                  if (open && history.length === 0) {
                    loadHistory();
                  }
                }}
                trigger={['click']}
                disabled={!taskId}
              >
                <Button
                  type="text"
                  size="small"
                  icon={<HistoryOutlined />}
                  onClick={(e) => e.stopPropagation()}
                  loading={loadingHistory}
                  style={{ color: '#fa8c16' }}
                >
                  历史
                </Button>
              </Dropdown>
              <Button
                type="text"
                size="small"
                icon={<EditOutlined />}
                onClick={handleEdit}
                style={{ color: '#fa8c16' }}
              >
                编辑
              </Button>
              <Button
                type="text"
                size="small"
                icon={<CopyOutlined />}
                onClick={() => {
                  console.log('架构设计拷贝按钮被点击 - 有内容页面');
                  setShowCopyModal(true);
                }}
                style={{ color: '#fa8c16' }}
              >
                拷贝
              </Button>
            </>
          )}
          {isEditing && (
            <>
              <Button
                type="primary"
                size="small"
                icon={<SaveOutlined />}
                onClick={saveArchitectureContent}
                loading={saving}
                style={{ backgroundColor: '#fa8c16', borderColor: '#fa8c16' }}
              >
                保存
              </Button>
              <Button
                size="small"
                icon={<CloseOutlined />}
                onClick={handleCancel}
              >
                取消
              </Button>
            </>
          )}
        </div>
      </div>
      
      <div style={{ 
        flex: 1, 
        overflow: 'auto', 
        background: '#fff7e6', 
        padding: 16, 
        borderRadius: 8,
        border: '1px solid #ffd591',
        minHeight: 0
      }}>
        {!taskId ? (
          <div style={{
            height: '100%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#999'
          }}>请选择一个任务以查看架构设计</div>
        ) : loading ? (
          <div style={{ 
            display: 'flex', 
            justifyContent: 'center', 
            alignItems: 'center', 
            height: '200px',
            flexDirection: 'column',
            gap: 12
          }}>
            <Spin size="large" />
            <div style={{ fontSize: '14px', color: '#666' }}>
              正在加载架构设计...
            </div>
          </div>
        ) : isEditing ? (
          <TextArea
            value={editContent}
            onChange={(e) => setEditContent(e.target.value)}
            placeholder="请输入架构设计内容（支持Markdown格式）"
            autoSize={{ minRows: 20, maxRows: 40 }}
            style={{
              fontSize: '14px',
              fontFamily: 'Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
            }}
          />
        ) : !exists ? (
          <div style={{ 
            flex: 1, 
            display: 'flex', 
            alignItems: 'center', 
            justifyContent: 'center',
            flexDirection: 'column',
            gap: 16
          }}>
            <div style={{ color: '#999', fontSize: '14px', textAlign: 'center' }}>
              <p>当前任务暂无架构设计文档</p>
              <p style={{ fontSize: '12px', marginTop: 8 }}>
                点击创建按钮开始编写 architecture_new.md 文件
              </p>
            </div>
          </div>
        ) : content ? (
          <div style={{ 
            fontSize: '14px', 
            lineHeight: '1.6',
            color: '#333'
          }}>
            <ReactMarkdown
              remarkPlugins={[remarkGfm] as any}
              components={{
                h1: ({ children }) => (
                  <h1 style={{ 
                    fontSize: '18px', 
                    color: '#d4380d', 
                    margin: '0 0 16px 0',
                    borderBottom: '2px solid #ffd591',
                    paddingBottom: '8px'
                  }}>
                    {children}
                  </h1>
                ),
                h2: ({ children }) => (
                  <h2 style={{ 
                    fontSize: '16px', 
                    color: '#fa8c16', 
                    margin: '16px 0 8px 0',
                    fontWeight: 600
                  }}>
                    {children}
                  </h2>
                ),
                h3: ({ children }) => (
                  <h3 style={{ 
                    fontSize: '15px', 
                    color: '#faad14', 
                    margin: '12px 0 6px 0',
                    fontWeight: 600
                  }}>
                    {children}
                  </h3>
                ),
                code: ({ children, className, ...props }) => {
                  const match = /language-(\w+)/.exec(className || '');
                  const language = match ? match[1] : '';
                  const codeContent = String(children).replace(/\n$/, '');
                  
                  // 检查是否是 Mermaid 图表
                  if (language === 'mermaid') {
                    return <MermaidChart chart={codeContent} />;
                  }
                  
                  const isBlock = className?.includes('language-');
                  if (isBlock) {
                    return (
                      <pre style={{
                        backgroundColor: '#f5f5f5',
                        border: '1px solid #ffd591',
                        borderRadius: '6px',
                        padding: '12px',
                        overflow: 'auto',
                        fontSize: '14px',
                        lineHeight: '1.4'
                      }}>
                        <code>{children}</code>
                      </pre>
                    );
                  }
                  return (
                    <code style={{
                      backgroundColor: '#fff1b8',
                      padding: '2px 4px',
                      borderRadius: '3px',
                      fontSize: '13px',
                      color: '#d4380d',
                      border: '1px solid #ffd591'
                    }}>
                      {children}
                    </code>
                  );
                },
                blockquote: ({ children }) => (
                  <blockquote style={{
                    borderLeft: '4px solid #ffd591',
                    margin: '16px 0',
                    padding: '12px 16px',
                    background: '#fff7e6',
                    fontStyle: 'italic',
                    color: '#595959'
                  }}>
                    {children}
                  </blockquote>
                )
              }}
            >
              {content}
            </ReactMarkdown>
          </div>
        ) : (
          <div style={{ color: '#999', fontSize: '12px', textAlign: 'center', marginTop: '40px' }}>
            暂无架构设计内容
          </div>
        )}
      </div>
      
      <div style={{ 
        fontSize: '11px', 
        color: '#999', 
        flexShrink: 0,
        textAlign: 'center',
        padding: '8px 0'
      }}>
        🏗️ 展示系统的整体架构和技术设计方案
      </div>

      {/* 拷贝任务选择模态框 */}
      <Modal
        title="选择拷贝源任务"
        open={showCopyModal}
        onCancel={handleCopyCancel}
        onOk={handleCopy}
        okText="复制"
        cancelText="取消"
      >
        <div style={{ marginBottom: 16 }}>
          <p>请选择要复制架构设计的源任务：</p>
          <TaskSelector
            currentTaskId={taskId}
            placeholder="选择源任务"
            onChange={setSourceTaskId}
          />
        </div>
      </Modal>

      {/* 差异对比模态框 */}
      <DiffModal
        visible={showDiffModal}
        title="架构设计内容对比"
        currentContent={content}
        sourceContent={sourceContent}
        onConfirm={performCopy}
        onCancel={handleCopyCancel}
        loading={copying}
      />
    </div>
  );
};