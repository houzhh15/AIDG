import { useState, useEffect, useCallback } from 'react';
import { createAudioError } from '../utils/audioUtils';
import { AudioErrorCode } from '../types/audio';

/**
 * 音频设备信息接口
 */
export interface AudioDevice {
  deviceId: string;
  label: string;
  kind: 'audioinput' | 'audiooutput' | 'videoinput';
  groupId: string;
}

interface UseAudioDevicesReturn {
  devices: AudioDevice[];
  loading: boolean;
  error: Error | null;
  refreshDevices: () => Promise<void>;
  hasPermission: boolean;
}

/**
 * 音频设备枚举 Hook
 * 用于获取系统中可用的音频输入设备列表
 */
export function useAudioDevices(): UseAudioDevicesReturn {
  const [devices, setDevices] = useState<AudioDevice[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [hasPermission, setHasPermission] = useState(false);

  /**
   * 枚举音频设备
   */
  const enumerateDevices = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      // 检查浏览器是否支持 enumerateDevices
      if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) {
        throw createAudioError(
          '您的浏览器不支持设备枚举功能',
          AudioErrorCode.MEDIA_RECORDER_NOT_SUPPORTED
        );
      }

      // 枚举所有媒体设备
      const allDevices = await navigator.mediaDevices.enumerateDevices();

      // 筛选音频输入设备
      const audioInputDevices = allDevices
        .filter(device => device.kind === 'audioinput')
        .map(device => ({
          deviceId: device.deviceId,
          label: device.label || `麦克风 ${device.deviceId.slice(0, 8)}`,
          kind: device.kind as 'audioinput',
          groupId: device.groupId,
        }));

      setDevices(audioInputDevices);
      
      // 检查是否已经获得权限
      // 如果设备有 label，说明已经有权限
      const hasLabels = audioInputDevices.some(d => d.label && !d.label.startsWith('麦克风'));
      setHasPermission(hasLabels);

      setLoading(false);
    } catch (err: any) {
      console.error('Failed to enumerate devices:', err);
      const deviceError = createAudioError(
        `获取设备列表失败: ${err.message}`,
        AudioErrorCode.AUDIO_CAPTURE_FAILED,
        err
      );
      setError(deviceError);
      setLoading(false);
    }
  }, []);

  /**
   * 刷新设备列表
   * 在获得权限后调用此方法可以获取设备的真实标签
   */
  const refreshDevices = useCallback(async () => {
    await enumerateDevices();
  }, [enumerateDevices]);

  /**
   * 初始化和监听设备变化
   */
  useEffect(() => {
    // 初始枚举
    enumerateDevices();

    // 监听设备变化（插拔设备）
    const handleDeviceChange = () => {
      console.log('[useAudioDevices] Device change detected');
      enumerateDevices();
    };

    if (navigator.mediaDevices) {
      navigator.mediaDevices.addEventListener('devicechange', handleDeviceChange);

      return () => {
        navigator.mediaDevices.removeEventListener('devicechange', handleDeviceChange);
      };
    }
  }, [enumerateDevices]);

  return {
    devices,
    loading,
    error,
    refreshDevices,
    hasPermission,
  };
}
