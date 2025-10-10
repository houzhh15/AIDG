import React, { useState } from 'react';
import { Space, Button, Typography, Alert, Progress, message } from 'antd';
import { AudioOutlined, PauseOutlined, PlayCircleOutlined, StopOutlined } from '@ant-design/icons';
import { useMicrophonePermission } from '../hooks/useMicrophonePermission';
import { useMediaRecorder } from '../hooks/useMediaRecorder';
import { useAudioUpload } from '../hooks/useAudioUpload';
import { formatDuration, formatFileSize } from '../utils/audioUtils';

const { Text } = Typography;

interface AudioRecorderProps {
  taskId: string;
  onUploadSuccess?: () => void;
  onError?: (error: Error) => void;
}

/**
 * 浏览器录音组件
 * 提供麦克风录音、分片上传和状态显示功能
 */
export const AudioRecorder: React.FC<AudioRecorderProps> = ({
  taskId,
  onUploadSuccess,
  onError
}) => {
  const [uploadedChunks, setUploadedChunks] = useState(0);
  const [uploadedSize, setUploadedSize] = useState(0);

  // 权限管理
  const { permissionStatus, stream, requestPermission, error: permissionError, isRequesting } =
    useMicrophonePermission();

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
      chunkDuration: 5 * 60 * 1000, // 5分钟
      onChunk: async (blob, index) => {
        await uploadChunk(blob, index);
        setUploadedSize(prev => prev + blob.size);
      },
      onError
    });

  // 处理开始录音
  const handleStart = async () => {
    try {
      if (permissionStatus !== 'granted') {
        await requestPermission();
      }
      // 短暂延迟确保stream已经可用
      setTimeout(async () => {
        await startRecording();
      }, 100);
    } catch (err) {
      console.error('Failed to start recording:', err);
    }
  };

  // 处理停止录音
  const handleStop = async () => {
    await stopRecording();
    // 重置状态
    setUploadedChunks(0);
    setUploadedSize(0);
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
          <Button size="small" onClick={requestPermission}>
            重新请求权限
          </Button>
        }
      />
    );
  }

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      {/* 录音控制按钮 */}
      <Space>
        <Button
          type="primary"
          icon={<AudioOutlined />}
          onClick={handleStart}
          disabled={status !== 'idle'}
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
