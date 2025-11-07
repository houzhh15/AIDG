import React, { useState } from 'react';
import { Space, Typography, Alert, message, Upload, List, Button } from 'antd';
import { InboxOutlined, AudioOutlined } from '@ant-design/icons';
import type { UploadProps } from 'antd';
import { useAudioUpload } from '../hooks/useAudioUpload';
import { validateAudioFile, formatFileSize } from '../utils/audioUtils';
import { UploadFileInfo } from '../types/audio';

const { Dragger } = Upload;
const { Text } = Typography;

interface AudioUploaderProps {
  taskId: string;
  maxFileSize?: number;
  acceptedFormats?: string[];
  onUploadSuccess?: (fileId: string) => void;
  onError?: (error: Error) => void;
}

/**
 * 音频文件上传组件
 * 提供拖拽上传、文件列表和试听功能
 */
export const AudioUploader: React.FC<AudioUploaderProps> = ({
  taskId,
  maxFileSize = 500 * 1024 * 1024, // 500MB
  acceptedFormats = ['wav', 'mp3', 'm4a', 'flac', 'ogg'],
  onUploadSuccess,
  onError
}) => {
  const [fileList, setFileList] = useState<UploadFileInfo[]>([]);
  const [currentUploadingUid, setCurrentUploadingUid] = useState<string | null>(null);

  // 上传管理
  const { uploadFile, progress } = useAudioUpload({
    taskId,
    onSuccess: (response: any) => {
      // 更新文件列表状态
      setFileList(prev =>
        prev.map(file =>
          file.uid === currentUploadingUid
            ? { ...file, status: 'done', percent: 100, response: response.data }
            : file
        )
      );
      setCurrentUploadingUid(null);
      message.success('文件上传成功');
      // 上传成功后调用回调（response 已经是 response.data）
      if (response?.data?.file_id) {
        onUploadSuccess?.(response.data.file_id);
      } else {
        // 即使没有 file_id 也调用成功回调，关闭上传窗口
        onUploadSuccess?.('');
      }
    },
    onError: (err) => {
      // 更新文件列表状态
      setFileList(prev =>
        prev.map(file =>
          file.uid === currentUploadingUid
            ? { ...file, status: 'error', error: err.message }
            : file
        )
      );
      setCurrentUploadingUid(null);
      message.error(`上传失败: ${err.message}`);
      onError?.(err);
    }
  });

  /**
   * 文件上传前的校验
   */
  const beforeUpload: UploadProps['beforeUpload'] = (file) => {
    // 验证文件
    const validation = validateAudioFile(file, acceptedFormats, maxFileSize);
    
    if (!validation.valid) {
      message.error(validation.error);
      return Upload.LIST_IGNORE;
    }

    // 添加到文件列表
    const newFile: UploadFileInfo = {
      uid: file.uid,
      name: file.name,
      size: file.size,
      status: 'uploading',
      percent: 0,
    };
    setFileList(prev => [...prev, newFile]);
    setCurrentUploadingUid(file.uid);

    // 开始上传
    uploadFile(file).catch((err) => {
      console.error('Upload failed:', err);
    });

    // 阻止默认上传行为
    return false;
  };

  /**
   * 更新上传进度
   */
  React.useEffect(() => {
    if (currentUploadingUid && progress > 0) {
      setFileList(prev =>
        prev.map(file =>
          file.uid === currentUploadingUid
            ? { ...file, percent: progress }
            : file
        )
      );
    }
  }, [currentUploadingUid, progress]);

  /**
   * 试听音频
   */
  const handlePlay = (file: UploadFileInfo) => {
    message.info('试听功能将在后续版本中实现');
  };

  /**
   * 删除文件
   */
  const handleRemove = (uid: string) => {
    setFileList(prev => prev.filter(file => file.uid !== uid));
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      {/* 文件上传区域 */}
      <Dragger
        multiple
        accept={acceptedFormats.map(ext => `.${ext}`).join(',')}
        beforeUpload={beforeUpload}
        showUploadList={false}
        disabled={!!currentUploadingUid}
      >
        <p className="ant-upload-drag-icon">
          <InboxOutlined />
        </p>
        <p className="ant-upload-text">拖拽文件到这里或点击选择</p>
        <p className="ant-upload-hint">
          支持: {acceptedFormats.join(', ')}
          <br />
          最大文件大小: {formatFileSize(maxFileSize)}
        </p>
      </Dragger>

      {/* 上传文件列表 */}
      {fileList.length > 0 && (
        <>
          <Text strong>已上传文件</Text>
          <List
            size="small"
            dataSource={fileList}
            renderItem={(file) => (
              <List.Item
                actions={[
                  file.status === 'done' && (
                    <Button
                      type="link"
                      size="small"
                      onClick={() => handlePlay(file)}
                    >
                      试听
                    </Button>
                  ),
                  <Button
                    type="link"
                    size="small"
                    danger
                    onClick={() => handleRemove(file.uid)}
                  >
                    删除
                  </Button>
                ].filter(Boolean)}
              >
                <List.Item.Meta
                  avatar={<AudioOutlined style={{ fontSize: 20 }} />}
                  title={file.name}
                  description={
                    <>
                      {formatFileSize(file.size)}
                      {file.status === 'uploading' && ` • 上传中 ${file.percent}%`}
                      {file.status === 'done' && ' • 上传成功'}
                      {file.status === 'error' && (
                        <Text type="danger"> • 上传失败: {file.error}</Text>
                      )}
                    </>
                  }
                />
              </List.Item>
            )}
          />
        </>
      )}

      {/* 提示信息 */}
      {fileList.length === 0 && (
        <Alert
          message="温馨提示"
          description="您可以一次上传多个音频文件，系统将自动关联到当前会议任务。"
          type="info"
          showIcon
        />
      )}
    </Space>
  );
};
