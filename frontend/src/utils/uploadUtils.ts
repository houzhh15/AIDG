import { authedApi } from '../api/auth';
import { AudioUploadResponse, AudioFileUploadResponse, AudioErrorCode } from '../types/audio';
import { createAudioError } from './audioUtils';
import { AxiosError } from 'axios';

interface ErrorResponse {
  message?: string;
  hint?: string;
}

/**
 * 上传音频分片
 * @param taskId 任务ID
 * @param blob 音频数据
 * @param index 分片索引
 * @param format 音频格式
 * @param durationMs 音频时长（毫秒）
 * @param onProgress 进度回调函数
 * @returns 上传响应
 */
export async function uploadAudioChunk(
  taskId: string,
  blob: Blob,
  index: number,
  format: string,
  durationMs: number,
  onProgress?: (percent: number) => void
): Promise<AudioUploadResponse> {
  try {
    const formData = new FormData();
    formData.append('audio', blob, `chunk_${index}.${format}`);
    formData.append('chunk_index', index.toString());
    formData.append('format', format);
    formData.append('duration_ms', durationMs.toString());

    const response = await authedApi.post(
      `/meetings/${taskId}/audio/upload`,
      formData,
      {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
        onUploadProgress: (progressEvent) => {
          if (onProgress && progressEvent.total) {
            const percent = Math.round((progressEvent.loaded * 100) / progressEvent.total);
            onProgress(percent);
          }
        },
      }
    );

    return response.data;
  } catch (error: unknown) {
    // 处理网络错误
    const axiosError = error as AxiosError<ErrorResponse>;
    if (axiosError.message === 'Network Error' || !axiosError.response) {
      throw createAudioError(
        '网络连接失败，请检查网络后重试',
        AudioErrorCode.NETWORK_ERROR,
        error
      );
    }

    // 处理服务器错误
    const status = axiosError.response?.status;
    const message = axiosError.response?.data?.message || '上传失败';

    if (status === 401) {
      throw createAudioError('未授权，请重新登录', AudioErrorCode.UNAUTHORIZED, error);
    }
    if (status === 403) {
      throw createAudioError('无权限上传音频', AudioErrorCode.FORBIDDEN, error);
    }
    if (status === 413) {
      throw createAudioError('文件过大', AudioErrorCode.FILE_TOO_LARGE, error);
    }
    if (status === 503) {
      // 服务不可用，通常是因为缺少依赖（如FFmpeg）
      const hint = axiosError.response?.data?.hint || '';
      const fullMessage = hint ? `${message}\n\n${hint}` : message;
      throw createAudioError(fullMessage, AudioErrorCode.SERVICE_UNAVAILABLE, error);
    }
    if (status && status >= 500) {
      throw createAudioError(
        `服务器错误: ${message}`,
        AudioErrorCode.SERVER_ERROR,
        error
      );
    }

    throw createAudioError(message, AudioErrorCode.UPLOAD_FAILED, error);
  }
}

/**
 * 上传完整音频文件
 * @param taskId 任务ID
 * @param file 文件对象
 * @param onProgress 进度回调函数
 * @returns 上传响应
 */
export async function uploadAudioFile(
  taskId: string,
  file: File,
  onProgress?: (percent: number) => void
): Promise<AudioFileUploadResponse> {
  try {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('filename', file.name);

    const response = await authedApi.post(
      `/meetings/${taskId}/audio/upload-file`,
      formData,
      {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
        onUploadProgress: (progressEvent) => {
          if (onProgress && progressEvent.total) {
            const percent = Math.round((progressEvent.loaded * 100) / progressEvent.total);
            onProgress(percent);
          }
        },
      }
    );

    return response.data;
  } catch (error: unknown) {
    // 处理网络错误
    const axiosError = error as AxiosError<ErrorResponse>;
    if (axiosError.message === 'Network Error' || !axiosError.response) {
      throw createAudioError(
        '网络连接失败，请检查网络后重试',
        AudioErrorCode.NETWORK_ERROR,
        error
      );
    }

    // 处理服务器错误
    const status = axiosError.response?.status;
    const message = axiosError.response?.data?.message || '上传失败';

    if (status === 401) {
      throw createAudioError('未授权，请重新登录', AudioErrorCode.UNAUTHORIZED, error);
    }
    if (status === 403) {
      throw createAudioError('无权限上传音频', AudioErrorCode.FORBIDDEN, error);
    }
    if (status === 413) {
      throw createAudioError('文件过大', AudioErrorCode.FILE_TOO_LARGE, error);
    }
    if (status === 503) {
      // 服务不可用，通常是因为缺少依赖（如FFmpeg）
      const hint = axiosError.response?.data?.hint || '';
      const fullMessage = hint ? `${message}\n\n${hint}` : message;
      throw createAudioError(fullMessage, AudioErrorCode.SERVICE_UNAVAILABLE, error);
    }
    if (status && status >= 500) {
      throw createAudioError(
        `服务器错误: ${message}`,
        AudioErrorCode.SERVER_ERROR,
        error
      );
    }

    throw createAudioError(message, AudioErrorCode.UPLOAD_FAILED, error);
  }
}
