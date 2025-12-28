/**
 * Prompts Editor Modal Component
 * Prompts 编辑弹窗 - 支持创建和编辑模式
 */

import React, { useState, useEffect } from 'react';
import {
  Modal,
  Form,
  Input,
  Radio,
  Button,
  Space,
  Checkbox,
  message,
} from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { Prompt, PromptArgument } from '../types/prompt';
import { authedApi } from '../api/auth';

const { TextArea } = Input;

interface PromptsEditorModalProps {
  visible: boolean;
  mode: 'create' | 'edit';
  initialPrompt?: Prompt;
  scope: 'global' | 'project' | 'personal';
  projectId?: string;
  onClose: () => void;
  onSuccess: () => void;
}

const PromptsEditorModal: React.FC<PromptsEditorModalProps> = ({
  visible,
  mode,
  initialPrompt,
  scope,
  projectId,
  onClose,
  onSuccess,
}) => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [arguments_, setArguments] = useState<PromptArgument[]>([]);
  const [formDirty, setFormDirty] = useState(false); // 追踪表单是否被修改

  // 初始化表单
  useEffect(() => {
    if (visible) {
      setFormDirty(false); // 重置修改状态
      if (mode === 'edit' && initialPrompt) {
        form.setFieldsValue({
          name: initialPrompt.name,
          description: initialPrompt.description || '',
          content: initialPrompt.content,
          visibility: initialPrompt.visibility,
        });
        setArguments(initialPrompt.arguments || []);
      } else {
        form.resetFields();
        // 根据 scope 自动设置 visibility
        const defaultVisibility = scope === 'personal' ? 'private' : 'public';
        form.setFieldsValue({ visibility: defaultVisibility });
        setArguments([]);
      }
    }
  }, [visible, mode, initialPrompt, form, scope]);

  // 添加参数
  const handleAddArgument = () => {
    setArguments([
      ...arguments_,
      { name: '', description: '', required: false },
    ]);
    setFormDirty(true);
  };

  // 删除参数
  const handleRemoveArgument = (index: number) => {
    const newArguments = [...arguments_];
    newArguments.splice(index, 1);
    setArguments(newArguments);
    setFormDirty(true);
  };

  // 更新参数
  const handleArgumentChange = (
    index: number,
    field: keyof PromptArgument,
    value: any
  ) => {
    const newArguments = [...arguments_];
    newArguments[index] = { ...newArguments[index], [field]: value };
    setArguments(newArguments);
    setFormDirty(true);
  };

  // 提交表单
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();

      // 校验参数名称
      for (const arg of arguments_) {
        if (arg.name && !/^[a-zA-Z0-9_]+$/.test(arg.name)) {
          message.error(`参数名称 "${arg.name}" 格式不正确，仅支持字母、数字和下划线`);
          return;
        }
      }

      setLoading(true);

      // 构造请求体
      const requestBody = {
        name: values.name,
        description: values.description || '',
        content: values.content,
        arguments: arguments_.filter((arg) => arg.name), // 过滤空参数
        visibility: values.visibility,
        scope,
        project_id: projectId,
      };

      // 确定 API URL 和方法（authedApi 已配置 baseURL='/api/v1'，使用相对路径）
      let url = '/prompts';
      let response;

      if (mode === 'edit' && initialPrompt) {
        url = `/prompts/${initialPrompt.prompt_id}`;
        response = await authedApi.put(url, requestBody);
      } else {
        if (scope === 'project' && projectId) {
          url = `/projects/${projectId}/prompts`;
        }
        response = await authedApi.post(url, requestBody);
      }

      if (response.data.success) {
        message.success(mode === 'create' ? '创建成功' : '更新成功');
        onSuccess();
      } else {
        throw new Error(response.data.error || '操作失败');
      }
    } catch (err) {
      message.error(`操作失败: ${err instanceof Error ? err.message : '未知错误'}`);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={mode === 'create' ? '创建 Prompt' : '编辑 Prompt'}
      open={visible}
      onCancel={() => {
        if (formDirty) {
          Modal.confirm({
            title: '未保存的更改',
            content: '当前有未保存的更改，关闭将丢失这些更改。确认关闭吗？',
            okText: '确认关闭',
            cancelText: '继续编辑',
            okType: 'danger',
            onOk: () => {
              setFormDirty(false);
              onClose();
            }
          });
        } else {
          onClose();
        }
      }}
      onOk={handleSubmit}
      confirmLoading={loading}
      width={800}
      destroyOnClose
    >
      <Form 
        form={form} 
        layout="vertical"
        onValuesChange={() => setFormDirty(true)}
      >
        <Form.Item
          label="Name"
          name="name"
          rules={[{ required: true, message: '请输入名称' }]}
        >
          <Input placeholder="如：Python代码重构助手" />
        </Form.Item>

        <Form.Item label="Description" name="description">
          <TextArea rows={2} placeholder="可选，描述 Prompt 的用途" />
        </Form.Item>

        <Form.Item
          label="Content"
          name="content"
          rules={[{ required: true, message: '请输入内容' }]}
        >
          <TextArea
            rows={8}
            placeholder="支持 Markdown 格式和参数占位符 {{key}}"
          />
        </Form.Item>

        <Form.Item label="Arguments">
          <Space direction="vertical" style={{ width: '100%' }}>
            {arguments_.map((arg, index) => (
              <Space key={index} style={{ width: '100%' }} align="start">
                <Input
                  placeholder="参数名"
                  value={arg.name}
                  onChange={(e) =>
                    handleArgumentChange(index, 'name', e.target.value)
                  }
                  style={{ width: 150 }}
                />
                <Input
                  placeholder="描述（可选）"
                  value={arg.description}
                  onChange={(e) =>
                    handleArgumentChange(index, 'description', e.target.value)
                  }
                  style={{ width: 300 }}
                />
                <Checkbox
                  checked={arg.required}
                  onChange={(e) =>
                    handleArgumentChange(index, 'required', e.target.checked)
                  }
                >
                  必填
                </Checkbox>
                <Button
                  icon={<DeleteOutlined />}
                  size="small"
                  danger
                  onClick={() => handleRemoveArgument(index)}
                />
              </Space>
            ))}
            <Button
              type="dashed"
              icon={<PlusOutlined />}
              onClick={handleAddArgument}
              style={{ width: '100%' }}
            >
              添加参数
            </Button>
          </Space>
        </Form.Item>

        {/* Visibility - 仅在编辑模式或特定场景下显示 */}
        {mode === 'edit' && (
          <Form.Item
            label="Visibility"
            name="visibility"
            rules={[{ required: true }]}
            help={
              scope === 'project' 
                ? '项目 Prompts 通常为 Public，项目成员可见' 
                : scope === 'personal'
                ? 'Personal Prompts 默认为 Private'
                : scope === 'global'
                ? 'Global Prompts 默认为 Public'
                : ''
            }
          >
            <Radio.Group>
              <Radio value="public">Public（所有人可见）</Radio>
              <Radio value="private">Private（仅自己可见）</Radio>
            </Radio.Group>
          </Form.Item>
        )}
        
        {/* 隐藏字段，保存默认值 */}
        {mode === 'create' && (
          <Form.Item name="visibility" hidden>
            <Input />
          </Form.Item>
        )}
      </Form>
    </Modal>
  );
};

export default PromptsEditorModal;
