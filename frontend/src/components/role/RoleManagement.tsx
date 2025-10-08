/**
 * 角色管理页面
 * 
 * 功能:
 * - 项目选择器 (左侧)
 * - 角色列表表格 (右侧)
 * - 创建/编辑/删除角色
 * - 权限详情展示
 */

import React, { useState, useEffect } from 'react';
import { 
  Layout, 
  Tree, 
  Table, 
  Button, 
  Space, 
  message, 
  Popconfirm, 
  Tag, 
  Card,
  Empty,
  Spin,
} from 'antd';
import { 
  PlusOutlined, 
  EditOutlined, 
  DeleteOutlined, 
  FolderOpenOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { listProjects, type ProjectSummary } from '../../api/projects';
import { getRoles, createRole, updateRole, deleteRole, type Role, type CreateRoleRequest, type UpdateRoleRequest } from '../../api/roles';
import { RoleFormModal } from './RoleFormModal';
import { getScopeLabel } from '../../constants/permissions';
import { PermissionGuard } from '../permission/PermissionGuard';
import { ScopeUserManage } from '../../constants/permissions';

const { Sider, Content } = Layout;

interface ProjectNode {
  id: string;
  name: string;
}

/**
 * 角色管理页面
 */
export const RoleManagement: React.FC = () => {
  // 项目列表
  const [projects, setProjects] = useState<ProjectNode[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');
  const [loadingProjects, setLoadingProjects] = useState(false);

  // 角色列表
  const [roles, setRoles] = useState<Role[]>([]);
  const [loadingRoles, setLoadingRoles] = useState(false);

  // 模态框状态
  const [modalVisible, setModalVisible] = useState(false);
  const [modalMode, setModalMode] = useState<'create' | 'edit'>('create');
  const [editingRole, setEditingRole] = useState<Role | undefined>();
  const [confirmLoading, setConfirmLoading] = useState(false);

  // 加载项目列表
  useEffect(() => {
    loadProjects();
  }, []);

  // 加载角色列表 (当项目切换时)
  useEffect(() => {
    if (selectedProjectId) {
      loadRoles(selectedProjectId);
    } else {
      setRoles([]);
    }
  }, [selectedProjectId]);

  // 加载项目列表
  const loadProjects = async () => {
    try {
      setLoadingProjects(true);
      const projectList = await listProjects();
      
      const nodes: ProjectNode[] = projectList.map(project => ({
        id: project.id,
        name: project.name || project.id,
      }));

      setProjects(nodes);
      
      // 默认选择第一个项目
      if (nodes.length > 0) {
        setSelectedProjectId(nodes[0].id);
      }
    } catch (error: any) {
      // 如果是 403 权限错误，不显示错误提示，让页面显示空状态
      if (error?.response?.status !== 403) {
        message.error('加载项目列表失败: ' + error.message);
      }
      // 权限不足时设置空项目列表
      setProjects([]);
    } finally {
      setLoadingProjects(false);
    }
  };

  // 加载角色列表
  const loadRoles = async (projectId: string) => {
    try {
      setLoadingRoles(true);
      const response = await getRoles(projectId);
      
      console.log('[RoleManagement] API response:', response);
      
      if (response.success && response.data) {
        // 处理嵌套的 data 结构
        const roles = Array.isArray(response.data) ? response.data : ((response.data as any).data || []);
        console.log('[RoleManagement] Extracted roles:', roles);
        
        if (Array.isArray(roles)) {
          roles.forEach((role, index) => {
            console.log(`[RoleManagement] Role ${index}:`, {
              role_id: role.role_id,
              name: role.name,
              scopes: role.scopes,
              scopesType: typeof role.scopes,
              isArray: Array.isArray(role.scopes),
            });
          });
          setRoles(roles);
        } else {
          console.error('[RoleManagement] Roles is not an array:', typeof roles, roles);
          setRoles([]);
        }
      } else {
        message.error(response.message);
        setRoles([]);
      }
    } catch (error: any) {
      message.error('加载角色列表失败: ' + error.message);
      setRoles([]);
    } finally {
      setLoadingRoles(false);
    }
  };

  // 打开创建模态框
  const handleCreate = () => {
    if (!selectedProjectId) {
      message.warning('请先选择项目');
      return;
    }
    setModalMode('create');
    setEditingRole(undefined);
    setModalVisible(true);
  };

  // 打开编辑模态框
  const handleEdit = (role: Role) => {
    setModalMode('edit');
    setEditingRole(role);
    setModalVisible(true);
  };

  // 提交表单
  const handleModalOk = async (data: CreateRoleRequest | UpdateRoleRequest) => {
    try {
      setConfirmLoading(true);
      
      if (modalMode === 'create') {
        const response = await createRole(data as CreateRoleRequest);
        if (response.success) {
          message.success('创建角色成功');
          setModalVisible(false);
          loadRoles(selectedProjectId);
        } else {
          throw new Error(response.message);
        }
      } else if (editingRole) {
        const response = await updateRole(
          editingRole.project_id,
          editingRole.role_id,
          data as UpdateRoleRequest
        );
        if (response.success) {
          message.success('更新角色成功');
          setModalVisible(false);
          loadRoles(selectedProjectId);
        } else {
          throw new Error(response.message);
        }
      }
    } catch (error: any) {
      message.error(error.message || '操作失败');
      throw error; // 让模态框显示错误
    } finally {
      setConfirmLoading(false);
    }
  };

  // 删除角色
  const handleDelete = async (role: Role) => {
    try {
      const response = await deleteRole(role.project_id, role.role_id);
      if (response.success) {
        message.success('删除角色成功');
        loadRoles(selectedProjectId);
      } else {
        message.error(response.message);
      }
    } catch (error: any) {
      message.error('删除角色失败: ' + error.message);
    }
  };

  // 项目树数据
  const treeData: DataNode[] = projects.map(project => ({
    key: project.id,
    title: project.name,
    icon: <FolderOpenOutlined />,
  }));

  // 表格列定义
  const columns = [
    {
      title: '角色名称',
      dataIndex: 'name',
      key: 'name',
      width: 150,
      render: (name: string) => (
        <Space>
          <SafetyOutlined />
          <strong>{name}</strong>
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '权限',
      dataIndex: 'scopes',
      key: 'scopes',
      render: (scopes: string[]) => (
        <Space wrap>
          {Array.isArray(scopes) && scopes.map(scope => (
            <Tag key={scope} color="blue">
              {getScopeLabel(scope) || scope}
            </Tag>
          ))}
        </Space>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (time: string) => {
        if (!time) return '-';
        try {
          const date = new Date(time);
          if (isNaN(date.getTime())) return '-';
          return date.toLocaleString('zh-CN');
        } catch {
          return '-';
        }
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_: any, record: Role) => (
        <Space>
          <PermissionGuard requiredPermission={ScopeUserManage} showLoading={false}>
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record)}
            >
              编辑
            </Button>
          </PermissionGuard>
          
          <PermissionGuard requiredPermission={ScopeUserManage} showLoading={false}>
            <Popconfirm
              title="确定删除此角色?"
              description="删除后该角色的所有用户映射也将被移除"
              onConfirm={() => handleDelete(record)}
              okText="确定"
              cancelText="取消"
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
              >
                删除
              </Button>
            </Popconfirm>
          </PermissionGuard>
        </Space>
      ),
    },
  ];

  return (
    <Layout style={{ height: 'calc(100vh - 120px)', background: '#fff' }}>
      {/* 左侧项目树 */}
      <Sider width={250} theme="light" style={{ borderRight: '1px solid #f0f0f0' }}>
        <div style={{ padding: '16px' }}>
          <h3>项目列表</h3>
        </div>
        <Spin spinning={loadingProjects}>
          {projects.length === 0 && !loadingProjects ? (
            <div style={{ padding: '16px', textAlign: 'center', color: '#999' }}>
              <Empty 
                description="暂无项目" 
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            </div>
          ) : (
            <Tree
              showLine
              showIcon
              defaultExpandAll
              selectedKeys={[selectedProjectId]}
              treeData={treeData}
              onSelect={(selectedKeys) => {
                if (selectedKeys.length > 0) {
                  setSelectedProjectId(selectedKeys[0] as string);
                }
              }}
            />
          )}
        </Spin>
      </Sider>

      {/* 右侧角色列表 */}
      <Content style={{ padding: '16px' }}>
        <Card
          title={
            <Space>
              <SafetyOutlined />
              <span>角色管理</span>
              {selectedProjectId && <Tag color="green">{selectedProjectId}</Tag>}
            </Space>
          }
          extra={
            <PermissionGuard requiredPermission={ScopeUserManage} showLoading={false}>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={handleCreate}
                disabled={!selectedProjectId}
              >
                创建角色
              </Button>
            </PermissionGuard>
          }
        >
          {!selectedProjectId ? (
            <Empty 
              description={projects.length === 0 ? "暂无可用项目，请先创建项目" : "请先在左侧选择项目"} 
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          ) : (
            <Table
              dataSource={roles}
              columns={columns}
              rowKey="id"
              loading={loadingRoles}
              pagination={{
                pageSize: 10,
                showTotal: (total) => `共 ${total} 个角色`,
              }}
            />
          )}
        </Card>
      </Content>

      {/* 角色表单模态框 */}
      <RoleFormModal
        visible={modalVisible}
        projectId={selectedProjectId}
        mode={modalMode}
        initialValues={editingRole}
        onOk={handleModalOk}
        onCancel={() => setModalVisible(false)}
        confirmLoading={confirmLoading}
      />
    </Layout>
  );
};

export default RoleManagement;
