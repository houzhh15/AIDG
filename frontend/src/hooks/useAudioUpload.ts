import { useState, useCallback, useRef } from 'react';
import { uploadAudioChunk, uploadAudioFile } from '../utils/uploadUtils';
import { AudioUploadResponse, AudioFileUploadResponse, AudioErrorCode } from '../types/audio';

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
  clearUploadedChunks: () => void; // 清除缓存
}

// Chunk上传状态缓存 - 使用 Map 存储每个 taskId 的已上传 chunk
const uploadedChunksCache = new Map<string, Set<number>>();

const MAX_RETRIES = 3;
const RETRY_DELAY = 2000; // 2秒

// 文件上传不重试（配置错误或服务器错误不应该重试）
const FILE_UPLOAD_MAX_RETRIES = 0;

/**
 * 判断错误是否可重试（仅网络错误可重试）
 */
function isRetryableError(error: any): boolean {
  // 检查错误代码
  if (error.code === AudioErrorCode.NETWORK_ERROR) {
    return true;
  }
  
  // 检查错误消息
  if (error.message && error.message.includes('网络')) {
    return true;
  }
  
  // 检查是否是网络超时
  if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') {
    return true;
  }
  
  // 其他错误（包括200响应后的任何错误）不重试
  return false;
}

/**
 * 音频上传 Hook
 * 封装上传逻辑，支持进度追踪和智能重试
 * - 只在网络错误时重试，服务器响应错误不重试
 * - 添加 chunk 上传状态缓存，避免重复上传
 */
export function useAudioUpload(options: UseAudioUploadOptions): UseAudioUploadReturn {
  const { taskId, onProgress, onSuccess, onError } = options;
  
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState<Error | null>(null);
  
  const retryCountRef = useRef(0);
  const abortControllerRef = useRef<AbortController | null>(null);

  // 初始化该 taskId 的上传缓存
  if (!uploadedChunksCache.has(taskId)) {
    uploadedChunksCache.set(taskId, new Set<number>());
  }

  /**
   * 延迟函数
   */
  const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

  /**
   * 清除已上传chunk缓存
   */
  const clearUploadedChunks = useCallback(() => {
    uploadedChunksCache.delete(taskId);
    console.log(`[AudioUpload] Cleared uploaded chunks cache for task ${taskId}`);
  }, [taskId]);

  /**
   * 上传音频分片（带智能重试和缓存）
   */
  const uploadChunk = useCallback(async (blob: Blob, index: number) => {
    // 检查是否已上传过
    const uploadedChunks = uploadedChunksCache.get(taskId);
    if (uploadedChunks?.has(index)) {
      console.log(`[AudioUpload] Chunk ${index} already uploaded, skipping`);
      setProgress(100);
      onSuccess?.({
        success: true,
        data: {
          chunk_id: `chunk_${String(index).padStart(4, '0')}`,
          file_path: '',
          processing_status: 'cached',
        },
      } as AudioUploadResponse);
      return;
    }

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

        // 上传成功，添加到缓存
        uploadedChunks?.add(index);
        console.log(`[AudioUpload] Chunk ${index} uploaded successfully and cached`);

        setUploading(false);
        setProgress(100);
        retryCountRef.current = 0;
        onSuccess?.(response);
      } catch (err: any) {
        console.error(`Upload chunk ${index} failed (attempt ${retryCountRef.current + 1}):`, err);

        // 只在网络错误时重试
        const canRetry = isRetryableError(err);
        
        if (canRetry && retryCountRef.current < MAX_RETRIES) {
          retryCountRef.current++;
          console.log(`[AudioUpload] Network error, retrying chunk ${index} (${retryCountRef.current}/${MAX_RETRIES})...`);
          
          // 等待后重试
          await delay(RETRY_DELAY);
          return attemptUpload();
        }

        // 不可重试的错误或重试次数用尽
        if (!canRetry) {
          console.error(`[AudioUpload] Non-retryable error for chunk ${index}:`, err.message);
        } else {
          console.error(`[AudioUpload] Max retries reached for chunk ${index}`);
        }

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

        // 文件上传失败不重试（配置问题或服务器错误）
        if (retryCountRef.current < FILE_UPLOAD_MAX_RETRIES) {
          retryCountRef.current++;
          console.log(`Retrying upload file (${retryCountRef.current}/${FILE_UPLOAD_MAX_RETRIES})...`);
          
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
    clearUploadedChunks,
  };
}
