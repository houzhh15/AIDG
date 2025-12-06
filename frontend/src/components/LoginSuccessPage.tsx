/**
 * 登录成功回调页面
 * 处理 OIDC 登录后的 token 回调
 */
import React, { useEffect, useState } from 'react';
import { Result, Button, Spin, Card } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined, HomeOutlined } from '@ant-design/icons';
import { saveAuthFromToken, StoredAuth } from '../api/auth';

interface LoginSuccessPageProps {
  onLoginSuccess?: (auth: StoredAuth) => void;
  onNavigateHome?: () => void;
}

const LoginSuccessPage: React.FC<LoginSuccessPageProps> = ({
  onLoginSuccess,
  onNavigateHome,
}) => {
  const [status, setStatus] = useState<'processing' | 'success' | 'error'>('processing');
  const [errorMessage, setErrorMessage] = useState<string>('');

  useEffect(() => {
    const processCallback = () => {
      try {
        // 从 URL 参数获取 token
        const urlParams = new URLSearchParams(window.location.search);
        const token = urlParams.get('token');
        const error = urlParams.get('error');

        if (error) {
          setStatus('error');
          setErrorMessage(error);
          return;
        }

        if (!token) {
          setStatus('error');
          setErrorMessage('未收到有效的登录凭证');
          return;
        }

        // 保存认证信息
        const auth = saveAuthFromToken(token);

        // 清理 URL 参数，避免 token 泄露
        const cleanUrl = window.location.pathname;
        window.history.replaceState({}, document.title, cleanUrl);

        setStatus('success');

        // 延迟调用回调，让用户看到成功状态
        setTimeout(() => {
          onLoginSuccess?.(auth);
        }, 1000);

      } catch (err: any) {
        console.error('[LoginSuccess] Error processing callback:', err);
        setStatus('error');
        setErrorMessage(err.message || '处理登录信息时发生错误');
      }
    };

    processCallback();
  }, [onLoginSuccess]);

  // 处理中状态
  if (status === 'processing') {
    return (
      <Card style={{ maxWidth: 400, margin: '100px auto', textAlign: 'center' }}>
        <Spin size="large" />
        <p style={{ marginTop: 16, color: '#666' }}>正在处理登录信息...</p>
      </Card>
    );
  }

  // 成功状态
  if (status === 'success') {
    return (
      <Card style={{ maxWidth: 400, margin: '100px auto' }}>
        <Result
          status="success"
          icon={<CheckCircleOutlined style={{ color: '#52c41a' }} />}
          title="登录成功"
          subTitle="正在跳转到主页面..."
        />
      </Card>
    );
  }

  // 错误状态
  return (
    <Card style={{ maxWidth: 400, margin: '100px auto' }}>
      <Result
        status="error"
        icon={<CloseCircleOutlined style={{ color: '#ff4d4f' }} />}
        title="登录失败"
        subTitle={errorMessage}
        extra={
          <Button
            type="primary"
            icon={<HomeOutlined />}
            onClick={() => {
              // 清理 URL 并返回首页
              window.history.replaceState({}, document.title, '/');
              onNavigateHome?.();
              // 如果没有 onNavigateHome 回调，则刷新页面
              if (!onNavigateHome) {
                window.location.href = '/';
              }
            }}
          >
            返回登录页
          </Button>
        }
      />
    </Card>
  );
};

export default LoginSuccessPage;
