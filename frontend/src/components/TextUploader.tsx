import React, { useState } from 'react';
import { Space, Typography, Alert, message, Upload, Card, Tag, Button, Radio, Progress } from 'antd';
import { FileTextOutlined, InboxOutlined } from '@ant-design/icons';
import type { UploadProps } from 'antd';
import { useTextUpload, validateTextFormat, TextType } from '../hooks/useTextUpload';

const { Dragger } = Upload;
const { Text, Paragraph } = Typography;

interface TextUploaderProps {
  taskId: string;
  maxFileSize?: number;
  onUploadSuccess?: (taskId: string, textType: TextType) => void;
  onError?: (error: Error) => void;
}

/**
 * 文本文件上传组件
 * 支持上传 merged_all（原始转录）和 polish（校准后）两种文本
 */
export const TextUploader: React.FC<TextUploaderProps> = ({
  taskId,
  maxFileSize = 10 * 1024 * 1024, // 10MB
  onUploadSuccess,
  onError
}) => {
  const [textType, setTextType] = useState<TextType | null>('merged_all'); // 默认选中 merged_all
  const [fileContent, setFileContent] = useState<string | null>(null);
  const [fileName, setFileName] = useState<string | null>(null);
  const [fileSize, setFileSize] = useState<number>(0);
  const [encoding, setEncoding] = useState<string>('UTF-8');
  const [speakerCount, setSpeakerCount] = useState<number>(0);
  const [lineCount, setLineCount] = useState<number>(0);
  const [validationError, setValidationError] = useState<string | null>(null);

  const { uploadText, uploading, progress } = useTextUpload({
    onSuccess: (taskId, textType) => {
      message.success(`${textType === 'merged_all' ? '原始转录文本' : '校准后文本'} 上传成功`);
      onUploadSuccess?.(taskId, textType);
      // 清空状态
      handleClear();
    },
    onError: (error) => {
      message.error(`上传失败: ${error.message}`);
      onError?.(error);
    }
  });

  /**
   * 文件选择前的处理
   */
  const beforeUpload: UploadProps['beforeUpload'] = (file) => {
    // 检查是否选择了文本类型
    if (!textType) {
      message.error('请先选择文本类型');
      return Upload.LIST_IGNORE;
    }

    // 检查文件大小
    if (file.size > maxFileSize) {
      message.error(`文件大小不能超过 ${(maxFileSize / 1024 / 1024).toFixed(0)}MB`);
      return Upload.LIST_IGNORE;
    }

    // 检查文件格式
    const fileExt = file.name.split('.').pop()?.toLowerCase();
    if (!['txt', 'md'].includes(fileExt || '')) {
      message.error('仅支持 TXT 和 MD 格式的文本文件');
      return Upload.LIST_IGNORE;
    }

    // 读取文件内容
    const reader = new FileReader();
    reader.onload = (e) => {
      const content = e.target?.result as string;
      
      // 验证文本格式（仅检查内容是否为空）
      const validation = validateTextFormat(content);
      
      if (!validation.isValid) {
        const errorMsg = validation.errors.join('\n');
        setValidationError(errorMsg);
        message.error('文本内容不能为空');
        return;
      }

      // 更新状态
      setFileContent(content);
      setFileName(file.name);
      setFileSize(file.size);
      setSpeakerCount(validation.speakerCount);
      setLineCount(validation.lineCount);
      setValidationError(null);

      message.success('文件加载成功，请检查预览后点击上传');
    };

    reader.onerror = () => {
      message.error('文件读取失败');
    };

    reader.readAsText(file, 'UTF-8');

    // 阻止默认上传
    return false;
  };

  /**
   * 上传文本
   */
  const handleUpload = async () => {
    if (!fileContent || !textType) {
      message.error('请先选择文件');
      return;
    }

    if (validationError) {
      message.error('文本格式不正确，无法上传');
      return;
    }

    try {
      await uploadText(taskId, textType, fileContent);
    } catch (error) {
      console.error('Upload error:', error);
    }
  };

  /**
   * 清空状态
   */
  const handleClear = () => {
    setFileContent(null);
    setFileName(null);
    setFileSize(0);
    setSpeakerCount(0);
    setLineCount(0);
    setValidationError(null);
  };

  /**
   * 格式化文件大小
   */
  const formatFileSize = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      {/* 文本类型选择 */}
      <div>
        <Paragraph strong style={{ marginBottom: 8 }}>
          请选择文本类型：
        </Paragraph>
        <Radio.Group
          value={textType}
          onChange={(e) => {
            setTextType(e.target.value);
            handleClear(); // 切换类型时清空已选文件
          }}
          style={{ width: '100%' }}
        >
          <Space direction="vertical" style={{ width: '100%' }}>
            <Radio value="merged_all">
              <Space direction="vertical" size={0}>
                <Text strong>原始转录文本 (merged_all)</Text>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  未经校准的转录文本，系统将基于此文本生成会议内容
                </Text>
              </Space>
            </Radio>
            <Radio value="polish">
              <Space direction="vertical" size={0}>
                <Text strong>校准后文本 (polish)</Text>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  人工校准的高质量文本，直接用于生成最终会议记录
                </Text>
              </Space>
            </Radio>
          </Space>
        </Radio.Group>
      </div>

      {/* 文件上传区域 */}
      <Dragger
        accept=".txt,.md"
        beforeUpload={beforeUpload}
        showUploadList={false}
        disabled={!textType || uploading}
      >
        <p className="ant-upload-drag-icon">
          <FileTextOutlined />
        </p>
        <p className="ant-upload-text">
          {textType ? '拖拽文件到这里或点击选择' : '请先选择文本类型'}
        </p>
        <p className="ant-upload-hint">
          支持: TXT, MD
          <br />
          最大: {(maxFileSize / 1024 / 1024).toFixed(0)}MB
          <br />
          编码: UTF-8 (推荐)
        </p>
      </Dragger>

      {/* 格式说明 */}
      <Alert
        type="info"
        message="支持的文件格式"
        description={
          <ul style={{ margin: 0, paddingLeft: '20px', fontSize: 12 }}>
            <li>支持 .txt 和 .md 格式的文本文件</li>
            <li>推荐使用 UTF-8 编码</li>
            <li>文件内容可以是任意文本，无格式限制</li>
            <li>如包含说话人标签（如 [SPK01]）会自动统计说话人数量</li>
          </ul>
        }
      />

      {/* 文件预览 */}
      {fileContent && (
        <Card
          title={
            <Space>
              <Text strong>文件预览</Text>
              <Tag color="blue">{fileName}</Tag>
            </Space>
          }
          extra={
            <Space>
              <Tag>说话人: {speakerCount}</Tag>
              <Tag>行数: {lineCount}</Tag>
              <Tag>大小: {formatFileSize(fileSize)}</Tag>
              <Tag>编码: {encoding}</Tag>
            </Space>
          }
        >
          <pre
            style={{
              maxHeight: '200px',
              overflow: 'auto',
              fontSize: '12px',
              margin: 0,
              padding: '8px',
              backgroundColor: '#f5f5f5',
              borderRadius: '4px',
            }}
          >
            {fileContent.slice(0, 500)}
            {fileContent.length > 500 && '\n...（已省略部分内容）'}
          </pre>
        </Card>
      )}

      {/* 验证错误提示 */}
      {validationError && (
        <Alert
          type="error"
          message="文本格式错误"
          description={<pre style={{ margin: 0, fontSize: 12 }}>{validationError}</pre>}
          showIcon
        />
      )}

      {/* 上传进度 */}
      {uploading && (
        <Progress percent={progress} status="active" />
      )}

      {/* 操作按钮 */}
      <Space>
        <Button
          type="primary"
          onClick={handleUpload}
          disabled={!fileContent || !!validationError || uploading}
          loading={uploading}
        >
          上传
        </Button>
        <Button onClick={handleClear} disabled={uploading}>
          清除
        </Button>
      </Space>
    </Space>
  );
};
