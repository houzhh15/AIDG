/**
 * ContextManagerDropdown - 上下文管理下拉组件
 * 在项目 Header 中提供 MCP 资源快捷访问入口
 */

import React, { useState } from 'react';
import {
  Dropdown,
  Button,
  Modal,
  message,
  Space,
  Menu
} from 'antd';
import type { MenuProps } from 'antd';
import {
  DatabaseOutlined,
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  DownOutlined
} from '@ant-design/icons';
import { getUserResources, deleteResource } from '../../api/resourceApi';
import { Resource } from '../../api/resourceApi';
import ResourceEditorModal from '../resources/ResourceEditorModal';

/**
 * ContextManagerDropdown 组件属性接口
 */
interface ContextManagerDropdownProps {
  /** 当前用户名 */
  username: string;
  
  /** 当前项目ID（用于过滤资源） */
  projectId?: string;
  
  /** 当前任务ID（可选） */
  taskId?: string;
}

/**
 * Modal 状态接口
 */
interface ModalState {
  visible: boolean;
  mode: 'create' | 'edit';
  resource?: Resource;
}

/**
 * ContextManagerDropdown 组件
 * 提供 MCP 资源的快捷管理入口
 */
const ContextManagerDropdown: React.FC<ContextManagerDropdownProps> = ({
  username,
  projectId,
  taskId
}) => {
  // 资源列表状态
  const [resources, setResources] = useState<Resource[]>([]);
  
  // 加载中状态
  const [loading, setLoading] = useState<boolean>(false);
  
  // 下拉菜单展开状态
  const [menuOpen, setMenuOpen] = useState<boolean>(false);

  // 编辑器 Modal 状态
  const [modalState, setModalState] = useState<ModalState>({
    visible: false,
    mode: 'create',
    resource: undefined
  });

  // 悬停状态（用于显示编辑/删除按钮）
  const [hoveredResourceId, setHoveredResourceId] = useState<string | null>(null);

  /**
   * 加载资源列表
   */
  const loadResources = async () => {
    if (!projectId) {
      return; // 无项目上下文时不加载
    }

    setLoading(true);
    try {
      const data = await getUserResources(username, {
        projectId
      });
      setResources(data);
    } catch (error: any) {
      console.error('加载 MCP 资源列表失败:', error);
      message.error('加载资源列表失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  /**
   * 下拉菜单展开/收起回调
   */
  const handleMenuOpenChange = (open: boolean) => {
    setMenuOpen(open);
    if (open) {
      loadResources();
    }
  };

  /**
   * 点击新增资源
   */
  const handleCreateClick = () => {
    setModalState({
      visible: true,
      mode: 'create',
      resource: undefined
    });
    setMenuOpen(false); // 关闭下拉菜单
  };

  /**
   * 点击编辑资源
   */
  const handleEditClick = (resource: Resource) => {
    setModalState({
      visible: true,
      mode: 'edit',
      resource
    });
    setMenuOpen(false); // 关闭下拉菜单
  };

  /**
   * 点击删除资源
   */
  const handleDeleteClick = (resource: Resource) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除资源 "${resource.name}" (${resource.resourceId}) 吗？此操作不可恢复。`,
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        setLoading(true);
        try {
          await deleteResource(username, resource.resourceId);
          message.success('资源已删除');
          loadResources(); // 刷新列表
        } catch (error: any) {
          console.error('删除 MCP 资源失败:', error);
          message.error('删除失败，请稍后重试');
        } finally {
          setLoading(false);
        }
      }
    });
  };

  /**
   * Modal 关闭回调
   */
  const handleModalClose = () => {
    setModalState({
      ...modalState,
      visible: false
    });
  };

  /**
   * Modal 保存成功回调
   */
  const handleModalSuccess = () => {
    handleModalClose();
    loadResources(); // 刷新资源列表
  };

  /**
   * 构建下拉菜单项
   */
  const menuItems: MenuProps['items'] = [
    // 新增资源按钮
    {
      key: 'create',
      label: (
        <Button
          type="text"
          icon={<PlusOutlined />}
          onClick={handleCreateClick}
          style={{ width: '100%', textAlign: 'left', minHeight: 40 }}
        >
          新增资源
        </Button>
      )
    },
    { type: 'divider' },
    // 资源列表项
    ...(resources.length > 0
      ? resources.map(resource => ({
          key: resource.resourceId,
          label: (
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                width: '100%',
                minHeight: 40
              }}
              onMouseEnter={() => setHoveredResourceId(resource.resourceId)}
              onMouseLeave={() => setHoveredResourceId(null)}
            >
              <span style={{ flex: 1 }}>{resource.name}</span>
              <Space size="small">
                <Button
                  type="text"
                  size="small"
                  icon={<EditOutlined />}
                  style={{
                    opacity: hoveredResourceId === resource.resourceId ? 1 : 0,
                    transition: 'opacity 0.2s ease',
                    pointerEvents: hoveredResourceId === resource.resourceId ? 'auto' : 'none'
                  }}
                  onClick={(e) => {
                    e.stopPropagation();
                    handleEditClick(resource);
                  }}
                />
                <Button
                  type="text"
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  style={{
                    opacity: hoveredResourceId === resource.resourceId ? 1 : 0,
                    transition: 'opacity 0.2s ease',
                    pointerEvents: hoveredResourceId === resource.resourceId ? 'auto' : 'none'
                  }}
                  onClick={(e) => {
                    e.stopPropagation();
                    handleDeleteClick(resource);
                  }}
                />
              </Space>
            </div>
          )
        }))
      : [
          {
            key: 'empty',
            label: <span style={{ color: '#999', minHeight: 40, display: 'flex', alignItems: 'center' }}>暂无资源</span>,
            disabled: true
          }
        ])
  ];

  /**
   * 判断按钮是否禁用
   */
  const isDisabled = !projectId;

  return (
    <>
      <Dropdown
        menu={{ 
          items: menuItems,
          style: { minWidth: 200 } // 增加下拉菜单宽度
        }}
        open={menuOpen}
        onOpenChange={handleMenuOpenChange}
        placement="bottomRight"
        disabled={isDisabled}
        trigger={['click']}
      >
        <Button
          icon={<DatabaseOutlined />}
          disabled={isDisabled}
          loading={loading}
        >
          上下文管理 (MCP Resources) <DownOutlined />
        </Button>
      </Dropdown>

      {/* 资源编辑器 Modal */}
      {modalState.visible && (
        <ResourceEditorModal
          mode={modalState.mode}
          visible={modalState.visible}
          initialResource={modalState.resource}
          username={username}
          projectId={projectId}
          onClose={handleModalClose}
          onSuccess={handleModalSuccess}
        />
      )}
    </>
  );
};

export default ContextManagerDropdown;
