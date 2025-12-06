/**
 * LDAP 配置表单子组件
 */
import React from 'react';
import { Form, Input, Switch, Select, Checkbox, Divider, FormInstance, Space, Typography } from 'antd';
import { AVAILABLE_SCOPES } from '../../api/users';

const { Text } = Typography;

interface LDAPConfigFormProps {
  form: FormInstance;
  isEdit: boolean;
}

// 同步间隔选项
const SYNC_INTERVAL_OPTIONS = [
  { value: '1h', label: '每小时' },
  { value: '6h', label: '每6小时' },
  { value: '12h', label: '每12小时' },
  { value: '24h', label: '每天' },
];

// 冲突策略选项
const CONFLICT_POLICY_OPTIONS = [
  { value: 'override', label: '覆盖本地' },
  { value: 'ignore', label: '忽略变更' },
];

const LDAPConfigForm: React.FC<LDAPConfigFormProps> = ({ isEdit }) => {
  return (
    <>
      <Divider orientation="left">LDAP 配置</Divider>
      
      <Form.Item
        name={['config', 'server_url']}
        label="服务器地址"
        rules={[{ required: true, message: '请输入 LDAP 服务器地址' }]}
        tooltip="LDAP 服务器 URL，如 ldap://ldap.example.com:389"
      >
        <Input placeholder="ldap://ldap.example.com:389" />
      </Form.Item>

      <Form.Item
        name={['config', 'base_dn']}
        label="Base DN"
        rules={[{ required: true, message: '请输入 Base DN' }]}
        tooltip="LDAP 搜索的根节点"
      >
        <Input placeholder="dc=example,dc=com" />
      </Form.Item>

      <Form.Item
        name={['config', 'bind_dn']}
        label="Bind DN"
        rules={[{ required: true, message: '请输入 Bind DN' }]}
        tooltip="用于绑定 LDAP 的管理员 DN"
      >
        <Input placeholder="cn=admin,dc=example,dc=com" />
      </Form.Item>

      <Form.Item
        name={['config', 'bind_password']}
        label="Bind Password"
        rules={[{ required: !isEdit, message: '请输入 Bind Password' }]}
        tooltip={isEdit ? '留空表示不修改' : undefined}
      >
        <Input.Password 
          placeholder={isEdit ? '留空保持不变' : '输入 Bind Password'} 
        />
      </Form.Item>

      <Form.Item
        name={['config', 'user_filter']}
        label="用户过滤器"
        rules={[{ required: true, message: '请输入用户过滤器' }]}
        tooltip="LDAP 用户搜索过滤器，使用 %s 作为用户名占位符"
        extra={
          <Text type="secondary" style={{ fontSize: 12 }}>
            示例：(&(objectClass=person)(sAMAccountName=%s))
          </Text>
        }
      >
        <Input placeholder="(&(objectClass=person)(sAMAccountName=%s))" />
      </Form.Item>

      <Form.Item
        name={['config', 'group_filter']}
        label="用户组过滤器"
        tooltip="可选，用于同步用户组"
      >
        <Input placeholder="(&(objectClass=group)(member=%s))" />
      </Form.Item>

      <Divider orientation="left" orientationMargin={0}>
        <Text type="secondary">属性映射</Text>
      </Divider>

      <Space direction="horizontal" style={{ width: '100%' }} size="large">
        <Form.Item
          name={['config', 'username_attribute']}
          label="用户名属性"
          initialValue="sAMAccountName"
          style={{ marginBottom: 8 }}
        >
          <Input placeholder="sAMAccountName" style={{ width: 180 }} />
        </Form.Item>

        <Form.Item
          name={['config', 'email_attribute']}
          label="邮箱属性"
          initialValue="mail"
          style={{ marginBottom: 8 }}
        >
          <Input placeholder="mail" style={{ width: 180 }} />
        </Form.Item>

        <Form.Item
          name={['config', 'fullname_attribute']}
          label="姓名属性"
          initialValue="displayName"
          style={{ marginBottom: 8 }}
        >
          <Input placeholder="displayName" style={{ width: 180 }} />
        </Form.Item>
      </Space>

      <Divider orientation="left" orientationMargin={0}>
        <Text type="secondary">安全设置</Text>
      </Divider>

      <Space direction="horizontal" size="large">
        <Form.Item
          name={['config', 'use_tls']}
          label="使用 TLS"
          valuePropName="checked"
          initialValue={false}
        >
          <Switch />
        </Form.Item>

        <Form.Item
          name={['config', 'skip_verify']}
          label="跳过证书验证"
          valuePropName="checked"
          initialValue={false}
          tooltip="仅用于测试环境，生产环境请勿开启"
        >
          <Switch />
        </Form.Item>
      </Space>

      <Divider orientation="left" orientationMargin={0}>
        <Text type="secondary">用户创建</Text>
      </Divider>

      <Form.Item
        name={['config', 'auto_create_user']}
        label="自动创建用户"
        valuePropName="checked"
        initialValue={true}
        tooltip="首次登录时是否自动创建本地用户"
      >
        <Switch />
      </Form.Item>

      <Form.Item
        name={['config', 'default_scopes']}
        label="默认权限"
        tooltip="自动创建的用户默认拥有的权限"
      >
        <Checkbox.Group>
          {AVAILABLE_SCOPES.map(scope => (
            <Checkbox key={scope.value} value={scope.value}>
              {scope.label}
            </Checkbox>
          ))}
        </Checkbox.Group>
      </Form.Item>

      <Divider orientation="left">用户同步配置</Divider>

      <Form.Item
        name={['sync', 'sync_enabled']}
        label="启用同步"
        valuePropName="checked"
        initialValue={false}
        tooltip="启用后将定期从 LDAP 同步用户"
      >
        <Switch />
      </Form.Item>

      <Form.Item
        noStyle
        shouldUpdate={(prevValues, currentValues) => 
          prevValues?.sync?.sync_enabled !== currentValues?.sync?.sync_enabled
        }
      >
        {({ getFieldValue }) => 
          getFieldValue(['sync', 'sync_enabled']) && (
            <>
              <Form.Item
                name={['sync', 'sync_interval']}
                label="同步间隔"
                initialValue="24h"
              >
                <Select options={SYNC_INTERVAL_OPTIONS} style={{ width: 150 }} />
              </Form.Item>

              <Form.Item
                name={['sync', 'conflict_policy']}
                label="冲突策略"
                initialValue="override"
                tooltip="当本地用户信息与 LDAP 不一致时的处理方式"
              >
                <Select options={CONFLICT_POLICY_OPTIONS} style={{ width: 150 }} />
              </Form.Item>

              <Form.Item
                name={['sync', 'disable_on_remove']}
                label="删除时禁用"
                valuePropName="checked"
                initialValue={true}
                tooltip="当用户从 LDAP 中删除时，是否禁用本地账号"
              >
                <Switch />
              </Form.Item>
            </>
          )
        }
      </Form.Item>
    </>
  );
};

export default LDAPConfigForm;
