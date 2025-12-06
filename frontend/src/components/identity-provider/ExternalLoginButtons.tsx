/**
 * 外部登录按钮组件
 * 在登录页显示外部身份源登录选项
 */
import React, { useState, useEffect, useMemo } from 'react';
import {
  Button,
  Divider,
  Form,
  Input,
  Space,
  Spin,
  message,
  Typography,
} from 'antd';
import {
  LoginOutlined,
  SafetyOutlined,
  ClusterOutlined,
  UserOutlined,
  LockOutlined,
} from '@ant-design/icons';
import { getPublicIdentityProviders } from '../../api/identityProviders';
import { loginWithIdP, StoredAuth } from '../../api/auth';
import { PublicIdentityProvider, IdPType } from '../../types/identityProvider';

const { Text } = Typography;

interface ExternalLoginButtonsProps {
  onLoginSuccess?: (auth: StoredAuth) => void;
}

const ExternalLoginButtons: React.FC<ExternalLoginButtonsProps> = ({
  onLoginSuccess,
}) => {
  const [publicIdps, setPublicIdps] = useState<PublicIdentityProvider[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedLDAPIdp, setSelectedLDAPIdp] = useState<PublicIdentityProvider | null>(null);
  const [ldapLoading, setLdapLoading] = useState(false);
  const [form] = Form.useForm();

  // 加载公开身份源列表
  useEffect(() => {
    const load = async () => {
      try {
        const res = await getPublicIdentityProviders();
        if (res.success && res.data) {
          setPublicIdps(res.data);
        }
      } catch (err) {
        console.log('[ExternalLogin] Failed to load public identity providers:', err);
        // 静默处理错误，不影响正常登录
      } finally {
        setLoading(false);
      }
    };
    load();
  }, []);

  // 按优先级排序
  const sortedIdps = useMemo(() => {
    return [...publicIdps].sort((a, b) => a.priority - b.priority);
  }, [publicIdps]);

  // 处理 OIDC 登录（跳转）
  const handleOIDCLogin = (idp: PublicIdentityProvider) => {
    // 跳转到后端 OIDC 登录入口
    window.location.href = `/auth/oidc/${idp.id}/login`;
  };

  // 处理 LDAP 登录点击
  const handleLDAPClick = (idp: PublicIdentityProvider) => {
    if (selectedLDAPIdp?.id === idp.id) {
      // 点击同一个则收起
      setSelectedLDAPIdp(null);
      form.resetFields();
    } else {
      setSelectedLDAPIdp(idp);
      form.resetFields();
    }
  };

  // 处理 LDAP 登录提交
  const handleLDAPSubmit = async (values: { username: string; password: string }) => {
    console.log('[ExternalLogin] LDAP submit triggered', { values, selectedLDAPIdp });
    if (!selectedLDAPIdp) {
      console.error('[ExternalLogin] No LDAP provider selected');
      return;
    }

    setLdapLoading(true);
    try {
      console.log('[ExternalLogin] Calling loginWithIdP...', {
        username: values.username,
        idpId: selectedLDAPIdp.id,
      });
      const auth = await loginWithIdP(values.username, values.password, selectedLDAPIdp.id);
      console.log('[ExternalLogin] Login successful', auth);
      message.success('登录成功');
      onLoginSuccess?.(auth);
    } catch (err: any) {
      console.error('[ExternalLogin] LDAP login error:', err);
      console.error('[ExternalLogin] Error response:', err.response?.data);
      const errorMsg = err.response?.data?.error || err.message || '登录失败';
      message.error(errorMsg);
    } finally {
      setLdapLoading(false);
    }
  };

  // 获取身份源图标
  const getIdPIcon = (type: IdPType) => {
    return type === 'OIDC' ? <SafetyOutlined /> : <ClusterOutlined />;
  };

  // 如果没有外部身份源或正在加载，不显示任何内容
  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '12px 0' }}>
        <Spin size="small" />
      </div>
    );
  }

  if (publicIdps.length === 0) {
    return null;
  }

  return (
    <div style={{ marginTop: 24 }}>
      <Divider plain>
        <Text type="secondary">其他登录方式</Text>
      </Divider>

      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        {sortedIdps.map((idp) => (
          <div key={idp.id}>
            <Button
              block
              icon={getIdPIcon(idp.type)}
              onClick={() => {
                if (idp.type === 'OIDC') {
                  handleOIDCLogin(idp);
                } else {
                  handleLDAPClick(idp);
                }
              }}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                height: 40,
              }}
            >
              <LoginOutlined style={{ marginLeft: 8 }} />
              <span style={{ marginLeft: 8 }}>使用 {idp.name} 登录</span>
            </Button>

            {/* LDAP 内联登录表单 */}
            {idp.type === 'LDAP' && selectedLDAPIdp?.id === idp.id && (
              <div
                style={{
                  marginTop: 12,
                  padding: 16,
                  background: '#fafafa',
                  borderRadius: 8,
                  border: '1px solid #d9d9d9',
                }}
              >
                <Form 
                  form={form} 
                  onFinish={handleLDAPSubmit} 
                  onFinishFailed={(errorInfo) => {
                    console.log('[ExternalLogin] Form validation failed:', errorInfo);
                  }}
                  layout="vertical"
                >
                  <Form.Item
                    name="username"
                    rules={[{ required: true, message: '请输入用户名' }]}
                  >
                    <Input
                      prefix={<UserOutlined />}
                      placeholder={`${idp.name} 用户名`}
                      autoComplete="username"
                    />
                  </Form.Item>
                  <Form.Item
                    name="password"
                    rules={[{ required: true, message: '请输入密码' }]}
                  >
                    <Input.Password
                      prefix={<LockOutlined />}
                      placeholder="密码"
                      autoComplete="current-password"
                    />
                  </Form.Item>
                  <Form.Item style={{ marginBottom: 0 }}>
                    <Button
                      type="primary"
                      htmlType="submit"
                      block
                      loading={ldapLoading}
                    >
                      登录
                    </Button>
                  </Form.Item>
                </Form>
              </div>
            )}
          </div>
        ))}
      </Space>
    </div>
  );
};

export default ExternalLoginButtons;
