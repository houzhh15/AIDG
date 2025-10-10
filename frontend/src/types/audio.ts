// 音频错误码枚举
export enum AudioErrorCode {
  // 权限相关
  PERMISSION_DENIED = 'PERMISSION_DENIED',
  PERMISSION_TIMEOUT = 'PERMISSION_TIMEOUT',
  
  // 录音相关
  MEDIA_RECORDER_NOT_SUPPORTED = 'MEDIA_RECORDER_NOT_SUPPORTED',
  MEDIA_RECORDER_START_FAILED = 'MEDIA_RECORDER_START_FAILED',
  AUDIO_CAPTURE_FAILED = 'AUDIO_CAPTURE_FAILED',
  
  // 上传相关
  UPLOAD_FAILED = 'UPLOAD_FAILED',
  UPLOAD_TIMEOUT = 'UPLOAD_TIMEOUT',
  FILE_TOO_LARGE = 'FILE_TOO_LARGE',
  INVALID_FILE_FORMAT = 'INVALID_FILE_FORMAT',
  NETWORK_ERROR = 'NETWORK_ERROR',
  
  // 服务器相关
  SERVER_ERROR = 'SERVER_ERROR',
  UNAUTHORIZED = 'UNAUTHORIZED',
  FORBIDDEN = 'FORBIDDEN',
}

// 音频错误接口
export interface AudioError extends Error {
  code: AudioErrorCode;
  details?: any;
}

// 录音状态类型
export type RecordingStatus = 'idle' | 'recording' | 'paused' | 'uploading' | 'error';

// 录音状态接口
export interface RecordingState {
  status: RecordingStatus;
  startTime: number | null;
  duration: number;
  uploadedChunks: number;
  uploadedSize: number;
  currentUploadProgress: number;
  error: string | null;
}

// 权限状态类型
export type PermissionStatus = 'prompt' | 'granted' | 'denied';

// 权限状态接口
export interface PermissionState {
  status: PermissionStatus;
  stream: MediaStream | null;
  error: string | null;
}

// 上传文件信息接口
export interface UploadFileInfo {
  uid: string;
  name: string;
  size: number;
  status: 'uploading' | 'done' | 'error';
  percent: number;
  response?: {
    file_id: string;
    file_path: string;
    processing_status: string;
  };
  error?: string;
}

// 音频上传请求接口
export interface AudioUploadRequest {
  audio: File;
  chunk_index: number;
  total_chunks?: number;
  format: string;
  duration_ms: number;
}

// 音频上传响应接口
export interface AudioUploadResponse {
  success: boolean;
  data?: {
    chunk_id: string;
    file_path: string;
    processing_status: 'queued' | 'processing' | 'completed';
  };
  message?: string;
}

// 音频文件上传请求接口
export interface AudioFileUploadRequest {
  file: File;
  filename: string;
}

// 音频文件上传响应接口
export interface AudioFileUploadResponse {
  success: boolean;
  data?: {
    file_id: string;
    file_path: string;
    duration_ms: number;
    size_bytes: number;
    processing_status: string;
  };
  message?: string;
}
