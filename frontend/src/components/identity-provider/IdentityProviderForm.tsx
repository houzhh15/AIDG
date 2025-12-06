/**
 * 身份源创建/编辑表单组件
 */
import React, { useState, useEffect } from 'react';
import {
  Form,
  Input,
  Button,
  Radio,
  Switch,
  InputNumber,
  Space,
  Card,
  message,
  Modal,
  Spin,
  Result,
} from 'antd';
import { ArrowLeftOutlined, SaveOutlined, ApiOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import {
  getIdentityProvider,
  createIdentityProvider,
  updateIdentityProvider,
  testConnection,
} from '../../api/identityProviders';
import { IdPType, IdPStatus, CreateIdPRequest, TestResult } from '../../types/identityProvider';
import OIDCConfigForm from './OIDCConfigForm';
import LDAPConfigForm from './LDAPConfigForm';
import { usePermission } from '../../hooks/usePermission';

interface IdentityProviderFormProps {
  idpId?: string;  // 编辑模式时传入
  onSuccess?: () => void;
  onCancel?: () => void;
}

const IdentityProviderForm: React.FC<IdentityProviderFormProps> = ({
  idpId,
  onSuccess,
  onCancel,
}) => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  const [testModalVisible, setTestModalVisible] = useState(false);

  const isEdit = !!idpId;
  const { hasPermission } = usePermission();
  const hasWritePermission = hasPermission('idp.write');

  // 加载现有数据（编辑模式）
  useEffect(() => {
    if (isEdit && idpId) {
      setLoading(true);
      getIdentityProvider(idpId)
        .then((res) => {
          if (res.success && res.data) {
            // 将数据填充到表单
            form.setFieldsValue({
              name: res.data.name,
              type: res.data.type,
              status: res.data.status === 'Enabled',
              priority: res.data.priority,
              config: res.data.config,
              sync: res.data.sync,
            });
          } else {
            message.error('加载身份源失败');
          }
        })
        .catch((err) => {
          console.error('[IdP] Failed to load identity provider:', err);
          message.error('加载身份源失败');
        })
        .finally(() => setLoading(false));
    }
  }, [idpId, isEdit, form]);

  // 测试连接
  const handleTestConnection = async () => {
    try {
      const values = await form.validateFields(['type', 'config']);
      
      setTesting(true);
      setTestModalVisible(true);
      setTestResult(null);

      // 编辑模式下传入 idp_id，让后端复用已保存的密码
      const res = await testConnection({
        id: isEdit ? idpId : undefined,
        type: values.type,
        config: values.config,
      });

      if (res.success && res.data) {
        setTestResult(res.data);
      } else {
        setTestResult({
          success: false,
          message: res.error || '测试失败',
        });
      }
    } catch (err: any) {
      if (err.errorFields) {
        message.warning('请先填写完整配置');
      } else {
        console.error('[IdP] Test connection error:', err);
        setTestResult({
          success: false,
          message: err.response?.data?.error || err.message || '测试失败',
        });
      }
    } finally {
      setTesting(false);
    }
  };

  // 提交表单
  const handleSubmit = async (values: any) => {
    setSubmitting(true);
    try {
      const data: CreateIdPRequest = {
        name: values.name,
        type: values.type,
        status: values.status ? 'Enabled' : 'Disabled',
        priority: values.priority || 0,
        config: values.config,
        sync: values.type === 'LDAP' ? values.sync : undefined,
      };

      // 编辑模式下，如果密码为空则删除该字段
      if (isEdit) {
        if (data.type === 'OIDC' && !(data.config as any).client_secret) {
          delete (data.config as any).client_secret;
        }
        if (data.type === 'LDAP' && !(data.config as any).bind_password) {
          delete (data.config as any).bind_password;
        }
      }

      let res;
      if (isEdit && idpId) {
        res = await updateIdentityProvider(idpId, data);
      } else {
        res = await createIdentityProvider(data);
      }

      if (res.success) {
        message.success(isEdit ? '身份源更新成功' : '身份源创建成功');
        onSuccess?.();
      } else {
        message.error(res.error || '操作失败');
      }
    } catch (err: any) {
      console.error('[IdP] Submit error:', err);
      message.error(err.response?.data?.error || err.message || '操作失败');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <Card>
        <div style={{ textAlign: 'center', padding: '50px 0' }}>
          <Spin size="large" />
        </div>
      </Card>
    );
  }

  return (
    <>
      <Card
        title={isEdit ? '编辑身份源' : '新建身份源'}
        extra={
          <Button icon={<ArrowLeftOutlined />} onClick={onCancel}>
            返回列表
          </Button>
        }
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{
            type: 'OIDC' as IdPType,
            status: true,
            priority: 0,
          }}
          disabled={!hasWritePermission}
        >
          <Form.Item
            name="name"
            label="名称"
            rules={[
              { required: true, message: '请输入身份源名称' },
              { max: 50, message: '名称不能超过50个字符' },
            ]}
          >
            <Input placeholder="如：Azure AD、公司 LDAP" />
          </Form.Item>

          <Form.Item
            name="type"
            label="类型"
            rules={[{ required: true, message: '请选择身份源类型' }]}
          >
            <Radio.Group disabled={isEdit}>
              <Radio.Button value="OIDC">OIDC</Radio.Button>
              <Radio.Button value="LDAP">LDAP</Radio.Button>
            </Radio.Group>
          </Form.Item>

          <Space size="large">
            <Form.Item
              name="status"
              label="启用状态"
              valuePropName="checked"
            >
              <Switch checkedChildren="启用" unCheckedChildren="禁用" />
            </Form.Item>

            <Form.Item
              name="priority"
              label="优先级"
              tooltip="数字越小优先级越高，在登录页按优先级排序显示"
            >
              <InputNumber min={0} max={100} />
            </Form.Item>
          </Space>

          {/* 根据类型动态渲染配置表单 */}
          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) =>
              prevValues?.type !== currentValues?.type
            }
          >
            {({ getFieldValue }) => {
              const type = getFieldValue('type') as IdPType;
              return type === 'OIDC' ? (
                <OIDCConfigForm form={form} isEdit={isEdit} />
              ) : (
                <LDAPConfigForm form={form} isEdit={isEdit} />
              );
            }}
          </Form.Item>

          <Form.Item style={{ marginTop: 24 }}>
            <Space>
              <Button
                type="primary"
                htmlType="submit"
                icon={<SaveOutlined />}
                loading={submitting}
                disabled={!hasWritePermission}
              >
                {isEdit ? '保存修改' : '创建身份源'}
              </Button>
              <Button
                icon={<ApiOutlined />}
                onClick={handleTestConnection}
                loading={testing}
                disabled={!hasWritePermission}
              >
                测试连接
              </Button>
              <Button onClick={onCancel}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>

      {/* 测试结果弹窗 */}
      <Modal
        title="连接测试结果"
        open={testModalVisible}
        onCancel={() => setTestModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setTestModalVisible(false)}>
            关闭
          </Button>,
        ]}
      >
        {testing ? (
          <div style={{ textAlign: 'center', padding: '30px 0' }}>
            <Spin size="large" />
            <p style={{ marginTop: 16 }}>正在测试连接...</p>
          </div>
        ) : testResult ? (
          <Result
            status={testResult.success ? 'success' : 'error'}
            icon={testResult.success ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
            title={testResult.success ? '连接成功' : '连接失败'}
            subTitle={testResult.message}
          />
        ) : null}
      </Modal>
    </>
  );
};

export default IdentityProviderForm;
