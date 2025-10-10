import { useState, useEffect, useRef, useCallback } from 'react';
import { RecordingStatus, AudioErrorCode } from '../types/audio';
import { createAudioError } from '../utils/audioUtils';

interface UseMediaRecorderOptions {
  mimeType?: string;
  audioBitsPerSecond?: number;
  chunkDuration?: number;
  onChunk?: (blob: Blob, index: number) => Promise<void>;
  onError?: (error: Error) => void;
}

interface UseMediaRecorderReturn {
  status: RecordingStatus;
  startRecording: (overrideStream?: MediaStream) => Promise<void>;
  pauseRecording: () => void;
  resumeRecording: () => void;
  stopRecording: () => Promise<void>;
  duration: number;
  error: Error | null;
}

/**
 * MediaRecorder 录音 Hook
 * 基于 MediaRecorder API 实现录音控制和自动分片上传
 */
export function useMediaRecorder(
  stream: MediaStream | null,
  options: UseMediaRecorderOptions = {}
): UseMediaRecorderReturn {
  const {
    mimeType = 'audio/webm;codecs=opus',
    audioBitsPerSecond = 128000,
    chunkDuration = 5 * 60 * 1000, // 5分钟
    onChunk,
    onError
  } = options;

  const [status, setStatus] = useState<RecordingStatus>('idle');
  const [duration, setDuration] = useState(0);
  const [error, setError] = useState<Error | null>(null);

  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunkIndexRef = useRef(0);
  const startTimeRef = useRef<number | null>(null);
  const timerRef = useRef<NodeJS.Timeout | null>(null);
  const pausedTimeRef = useRef(0); // 累计暂停时长

  /**
   * 初始化 MediaRecorder
   */
  const initRecorder = useCallback((overrideStream?: MediaStream) => {
    const audioStream = overrideStream || stream;
    
    if (!audioStream) {
      // 不在这里显示错误消息，因为stream可能还未初始化
      // 实际的错误处理会在startRecording中进行
      return false;
    }

    try {
      // 检查浏览器是否支持MediaRecorder
      if (!window.MediaRecorder) {
        throw createAudioError(
          '您的浏览器不支持录音功能',
          AudioErrorCode.MEDIA_RECORDER_NOT_SUPPORTED
        );
      }

      // 检查是否支持指定的MIME类型
      if (!MediaRecorder.isTypeSupported(mimeType)) {
        console.warn(`MIME type ${mimeType} not supported, using default`);
        // 尝试使用默认MIME类型
        const recorder = new MediaRecorder(audioStream);
        recorderRef.current = recorder;
      } else {
        const recorder = new MediaRecorder(audioStream, {
          mimeType,
          audioBitsPerSecond
        });
        recorderRef.current = recorder;
      }

      // 设置事件处理器
      recorderRef.current.ondataavailable = async (event) => {
        console.log('[MediaRecorder] Data available', {
          size: event.data.size,
          type: event.data.type,
          chunkIndex: chunkIndexRef.current
        });
        
        if (event.data.size > 0 && onChunk) {
          try {
            await onChunk(event.data, chunkIndexRef.current);
            chunkIndexRef.current++;
          } catch (err) {
            console.error('Failed to upload chunk:', err);
            const uploadError = err as Error;
            setError(uploadError);
            setStatus('error');
            onError?.(uploadError);
          }
        } else {
          console.warn('[MediaRecorder] Empty data received or no onChunk handler');
        }
      };

      recorderRef.current.onerror = (event: Event) => {
        const errorEvent = event as ErrorEvent;
        const recorderError = createAudioError(
          `录音错误: ${errorEvent.message || '未知错误'}`,
          AudioErrorCode.AUDIO_CAPTURE_FAILED,
          errorEvent
        );
        setError(recorderError);
        setStatus('error');
        onError?.(recorderError);
      };

      recorderRef.current.onstop = () => {
        console.log('MediaRecorder stopped');
      };

      return true; // 初始化成功

    } catch (err: any) {
      const initError = createAudioError(
        `初始化录音失败: ${err.message}`,
        AudioErrorCode.MEDIA_RECORDER_START_FAILED,
        err
      );
      setError(initError);
      onError?.(initError);
      return false; // 初始化失败
    }
  }, [stream, mimeType, audioBitsPerSecond, onChunk, onError]);

  /**
   * 开始录音
   */
  const startRecording = useCallback(async (overrideStream?: MediaStream) => {
    try {
      setError(null);
      
      const audioStream = overrideStream || stream;
      
      // 检查 stream 是否可用
      if (!audioStream) {
        throw createAudioError(
          '音频流未初始化，请先授予麦克风权限',
          AudioErrorCode.AUDIO_CAPTURE_FAILED
        );
      }
      
      if (!recorderRef.current) {
        const initialized = initRecorder(audioStream);
        if (!initialized) {
          throw createAudioError(
            '录音器初始化失败',
            AudioErrorCode.MEDIA_RECORDER_START_FAILED
          );
        }
      }

      if (!recorderRef.current) {
        throw createAudioError(
          '录音器初始化失败',
          AudioErrorCode.MEDIA_RECORDER_START_FAILED
        );
      }

      recorderRef.current.start(chunkDuration);
      setStatus('recording');
      startTimeRef.current = Date.now();
      chunkIndexRef.current = 0;
      pausedTimeRef.current = 0;

      // 添加调试日志
      console.log('[MediaRecorder] Recording started', {
        state: recorderRef.current.state,
        mimeType: recorderRef.current.mimeType,
        audioBitsPerSecond: recorderRef.current.audioBitsPerSecond,
        chunkDuration,
        streamActive: audioStream.active,
        audioTracks: audioStream.getAudioTracks().map(track => ({
          id: track.id,
          label: track.label,
          enabled: track.enabled,
          muted: track.muted,
          readyState: track.readyState
        }))
      });

      // 启动时长计时器（每秒更新一次）
      timerRef.current = setInterval(() => {
        if (startTimeRef.current) {
          setDuration(Date.now() - startTimeRef.current - pausedTimeRef.current);
        }
      }, 1000);
    } catch (err: any) {
      const startError = createAudioError(
        `开始录音失败: ${err.message}`,
        AudioErrorCode.MEDIA_RECORDER_START_FAILED,
        err
      );
      setError(startError);
      setStatus('error');
      onError?.(startError);
    }
  }, [stream, initRecorder, chunkDuration, onError]);

  /**
   * 暂停录音
   */
  const pauseRecording = useCallback(() => {
    if (recorderRef.current && recorderRef.current.state === 'recording') {
      recorderRef.current.pause();
      setStatus('paused');
      
      // 记录暂停开始时间
      if (startTimeRef.current) {
        pausedTimeRef.current = Date.now() - startTimeRef.current;
      }
    }
  }, []);

  /**
   * 恢复录音
   */
  const resumeRecording = useCallback(() => {
    if (recorderRef.current && recorderRef.current.state === 'paused') {
      recorderRef.current.resume();
      setStatus('recording');
      
      // 更新开始时间（减去暂停时长）
      if (startTimeRef.current) {
        const pausedDuration = Date.now() - startTimeRef.current - pausedTimeRef.current;
        startTimeRef.current = Date.now() - duration;
      }
    }
  }, [duration]);

  /**
   * 停止录音
   */
  const stopRecording = useCallback(async () => {
    if (recorderRef.current && recorderRef.current.state !== 'inactive') {
      console.log('[MediaRecorder] Stopping recording', {
        state: recorderRef.current.state,
        duration: Date.now() - (startTimeRef.current || 0)
      });
      
      recorderRef.current.stop();
      setStatus('idle');
      setDuration(0);

      // 清除计时器
      if (timerRef.current) {
        clearInterval(timerRef.current);
        timerRef.current = null;
      }

      // 重置状态
      startTimeRef.current = null;
      pausedTimeRef.current = 0;
    }
  }, []);

  /**
   * 清理函数
   */
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
      if (recorderRef.current && recorderRef.current.state !== 'inactive') {
        recorderRef.current.stop();
      }
    };
  }, []);

  return {
    status,
    startRecording,
    pauseRecording,
    resumeRecording,
    stopRecording,
    duration,
    error
  };
}
