/**
 * ResourceEditorModal - 资源编辑器对话框
 * 以 Modal 包裹 ResourceEditor，提供弹窗形式的资源编辑体验
 */

import React, { useState } from 'react';
import { Modal } from 'antd';
import ResourceEditor from './ResourceEditor';
import {
  Resource,
  addCustomResource,
  updateResource
} from '../../api/resourceApi';
import { ResourceEditorMode, ResourcePayload } from './types';
import { useTaskRefresh } from '../../contexts/TaskRefreshContext';

/**
 * ResourceEditorModal 组件属性接口
 */
export interface ResourceEditorModalProps {
  /** 编辑模式 */
  mode: ResourceEditorMode;
  
  /** 对话框可见性 */
  visible: boolean;
  
  /** 初始资源数据（编辑模式时传入） */
  initialResource?: Resource;
  
  /** 当前用户名（用于 API 调用） */
  username: string;
  
  /** 当前项目ID（可选，用于自动关联） */
  projectId?: string;
  
  /** 关闭回调 */
  onClose: () => void;
  
  /** 保存成功回调 */
  onSuccess: () => void;
}

/**
 * ResourceEditorModal 组件
 * 在对话框中渲染资源编辑器
 */
const ResourceEditorModal: React.FC<ResourceEditorModalProps> = ({
  mode,
  visible,
  initialResource,
  username,
  projectId,
  onClose,
  onSuccess
}) => {
  // 跟踪编辑器内容是否被修改（用于关闭前提示）
  const [isDirty, setIsDirty] = useState<boolean>(false);
  
  // 获取刷新函数
  const { triggerRefreshFor } = useTaskRefresh();

  /**
   * 动态生成对话框标题
   */
  const getTitle = () => {
    if (mode === 'create') {
      return '新增资源';
    }
    return `编辑资源 - ${initialResource?.name || '未命名'}`;
  };

  /**
   * 处理提交（API 调用逻辑）
   */
  const handleSubmit = async (payload: ResourcePayload) => {
    try {
      if (mode === 'create') {
        // 新增资源
        await addCustomResource(username, payload);
      } else if (mode === 'edit' && initialResource) {
        // 编辑资源
        await updateResource(username, initialResource.resourceId, payload);
      }
      
      // 触发用户资源刷新
      triggerRefreshFor('user-resource');
      
      // 注意：不在这里调用 onSuccess()
      // ResourceEditor 会在底部保存按钮保存后调用 onCancel()，由 handleCancel 处理刷新
    } catch (error: any) {
      // 抛出错误让 ResourceEditor 的 handleSave 捕获并显示
      console.error('ResourceEditorModal submit failed:', error);
      throw error;
    }
  };

  /**
   * 处理取消/关闭
   * 关闭窗口并刷新列表
   */
  const handleCancel = () => {
    // 关闭窗口并刷新列表
    onSuccess();
  };

  return (
    <Modal
      title={getTitle()}
      open={visible}
      onCancel={handleCancel}
      width="min(80vw, 1200px)"
      footer={null}
      destroyOnClose
      centered
      style={{ top: 20 }}
      bodyStyle={{
        padding: '24px',
        maxHeight: 'calc(100vh - 200px)',
        overflowY: 'auto'
      }}
    >
      <ResourceEditor
        mode={mode}
        initialValue={
          initialResource
            ? {
                name: initialResource.name,
                description: initialResource.description,
                content: initialResource.content,
                visibility: initialResource.visibility,
                projectId: initialResource.projectId || projectId,
                taskId: initialResource.taskId
              }
            : {
                visibility: 'private',
                projectId: projectId // 新增时自动关联当前项目
              }
        }
        onSubmit={handleSubmit}
        onCancel={handleCancel}
      />
    </Modal>
  );
};

export default ResourceEditorModal;
