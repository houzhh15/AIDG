import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Card, Input, Button, List, Tag, Empty, Spin, Space, Typography, Tooltip, Badge, Switch, message } from 'antd';
import { SearchOutlined, BulbOutlined, FileTextOutlined, SettingOutlined } from '@ant-design/icons';
import { getRecommendationsByQuery, getRecommendationsLive, Recommendation } from '../api/recommendations';

const { TextArea } = Input;
const { Text, Link } = Typography;

// 自定义防抖函数
function debounce<T extends (...args: never[]) => void>(func: T, delay: number) {
  let timeoutId: ReturnType<typeof setTimeout>;
  const debounced = (...args: Parameters<T>) => {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => func(...args), delay);
  };
  debounced.cancel = () => clearTimeout(timeoutId);
  debounced.flush = () => {
    clearTimeout(timeoutId);
  };
  return debounced;
}

interface SmartRecommendationSidebarProps {
  projectId: string;
  taskId: string;
  docType: 'requirements' | 'design' | 'test';
  currentContent?: string;
  mode: 'preview' | 'live';
  onRecommendationClick?: (taskId: string, sectionId: string) => void;
}

const SmartRecommendationSidebar: React.FC<SmartRecommendationSidebarProps> = ({
  projectId,
  taskId,
  docType,
  currentContent,
  mode,
  onRecommendationClick
}) => {
  console.log('[SmartRecommendationSidebar] Component mounted/updated:', {
    projectId,
    taskId,
    docType,
    mode,
    currentContentLength: currentContent?.length || 0,
    currentContentPreview: currentContent?.substring(0, 50)
  });

  const [queryText, setQueryText] = useState('');
  const [recommendations, setRecommendations] = useState<Recommendation[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [newRecommendationsCount, setNewRecommendationsCount] = useState(0);
  const [liveRecommendationEnabled, setLiveRecommendationEnabled] = useState(true);
  const [debounceDelay, setDebounceDelay] = useState(3000);
  const lastContentLength = useRef(0);
  const lastSearchTime = useRef(0);
  const abortControllerRef = useRef<AbortController | null>(null);
  
  // 调试状态
  const [debugInfo, setDebugInfo] = useState<string>('初始化');
  const [lastTriggerTime, setLastTriggerTime] = useState<string>('未触发');

  // 写作前推荐：手动搜索
  const handleSearch = async () => {
    if (!queryText.trim()) {
      return;
    }

    setLoading(true);
    setError(null);
    
    try {
      const result = await getRecommendationsByQuery(projectId, taskId, {
        query_text: queryText,
        doc_type: docType,
        top_k: 5,
        threshold: 0.6
      });
      setRecommendations(result.data?.recommendations || []);
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : '推荐失败';
      setError(errorMessage);
      console.error('获取推荐失败:', err);
    } finally {
      setLoading(false);
    }
  };

  // 半实时推荐：简化触发逻辑（仅防抖，无阈值限制）
  const debouncedLiveSearch = useCallback(
    debounce(async (content: string) => {
      const timestamp = new Date().toLocaleTimeString();
      setLastTriggerTime(timestamp);
      
      console.log('[Live Recommendation] debouncedLiveSearch called:', {
        enabled: liveRecommendationEnabled,
        contentLength: content.length,
        contentPreview: content.substring(0, 100)
      });
      
      setDebugInfo(`触发时间: ${timestamp}`);

      if (!liveRecommendationEnabled) {
        console.log('[Live Recommendation] 实时推荐已关闭');
        setDebugInfo('实时推荐已关闭');
        return;
      }

      // 最小内容要求：50字符（与查询API一致）
      if (content.length < 50) {
        console.log('[Live Recommendation] 内容不足50字，跳过推荐');
        setDebugInfo(`内容不足50字 (${content.length})`);
        return;
      }

      console.log('[Live Recommendation] 开始查询推荐...');
      setDebugInfo('开始查询推荐...');

      // 取消前一次未完成的请求（避免积压）
      if (abortControllerRef.current) {
        console.log('[Live Recommendation] 取消前一次请求');
        abortControllerRef.current.abort();
      }

      setLoading(true);
      setError(null);

      // 创建新的取消控制器
      abortControllerRef.current = new AbortController();

      try {
        const result = await getRecommendationsLive(
          projectId, 
          taskId, 
          {
            query_text: content.substring(0, 500),
            doc_type: docType,
            top_k: 5,
            threshold: 0.5,  // 降低阈值：0.7 -> 0.5
            exclude_task_id: taskId
          },
          abortControllerRef.current.signal
        );
        
        console.log('[Live Recommendation] API响应:', result);
        
        // 检查是否有 reason 字段（后端返回的跳过原因）
        if (result.data?.reason) {
          setDebugInfo(`后端跳过: ${result.data.reason}`);
          console.log('[Live Recommendation] 后端跳过原因:', result.data.reason);
          setRecommendations([]);
          return;
        }
        
        setDebugInfo(`API成功: ${result.data?.recommendations?.length || 0}条`);
        
        const newRecs = result.data?.recommendations || [];
        
        // 比较推荐结果，更新徽章
        if (newRecs.length > 0 && JSON.stringify(newRecs) !== JSON.stringify(recommendations)) {
          setNewRecommendationsCount(newRecs.length);
        }
        
        setRecommendations(newRecs);
        lastContentLength.current = content.length;
        lastSearchTime.current = Date.now();
        abortControllerRef.current = null;
      } catch (err: unknown) {
        console.error('[Live Recommendation] 错误:', err);
        if (err instanceof Error && err.name === 'AbortError') {
          console.log('[Live Recommendation] 请求已取消');
          setDebugInfo('请求已取消');
        } else {
          const errMsg = err instanceof Error ? err.message : String(err);
          console.error('实时推荐失败:', errMsg);
          setDebugInfo(`错误: ${errMsg}`);
          setError(errMsg);
        }
      } finally {
        setLoading(false);
      }
    }, debounceDelay),
    [projectId, taskId, docType, liveRecommendationEnabled, debounceDelay, recommendations]
  );

  // 手动触发推荐（Cmd+K快捷键）：与自动触发保持一致
  const handleManualTrigger = useCallback(() => {
    if (currentContent && currentContent.length >= 50) {
      console.log('[Manual Trigger] 手动触发推荐（Cmd+K）');
      debouncedLiveSearch.cancel();
      debouncedLiveSearch.flush();
    } else {
      message.warning('内容至少需要50个字符才能触发推荐');
    }
  }, [currentContent, debouncedLiveSearch]);

  // 监听Cmd+K快捷键
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        handleManualTrigger();
      }
    };

    if (mode === 'live') {
      window.addEventListener('keydown', handleKeyDown);
      return () => window.removeEventListener('keydown', handleKeyDown);
    }
  }, [mode, handleManualTrigger]);

  // 监听currentContent变化（半实时模式）
  useEffect(() => {
    console.log('[SmartRecommendation] currentContent changed:', {
      mode,
      contentLength: currentContent?.length || 0,
      hasContent: !!currentContent,
      preview: currentContent?.substring(0, 100)
    });
    
    if (mode === 'live' && currentContent) {
      debouncedLiveSearch(currentContent);
    }
  }, [currentContent, mode, debouncedLiveSearch]);

  // 组件卸载时取消未完成的请求
  useEffect(() => {
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, []);

  // 点击推荐卡片后清除徽章
  const handleRecommendationClick = (taskId: string, sectionId: string) => {
    setNewRecommendationsCount(0);
    onRecommendationClick?.(taskId, sectionId);
  };

  const getSimilarityColor = (similarity: number) => {
    if (similarity >= 0.8) return 'green';
    if (similarity >= 0.7) return 'blue';
    return 'orange';
  };

  const renderPreviewMode = () => (
    <>
      <Space direction="vertical" style={{ width: '100%', marginBottom: 16 }}>
        <Text type="secondary">输入任务描述或关键词，查找相似的历史文档</Text>
        <TextArea
          placeholder="例如：实现用户登录功能，支持手机号和邮箱登录"
          value={queryText}
          onChange={(e) => setQueryText(e.target.value)}
          rows={4}
          onPressEnter={(e) => {
            if (e.ctrlKey || e.metaKey) {
              handleSearch();
            }
          }}
        />
        <Button
          type="primary"
          icon={<SearchOutlined />}
          onClick={handleSearch}
          loading={loading}
          block
        >
          查找相似文档
        </Button>
      </Space>
    </>
  );

  const renderLiveMode = () => (
    <div style={{ marginBottom: 16 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
        <Space>
          <Badge count={newRecommendationsCount} offset={[10, 0]}>
            <BulbOutlined style={{ color: '#1890ff', fontSize: 16 }} />
          </Badge>
          <Text strong>半实时智能推荐</Text>
          {loading && <Spin size="small" />}
        </Space>
        <Tooltip title="关闭实时推荐">
          <Switch
            size="small"
            checked={liveRecommendationEnabled}
            onChange={setLiveRecommendationEnabled}
          />
        </Tooltip>
      </Space>

      <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
        根据您编辑内容自动推荐（停顿3秒后触发）
      </Text>
      <Text type="secondary" style={{ display: 'block', marginTop: 4, fontSize: 11, color: '#8c8c8c' }}>
        💡 按 <Tag style={{ margin: '0 2px' }}>Cmd+K</Tag> 立即触发推荐
      </Text>

      {/* 用户偏好设置（可折叠） */}
      <details style={{ marginTop: 12, fontSize: 12 }}>
        <summary style={{ cursor: 'pointer', color: '#1890ff' }}>
          <SettingOutlined /> 偏好设置
        </summary>
        <Space direction="vertical" style={{ width: '100%', marginTop: 8, paddingLeft: 16 }}>
          <div>
            <Text type="secondary">防抖延迟：</Text>
            <Input
              type="number"
              size="small"
              value={debounceDelay / 1000}
              onChange={(e) => setDebounceDelay(Number(e.target.value) * 1000)}
              suffix="秒"
              style={{ width: 80, marginLeft: 8 }}
              min={1}
              max={10}
            />
          </div>
        </Space>
      </details>
    </div>
  );

  const renderRecommendations = () => {
    if (error) {
      return (
        <Empty
          description={error}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        >
          <Button size="small" onClick={mode === 'preview' ? handleSearch : undefined}>
            重试
          </Button>
        </Empty>
      );
    }

    if (recommendations.length === 0) {
      return (
        <Empty
          description={mode === 'preview' ? '暂无推荐，请尝试输入更多关键词' : '暂无相似文档'}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      );
    }

    return (
      <List
        dataSource={recommendations}
        renderItem={(item) => (
          <List.Item 
            style={{ 
              padding: '12px 0',
              animation: newRecommendationsCount > 0 ? 'fadeIn 0.5s ease-in' : 'none'
            }}
          >
            <List.Item.Meta
              avatar={<FileTextOutlined style={{ fontSize: 18, color: '#1890ff' }} />}
              title={
                <Space>
                  <Link
                    onClick={() => handleRecommendationClick(item.task_id, item.section_id)}
                    style={{ fontSize: 13 }}
                  >
                    {item.title}
                  </Link>
                  <Tag color={getSimilarityColor(item.similarity)} style={{ fontSize: 11 }}>
                    {(item.similarity * 100).toFixed(0)}%
                  </Tag>
                </Space>
              }
              description={
                <div>
                  <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>
                    {item.snippet}
                  </Text>
                  <Text type="secondary" style={{ fontSize: 10, marginTop: 4, display: 'block' }}>
                    {item.task_id} / {item.doc_type}
                  </Text>
                </div>
              }
            />
          </List.Item>
        )}
      />
    );
  };

  return (
    <Card
      title={
        <Space>
          <BulbOutlined />
          <span>智能推荐</span>
          {mode === 'live' && (
            <Tooltip title="基于您当前编辑的内容自动推荐（3秒防抖 + 请求去重）">
              <Tag color="blue" style={{ marginLeft: 8 }}>半实时</Tag>
            </Tooltip>
          )}
        </Space>
      }
      size="small"
      style={{ height: '100%', display: 'flex', flexDirection: 'column' }}
      bodyStyle={{ flex: 1, overflow: 'auto' }}
    >
      {/* 调试信息 */}
      <div style={{ background: '#fff1f0', border: '1px solid #ffa39e', padding: '8px', marginBottom: '12px', fontSize: '12px' }}>
        <div>模式: {mode}</div>
        <div>内容长度: {currentContent?.length || 0}</div>
        <div>内容预览: {currentContent?.substring(0, 50)}...</div>
        <div>实时推荐: {liveRecommendationEnabled ? '开启' : '关闭'}</div>
        <div>防抖延迟: {debounceDelay}ms</div>
        <div>推荐数量: {recommendations.length}</div>
        <div>加载中: {loading ? '是' : '否'}</div>
        <div>错误: {error || '无'}</div>
        <div style={{ marginTop: '4px', paddingTop: '4px', borderTop: '1px solid #ffa39e' }}>
          <div>上次触发: {lastTriggerTime}</div>
          <div>状态: {debugInfo}</div>
        </div>
      </div>
      
      {mode === 'preview' ? renderPreviewMode() : renderLiveMode()}
      {renderRecommendations()}
    </Card>
  );
};

// 添加CSS动画
const style = document.createElement('style');
style.textContent = `
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(-10px); }
    to { opacity: 1; transform: translateY(0); }
  }
`;
document.head.appendChild(style);

export default SmartRecommendationSidebar;
