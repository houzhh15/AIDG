/**
 * 用户个人中心页面
 * 
 * 功能:
 * - 基本信息展示
 * - 项目角色列表
 * - 默认权限展示 (任务负责人/会议创建者)
 * - 修改密码
 */

import React, { useState, useEffect } from 'react';
import {
  Card,
  Descriptions,
  Table,
  Tag,
  Button,
  Modal,
  Form,
  Input,
  message,
  Space,
  Spin,
  Typography,
  Alert,
} from 'antd';
import {
  UserOutlined,
  LockOutlined,
  SafetyOutlined,
  KeyOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import { getUserProfile, changePassword, type UserProfileData } from '../api/permissions';
import { getScopeLabel } from '../constants/permissions';

const { Title, Text } = Typography;

/**
 * 用户个人中心页面
 */
export const UserProfile: React.FC = () => {
  const [profile, setProfile] = useState<UserProfileData | null>(null);
  const [loading, setLoading] = useState(false);
  const [passwordModalVisible, setPasswordModalVisible] = useState(false);
  const [passwordForm] = Form.useForm();
  const [changingPassword, setChangingPassword] = useState(false);

  // 加载用户档案
  useEffect(() => {
    loadProfile();
  }, []);

  const loadProfile = async () => {
    try {
      setLoading(true);
      const data = await getUserProfile();
      setProfile(data);
    } catch (error: any) {
      message.error('加载用户档案失败: ' + error.message);
    } finally {
      setLoading(false);
    }
  };

  // 修改密码
  const handleChangePassword = async () => {
    try {
      const values = await passwordForm.validateFields();
      setChangingPassword(true);

      await changePassword({
        old_password: values.oldPassword,
        new_password: values.newPassword,
      });

      message.success('密码修改成功,请重新登录');
      setPasswordModalVisible(false);
      passwordForm.resetFields();

      // 3秒后跳转到登录页
      setTimeout(() => {
        window.location.href = '/';
      }, 3000);
    } catch (error: any) {
      if (error.errorFields) {
        // 表单验证错误
        return;
      }
      message.error(error.message || '密码修改失败');
    } finally {
      setChangingPassword(false);
    }
  };

  // 项目角色表格列
  const roleColumns = [
    {
      title: '项目',
      dataIndex: 'project_name',
      key: 'project_name',
      render: (name: string, record: any) => name || record.project_id,
    },
    {
      title: '角色',
      dataIndex: 'role_name',
      key: 'role_name',
      render: (name: string) => (
        <Space>
          <SafetyOutlined />
          <strong>{name}</strong>
        </Space>
      ),
    },
    {
      title: '权限',
      dataIndex: 'scopes',
      key: 'scopes',
      render: (scopes: string[]) => (
        <Space wrap>
          {scopes?.map((scope) => (
            <Tag key={scope} color="blue">
              {getScopeLabel(scope) || scope}
            </Tag>
          ))}
        </Space>
      ),
    },
    {
      title: '分配时间',
      dataIndex: 'assigned_at',
      key: 'assigned_at',
      render: (time: string) => new Date(time).toLocaleString('zh-CN'),
    },
  ];

  // 默认权限表格列
  const defaultPermissionColumns = [
    {
      title: '权限类型',
      dataIndex: 'reason',
      key: 'reason',
      render: (reason: string) => <strong>{reason}</strong>,
    },
    {
      title: '权限范围',
      dataIndex: 'scopes',
      key: 'scopes',
      render: (scopes: string[]) => (
        <Space wrap>
          {scopes?.map((scope) => (
            <Tag key={scope} color="green" icon={<CheckCircleOutlined />}>
              {getScopeLabel(scope) || scope}
            </Tag>
          ))}
        </Space>
      ),
    },
  ];

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '50px' }}>
        <Spin size="large" tip="加载中..." />
      </div>
    );
  }

  if (!profile) {
    return (
      <div style={{ padding: '20px' }}>
        <Alert message="无法加载用户档案" type="error" showIcon />
      </div>
    );
  }

  // 合并所有权限
  const allScopes = [
    ...profile.roles.flatMap((role) => role.scopes || []),
    ...profile.default_permissions.flatMap((perm) => perm.scopes || []),
  ];
  const uniqueScopes = Array.from(new Set(allScopes));

  return (
    <div style={{ padding: '24px', maxWidth: '1200px', margin: '0 auto' }}>
      <Title level={2}>
        <UserOutlined /> 个人中心
      </Title>

      {/* 基本信息 */}
      <Card
        title={
          <Space>
            <UserOutlined />
            <span>基本信息</span>
          </Space>
        }
        style={{ marginBottom: 16 }}
        extra={
          <Button
            type="primary"
            icon={<LockOutlined />}
            onClick={() => setPasswordModalVisible(true)}
          >
            修改密码
          </Button>
        }
      >
        <Descriptions column={2} bordered>
          <Descriptions.Item label="用户名">{profile.username}</Descriptions.Item>
          <Descriptions.Item label="总权限数">
            <Tag color="cyan">{uniqueScopes.length} 个</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="项目角色数">
            {profile.roles.length} 个
          </Descriptions.Item>
          <Descriptions.Item label="默认权限数">
            {profile.default_permissions.length} 个
          </Descriptions.Item>
        </Descriptions>
      </Card>

      {/* 项目角色 */}
      <Card
        title={
          <Space>
            <SafetyOutlined />
            <span>项目角色</span>
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        {profile.roles.length === 0 ? (
          <Alert message="暂无项目角色" type="info" showIcon />
        ) : (
          <Table
            dataSource={profile.roles}
            columns={roleColumns}
            rowKey={(record) => `${record.project_id}-${record.role_id}`}
            pagination={false}
          />
        )}
      </Card>

      {/* 默认权限 */}
      <Card
        title={
          <Space>
            <KeyOutlined />
            <span>默认权限</span>
          </Space>
        }
      >
        <Alert
          message="默认权限说明"
          description="作为任务负责人或会议创建者时,您将自动获得相应的操作权限,无需额外分配角色。"
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
        {profile.default_permissions.length === 0 ? (
          <Alert message="暂无默认权限" type="info" showIcon />
        ) : (
          <Table
            dataSource={profile.default_permissions}
            columns={defaultPermissionColumns}
            rowKey="reason"
            pagination={false}
          />
        )}
      </Card>

      {/* 修改密码模态框 */}
      <Modal
        title={
          <Space>
            <LockOutlined />
            <span>修改密码</span>
          </Space>
        }
        open={passwordModalVisible}
        onOk={handleChangePassword}
        onCancel={() => {
          setPasswordModalVisible(false);
          passwordForm.resetFields();
        }}
        confirmLoading={changingPassword}
        okText="确定"
        cancelText="取消"
      >
        <Form form={passwordForm} layout="vertical" preserve={false}>
          <Form.Item
            label="当前密码"
            name="oldPassword"
            rules={[{ required: true, message: '请输入当前密码' }]}
          >
            <Input.Password placeholder="请输入当前密码" prefix={<LockOutlined />} />
          </Form.Item>

          <Form.Item
            label="新密码"
            name="newPassword"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 6, message: '密码至少 6 个字符' },
            ]}
          >
            <Input.Password placeholder="请输入新密码" prefix={<KeyOutlined />} />
          </Form.Item>

          <Form.Item
            label="确认新密码"
            name="confirmPassword"
            dependencies={['newPassword']}
            rules={[
              { required: true, message: '请确认新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('newPassword') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次输入的密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password placeholder="请再次输入新密码" prefix={<KeyOutlined />} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default UserProfile;
