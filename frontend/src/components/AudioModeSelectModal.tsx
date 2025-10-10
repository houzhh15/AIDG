import React, { useState } from 'react';
import { Modal, Radio, Space, Typography } from 'antd';
import { AudioOutlined, UploadOutlined } from '@ant-design/icons';

const { Text, Paragraph } = Typography;

export type AudioMode = 'browser_record' | 'file_upload';

interface AudioModeSelectModalProps {
  open: boolean;
  onCancel: () => void;
  onConfirm: (mode: AudioMode) => void;
}

/**
 * 音频录制模式选择对话框
 * 用户点击会议侧边栏的"开始"按钮时弹出
 */
export const AudioModeSelectModal: React.FC<AudioModeSelectModalProps> = ({
  open,
  onCancel,
  onConfirm
}) => {
  const [selectedMode, setSelectedMode] = useState<AudioMode>('browser_record');

  const handleOk = () => {
    onConfirm(selectedMode);
  };

  return (
    <Modal
      title="选择录音方式"
      open={open}
      onCancel={onCancel}
      onOk={handleOk}
      okText="确定"
      cancelText="取消"
      width={500}
    >
      <Radio.Group
        value={selectedMode}
        onChange={(e) => setSelectedMode(e.target.value)}
        style={{ width: '100%' }}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          {/* 浏览器录音选项 */}
          <Radio value="browser_record" style={{ display: 'block', padding: '16px', border: '1px solid #d9d9d9', borderRadius: '4px' }}>
            <Space>
              <AudioOutlined style={{ fontSize: 20, color: '#1890ff' }} />
              <div>
                <Text strong>浏览器录音（推荐）</Text>
                <Paragraph style={{ margin: 0, color: '#8c8c8c', fontSize: 12 }}>
                  使用麦克风实时录制，自动分片上传
                </Paragraph>
              </div>
            </Space>
          </Radio>

          {/* 文件上传选项 */}
          <Radio value="file_upload" style={{ display: 'block', padding: '16px', border: '1px solid #d9d9d9', borderRadius: '4px' }}>
            <Space>
              <UploadOutlined style={{ fontSize: 20, color: '#52c41a' }} />
              <div>
                <Text strong>文件上传</Text>
                <Paragraph style={{ margin: 0, color: '#8c8c8c', fontSize: 12 }}>
                  上传本地音频文件，自动分割处理
                </Paragraph>
              </div>
            </Space>
          </Radio>
        </Space>
      </Radio.Group>
    </Modal>
  );
};
