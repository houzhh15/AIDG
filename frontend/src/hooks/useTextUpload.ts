/**
 * useTextUpload Hook
 * 用于上传文本文件（merged_all 和 polish）
 */

import { useState } from 'react';
import { message } from 'antd';
import { authedApi } from '../api/auth';

export type TextType = 'merged_all' | 'polish';

interface UseTextUploadOptions {
  onSuccess?: (taskId: string, textType: TextType) => void;
  onError?: (error: Error) => void;
}

interface UseTextUploadReturn {
  uploadText: (taskId: string, textType: TextType, content: string) => Promise<void>;
  uploading: boolean;
  progress: number;
}

export function useTextUpload(options?: UseTextUploadOptions): UseTextUploadReturn {
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);

  const uploadText = async (
    taskId: string,
    textType: TextType,
    content: string
  ): Promise<void> => {
    try {
      setUploading(true);
      setProgress(0);

      // 根据文本类型选择不同的接口
      // 注意：authedApi 的 baseURL 已经是 /api/v1，所以这里不需要再加前缀
      const endpoint = textType === 'merged_all' 
        ? `/tasks/${encodeURIComponent(taskId)}/merged_all`
        : `/tasks/${encodeURIComponent(taskId)}/polish`;

      // 发送 PUT 请求（使用 authedApi 自动携带认证 token）
      const response = await authedApi.put(
        endpoint,
        {
          content: content
        },
        {
          onUploadProgress: (progressEvent: any) => {
            if (progressEvent.total) {
              const percentCompleted = Math.round(
                (progressEvent.loaded * 100) / progressEvent.total
              );
              setProgress(percentCompleted);
            }
          }
        }
      );

      setProgress(100);
      
      // 成功回调
      options?.onSuccess?.(taskId, textType);
      
      return response.data;
    } catch (error: any) {
      const errorMessage = error.response?.data?.error || error.message || '上传失败';
      
      // 错误回调
      options?.onError?.(new Error(errorMessage));
      
      throw error;
    } finally {
      setUploading(false);
      setProgress(0);
    }
  };

  return {
    uploadText,
    uploading,
    progress
  };
}

/**
 * 验证文本格式（可选的说话人标签统计）
 */
export interface TextValidationResult {
  isValid: boolean;
  speakerCount: number;
  lineCount: number;
  speakerTags: string[];
  errors: string[];
  warnings: string[];
}

export function validateTextFormat(content: string): TextValidationResult {
  const speakerPattern = /\[SPK\d+\]|\[Speaker \d+\]/g;
  const lines = content.split('\n');
  
  const result: TextValidationResult = {
    isValid: true, // 默认有效，不强制要求说话人标签
    speakerCount: 0,
    lineCount: lines.length,
    speakerTags: [],
    errors: [],
    warnings: []
  };
  
  // 检测是否包含说话人标签（可选，仅用于统计）
  const speakerSet = new Set<string>();
  const allMatches = content.match(speakerPattern);
  
  if (allMatches) {
    allMatches.forEach(match => speakerSet.add(match));
    result.speakerTags = Array.from(speakerSet);
    result.speakerCount = result.speakerTags.length;
  }
  
  // 基本验证：内容不能为空
  if (!content.trim()) {
    result.isValid = false;
    result.errors.push('文本内容不能为空');
  }
  
  return result;
}

/**
 * 检测文本编码
 */
export function detectEncoding(file: File): Promise<string> {
  return new Promise((resolve) => {
    const reader = new FileReader();
    
    reader.onload = (e) => {
      const content = e.target?.result as string;
      
      // 简单的编码检测（实际应用中可能需要更复杂的逻辑）
      if (content.includes('�') || !/[\u4e00-\u9fa5]/.test(content)) {
        resolve('GBK');
      } else {
        resolve('UTF-8');
      }
    };
    
    reader.readAsText(file, 'UTF-8');
  });
}

/**
 * 使用示例：
 * 
 * const { uploadText, uploading, progress } = useTextUpload({
 *   onSuccess: (taskId, textType) => {
 *     message.success(`${textType} 上传成功`);
 *   },
 *   onError: (error) => {
 *     message.error(`上传失败: ${error.message}`);
 *   }
 * });
 * 
 * // 上传 merged_all
 * await uploadText(taskId, 'merged_all', fileContent);
 * 
 * // 上传 polish
 * await uploadText(taskId, 'polish', fileContent);
 */
