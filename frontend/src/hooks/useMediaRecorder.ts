import { useState, useEffect, useRef, useCallback } from 'react';
import { RecordingStatus, AudioErrorCode } from '../types/audio';
import { createAudioError } from '../utils/audioUtils';
import { listChunks } from '../api/client';

interface UseMediaRecorderOptions {
  mimeType?: string;
  audioBitsPerSecond?: number;
  chunkDuration?: number;
  taskId?: string; // 新增: 任务 ID,用于获取已有 chunk
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
    taskId,
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
  const initializedTaskIdRef = useRef<string | null>(null); // 记录已初始化的 taskId

  /**
   * 初始化 chunk index - 从服务器获取已有的 chunk 列表
   * 当 taskId 变化时重新初始化
   */
  useEffect(() => {
    if (!taskId) {
      return;
    }

    // 如果已经为当前 taskId 初始化过，跳过
    if (initializedTaskIdRef.current === taskId) {
      return;
    }

    const initChunkIndex = async () => {
      try {
        const chunks = await listChunks(taskId);
        if (chunks && chunks.length > 0) {
          // 提取所有 chunk ID (格式: "0000", "0001", etc.)
          const chunkIds = chunks.map(c => parseInt(c.id, 10)).filter(id => !isNaN(id));
          if (chunkIds.length > 0) {
            const maxId = Math.max(...chunkIds);
            // 从下一个 ID 开始
            chunkIndexRef.current = maxId + 1;
            console.log(`[MediaRecorder] Initialized chunk index for task ${taskId}: ${chunkIndexRef.current} (max existing: ${maxId})`);
          } else {
            // 没有有效的 chunk ID，从 0 开始
            chunkIndexRef.current = 0;
            console.log(`[MediaRecorder] No existing chunks for task ${taskId}, starting from 0`);
          }
        } else {
          // 空列表，从 0 开始
          chunkIndexRef.current = 0;
          console.log(`[MediaRecorder] Empty chunk list for task ${taskId}, starting from 0`);
        }
        initializedTaskIdRef.current = taskId; // 标记当前 taskId 已初始化
      } catch (err) {
        console.error('[MediaRecorder] Failed to initialize chunk index:', err);
        // 失败时使用默认值 0
        chunkIndexRef.current = 0;
        initializedTaskIdRef.current = taskId;
      }
    };

    initChunkIndex();
  }, [taskId]);

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
      
      // 如果存在旧的recorder，先清理（无论状态如何）
      if (recorderRef.current) {
        try {
          if (recorderRef.current.state !== 'inactive') {
            recorderRef.current.stop();
          }
        } catch (e) {
          console.warn('[MediaRecorder] Failed to stop old recorder:', e);
        }
        recorderRef.current = null;
      }
      
      // 重新初始化recorder
      const initialized = initRecorder(audioStream);
      if (!initialized || !recorderRef.current) {
        throw createAudioError(
          '录音器初始化失败',
          AudioErrorCode.MEDIA_RECORDER_START_FAILED
        );
      }

      // 类型断言：此时 recorderRef.current 必定存在
      const recorder = recorderRef.current as MediaRecorder;
      recorder.start(chunkDuration);
      setStatus('recording');
      startTimeRef.current = Date.now();
      // 不重置 chunkIndexRef,保持连续计数 (停止后再开始时从上次位置继续)
      // chunkIndexRef.current = 0;  // ❌ 已移除
      pausedTimeRef.current = 0;

      // 添加调试日志
      console.log('[MediaRecorder] Recording started', {
        state: recorder.state,
        mimeType: recorder.mimeType,
        audioBitsPerSecond: recorder.audioBitsPerSecond,
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
    }
    
    // 清理状态（无论recorder是否存在）
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
    
    // 清理recorder引用（重要！避免下次启动时状态冲突）
    recorderRef.current = null;
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
