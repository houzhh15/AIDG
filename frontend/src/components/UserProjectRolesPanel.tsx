/**
 * 用户项目角色管理面板
 * 
 * 功能:
 * - 显示用户在所有项目中的角色（包括无角色的项目）
 * - 行内分配/更改角色
 * - 撤销角色
 * 
 * 使用:
 * ```tsx
 * <UserProjectRolesPanel username="admin" />
 * ```
 */

import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  message,
  Tag,
  Popconfirm,
  Spin,
  Alert,
} from 'antd';
import {
  UserOutlined,
} from '@ant-design/icons';
import { listProjects, type ProjectSummary } from '../api/projects';
import { getUserProfile, assignRole, removeUserRole, type UserProjectRoleInfo } from '../api/userRoles';
import { PermissionGuard } from './permission/PermissionGuard';
import { ScopeUserManage } from '../constants/permissions';
import { RoleSelector } from './RoleSelector';

interface UserProjectRolesPanelProps {
  /** 用户名 */
  username: string;
}

/**
 * 项目角色行数据
 */
interface ProjectRoleRow {
  project_id: string;
  project_name: string;
  role_id: string | null;
  role_name: string | null;
}

/**
 * 用户项目角色管理面板
 */
export const UserProjectRolesPanel: React.FC<UserProjectRolesPanelProps> = ({ username }) => {
  const [projectRoles, setProjectRoles] = useState<ProjectRoleRow[]>([]);
  const [loading, setLoading] = useState(false);

  // 加载用户在所有项目中的角色
  useEffect(() => {
    loadUserProjectRoles();
  }, [username]);

  const loadUserProjectRoles = async () => {
    try {
      setLoading(true);
      
      // 并行获取所有项目和用户profile
      const [projects, profileResponse] = await Promise.all([
        listProjects(),
        getUserProfile(username),
      ]);

      // 构建项目角色映射
      const roleMap = new Map<string, UserProjectRoleInfo>();
      if (profileResponse.success && profileResponse.data) {
        profileResponse.data.project_roles.forEach((role) => {
          roleMap.set(role.project_id, role);
        });
      }

      // 左连接：所有项目 + 用户角色（可能为null）
      const rows: ProjectRoleRow[] = projects.map((project) => {
        const role = roleMap.get(project.id);
        return {
          project_id: project.id,
          project_name: project.name || project.id,
          role_id: role?.role_id || null,
          role_name: role?.role_name || null,
        };
      });

      setProjectRoles(rows);
    } catch (error: any) {
      // 对 403 权限错误不显示提示，让页面显示空状态
      if (error?.response?.status !== 403) {
        message.error('加载用户角色失败: ' + error.message);
      }
      // 权限不足时设置空项目列表
      setProjectRoles([]);
    } finally {
      setLoading(false);
    }
  };

  const handleChangeRole = async (projectId: string, roleId: string | null) => {
    if (roleId) {
      // 分配角色
      const response = await assignRole({
        project_id: projectId,
        username,
        role_id: roleId,
      });

      if (response.success) {
        message.success('角色分配成功');
        loadUserProjectRoles();
      } else {
        message.error(response.message);
        throw new Error(response.message);
      }
    }
  };

  const handleRevokeRole = async (projectId: string, roleId: string) => {
    try {
      const response = await removeUserRole(projectId, username, roleId);
      if (response.success) {
        message.success('角色撤销成功');
        loadUserProjectRoles();
      } else {
        message.error(response.message);
      }
    } catch (error: any) {
      message.error('撤销角色失败: ' + error.message);
    }
  };

  const columns = [
    {
      title: '项目名称',
      dataIndex: 'project_name',
      key: 'project_name',
    },
    {
      title: '当前角色',
      dataIndex: 'role_name',
      key: 'role_name',
      render: (name: string | null) =>
        name ? <strong>{name}</strong> : <Tag color="default">无权限</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: ProjectRoleRow) => (
        <PermissionGuard requiredPermission={ScopeUserManage} showLoading={false}>
          <Space>
            <RoleSelector
              projectId={record.project_id}
              currentRoleId={record.role_id}
              onChange={(roleId) => handleChangeRole(record.project_id, roleId)}
            />
            {record.role_id && (
              <Popconfirm
                title="确认撤销该用户在此项目的角色?"
                onConfirm={() => handleRevokeRole(record.project_id, record.role_id!)}
                okText="确定"
                cancelText="取消"
              >
                <Button type="link" danger>
                  撤销
                </Button>
              </Popconfirm>
            )}
          </Space>
        </PermissionGuard>
      ),
    },
  ];

  return (
    <Card
      title={
        <Space>
          <UserOutlined />
          <span>用户: {username} - 项目角色</span>
        </Space>
      }
    >
      <Alert
        message="项目角色说明"
        description="为用户分配项目角色后,用户将在该项目中获得角色对应的权限。"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />

      {loading ? (
        <Spin />
      ) : (
        <Table
          dataSource={projectRoles}
          columns={columns}
          rowKey={(record) => record.project_id}
          pagination={false}
          locale={{
            emptyText: '系统中暂无项目',
          }}
        />
      )}
    </Card>
  );
};

export default UserProjectRolesPanel;
