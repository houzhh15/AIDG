import React, { useState, useEffect } from 'react';
import { message, Typography, Button, Spin, Space, Input, Empty, Dropdown, Modal } from 'antd';
import type { MenuProps } from 'antd';
import { ReloadOutlined, EyeOutlined, EditOutlined, SaveOutlined, CheckCircleOutlined, CopyOutlined, DeleteOutlined, HistoryOutlined, CloseOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { TaskSelector } from './TaskSelector';
import { DiffModal } from './DiffModal';
import { MermaidChart } from './MermaidChart';
import { authedApi } from '../api/auth';

const { TextArea } = Input;

const { Title } = Typography;

interface FeatureListProps {
  taskId: string;
}

export const FeatureList: React.FC<FeatureListProps> = ({ taskId }) => {
  const [featureContent, setFeatureContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [featureExists, setFeatureExists] = useState(false);
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
  const [history, setHistory] = useState<Array<{timestamp: string, content: string, version: number}>>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // 加载特性列表内容
  const loadFeatureContent = async () => {
    if (!taskId) return;
    setLoading(true);
    try {
      const response = await authedApi.get(`/tasks/${taskId}/feature-list`);
      setFeatureContent(response.data.content || '');
      setFeatureExists(response.data.exists || false);
    } catch (error) {
      console.error('Failed to load feature list content:', error);
      setFeatureExists(false);
    } finally {
      setLoading(false);
    }
  };

  // 保存特性列表内容
  const saveFeatureContent = async () => {
    setSaving(true);
    try {
      await authedApi.put(`/tasks/${taskId}/feature-list`, {
        content: editContent,
      });

      setFeatureContent(editContent);
      setFeatureExists(true);
      setIsEditing(false);
      message.success('特性列表保存成功！');
      // 重新加载历史记录以反映最新状态
      if (history.length > 0) {
        loadHistory();
      }
    } catch (error) {
      console.error('Failed to save feature list:', error);
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
      const response = await authedApi.get(`/tasks/${taskId}/feature-list/history`);
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
  const deleteHistoryVersion = async (version: string) => {
    if (!taskId) return;
    try {
      await authedApi.delete(`/tasks/${taskId}/feature-list/history/${version}`);
      message.success('删除成功');
      loadHistory();
    } catch (error) {
      console.error('Failed to delete history version:', error);
      message.error('删除失败');
    }
  };

  // 开始编辑
  const handleEdit = () => {
    setEditContent(featureContent);
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
      const response = await authedApi.get(`/tasks/${sourceId}/feature-list`);
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

    const content = await fetchSourceContent(sourceTaskId);
    if (!content) {
      message.error('源任务中没有找到特性列表文件');
      return;
    }

    setSourceContent(content);
    setShowCopyModal(false);

    // 如果当前任务已有内容，显示差异对比
    if (featureExists && featureContent) {
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
      await authedApi.post(`/tasks/${taskId}/copy-feature-list`, {
        sourceTaskId: sourceTaskId,
      });

      message.success('特性列表复制成功！');
      setShowDiffModal(false);
      loadFeatureContent(); // 重新加载内容
    } catch (error) {
      console.error('Failed to copy feature list:', error);
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

  // 当taskId改变时加载内容
  useEffect(() => {
    loadFeatureContent();
    setIsEditing(false);
  }, [taskId]);

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexShrink: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <CheckCircleOutlined style={{ color: '#52c41a', fontSize: '16px' }} />
          <Title level={4} style={{ margin: 0, color: '#52c41a' }}>特性列表</Title>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          {!featureExists && !loading && !!taskId && !isEditing && (
            <>
              <Button
                type="primary"
                size="small"
                icon={<EditOutlined />}
                onClick={() => {
                  setEditContent('# 项目特性列表\n\n## 核心功能\n\n- 功能1\n- 功能2\n- 功能3\n\n## 技术特性\n\n- 技术特性1\n- 技术特性2');
                  setIsEditing(true);
                }}
                style={{ backgroundColor: '#52c41a', borderColor: '#52c41a' }}
              >
                创建
              </Button>
              <Button
                size="small"
                icon={<CopyOutlined />}
                onClick={() => {
                  console.log('拷贝按钮被点击 - 空白页面');
                  setShowCopyModal(true);
                }}
                style={{ color: '#52c41a', borderColor: '#52c41a' }}
              >
                拷贝
              </Button>
            </>
          )}
          {featureExists && !isEditing && (
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
                            deleteHistoryVersion(String(index + 1));
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
                  style={{ color: '#52c41a' }}
                >
                  历史
                </Button>
              </Dropdown>
              <Button
                type="text"
                size="small"
                icon={<EditOutlined />}
                onClick={handleEdit}
                style={{ color: '#52c41a' }}
              >
                编辑
              </Button>
              <Button
                type="text"
                size="small"
                icon={<CopyOutlined />}
                onClick={() => {
                  console.log('拷贝按钮被点击 - 有内容页面');
                  setShowCopyModal(true);
                }}
                style={{ color: '#52c41a' }}
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
                onClick={saveFeatureContent}
                loading={saving}
                style={{ backgroundColor: '#52c41a', borderColor: '#52c41a' }}
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
        background: '#f6ffed', 
        padding: 16, 
        borderRadius: 8,
        border: '1px solid #b7eb8f',
        minHeight: 0
      }}>
        {!taskId ? (
          <div style={{
            height: '100%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#999'
          }}>请选择一个任务以查看特性列表</div>
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
              正在加载特性列表...
            </div>
          </div>
        ) : isEditing ? (
          <TextArea
            value={editContent}
            onChange={(e) => setEditContent(e.target.value)}
            placeholder="请输入特性列表内容（支持Markdown格式）"
            autoSize={{ minRows: 20, maxRows: 40 }}
            style={{
              fontSize: '14px',
              fontFamily: 'Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
            }}
          />
        ) : !featureExists ? (
          <Empty 
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <div style={{ color: '#999', fontSize: '14px', textAlign: 'center' }}>
                <p>当前任务暂无特性列表文件</p>
                <p style={{ fontSize: '12px', marginTop: 8 }}>
                  系统会在处理完成后自动生成 feature_list.md 文件
                </p>
              </div>
            }
            style={{ margin: '40px 0' }}
          />
        ) : featureContent ? (
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
                    color: '#389e0d', 
                    margin: '0 0 16px 0',
                    borderBottom: '2px solid #b7eb8f',
                    paddingBottom: '8px'
                  }}>
                    {children}
                  </h1>
                ),
                h2: ({ children }) => (
                  <h2 style={{ 
                    fontSize: '16px', 
                    color: '#52c41a', 
                    margin: '16px 0 8px 0',
                    fontWeight: 600
                  }}>
                    {children}
                  </h2>
                ),
                h3: ({ children }) => (
                  <h3 style={{ 
                    fontSize: '15px', 
                    color: '#73d13d', 
                    margin: '12px 0 6px 0',
                    fontWeight: 600
                  }}>
                    {children}
                  </h3>
                ),
                p: ({ children }) => (
                  <p style={{ 
                    margin: '8px 0', 
                    color: '#434343',
                    lineHeight: '1.6'
                  }}>
                    {children}
                  </p>
                ),
                ul: ({ children }) => (
                  <ul style={{ 
                    margin: '8px 0', 
                    paddingLeft: 20, 
                    color: '#434343' 
                  }}>
                    {children}
                  </ul>
                ),
                ol: ({ children }) => (
                  <ol style={{ 
                    margin: '8px 0', 
                    paddingLeft: 20, 
                    color: '#434343' 
                  }}>
                    {children}
                  </ol>
                ),
                li: ({ children }) => (
                  <li style={{ 
                    margin: '4px 0',
                    position: 'relative'
                  }}>
                    <span style={{
                      position: 'absolute',
                      left: '-16px',
                      color: '#52c41a',
                      fontWeight: 'bold'
                    }}>
                      ✓
                    </span>
                    {children}
                  </li>
                ),
                strong: ({ children }) => (
                  <strong style={{ 
                    color: '#389e0d', 
                    fontWeight: 600 
                  }}>
                    {children}
                  </strong>
                ),
                em: ({ children }) => (
                  <em style={{ 
                    color: '#73d13d',
                    fontStyle: 'italic'
                  }}>
                    {children}
                  </em>
                ),
                code: ({ children, className, ...props }) => {
                  const match = /language-(\w+)/.exec(className || '');
                  const language = match ? match[1] : '';
                  const codeContent = String(children).replace(/\n$/, '');
                  
                  // 检查是否是 Mermaid 图表
                  if (language === 'mermaid') {
                    return <MermaidChart chart={codeContent} />;
                  }
                  
                  // 默认内联代码样式
                  if (!className?.includes('language-')) {
                    return (
                      <code style={{
                        background: '#f4ffb8',
                        padding: '2px 6px',
                        borderRadius: '3px',
                        fontSize: '13px',
                        color: '#389e0d',
                        border: '1px solid #d9f7be'
                      }}>
                        {children}
                      </code>
                    );
                  }
                  
                  // 代码块
                  return (
                    <pre style={{
                      backgroundColor: '#f6ffed',
                      border: '1px solid #d9f7be',
                      borderRadius: '6px',
                      padding: '12px',
                      overflow: 'auto',
                      fontSize: '14px',
                      lineHeight: '1.4'
                    }}>
                      <code>{children}</code>
                    </pre>
                  );
                },
                blockquote: ({ children }) => (
                  <blockquote style={{
                    borderLeft: '4px solid #b7eb8f',
                    margin: '16px 0',
                    padding: '12px 16px',
                    background: '#f6ffed',
                    fontStyle: 'italic',
                    color: '#595959'
                  }}>
                    {children}
                  </blockquote>
                )
              }}
            >
              {featureContent}
            </ReactMarkdown>
          </div>
        ) : (
          <Empty 
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <span style={{ color: '#999', fontSize: '12px' }}>
                暂无特性列表内容
              </span>
            }
            style={{ margin: '40px 0' }}
          />
        )}
      </div>
      
      <div style={{ 
        fontSize: '11px', 
        color: '#999', 
        flexShrink: 0,
        textAlign: 'center',
        padding: '8px 0'
      }}>
        📋 展示项目或会议的核心特性和功能清单
      </div>

      {/* 拷贝任务选择模态框 */}
      <Modal
        title="选择拷贝源任务"
        open={showCopyModal}
        onCancel={handleCopyCancel}
        onOk={handleCopy}
        okText="复制"
        cancelText="取消"
        afterOpenChange={(open) => console.log('Copy Modal open state changed:', open)}
      >
        <div style={{ marginBottom: 16 }}>
          <p>请选择要复制特性列表的源任务：</p>
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
        title="特性列表内容对比"
        currentContent={featureContent}
        sourceContent={sourceContent}
        onConfirm={performCopy}
        onCancel={handleCopyCancel}
        loading={copying}
      />
    </div>
  );
};