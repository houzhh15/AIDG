/**
 * FileUploadArea.tsx
 * 文件上传区域组件 - 支持拖拽上传和文件格式转换
 */

import React, { useState } from 'react';
import { Upload, message, Progress, Typography, Space, Alert } from 'antd';
import { InboxOutlined, FileOutlined, CheckCircleOutlined } from '@ant-design/icons';
import type { UploadProps } from 'antd';
import { documentsAPI, ImportMeta, ImportFileResponse } from '../../api/documents';

const { Dragger } = Upload;
const { Text } = Typography;

// 支持的文件类型
const ACCEPTED_TYPES = '.pdf,.ppt,.pptx,.doc,.docx,.xls,.xlsx,.svg';
const MAX_FILE_SIZE = 20 * 1024 * 1024; // 20MB

interface FileUploadAreaProps {
  projectId: string;
  onImportComplete: (content: string, importMeta: ImportMeta) => void;
  onError?: (error: string) => void;
}

type UploadStatus = 'idle' | 'uploading' | 'converting' | 'success' | 'error';

const FileUploadArea: React.FC<FileUploadAreaProps> = ({
  projectId,
  onImportComplete,
  onError,
}) => {
  const [status, setStatus] = useState<UploadStatus>('idle');
  const [progress, setProgress] = useState<number>(0);
  const [fileName, setFileName] = useState<string>('');
  const [warnings, setWarnings] = useState<string[]>([]);

  const handleUpload = async (file: File): Promise<boolean> => {
    // 验证文件大小
    if (file.size > MAX_FILE_SIZE) {
      const errorMsg = `文件大小超过限制（最大 20MB）`;
      message.error(errorMsg);
      onError?.(errorMsg);
      return false;
    }

    setFileName(file.name);
    setStatus('uploading');
    setProgress(0);
    setWarnings([]);

    try {
      // 模拟上传进度
      const progressInterval = setInterval(() => {
        setProgress((prev) => {
          if (prev >= 30) {
            clearInterval(progressInterval);
            return 30;
          }
          return prev + 10;
        });
      }, 100);

      setStatus('converting');
      setProgress(50);

      // 调用 API 上传并转换
      const response: ImportFileResponse = await documentsAPI.importFile(projectId, file);

      clearInterval(progressInterval);
      setProgress(100);
      setStatus('success');

      if (response.warnings && response.warnings.length > 0) {
        setWarnings(response.warnings);
      }

      // 构建导入元数据
      const importMeta: ImportMeta = {
        source_type: 'file_import',
        original_filename: response.original_filename,
        file_size: response.file_size,
        content_type: response.content_type,
      };

      // 回调通知父组件
      onImportComplete(response.content, importMeta);
      message.success('文件导入成功');
    } catch (error: any) {
      setStatus('error');
      const errorMsg = error?.response?.data?.error || error?.message || '文件导入失败';
      message.error(errorMsg);
      onError?.(errorMsg);
    }

    return false; // 阻止 antd 默认上传行为
  };

  const uploadProps: UploadProps = {
    name: 'file',
    multiple: false,
    maxCount: 1,
    accept: ACCEPTED_TYPES,
    showUploadList: false,
    beforeUpload: handleUpload,
  };

  const renderContent = () => {
    switch (status) {
      case 'uploading':
        return (
          <Space direction="vertical" align="center">
            <Progress type="circle" percent={progress} size={60} />
            <Text>正在上传 {fileName}...</Text>
          </Space>
        );
      case 'converting':
        return (
          <Space direction="vertical" align="center">
            <Progress type="circle" percent={progress} size={60} status="active" />
            <Text>正在转换文件格式...</Text>
          </Space>
        );
      case 'success':
        return (
          <Space direction="vertical" align="center">
            <CheckCircleOutlined style={{ fontSize: 48, color: '#52c41a' }} />
            <Text strong>{fileName}</Text>
            <Text type="secondary">导入成功，可以预览内容</Text>
            {warnings.length > 0 && (
              <Alert
                type="warning"
                message="转换警告"
                description={warnings.join('; ')}
                showIcon
                style={{ marginTop: 8, textAlign: 'left' }}
              />
            )}
          </Space>
        );
      case 'error':
        return (
          <Space direction="vertical" align="center">
            <FileOutlined style={{ fontSize: 48, color: '#ff4d4f' }} />
            <Text type="danger">导入失败，请重试</Text>
          </Space>
        );
      default:
        return (
          <>
            <p className="ant-upload-drag-icon">
              <InboxOutlined />
            </p>
            <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
            <p className="ant-upload-hint">
              支持 PDF、PPT、DOC、EXCEL、SVG 格式，最大 20MB
            </p>
          </>
        );
    }
  };

  return (
    <Dragger
      {...uploadProps}
      style={{
        padding: '20px',
        minHeight: 200,
        background: status === 'success' ? '#f6ffed' : undefined,
      }}
    >
      {renderContent()}
    </Dragger>
  );
};

export default FileUploadArea;
