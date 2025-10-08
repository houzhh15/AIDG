/**
 * RoleSelector - 行内角色选择器组件
 * 
 * 用于在用户项目角色表格中选择/更改角色
 */

import React, { useState, useEffect } from 'react';
import { Select, Button, message } from 'antd';
import { PlusOutlined, EditOutlined } from '@ant-design/icons';
import { getRoles, type Role } from '../api/roles';

interface RoleSelectorProps {
  /** 项目ID */
  projectId: string;
  /** 当前角色ID（null表示无角色） */
  currentRoleId: string | null;
  /** 角色变更回调 */
  onChange: (roleId: string | null) => Promise<void>;
}

/**
 * 行内角色选择器
 */
export const RoleSelector: React.FC<RoleSelectorProps> = ({
  projectId,
  currentRoleId,
  onChange,
}) => {
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(false);
  const [selecting, setSelecting] = useState(false);
  const [selectedRoleId, setSelectedRoleId] = useState<string | null>(currentRoleId);
  const [changing, setChanging] = useState(false);

  // 加载项目角色
  useEffect(() => {
    loadRoles();
  }, [projectId]);

  const loadRoles = async () => {
    try {
      setLoading(true);
      const response = await getRoles(projectId);
      if (response.success && response.data) {
        setRoles(response.data);
      } else {
        message.error(response.message);
        setRoles([]);
      }
    } catch (error: any) {
      message.error('加载角色列表失败: ' + error.message);
      setRoles([]);
    } finally {
      setLoading(false);
    }
  };

  const handleChange = async (roleId: string) => {
    try {
      setChanging(true);
      await onChange(roleId);
      setSelectedRoleId(roleId);
      setSelecting(false);
    } catch (error: any) {
      message.error('角色分配失败: ' + error.message);
    } finally {
      setChanging(false);
    }
  };

  // 如果正在显示选择器
  if (selecting) {
    return (
      <Select
        style={{ width: 200 }}
        placeholder="请选择角色"
        loading={loading || changing}
        value={selectedRoleId}
        onChange={handleChange}
        onBlur={() => setSelecting(false)}
        autoFocus
        showSearch
        optionFilterProp="children"
        options={roles.map((role) => ({
          label: `${role.name}${role.description ? ` - ${role.description}` : ''}`,
          value: role.role_id || role.id || '',
        }))}
      />
    );
  }

  // 如果当前无角色，显示"指派角色"按钮
  if (!currentRoleId) {
    return (
      <Button
        type="link"
        icon={<PlusOutlined />}
        onClick={() => setSelecting(true)}
        disabled={roles.length === 0}
      >
        指派角色
      </Button>
    );
  }

  // 如果已有角色，显示"更改"按钮
  return (
    <Button
      type="link"
      icon={<EditOutlined />}
      onClick={() => setSelecting(true)}
      disabled={roles.length === 0}
    >
      更改
    </Button>
  );
};

export default RoleSelector;
