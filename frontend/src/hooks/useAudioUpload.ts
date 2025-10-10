import { useState, useCallback, useRef } from 'react';
import { uploadAudioChunk, uploadAudioFile } from '../utils/uploadUtils';
import { AudioUploadResponse, AudioFileUploadResponse } from '../types/audio';

interface UseAudioUploadOptions {
  taskId: string;
  maxFileSize?: number;
  onProgress?: (percent: number) => void;
  onSuccess?: (response: AudioUploadResponse | AudioFileUploadResponse) => void;
  onError?: (error: Error) => void;
}

interface UseAudioUploadReturn {
  uploadFile: (file: File) => Promise<void>;
  uploadChunk: (blob: Blob, index: number) => Promise<void>;
  uploading: boolean;
  progress: number;
  error: Error | null;
}

const MAX_RETRIES = 3;
const RETRY_DELAY = 2000; // 2秒

/**
 * 音频上传 Hook
 * 封装上传逻辑，支持进度追踪和自动重试
 */
export function useAudioUpload(options: UseAudioUploadOptions): UseAudioUploadReturn {
  const { taskId, onProgress, onSuccess, onError } = options;
  
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState<Error | null>(null);
  
  const retryCountRef = useRef(0);
  const abortControllerRef = useRef<AbortController | null>(null);

  /**
   * 延迟函数
   */
  const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

  /**
   * 上传音频分片（带重试）
   */
  const uploadChunk = useCallback(async (blob: Blob, index: number) => {
    retryCountRef.current = 0;
    setError(null);
    setUploading(true);
    setProgress(0);

    const attemptUpload = async (): Promise<void> => {
      try {
        // 获取当前录音时长（用于后端记录）
        const durationMs = Date.now();

        const response = await uploadAudioChunk(
          taskId,
          blob,
          index,
          'webm',
          durationMs,
          (percent) => {
            setProgress(percent);
            onProgress?.(percent);
          }
        );

        setUploading(false);
        setProgress(100);
        retryCountRef.current = 0;
        onSuccess?.(response);
      } catch (err: any) {
        console.error(`Upload chunk ${index} failed (attempt ${retryCountRef.current + 1}):`, err);

        // 如果还有重试次数
        if (retryCountRef.current < MAX_RETRIES) {
          retryCountRef.current++;
          console.log(`Retrying upload chunk ${index} (${retryCountRef.current}/${MAX_RETRIES})...`);
          
          // 等待后重试
          await delay(RETRY_DELAY);
          return attemptUpload();
        }

        // 重试次数用尽，抛出错误
        setError(err);
        setUploading(false);
        setProgress(0);
        onError?.(err);
        throw err;
      }
    };

    return attemptUpload();
  }, [taskId, onProgress, onSuccess, onError]);

  /**
   * 上传完整文件（带重试）
   */
  const uploadFile = useCallback(async (file: File) => {
    retryCountRef.current = 0;
    setError(null);
    setUploading(true);
    setProgress(0);

    const attemptUpload = async (): Promise<void> => {
      try {
        const response = await uploadAudioFile(
          taskId,
          file,
          (percent) => {
            setProgress(percent);
            onProgress?.(percent);
          }
        );

        setUploading(false);
        setProgress(100);
        retryCountRef.current = 0;
        onSuccess?.(response);
      } catch (err: any) {
        console.error(`Upload file failed (attempt ${retryCountRef.current + 1}):`, err);

        // 如果还有重试次数
        if (retryCountRef.current < MAX_RETRIES) {
          retryCountRef.current++;
          console.log(`Retrying upload file (${retryCountRef.current}/${MAX_RETRIES})...`);
          
          // 等待后重试
          await delay(RETRY_DELAY);
          return attemptUpload();
        }

        // 重试次数用尽，抛出错误
        setError(err);
        setUploading(false);
        setProgress(0);
        onError?.(err);
        throw err;
      }
    };

    return attemptUpload();
  }, [taskId, onProgress, onSuccess, onError]);

  return {
    uploadFile,
    uploadChunk,
    uploading,
    progress,
    error,
  };
}
