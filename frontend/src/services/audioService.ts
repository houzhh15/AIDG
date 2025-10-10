/**
 * 音频录制服务管理器
 * 用于管理浏览器录音和文件上传的状态和逻辑
 */

import { useCallback, useState } from 'react';
import { useMicrophonePermission } from '../hooks/useMicrophonePermission';
import { useMediaRecorder } from '../hooks/useMediaRecorder';
import { useAudioUpload } from '../hooks/useAudioUpload';
import { message } from 'antd';

export type AudioMode = 'browser_record' | 'file_upload' | null;

interface UseAudioServiceOptions {
  taskId: string;
  onSuccess?: () => void;
  onError?: (error: Error) => void;
}

interface UseAudioServiceReturn {
  mode: AudioMode;
  isRecording: boolean;
  duration: number;
  uploadedChunks: number;
  uploadedSize: number;
  uploadProgress: number;
  
  // 浏览器录音相关
  startBrowserRecording: () => Promise<void>;
  pauseRecording: () => void;
  resumeRecording: () => void;
  stopRecording: () => Promise<void>;
  
  // 文件上传相关
  triggerFileUpload: () => void;
  
  // 通用
  reset: () => void;
}

/**
 * 音频录制服务Hook
 * 统一管理浏览器录音和文件上传逻辑
 */
export function useAudioService({
  taskId,
  onSuccess,
  onError
}: UseAudioServiceOptions): UseAudioServiceReturn {
  const [mode, setMode] = useState<AudioMode>(null);
  const [uploadedChunks, setUploadedChunks] = useState(0);
  const [uploadedSize, setUploadedSize] = useState(0);

  // 麦克风权限管理
  const { permissionStatus, stream, requestPermission, error: permissionError } =
    useMicrophonePermission();

  // 音频上传管理
  const { uploadChunk, uploadFile, progress: uploadProgress } = useAudioUpload({
    taskId,
    onSuccess: () => {
      setUploadedChunks(prev => prev + 1);
      message.success('音频分片上传成功');
      onSuccess?.();
    },
    onError: (err) => {
      message.error(`上传失败: ${err.message}`);
      onError?.(err);
    }
  });

  // MediaRecorder录音管理
  const { status, startRecording, pauseRecording, resumeRecording, stopRecording, duration } =
    useMediaRecorder(stream, {
      chunkDuration: 5 * 60 * 1000, // 5分钟
      onChunk: async (blob, index) => {
        await uploadChunk(blob, index);
        setUploadedSize(prev => prev + blob.size);
      },
      onError
    });

  /**
   * 开始浏览器录音
   */
  const startBrowserRecording = useCallback(async () => {
    try {
      setMode('browser_record');
      
      // 请求麦克风权限并获取 stream
      let mediaStream = stream;
      if (permissionStatus !== 'granted' || !mediaStream) {
        mediaStream = await requestPermission();
      }

      // 直接将获取到的 stream 传递给 startRecording
      // 避免依赖 React 状态更新的异步性
      await startRecording(mediaStream);
      message.success('开始录音');
    } catch (err: any) {
      console.error('Failed to start browser recording:', err);
      message.error(err.message || '启动录音失败');
      setMode(null);
      throw err;
    }
  }, [stream, permissionStatus, requestPermission, startRecording]);

  /**
   * 停止录音
   */
  const stopRecordingHandler = useCallback(async () => {
    try {
      await stopRecording();
      message.success('录音已停止');
      // 重置状态
      setUploadedChunks(0);
      setUploadedSize(0);
      setMode(null);
    } catch (err: any) {
      console.error('Failed to stop recording:', err);
      message.error('停止录音失败');
    }
  }, [stopRecording]);

  /**
   * 触发文件上传
   */
  const triggerFileUpload = useCallback(() => {
    setMode('file_upload');
    
    // 创建隐藏的文件输入
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.wav,.mp3,.m4a,.flac,.ogg,.webm';
    input.multiple = true;
    
    input.onchange = async (e) => {
      const target = e.target as HTMLInputElement;
      const files = target.files;
      
      if (!files || files.length === 0) {
        setMode(null);
        return;
      }

      // 上传所有选中的文件
      for (let i = 0; i < files.length; i++) {
        const file = files[i];
        try {
          message.loading(`正在上传文件 ${i + 1}/${files.length}: ${file.name}`, 0);
          await uploadFile(file);
          message.destroy();
          message.success(`文件 ${file.name} 上传成功`);
        } catch (err: any) {
          message.destroy();
          message.error(`文件 ${file.name} 上传失败: ${err.message}`);
        }
      }
      
      setMode(null);
    };
    
    input.click();
  }, [uploadFile]);

  /**
   * 重置状态
   */
  const reset = useCallback(() => {
    setMode(null);
    setUploadedChunks(0);
    setUploadedSize(0);
  }, []);

  return {
    mode,
    isRecording: status === 'recording' || status === 'paused',
    duration,
    uploadedChunks,
    uploadedSize,
    uploadProgress,
    
    startBrowserRecording,
    pauseRecording,
    resumeRecording,
    stopRecording: stopRecordingHandler,
    
    triggerFileUpload,
    
    reset
  };
}
