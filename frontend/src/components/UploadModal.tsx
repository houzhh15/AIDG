import React from 'react';
import { Modal, Tabs } from 'antd';
import { AudioOutlined, FileTextOutlined } from '@ant-design/icons';
import { AudioUploader } from './AudioUploader';
import { TextUploader } from './TextUploader';
import { AudioMode } from './AudioModeSelectModal';

const { TabPane } = Tabs;

interface UploadModalProps {
  open: boolean;
  mode: AudioMode;
  taskId: string;
  onCancel: () => void;
  onSuccess?: () => void;
}

/**
 * 文件上传 Modal
 * 根据选择的模式显示音频上传或文本上传组件
 */
export const UploadModal: React.FC<UploadModalProps> = ({
  open,
  mode,
  taskId,
  onCancel,
  onSuccess
}) => {
  // 不显示浏览器录音模式（该模式直接开始录音）
  if (mode === 'browser_record') {
    return null;
  }

  return (
    <Modal
      title="上传文件"
      open={open}
      onCancel={onCancel}
      footer={null}
      width={700}
      destroyOnClose
    >
      {mode === 'file_upload' && (
        <AudioUploader
          taskId={taskId}
          onUploadSuccess={() => {
            onSuccess?.();
            onCancel();
          }}
          onError={(error) => {
            console.error('Upload error:', error);
          }}
        />
      )}

      {mode === 'text_upload' && (
        <TextUploader
          taskId={taskId}
          onUploadSuccess={(taskId, textType) => {
            console.log(`Text upload success: taskId=${taskId}, textType=${textType}`);
            onSuccess?.();
            onCancel();
          }}
          onError={(error) => {
            console.error('Upload error:', error);
          }}
        />
      )}
    </Modal>
  );
};
