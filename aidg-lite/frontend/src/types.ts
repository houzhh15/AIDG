export interface TaskSummary {
  id: string;
  state: string;
  output_dir: string;
  created_at: string;
  ffmpeg_device?: string;
  record_chunk_seconds?: number;
  initial_embeddings_path?: string;
  diarization_backend?: string;
  product_line?: string;
  meeting_time?: string;
}

export interface ChunkFlag {
  id: string;
  wav: boolean;
  segments: boolean;
  speakers: boolean;
  embeddings: boolean;
  mapped: boolean;
  merged: boolean;
}

export interface SegmentsFile {
  segments: Array<{
    start: number;
    end: number;
    text?: string;
    speaker?: string;
  }>;
  [k: string]: any;
}

export interface AvDevice {
  index: string;
  name: string;
  kind: string; // 'audio' | 'video'
}

export interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  message?: string;
  error?: string;
}
