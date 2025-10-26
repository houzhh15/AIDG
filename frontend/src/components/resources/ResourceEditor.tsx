/**
 * ResourceEditor - 资源编辑器组件
 * 提供统一的 MCP 资源编辑体验，复用 MarkdownEditor 能力
 */

import React, { useState, useEffect } from 'react';
import {
  Form,
  Input,
  Select,
  Space,
  Button,
  Row,
  Col,
  message,
  Modal
} from 'antd';
import { SaveOutlined, CloseOutlined } from '@ant-design/icons';
import MarkdownEditor from '../documents/MarkdownEditor';
import { ResourceEditorProps, ResourceFormData } from './types';

const { TextArea } = Input;
const { Option } = Select;

/**
 * ResourceEditor 组件
 * 用于新增或编辑 MCP Resources
 */
const ResourceEditor: React.FC<ResourceEditorProps> = ({
  mode,
  initialValue,
  onSubmit,
  onCancel
}) => {
  // 表单数据状态
  const [formData, setFormData] = useState<ResourceFormData>({
    name: initialValue?.name || '',
    description: initialValue?.description || '',
    visibility: initialValue?.visibility || 'private',
    projectId: initialValue?.projectId || '',
    taskId: initialValue?.taskId || ''
  });

  // 资源内容状态（Markdown）
  const [content, setContent] = useState<string>(initialValue?.content || '');

  // 保存中状态
  const [saving, setSaving] = useState<boolean>(false);

  // 脏数据标记（是否有未保存的修改）
  const [dirty, setDirty] = useState<boolean>(false);

  /**
   * 监听 initialValue 变化，同步到组件状态
   * 用于编辑模式下数据预填充
   */
  useEffect(() => {
    if (initialValue) {
      setFormData({
        name: initialValue.name || '',
        description: initialValue.description || '',
        visibility: initialValue.visibility || 'private',
        projectId: initialValue.projectId || '',
        taskId: initialValue.taskId || ''
      });
      setContent(initialValue.content || '');
      setDirty(false); // 重置脏标记
    }
  }, [initialValue]);

  /**
   * 表单字段变更处理
   */
  const handleFormChange = (field: keyof ResourceFormData, value: string) => {
    setFormData(prev => ({ ...prev, [field]: value }));
    setDirty(true);
  };

  /**
   * Markdown 内容变更处理
   */
  const handleContentChange = (newContent: string) => {
    setContent(newContent);
    setDirty(true);
  };

  /**
   * 保存处理（通用）
   * @param shouldClose - 保存后是否关闭窗口
   */
  const handleSave = async (shouldClose: boolean = false) => {
    // 校验必填字段
    if (!formData.name.trim()) {
      message.error('资源名称不能为空');
      return;
    }
    if (!formData.description.trim()) {
      message.error('资源描述不能为空');
      return;
    }

    setSaving(true);
    try {
      // 构造 payload
      const payload = {
        name: formData.name.trim(),
        description: formData.description.trim(),
        content: content,
        visibility: formData.visibility,
        projectId: formData.projectId?.trim() || undefined,
        taskId: formData.taskId?.trim() || undefined
      };

      // 调用父组件的提交回调
      await onSubmit(payload);

      setDirty(false);
      message.success('保存成功');
      
      // 如果需要关闭窗口，调用取消回调
      if (shouldClose) {
        onCancel();
      }
    } catch (error: any) {
      console.error('ResourceEditor save failed:', error);
      message.error(error.message || '保存失败，请稍后重试');
    } finally {
      setSaving(false);
    }
  };

  /**
   * Markdown 编辑器内部保存（不关闭窗口）
   */
  const handleMarkdownSave = () => {
    handleSave(false);
  };

  /**
   * 底部保存按钮（关闭窗口）
   */
  const handleSaveAndClose = () => {
    handleSave(true);
  };

  /**
   * 取消处理（带未保存提示）
   */
  const handleCancel = () => {
    if (dirty) {
      Modal.confirm({
        title: '确认放弃更改？',
        content: '您有未保存的更改，是否确认放弃并关闭？',
        okText: '放弃更改',
        cancelText: '继续编辑',
        okType: 'danger',
        onOk: () => {
          onCancel();
        }
      });
    } else {
      onCancel();
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* 顶部表单区域 */}
      <Form layout="vertical" style={{ marginBottom: 16 }}>
        {/* 资源名称 */}
        <Form.Item
          label="资源名称"
          required
          style={{ marginBottom: 12 }}
        >
          <Input
            placeholder="请输入资源名称"
            value={formData.name}
            onChange={e => handleFormChange('name', e.target.value)}
            maxLength={100}
            showCount
          />
        </Form.Item>

        {/* 资源描述 */}
        <Form.Item
          label="资源描述"
          required
          style={{ marginBottom: 12 }}
        >
          <TextArea
            placeholder="请输入资源描述（不超过500字符）"
            value={formData.description}
            onChange={e => handleFormChange('description', e.target.value)}
            maxLength={500}
            showCount
            rows={3}
          />
        </Form.Item>

        {/* 可见性选择 */}
        <Form.Item
          label="可见性"
          style={{ marginBottom: 12 }}
        >
          <Select
            value={formData.visibility}
            onChange={value => handleFormChange('visibility', value)}
          >
            <Option value="private">私有（仅自己可见）</Option>
            <Option value="public">公开（项目成员可见）</Option>
          </Select>
        </Form.Item>

        {/* 项目ID 和 任务ID（两列布局） */}
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label="项目ID（可选）" style={{ marginBottom: 12 }}>
              <Input
                placeholder="关联的项目ID"
                value={formData.projectId}
                onChange={e => handleFormChange('projectId', e.target.value)}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label="任务ID（可选）" style={{ marginBottom: 12 }}>
              <Input
                placeholder="关联的任务ID"
                value={formData.taskId}
                onChange={e => handleFormChange('taskId', e.target.value)}
              />
            </Form.Item>
          </Col>
        </Row>
      </Form>

      {/* 中间 Markdown 编辑器区域 */}
      <div style={{ flex: 1, minHeight: 400, marginBottom: 16 }}>
        <MarkdownEditor
          value={content}
          onChange={handleContentChange}
          onSave={handleMarkdownSave}
          showToolbar={true}
          showPreview={true}
          autoSave={false}
          height={400}
          placeholder="请输入资源内容（支持 Markdown 格式）..."
        />
      </div>

      {/* 底部按钮区域 */}
      <Space style={{ justifyContent: 'flex-end', width: '100%' }}>
        <Button onClick={handleCancel} icon={<CloseOutlined />}>
          取消
        </Button>
        <Button
          type="primary"
          loading={saving}
          onClick={handleSaveAndClose}
          icon={<SaveOutlined />}
        >
          保存
        </Button>
      </Space>
    </div>
  );
};

export default ResourceEditor;
