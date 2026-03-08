import React, { useState, useEffect, useRef } from 'react';
import { Select, Spin, message, Button, Popconfirm } from 'antd';
import { ClockCircleOutlined, DeleteOutlined } from '@ant-design/icons';
import { tagService, TagInfo } from '../../services/tagService';

const { Option } = Select;

interface TagVersionSelectProps {
  projectId: string;
  taskId: string;
  docType: 'requirements' | 'design' | 'test' | 'execution-plan';
  currentVersion?: string;
  onSwitchTag: (tagName: string) => Promise<void>;
  onTagDeleted?: () => void; // 删除tag后的回调
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
  onTagDeleted,
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
        let response;
        if (docType === 'execution-plan') {
          response = await tagService.listExecutionPlanTags(projectId, taskId);
        } else {
          response = await tagService.listTags(projectId, taskId, docType);
        }
        
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

  const handleDeleteTag = async (tagName: string, e?: React.MouseEvent) => {
    if (e) {
      e.stopPropagation(); // 阻止触发下拉选择
    }
    
    try {
      setLoading(true);
      if (docType === 'execution-plan') {
        await tagService.deleteExecutionPlanTag(projectId, taskId, tagName);
      } else {
        await tagService.deleteTag(projectId, taskId, docType, tagName);
      }
      message.success(`标签 "${tagName}" 删除成功`);
      
      // 从列表中移除已删除的tag
      setTags(prevTags => prevTags.filter(t => t.tag_name !== tagName));
      
      // 如果删除的是当前选中的tag，切换回当前版本
      if (selectedTag === tagName) {
        setSelectedTag(currentVersion);
      }
      
      // 调用回调通知父组件
      if (onTagDeleted) {
        onTagDeleted();
      }
    } catch (error: any) {
      message.error(`删除标签失败: ${error.message || '未知错误'}`);
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
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%' }}>
            <div style={{ display: 'flex', alignItems: 'center', flex: 1 }}>
              <span>🏷️ {tag.tag_name}</span>
              <span style={{ fontSize: '12px', color: '#8c8c8c', marginLeft: '8px' }}>
                {formatDate(tag.created_at)}
              </span>
            </div>
            <Popconfirm
              title="确认删除标签?"
              description={`删除后无法恢复，确定要删除标签 "${tag.tag_name}" 吗？`}
              onConfirm={(e) => {
                e?.stopPropagation();
                handleDeleteTag(tag.tag_name);
              }}
              onCancel={(e) => e?.stopPropagation()}
              okText="删除"
              cancelText="取消"
            >
              <Button
                type="text"
                size="small"
                icon={<DeleteOutlined />}
                danger
                onClick={(e) => e.stopPropagation()}
                style={{ marginLeft: 8, padding: '0 4px' }}
              />
            </Popconfirm>
          </div>
        </Option>
      ))}
    </Select>
  );
};

export default TagVersionSelect;
