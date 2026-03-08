/**
 * MCP Resources 组件类型定义
 * 用于统一资源编辑器和相关组件的类型接口
 */

import { Resource } from '../../api/resourceApi';

/**
 * 资源编辑模式
 */
export type ResourceEditorMode = 'create' | 'edit';

/**
 * 资源编辑器属性接口
 */
export interface ResourceEditorProps {
  /** 编辑模式：create（新增）或 edit（编辑） */
  mode: ResourceEditorMode;
  
  /** 初始值（编辑模式时传入现有资源数据） */
  initialValue?: Partial<Resource>;
  
  /** 提交回调函数（保存时调用） */
  onSubmit: (payload: ResourcePayload) => Promise<void>;
  
  /** 取消回调函数 */
  onCancel: () => void;
}

/**
 * 资源提交数据载体
 * 包含创建或更新资源所需的所有字段
 */
export interface ResourcePayload {
  /** 资源名称 */
  name: string;
  
  /** 资源描述 */
  description: string;
  
  /** 资源内容（Markdown 格式） */
  content: string;
  
  /** 资源可见性 */
  visibility: 'public' | 'private';
  
  /** 所属项目ID（可选） */
  projectId?: string;
  
  /** 所属任务ID（可选） */
  taskId?: string;
}

/**
 * 资源编辑器内部表单状态
 */
export interface ResourceFormData {
  name: string;
  description: string;
  visibility: 'public' | 'private';
  projectId?: string;
  taskId?: string;
}
