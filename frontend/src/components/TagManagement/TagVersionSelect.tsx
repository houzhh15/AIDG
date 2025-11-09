import React, { useState, useEffect, useRef } from 'react';
import { Select, Spin, message } from 'antd';
import { ClockCircleOutlined } from '@ant-design/icons';
import { tagService, TagInfo } from '../../services/tagService';

const { Option } = Select;

interface TagVersionSelectProps {
  projectId: string;
  taskId: string;
  docType: 'requirements' | 'design' | 'test';
  currentVersion?: string;
  onSwitchTag: (tagName: string) => Promise<void>;
  disabled?: boolean;
  style?: React.CSSProperties;
  refreshKey?: number;
  size?: 'large' | 'middle' | 'small';
}

export const TagVersionSelect: React.FC<TagVersionSelectProps> = ({
  projectId,
  taskId,
  docType,
  currentVersion = '当前版本',
  onSwitchTag,
  disabled = false,
  style,
  refreshKey = 0,
  size = 'middle'
}) => {
  const [loading, setLoading] = useState(false);
  const [tags, setTags] = useState<TagInfo[]>([]);
  const [selectedTag, setSelectedTag] = useState<string>(currentVersion);
  
  // 使用 ref 存储上次加载的参数，用于对比
  const lastLoadParamsRef = useRef<string>('');

  // 当 currentVersion prop 变化时，同步更新内部状态
  useEffect(() => {
    setSelectedTag(currentVersion);
  }, [currentVersion]);

  // 加载标签列表
  useEffect(() => {
    const loadParams = `${projectId}-${taskId}-${docType}-${refreshKey}`;
    
    // 防止重复加载
    if (lastLoadParamsRef.current === loadParams && tags.length > 0) {
      return;
    }
    
    lastLoadParamsRef.current = loadParams;

    const loadTags = async () => {
      try {
        setLoading(true);
        const response = await tagService.listTags(projectId, taskId, docType);
        
        // 直接替换，不要任何合并逻辑
        const newTags = response.tags || [];
        setTags(newTags);
      } catch (error: any) {
        message.error(`加载标签列表失败: ${error.message || '未知错误'}`);
        setTags([]);
      } finally {
        setLoading(false);
      }
    };

    loadTags();
  }, [projectId, taskId, docType, refreshKey]); // 直接依赖所有参数

  const handleChange = async (value: string) => {
    if (value === currentVersion) {
      // 切换回当前版本，不需要调用API
      setSelectedTag(value);
      return;
    }

    try {
      setLoading(true);
      await onSwitchTag(value);
      setSelectedTag(value);
    } catch (error: any) {
      message.error(`切换标签失败: ${error.message || '未知错误'}`);
      // 保持原来的选择
    } finally {
      setLoading(false);
    }
  };

  const formatDate = (dateStr: string) => {
    try {
      const date = new Date(dateStr);
      return date.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit'
      });
    } catch {
      return dateStr;
    }
  };

  // 生成唯一的 key 强制 Select 重新渲染
  const selectKey = `${projectId}-${taskId}-${docType}-${tags.length}-${tags.map(t => t.tag_name).join('-')}`;

  return (
    <Select
      key={selectKey}
      value={selectedTag}
      onChange={handleChange}
      loading={loading}
      disabled={disabled || loading}
      style={{ minWidth: 200, ...style }}
      placeholder="选择标签版本"
      notFoundContent={loading ? <Spin size="small" /> : '暂无标签'}
      suffixIcon={<ClockCircleOutlined />}
      size={size}
    >
      <Option key={`current-${currentVersion}`} value={currentVersion}>
        <span style={{ fontWeight: 'bold' }}>📝 {currentVersion}</span>
      </Option>
      
      {tags.map((tag) => (
        <Option key={`tag-${tag.tag_name}`} value={tag.tag_name}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span>🏷️ {tag.tag_name}</span>
            <span style={{ fontSize: '12px', color: '#8c8c8c', marginLeft: '8px' }}>
              {formatDate(tag.created_at)}
            </span>
          </div>
        </Option>
      ))}
    </Select>
  );
};

export default TagVersionSelect;
