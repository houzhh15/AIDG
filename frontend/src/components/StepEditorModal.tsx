/**
 * StepEditorModal.tsx
 * 执行计划步骤编辑模态框组件
 * 支持创建、插入、编辑三种模式
 */

import React, { useEffect } from 'react';
import { Modal, Form, Input, Select, Space, message } from 'antd';
import type { ExecutionPlanStep, StepStatus, StepPriority } from '../utils/planMarkdownBuilder';

const { TextArea } = Input;

export type StepEditMode = 'create' | 'insert' | 'edit';

export interface StepFormData {
  description: string;
  status: StepStatus;
  priority?: StepPriority;
  dependencies?: string[];
  insertPosition?: number; // 仅用于 insert 模式
}

export interface StepEditorModalProps {
  mode: StepEditMode;
  visible: boolean;
  initialStep?: ExecutionPlanStep;
  availableSteps?: ExecutionPlanStep[];
  insertPosition?: number; // insert 模式的插入位置
  planStatus?: string; // 执行计划状态，用于控制字段显示
  onSubmit: (data: StepFormData) => void;
  onCancel: () => void;
}

const statusOptions: { label: string; value: StepStatus }[] = [
  { label: '待执行', value: 'pending' },
  { label: '执行中', value: 'in-progress' },
  { label: '已完成', value: 'succeeded' },
  { label: '失败', value: 'failed' },
  { label: '已取消', value: 'cancelled' },
];

const priorityOptions: { label: string; value: StepPriority }[] = [
  { label: '高', value: 'high' },
  { label: '中', value: 'medium' },
  { label: '低', value: 'low' },
];

/**
 * 检测循环依赖
 * @param stepId - 当前步骤 ID
 * @param selectedDeps - 用户选择的依赖 ID 列表
 * @param allSteps - 所有步骤
 * @returns 是否存在循环依赖
 */
function detectCyclicDependency(
  stepId: string,
  selectedDeps: string[],
  allSteps: ExecutionPlanStep[]
): boolean {
  const visited = new Set<string>();
  const stack = [...selectedDeps];

  while (stack.length > 0) {
    const current = stack.pop()!;
    
    if (current === stepId) {
      return true; // 发现循环
    }

    if (visited.has(current)) {
      continue;
    }

    visited.add(current);

    const step = allSteps.find((s) => s.id === current);
    if (step?.dependencies) {
      stack.push(...step.dependencies);
    }
  }

  return false;
}

export const StepEditorModal: React.FC<StepEditorModalProps> = ({
  mode,
  visible,
  initialStep,
  availableSteps = [],
  insertPosition,
  planStatus,
  onSubmit,
  onCancel,
}) => {
  const [form] = Form.useForm();

  useEffect(() => {
    if (visible) {
      if (mode === 'edit' && initialStep) {
        // 编辑模式，预填数据
        form.setFieldsValue({
          description: initialStep.description,
          status: initialStep.status,
          priority: initialStep.priority,
          dependencies: initialStep.dependencies || [],
        });
      } else {
        // 创建或插入模式，清空表单
        form.resetFields();
        form.setFieldsValue({
          status: 'pending',
          priority: 'medium',
          dependencies: [],
        });
      }
    }
  }, [visible, mode, initialStep, form]);

  const handleOk = async () => {
    try {
      const values = await form.validateFields();

      // 如果描述字段被隐藏，则使用初始值
      if (!shouldShowDescription() && initialStep) {
        values.description = initialStep.description;
      }

      // 如果优先级字段被隐藏，则使用初始值
      if (!shouldShowPriority() && initialStep) {
        values.priority = initialStep.priority;
      }

      // 如果依赖字段被隐藏，则使用初始值
      if (!shouldShowDependencies() && initialStep) {
        values.dependencies = initialStep.dependencies || [];
      }

      // 依赖校验：不能包含自身
      if (initialStep && values.dependencies?.includes(initialStep.id)) {
        message.error('依赖列表不能包含当前步骤');
        return;
      }

      // 循环依赖检测
      if (initialStep && values.dependencies?.length > 0) {
        const hasCycle = detectCyclicDependency(
          initialStep.id,
          values.dependencies,
          availableSteps
        );
        if (hasCycle) {
          message.error('检测到循环依赖，请重新选择');
          return;
        }
      }

      const formData: StepFormData = {
        description: values.description,
        status: values.status,
        priority: values.priority,
        dependencies: values.dependencies || [],
      };

      if (mode === 'insert' && insertPosition !== undefined) {
        formData.insertPosition = insertPosition;
      }

      onSubmit(formData);
      form.resetFields();
    } catch (error) {
      console.error('表单校验失败:', error);
    }
  };

  const handleCancel = () => {
    form.resetFields();
    onCancel();
  };

  const getTitle = () => {
    switch (mode) {
      case 'create':
        return '新建步骤';
      case 'insert':
        return '插入步骤';
      case 'edit':
        return '编辑步骤';
      default:
        return '步骤编辑';
    }
  };

  // 过滤掉当前步骤自身（编辑模式）
  const dependencyOptions = availableSteps
    .filter((step) => step.id !== initialStep?.id)
    .map((step) => ({
      label: `${step.id}: ${step.description}`,
      value: step.id,
    }));

  // 判断是否显示描述字段
  // 在 Approved/Executing/Completed 状态下，编辑步骤时隐藏描述字段，只允许修改状态
  const shouldShowDescription = () => {
    if (!planStatus) return true;
    
    const restrictedStatuses = ['Approved', 'Executing', 'Completed'];
    if (!restrictedStatuses.includes(planStatus)) return true;
    
    // 在受限状态下，只有创建和插入模式才显示描述字段
    return mode !== 'edit';
  };

  // 判断是否显示优先级字段
  const shouldShowPriority = () => {
    if (!planStatus) return true;
    
    const restrictedStatuses = ['Approved', 'Executing', 'Completed'];
    if (!restrictedStatuses.includes(planStatus)) return true;
    
    // 在受限状态下，编辑模式不显示优先级字段
    return mode !== 'edit';
  };

  // 判断是否显示依赖字段
  const shouldShowDependencies = () => {
    if (!planStatus) return true;
    
    const restrictedStatuses = ['Approved', 'Executing', 'Completed'];
    if (!restrictedStatuses.includes(planStatus)) return true;
    
    // 在受限状态下，编辑模式不显示依赖字段
    return mode !== 'edit';
  };

  return (
    <Modal
      title={getTitle()}
      open={visible}
      onOk={handleOk}
      onCancel={handleCancel}
      width={600}
      okText="确定"
      cancelText="取消"
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          status: 'pending',
          priority: 'medium',
          dependencies: [],
        }}
      >
        {shouldShowDescription() && (
          <Form.Item
            label="描述"
            name="description"
            rules={[
              { required: true, message: '请输入步骤描述' },
              { max: 500, message: '描述不能超过500个字符' },
            ]}
          >
            <TextArea
              rows={4}
              placeholder="输入步骤的详细描述..."
              showCount
              maxLength={500}
            />
          </Form.Item>
        )}

        <Space style={{ width: '100%' }} size="large">
          <Form.Item
            label="状态"
            name="status"
            rules={[{ required: true, message: '请选择状态' }]}
            style={{ flex: 1, minWidth: 200 }}
          >
            <Select options={statusOptions} placeholder="选择状态" />
          </Form.Item>

          {shouldShowPriority() && (
            <Form.Item
              label="优先级"
              name="priority"
              style={{ flex: 1, minWidth: 200 }}
            >
              <Select
                options={priorityOptions}
                placeholder="选择优先级"
                allowClear
              />
            </Form.Item>
          )}
        </Space>

        {shouldShowDependencies() && (
          <Form.Item
            label="依赖步骤"
            name="dependencies"
            tooltip="选择当前步骤依赖的前置步骤"
          >
            <Select
              mode="multiple"
              options={dependencyOptions}
              placeholder="选择依赖的步骤（可多选）"
              filterOption={(input, option) =>
                (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
        )}

        {mode === 'insert' && insertPosition !== undefined && (
          <Form.Item>
            <div style={{ color: '#8c8c8c', fontSize: '12px' }}>
              将在位置 {insertPosition} 插入新步骤
            </div>
          </Form.Item>
        )}
      </Form>
    </Modal>
  );
};

export default StepEditorModal;
