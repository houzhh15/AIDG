import React, { useState, useEffect } from 'react';
import { Spin, Typography, Empty, Button, Input, message, Modal, Dropdown, MenuProps } from 'antd';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { CodeOutlined, EditOutlined, SaveOutlined, CloseOutlined, CopyOutlined, HistoryOutlined, DeleteOutlined } from '@ant-design/icons';
import { TaskSelector } from './TaskSelector';
import { DiffModal } from './DiffModal';
import { authedApi } from '../api/auth';
import { MermaidChart } from './MermaidChart';

const { TextArea } = Input;

const { Title } = Typography;

interface TechDesignProps {
  taskId: string;
}

interface TechDesignResponse {
  content: string;
  exists: boolean;
}

export const TechDesign: React.FC<TechDesignProps> = ({ taskId }) => {
  const [content, setContent] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(true);
  const [exists, setExists] = useState<boolean>(false);
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

  // 加载技术设计内容
  const loadTechDesignContent = async () => {
    if (!taskId) return;
    setLoading(true);
    try {
      const response = await authedApi.get(`/tasks/${taskId}/tech-design`);
      setContent(response.data.content || '');
      setExists(response.data.exists || false);
    } catch (error) {
      console.error('Failed to load tech design:', error);
      setExists(false);
    } finally {
      setLoading(false);
    }
  };

  // 保存技术设计内容
  const saveTechDesignContent = async () => {
    setSaving(true);
    try {
      await authedApi.put(`/tasks/${taskId}/tech-design`, {
        content: editContent,
      });

      setContent(editContent);
      setExists(true);
      setIsEditing(false);
      message.success('方案设计保存成功！');
      // 重新加载历史记录以反映最新状态
      if (history.length > 0) {
        loadHistory();
      }
    } catch (error) {
      console.error('Failed to save tech design:', error);
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
      const response = await authedApi.get(`/tasks/${taskId}/tech-design/history`);
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
      await authedApi.delete(`/tasks/${taskId}/tech-design/history/${version}`);
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
      const response = await authedApi.get(`/tasks/${sourceId}/tech-design`);
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
      message.error('源任务中没有找到方案设计文件');
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
      await authedApi.post(`/tasks/${taskId}/copy-tech-design`, {
        sourceTaskId: sourceTaskId,
      });

      message.success('方案设计复制成功！');
      setShowDiffModal(false);
      loadTechDesignContent(); // 重新加载内容
    } catch (error) {
      console.error('Failed to copy tech design:', error);
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

  // 渲染内容
  const renderContent = () => {
    if (isEditing) {
      return (
        <TextArea
          value={editContent}
          onChange={(e) => setEditContent(e.target.value)}
          placeholder="请输入技术方案设计内容（支持Markdown格式）"
          autoSize={{ minRows: 20, maxRows: 40 }}
          style={{
            fontSize: '14px',
            fontFamily: 'Monaco, Consolas, "Liberation Mono", "Courier New", monospace'
          }}
        />
      );
    }

    return (
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          code({ node, className, children, ...props }) {
            console.log('TechDesign ReactMarkdown code block detected:', { 
              className, 
              childrenType: typeof children, 
              childrenLength: String(children).length,
              childrenPreview: String(children).substring(0, 50) 
            });
            
            const match = /language-(\w+)/.exec(className || '');
            // 检查是否为代码块（非内联代码）
            const isCodeBlock = className && className.includes('language-');
            
            if (isCodeBlock && match && match[1] === 'mermaid') {
              console.log('TechDesign rendering Mermaid chart via ReactMarkdown');
              const chartContent = String(children).replace(/\n$/, '');
              return <MermaidChart chart={chartContent} />;
            }
            return (
              <code className={className} {...props}>
                {children}
              </code>
            );
          },
        }}
      >
        {content}
      </ReactMarkdown>
    );
  };

  useEffect(() => {
    loadTechDesignContent();
    setIsEditing(false);
  }, [taskId]);

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexShrink: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <CodeOutlined style={{ color: '#1890ff', fontSize: '16px' }} />
          <Title level={4} style={{ margin: 0, color: '#1890ff' }}>方案设计</Title>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          {!taskId && (
            <div style={{ color: '#1890ff', fontSize: 12 }}>未选择任务</div>
          )}
          {taskId && !exists && !loading && !isEditing && (
            <>
              <Button
                type="primary"
                size="small"
                icon={<EditOutlined />}
                onClick={() => {
                  setEditContent('# 技术方案设计\n\n## 1. 方案概述\n\n### 1.1 设计目标\n\n### 1.2 技术选型\n\n## 2. 详细设计\n\n### 2.1 核心功能\n\n### 2.2 技术实现\n\n## 3. 实施计划\n\n### 3.1 开发阶段\n\n### 3.2 测试验证');
                  setIsEditing(true);
                }}
                style={{ backgroundColor: '#1890ff', borderColor: '#1890ff' }}
              >
                创建
              </Button>
              <Button
                size="small"
                icon={<CopyOutlined />}
                onClick={() => setShowCopyModal(true)}
                style={{ color: '#1890ff', borderColor: '#1890ff' }}
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
                  style={{ color: '#1890ff' }}
                >
                  历史
                </Button>
              </Dropdown>
              <Button
                type="text"
                size="small"
                icon={<EditOutlined />}
                onClick={handleEdit}
                style={{ color: '#1890ff' }}
              >
                编辑
              </Button>
              <Button
                type="text"
                size="small"
                icon={<CopyOutlined />}
                onClick={() => setShowCopyModal(true)}
                style={{ color: '#1890ff' }}
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
                onClick={saveTechDesignContent}
                loading={saving}
                style={{ backgroundColor: '#1890ff', borderColor: '#1890ff' }}
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
      {/* 内容区域 */}
      <div
        style={{
          flex: 1,
          overflow: 'auto',
          background: '#f0f8ff',
          padding: 16,
          borderRadius: 8,
          border: '1px solid #91d5ff',
          minHeight: 0
        }}
      >
        {!taskId && (
          <div
            style={{
              height: '100%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#999'
            }}
          >
            请选择一个任务以查看方案设计
          </div>
        )}

        {taskId && loading && (
          <div
            style={{
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              height: '200px',
              gap: 12
            }}
          >
            <Spin size="large" />
            <div style={{ fontSize: '14px', color: '#666' }}>正在加载方案设计...</div>
          </div>
        )}

        {taskId && !loading && isEditing && (
          <TextArea
            value={editContent}
            onChange={(e) => setEditContent(e.target.value)}
            placeholder="请输入技术方案设计内容（支持Markdown格式）"
            autoSize={{ minRows: 20, maxRows: 40 }}
            style={{
              fontSize: '14px',
              fontFamily: 'Monaco, Consolas, "Liberation Mono", "Courier New", monospace'
            }}
          />
        )}

        {taskId && !loading && !isEditing && !exists && (
          <div
            style={{
              flex: 1,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexDirection: 'column',
              gap: 16
            }}
          >
            <div style={{ color: '#999', fontSize: '14px', textAlign: 'center' }}>
              <p>当前任务暂无技术方案设计</p>
              <p style={{ fontSize: '12px', marginTop: 8 }}>点击创建或拷贝以生成 tech_design_*.md 文件</p>
            </div>
          </div>
        )}

        {taskId && !loading && !isEditing && exists && content && (
          <div style={{ fontSize: '14px', lineHeight: '1.6', color: '#333' }}>
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                h1: ({ children }) => (
                  <h1
                    style={{
                      fontSize: '18px',
                      color: '#0050b3',
                      margin: '0 0 16px 0',
                      borderBottom: '2px solid #91d5ff',
                      paddingBottom: '8px'
                    }}
                  >
                    {children}
                  </h1>
                ),
                h2: ({ children }) => (
                  <h2
                    style={{
                      fontSize: '16px',
                      color: '#1890ff',
                      margin: '16px 0 8px 0',
                      fontWeight: 600
                    }}
                  >
                    {children}
                  </h2>
                ),
                h3: ({ children }) => (
                  <h3
                    style={{
                      fontSize: '15px',
                      color: '#40a9ff',
                      margin: '12px 0 6px 0',
                      fontWeight: 600
                    }}
                  >
                    {children}
                  </h3>
                ),
                code: ({ children, className }) => {
                  const isBlock = className?.includes('language-');
                  if (isBlock) {
                    return (
                      <pre
                        style={{
                          backgroundColor: '#f5f5f5',
                          border: '1px solid #91d5ff',
                          borderRadius: '6px',
                          padding: '12px',
                          overflow: 'auto',
                          fontSize: '14px',
                          lineHeight: '1.4'
                        }}
                      >
                        <code>{children}</code>
                      </pre>
                    );
                  }
                  return (
                    <code
                      style={{
                        backgroundColor: '#e6f7ff',
                        padding: '2px 4px',
                        borderRadius: '3px',
                        fontSize: '13px',
                        color: '#0050b3',
                        border: '1px solid #91d5ff'
                      }}
                    >
                      {children}
                    </code>
                  );
                },
                blockquote: ({ children }) => (
                  <blockquote
                    style={{
                      borderLeft: '4px solid #91d5ff',
                      margin: '16px 0',
                      padding: '12px 16px',
                      background: '#f0f8ff',
                      fontStyle: 'italic',
                      color: '#595959'
                    }}
                  >
                    {children}
                  </blockquote>
                )
              }}
            >
              {content}
            </ReactMarkdown>
          </div>
        )}

        {taskId && !loading && !isEditing && exists && !content && (
          <div style={{ color: '#999', fontSize: '12px', textAlign: 'center', marginTop: '40px' }}>暂无方案设计内容</div>
        )}
      </div>
      
      <div style={{ 
        fontSize: '11px', 
        color: '#999', 
        flexShrink: 0,
        textAlign: 'center',
        padding: '8px 0'
      }}>
        🔧 展示项目的技术方案和实施设计
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
          <p>请选择要复制方案设计的源任务：</p>
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
        title="方案设计内容对比"
        currentContent={content}
        sourceContent={sourceContent}
        onConfirm={performCopy}
        onCancel={handleCopyCancel}
        loading={copying}
      />
    </div>
  );
};