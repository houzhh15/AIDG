/**
 * 角色表单模态框
 * 
 * 功能:
 * - 创建/编辑角色
 * - 表单验证
 * - 权限选择
 * 
 * 使用:
 * ```tsx
 * <RoleFormModal
 *   visible={true}
 *   projectId="AI-Dev-Gov"
 *   mode="create"
 *   onOk={handleCreate}
 *   onCancel={handleCancel}
 * />
 * 
 * <RoleFormModal
 *   visible={true}
 *   projectId="AI-Dev-Gov"
 *   mode="edit"
 *   initialValues={roleData}
 *   onOk={handleUpdate}
 *   onCancel={handleCancel}
 * />
 * ```
 */

import React, { useEffect } from 'react';
import { Modal, Form, Input, message } from 'antd';
import { PermissionSelector } from './PermissionSelector';
import type { Role, CreateRoleRequest, UpdateRoleRequest } from '../../api/roles';

interface RoleFormModalProps {
  /** 是否显示模态框 */
  visible: boolean;
  
  /** 项目 ID */
  projectId: string;
  
  /** 模式: 创建 or 编辑 */
  mode: 'create' | 'edit';
  
  /** 初始值 (edit 模式必填) */
  initialValues?: Role;
  
  /** 确定回调 */
  onOk: (data: CreateRoleRequest | UpdateRoleRequest) => Promise<void>;
  
  /** 取消回调 */
  onCancel: () => void;
  
  /** 确认按钮 loading 状态 */
  confirmLoading?: boolean;
}

/**
 * 角色表单模态框
 */
export const RoleFormModal: React.FC<RoleFormModalProps> = ({
  visible,
  projectId,
  mode,
  initialValues,
  onOk,
  onCancel,
  confirmLoading = false,
}) => {
  const [form] = Form.useForm();

  // 初始化表单
  useEffect(() => {
    if (visible) {
      if (mode === 'edit' && initialValues) {
        form.setFieldsValue({
          name: initialValues.name,
          description: initialValues.description,
          scopes: initialValues.scopes,
        });
      } else {
        form.resetFields();
      }
    }
  }, [visible, mode, initialValues, form]);

  // 提交表单
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      
      if (mode === 'create') {
        const data: CreateRoleRequest = {
          project_id: projectId,
          name: values.name.trim(),
          description: values.description?.trim() || '',
          scopes: values.scopes || [],
        };
        await onOk(data);
      } else {
        const data: UpdateRoleRequest = {
          name: values.name.trim(),
          description: values.description?.trim() || '',
          scopes: values.scopes || [],
        };
        await onOk(data);
      }
      
      form.resetFields();
    } catch (error: any) {
      // 表单验证失败或提交失败
      if (error.errorFields) {
        // Ant Design 表单验证错误
        return;
      }
      message.error(error.message || '操作失败');
    }
  };

  return (
    <Modal
      title={mode === 'create' ? '创建角色' : '编辑角色'}
      open={visible}
      onOk={handleSubmit}
      onCancel={onCancel}
      confirmLoading={confirmLoading}
      width={700}
      okText="确定"
      cancelText="取消"
      destroyOnClose
    >
      <Form
        form={form}
        layout="vertical"
        preserve={false}
      >
        <Form.Item
          label="角色名称"
          name="name"
          rules={[
            { required: true, message: '请输入角色名称' },
            { max: 50, message: '角色名称最多 50 个字符' },
            { 
              pattern: /^[\u4e00-\u9fa5a-zA-Z0-9_\-\s]+$/, 
              message: '角色名称只能包含中文、英文、数字、下划线和连字符' 
            },
          ]}
        >
          <Input placeholder="例如: 项目经理" />
        </Form.Item>

        <Form.Item
          label="角色描述"
          name="description"
          rules={[
            { max: 200, message: '角色描述最多 200 个字符' },
          ]}
        >
          <Input.TextArea
            placeholder="描述该角色的职责和权限范围"
            rows={3}
            showCount
            maxLength={200}
          />
        </Form.Item>

        <Form.Item
          label="权限配置"
          name="scopes"
          rules={[
            { required: true, message: '请至少选择一个权限' },
            {
              validator: (_, value) => {
                if (!value || value.length === 0) {
                  return Promise.reject(new Error('请至少选择一个权限'));
                }
                return Promise.resolve();
              },
            },
          ]}
        >
          <PermissionSelector />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default RoleFormModal;
