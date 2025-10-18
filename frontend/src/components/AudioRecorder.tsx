import React, { useState, useEffect } from 'react';
import { Space, Button, Typography, Alert, Progress, message, Select, Spin } from 'antd';
import { AudioOutlined, PauseOutlined, PlayCircleOutlined, StopOutlined, ReloadOutlined } from '@ant-design/icons';
import { useMicrophonePermission } from '../hooks/useMicrophonePermission';
import { useMediaRecorder } from '../hooks/useMediaRecorder';
import { useAudioUpload } from '../hooks/useAudioUpload';
import { useAudioDevices } from '../hooks/useAudioDevices';
import { formatDuration, formatFileSize } from '../utils/audioUtils';

const { Text } = Typography;

interface AudioRecorderProps {
  taskId: string;
  selectedDeviceId?: string;  // 可选：预选的音频设备ID
  showDeviceSelector?: boolean;  // 是否显示设备选择器（默认true）
  onUploadSuccess?: () => void;
  onError?: (error: Error) => void;
  onDeviceChange?: (deviceId: string) => void;  // 设备切换回调
}

/**
 * 浏览器录音组件
 * 提供麦克风录音、分片上传和状态显示功能
 * 支持音频设备选择
 */
export const AudioRecorder: React.FC<AudioRecorderProps> = ({
  taskId,
  selectedDeviceId,
  showDeviceSelector = true,
  onUploadSuccess,
  onError,
  onDeviceChange
}) => {
  const [uploadedChunks, setUploadedChunks] = useState(0);
  const [uploadedSize, setUploadedSize] = useState(0);
  const [currentDeviceId, setCurrentDeviceId] = useState<string | undefined>(selectedDeviceId);

  // 设备枚举
  const { devices, loading: devicesLoading, error: devicesError, refreshDevices, hasPermission } = useAudioDevices();

  // 权限管理
  const { permissionStatus, stream, requestPermission, error: permissionError, isRequesting } =
    useMicrophonePermission({ deviceId: currentDeviceId });

  // 上传管理
  const { uploadChunk, progress: uploadProgress } = useAudioUpload({
    taskId,
    onSuccess: () => {
      setUploadedChunks(prev => prev + 1);
      message.success('音频分片上传成功');
      onUploadSuccess?.();
    },
    onError: (err) => {
      message.error(`上传失败: ${err.message}`);
      onError?.(err);
    }
  });

  // 录音管理
  const { status, startRecording, pauseRecording, resumeRecording, stopRecording, duration } =
    useMediaRecorder(stream, {
      taskId, // 传递 taskId 用于初始化 chunk index
      chunkDuration: 5 * 60 * 1000, // 5分钟
      onChunk: async (blob, index) => {
        await uploadChunk(blob, index);
        setUploadedSize(prev => prev + blob.size);
      },
      onError
    });

  // 设备变化时刷新设备列表
  useEffect(() => {
    if (permissionStatus === 'granted' && !hasPermission) {
      // 获得权限后刷新设备列表以获取真实标签
      refreshDevices();
    }
  }, [permissionStatus, hasPermission, refreshDevices]);

  // 处理设备选择变化
  const handleDeviceChange = (deviceId: string) => {
    setCurrentDeviceId(deviceId);
    onDeviceChange?.(deviceId);
    
    // 如果正在录音，提示需要重新开始
    if (status !== 'idle') {
      message.warning('切换设备需要重新开始录音');
    }
  };

  // 处理开始录音
  const handleStart = async () => {
    try {
      if (permissionStatus !== 'granted' || !stream) {
        const newStream = await requestPermission(currentDeviceId);
        // 等待 stream 状态更新
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      await startRecording();
      message.success('开始录音');
    } catch (err) {
      console.error('Failed to start recording:', err);
      const error = err as Error;
      message.error(error.message || '启动录音失败');
    }
  };

  // 处理停止录音
  const handleStop = async () => {
    await stopRecording();
    // 重置状态
    setUploadedChunks(0);
    setUploadedSize(0);
  };

  // 处理重新请求权限
  const handleRequestPermission = async () => {
    try {
      await requestPermission(currentDeviceId);
      message.success('已获得麦克风权限');
    } catch (err) {
      console.error('Failed to request permission:', err);
    }
  };

  // 权限被拒绝的提示
  if (permissionStatus === 'denied') {
    return (
      <Alert
        message="需要麦克风权限"
        description="此功能需要访问您的麦克风进行录音。请在浏览器设置中允许访问，然后点击下方按钮重新请求。"
        type="error"
        showIcon
        action={
          <Button size="small" onClick={handleRequestPermission}>
            重新请求权限
          </Button>
        }
      />
    );
  }

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      {/* 设备选择器 */}
      {showDeviceSelector && (
        <Space style={{ width: '100%' }}>
          <Text>音频输入设备：</Text>
          <Select
            style={{ flex: 1, minWidth: 200 }}
            value={currentDeviceId}
            onChange={handleDeviceChange}
            disabled={status !== 'idle' || devicesLoading}
            loading={devicesLoading}
            placeholder="选择音频设备"
            notFoundContent={devicesLoading ? <Spin size="small" /> : '未找到音频设备'}
            options={devices.map(device => ({
              label: device.label,
              value: device.deviceId,
            }))}
          />
          <Button
            icon={<ReloadOutlined />}
            onClick={refreshDevices}
            disabled={devicesLoading || status !== 'idle'}
            title="刷新设备列表"
          />
        </Space>
      )}

      {/* 设备错误提示 */}
      {devicesError && (
        <Alert
          message="设备枚举失败"
          description={devicesError.message}
          type="warning"
          showIcon
          closable
        />
      )}

      {/* 设备选择提示 */}
      {!devicesLoading && devices.length === 0 && (
        <Alert
          message="未检测到音频设备"
          description="请确保已连接麦克风或其他音频输入设备"
          type="info"
          showIcon
        />
      )}

      {/* 录音控制按钮 */}
      <Space>
        <Button
          type="primary"
          icon={<AudioOutlined />}
          onClick={handleStart}
          disabled={status !== 'idle' || devices.length === 0 || !currentDeviceId}
          loading={isRequesting}
        >
          {isRequesting ? '请求权限中...' : '开始录音'}
        </Button>

        <Button
          icon={status === 'paused' ? <PlayCircleOutlined /> : <PauseOutlined />}
          onClick={status === 'paused' ? resumeRecording : pauseRecording}
          disabled={status !== 'recording' && status !== 'paused'}
        >
          {status === 'paused' ? '恢复' : '暂停'}
        </Button>

        <Button
          danger
          icon={<StopOutlined />}
          onClick={handleStop}
          disabled={status === 'idle'}
        >
          停止录音
        </Button>
      </Space>

      {/* 录音状态显示 */}
      {status !== 'idle' && (
        <Space direction="vertical" style={{ width: '100%' }}>
          <Text>
            ⏱️ 录音时长: <Text strong>{formatDuration(duration)}</Text>
          </Text>
          <Text>
            💾 已上传: <Text strong>{uploadedChunks}</Text> 个分片 (
            <Text strong>{formatFileSize(uploadedSize)}</Text>)
          </Text>
        </Space>
      )}

      {/* 上传进度 */}
      {uploadProgress > 0 && uploadProgress < 100 && (
        <div>
          <Text type="secondary">上传中...</Text>
          <Progress percent={uploadProgress} status="active" />
        </div>
      )}

      {/* 错误提示 */}
      {permissionError && (
        <Alert
          message="麦克风访问失败"
          description={permissionError.message}
          type="warning"
          showIcon
          closable
        />
      )}
    </Space>
  );
};
