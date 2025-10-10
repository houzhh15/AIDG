import { AudioErrorCode } from '../types/audio';

/**
 * 格式化时长为 HH:MM:SS 格式
 * @param ms 毫秒数
 * @returns 格式化后的时长字符串
 */
export function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;

  if (hours > 0) {
    return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  }
  return `${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
}

/**
 * 格式化文件大小
 * @param bytes 字节数
 * @returns 格式化后的文件大小字符串
 */
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
}

/**
 * 验证音频文件格式和大小
 * @param file 文件对象
 * @param acceptedFormats 接受的文件格式数组
 * @param maxSize 最大文件大小（字节），默认500MB
 * @returns 验证结果
 */
export function validateAudioFile(
  file: File,
  acceptedFormats: string[] = ['wav', 'mp3', 'm4a', 'flac', 'ogg', 'webm'],
  maxSize: number = 500 * 1024 * 1024
): { valid: boolean; error?: string; code?: AudioErrorCode } {
  // 检查文件大小
  if (file.size > maxSize) {
    return {
      valid: false,
      error: `文件大小超过${formatFileSize(maxSize)}限制`,
      code: AudioErrorCode.FILE_TOO_LARGE
    };
  }

  // 检查文件格式
  const fileExtension = file.name.split('.').pop()?.toLowerCase();
  if (!fileExtension || !acceptedFormats.includes(fileExtension)) {
    return {
      valid: false,
      error: `不支持的文件格式。支持的格式: ${acceptedFormats.join(', ')}`,
      code: AudioErrorCode.INVALID_FILE_FORMAT
    };
  }

  // 验证MIME类型
  const audioMimeTypes = [
    'audio/wav', 'audio/wave', 'audio/x-wav',
    'audio/mpeg', 'audio/mp3',
    'audio/mp4', 'audio/m4a', 'audio/x-m4a',
    'audio/flac',
    'audio/ogg', 'audio/opus',
    'audio/webm'
  ];
  
  if (file.type && !audioMimeTypes.includes(file.type)) {
    return {
      valid: false,
      error: `无效的MIME类型: ${file.type}`,
      code: AudioErrorCode.INVALID_FILE_FORMAT
    };
  }

  return { valid: true };
}

/**
 * 创建音频错误对象
 * @param message 错误消息
 * @param code 错误码
 * @param details 错误详情
 * @returns AudioError对象
 */
export function createAudioError(
  message: string,
  code: AudioErrorCode,
  details?: any
): Error {
  const error = new Error(message) as any;
  error.code = code;
  error.details = details;
  return error;
}
