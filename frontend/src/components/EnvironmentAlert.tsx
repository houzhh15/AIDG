import React, { useEffect, useState } from 'react';
import { Alert, Button, Space, Typography, Spin } from 'antd';
import { ReloadOutlined, FileTextOutlined } from '@ant-design/icons';

const { Title, Text } = Typography;

interface EnvironmentStatus {
  ready: boolean;
  issues: string[];
  warnings: string[];
  details: {
    huggingface_token: {
      configured: boolean;
      masked?: string;
    };
    pyannote_model: {
      exists: boolean;
      path: string;
      size?: string;
    };
    whisper_service: {
      reachable: boolean;
      url: string;
      latency?: string;
      error?: string;
    };
    ffmpeg: {
      available: boolean;
      version?: string;
      error?: string;
    };
  };
}

export const EnvironmentAlert: React.FC = () => {
  const [status, setStatus] = useState<EnvironmentStatus | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const checkEnvironment = async (force = false) => {
    setChecking(true);
    setError(null);
    try {
      const url = `/api/v1/environment/status${force ? '?force=true' : ''}`;
      const res = await fetch(url);
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: ${res.statusText}`);
      }
      const data = await res.json();
      setStatus(data);
    } catch (err) {
      console.error('Environment check failed:', err);
      setError(err instanceof Error ? err.message : '环境检查失败');
    } finally {
      setChecking(false);
    }
  };

  useEffect(() => {
    checkEnvironment();
    // 每 5 分钟自动重新检查
    const interval = setInterval(() => checkEnvironment(), 5 * 60 * 1000);
    return () => clearInterval(interval);
  }, []);

  if (checking && !status) {
    return (
      <div style={{ marginBottom: 16 }}>
        <Spin tip="正在检查环境..." />
      </div>
    );
  }

  if (error) {
    return (
      <Alert
        type="warning"
        message="环境检查失败"
        description={error}
        style={{ marginBottom: 16 }}
        action={
          <Button
            size="small"
            onClick={() => checkEnvironment(true)}
            loading={checking}
            icon={<ReloadOutlined />}
          >
            重试
          </Button>
        }
      />
    );
  }

  if (!status || status.ready) {
    return null;
  }

  return (
    <Alert
      type="error"
      message="音频处理环境未就绪"
      style={{ marginBottom: 16 }}
      description={
        <div>
          <Title level={5} style={{ marginTop: 8 }}>问题：</Title>
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            {status.issues.map((issue, i) => (
              <li key={i}>
                <Text>{issue}</Text>
              </li>
            ))}
          </ul>
          {status.warnings.length > 0 && (
            <>
              <Title level={5} style={{ marginTop: 8 }}>警告：</Title>
              <ul style={{ margin: 0, paddingLeft: 20 }}>
                {status.warnings.map((warning, i) => (
                  <li key={i}>
                    <Text type="warning">{warning}</Text>
                  </li>
                ))}
              </ul>
            </>
          )}
          <Space style={{ marginTop: 16 }}>
            <Button
              size="small"
              onClick={() => checkEnvironment(true)}
              loading={checking}
              icon={<ReloadOutlined />}
            >
              重新检查
            </Button>
            <Button
              size="small"
              href="/docs/deployment.md"
              target="_blank"
              icon={<FileTextOutlined />}
            >
              查看配置指南
            </Button>
          </Space>
        </div>
      }
    />
  );
};

export default EnvironmentAlert;
