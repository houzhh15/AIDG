/**
 * OIDC 配置表单子组件
 */
import React from 'react';
import { Form, Input, Switch, Select, Checkbox, Divider, FormInstance } from 'antd';
import { AVAILABLE_SCOPES } from '../../api/users';

interface OIDCConfigFormProps {
  form: FormInstance;
  isEdit: boolean;
}

const DEFAULT_SCOPES = ['openid', 'profile', 'email'];

const OIDCConfigForm: React.FC<OIDCConfigFormProps> = ({ isEdit }) => {
  return (
    <>
      <Divider orientation="left">OIDC 配置</Divider>
      
      <Form.Item
        name={['config', 'issuer_url']}
        label="Issuer URL"
        rules={[
          { required: true, message: '请输入 Issuer URL' },
          { type: 'url', message: '请输入有效的 URL 格式' }
        ]}
        tooltip="OIDC 身份提供商的发现端点 URL"
      >
        <Input placeholder="https://login.example.com" />
      </Form.Item>

      <Form.Item
        name={['config', 'client_id']}
        label="Client ID"
        rules={[{ required: true, message: '请输入 Client ID' }]}
      >
        <Input placeholder="your-client-id" />
      </Form.Item>

      <Form.Item
        name={['config', 'client_secret']}
        label="Client Secret"
        rules={[{ required: !isEdit, message: '请输入 Client Secret' }]}
        tooltip={isEdit ? '留空表示不修改' : undefined}
      >
        <Input.Password 
          placeholder={isEdit ? '留空保持不变' : '输入 Client Secret'} 
        />
      </Form.Item>

      <Form.Item
        name={['config', 'redirect_uri']}
        label="Redirect URI"
        rules={[{ required: true, message: '请输入 Redirect URI' }]}
        tooltip="OIDC 回调地址，请在身份提供商中配置此地址"
        initialValue={`${window.location.origin}/auth/callback`}
      >
        <Input placeholder={`${window.location.origin}/auth/callback`} />
      </Form.Item>

      <Form.Item
        name={['config', 'scopes']}
        label="Scopes"
        initialValue={DEFAULT_SCOPES}
        tooltip="请求的 OIDC 作用域"
      >
        <Select
          mode="tags"
          placeholder="输入或选择 Scopes"
          options={[
            { value: 'openid', label: 'openid' },
            { value: 'profile', label: 'profile' },
            { value: 'email', label: 'email' },
            { value: 'groups', label: 'groups' },
          ]}
        />
      </Form.Item>

      <Form.Item
        name={['config', 'username_claim']}
        label="用户名 Claim"
        initialValue="preferred_username"
        tooltip="用于提取用户名的 JWT claim 字段"
      >
        <Input placeholder="preferred_username" />
      </Form.Item>

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
    </>
  );
};

export default OIDCConfigForm;
