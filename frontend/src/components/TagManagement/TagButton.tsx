import React, { useState } from 'react';
import { Button, Modal, Form, Input, message } from 'antd';
import { TagOutlined } from '@ant-design/icons';

interface TagButtonProps {
  onCreateTag: (tagName: string) => Promise<void>;
  docType?: 'requirements' | 'design' | 'test' | 'execution_plan';
  disabled?: boolean;
  size?: 'large' | 'middle' | 'small';
}

export const TagButton: React.FC<TagButtonProps> = ({ 
  onCreateTag, 
  docType = 'requirements',
  disabled = false,
  size = 'middle'
}) => {
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm();

  const showModal = () => {
    setIsModalVisible(true);
  };

  const handleCancel = () => {
    setIsModalVisible(false);
    form.resetFields();
  };

  const handleOk = async () => {
    try {
      const values = await form.validateFields();
      const tagName = values.tagName.trim();
      
      setLoading(true);
      await onCreateTag(tagName);
      
      message.success(`标签 "${tagName}" 创建成功`);
      setIsModalVisible(false);
      form.resetFields();
    } catch (error: any) {
      if (error.errorFields) {
        // Form validation error
        return;
      }
      message.error(`创建标签失败: ${error.message || '未知错误'}`);
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button
        type="primary"
        icon={<TagOutlined />}
        onClick={showModal}
        disabled={disabled}
        size={size}
      >
        创建标签
      </Button>

      <Modal
        title="创建新标签"
        open={isModalVisible}
        onOk={handleOk}
        onCancel={handleCancel}
        confirmLoading={loading}
        okText="创建"
        cancelText="取消"
        destroyOnClose
      >
        <Form
          form={form}
          layout="vertical"
          name="createTagForm"
        >
          <Form.Item
            name="tagName"
            label="标签名称"
            rules={[
              { required: true, message: '请输入标签名称' },
              { 
                pattern: /^[a-zA-Z0-9_-]{1,50}$/, 
                message: '标签名称只能包含字母、数字、下划线和连字符，长度1-50字符' 
              },
              { max: 50, message: '标签名称不能超过50个字符' }
            ]}
          >
            <Input 
              placeholder="例如: v1.0, feature-auth, sprint-2024-01"
              maxLength={50}
            />
          </Form.Item>
          
          <Form.Item label="标签说明" style={{ marginBottom: 0 }}>
            <div style={{ fontSize: '12px', color: '#8c8c8c' }}>
              • 标签将保存当前文档的快照版本<br />
              • 标签名称格式：字母、数字、下划线(_)、连字符(-)<br />
              • 建议使用版本号或描述性名称，如 v1.0、milestone-1
            </div>
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

export default TagButton;
