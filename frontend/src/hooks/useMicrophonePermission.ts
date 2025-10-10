import { useState, useCallback, useRef, useEffect } from 'react';
import { PermissionStatus, AudioErrorCode } from '../types/audio';
import { createAudioError } from '../utils/audioUtils';

interface UseMicrophonePermissionReturn {
  permissionStatus: PermissionStatus;
  stream: MediaStream | null;
  requestPermission: () => Promise<MediaStream>; // 返回 MediaStream
  error: Error | null;
  isRequesting: boolean; // 新增：是否正在请求权限
}

/**
 * 麦克风权限管理 Hook
 * 处理麦克风权限请求、状态管理和错误处理
 */
export function useMicrophonePermission(): UseMicrophonePermissionReturn {
  const [permissionStatus, setPermissionStatus] = useState<PermissionStatus>('prompt');
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [isRequesting, setIsRequesting] = useState(false); // 新增：请求中状态
  const streamRef = useRef<MediaStream | null>(null);

  /**
   * 请求麦克风权限
   */
  const requestPermission = useCallback(async (): Promise<MediaStream> => {
    try {
      setError(null);
      setIsRequesting(true); // 开始请求

      // 检查浏览器是否支持getUserMedia
      if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
        throw createAudioError(
          '您的浏览器不支持录音功能，请使用Chrome 60+或Firefox 55+',
          AudioErrorCode.MEDIA_RECORDER_NOT_SUPPORTED
        );
      }

      // 请求麦克风权限
      const mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true, // 回声消除
          noiseSuppression: true, // 噪声抑制
          autoGainControl: true,  // 自动增益控制
        },
      });

      // 保存stream引用
      streamRef.current = mediaStream;
      setStream(mediaStream);
      setPermissionStatus('granted');
      setIsRequesting(false); // 请求成功
      
      // 返回 stream，让调用者可以立即使用
      return mediaStream;
    } catch (err: any) {
      console.error('Failed to get microphone permission:', err);
      setIsRequesting(false); // 请求结束

      // 处理用户拒绝权限
      if (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError') {
        const deniedError = createAudioError(
          '麦克风权限被拒绝，请在浏览器设置中允许访问',
          AudioErrorCode.PERMISSION_DENIED,
          err
        );
        setError(deniedError);
        setPermissionStatus('denied');
        throw deniedError;
      }

      // 处理设备不可用
      if (err.name === 'NotFoundError' || err.name === 'DevicesNotFoundError') {
        const notFoundError = createAudioError(
          '未检测到麦克风设备，请确保已连接麦克风',
          AudioErrorCode.AUDIO_CAPTURE_FAILED,
          err
        );
        setError(notFoundError);
        setPermissionStatus('denied');
        throw notFoundError;
      }

      // 处理设备被占用
      if (err.name === 'NotReadableError' || err.name === 'TrackStartError') {
        const busyError = createAudioError(
          '麦克风设备被占用，请关闭其他使用麦克风的应用',
          AudioErrorCode.AUDIO_CAPTURE_FAILED,
          err
        );
        setError(busyError);
        setPermissionStatus('denied');
        throw busyError;
      }

      // 处理安全限制（需要HTTPS）
      if (err.name === 'SecurityError') {
        const securityError = createAudioError(
          '安全限制：请使用HTTPS访问或在localhost测试',
          AudioErrorCode.PERMISSION_DENIED,
          err
        );
        setError(securityError);
        setPermissionStatus('denied');
        throw securityError;
      }

      // 处理超时
      if (err.name === 'TimeoutError') {
        const timeoutError = createAudioError(
          '请求权限超时，请重试',
          AudioErrorCode.PERMISSION_TIMEOUT,
          err
        );
        setError(timeoutError);
        setPermissionStatus('prompt');
        throw timeoutError;
      }

      // 其他错误
      const unknownError = createAudioError(
        `获取麦克风权限失败: ${err.message}`,
        AudioErrorCode.AUDIO_CAPTURE_FAILED,
        err
      );
      setError(unknownError);
      setPermissionStatus('denied');
      throw unknownError;
    }
  }, []);

  /**
   * 清理函数：停止所有音频轨道
   */
  useEffect(() => {
    return () => {
      if (streamRef.current) {
        streamRef.current.getTracks().forEach(track => track.stop());
        streamRef.current = null;
      }
    };
  }, []);

  return {
    permissionStatus,
    stream,
    requestPermission,
    error,
    isRequesting,
  };
}
