import React from 'react';
import { Modal, Button, Space, Alert } from 'antd';
import { ExclamationCircleOutlined, SaveOutlined, DeleteOutlined, CloseOutlined } from '@ant-design/icons';

export type ConfirmAction = 'create' | 'discard' | 'cancel';

interface TagConfirmModalProps {
  visible: boolean;
  currentMd5?: string;
  targetTag?: string;
  onConfirm: (action: 'create' | 'discard') => Promise<void>;
  onCancel: () => void;
  loading?: boolean;
}

export const TagConfirmModal: React.FC<TagConfirmModalProps> = ({
  visible,
  currentMd5,
  targetTag,
  onConfirm,
  onCancel,
  loading = false
}) => {
  const [actionLoading, setActionLoading] = React.useState<{
    create: boolean;
    discard: boolean;
  }>({ create: false, discard: false });

  const handleAction = async (action: 'create' | 'discard') => {
    try {
      setActionLoading({ ...actionLoading, [action]: true });
      await onConfirm(action);
    } finally {
      setActionLoading({ ...actionLoading, [action]: false });
    }
  };

  return (
    <Modal
      title={
        <Space>
          <ExclamationCircleOutlined style={{ color: '#faad14', fontSize: '20px' }} />
          <span>版本切换确认</span>
        </Space>
      }
      open={visible}
      onCancel={onCancel}
      footer={null}
      width={520}
      maskClosable={false}
    >
      <Alert
        message="检测到未保存的修改"
        description={
          <div style={{ marginTop: '8px' }}>
            <p>当前文档存在未打标签的修改，切换版本前需要处理这些更改：</p>
            <ul style={{ paddingLeft: '20px', marginTop: '8px', marginBottom: '8px' }}>
              <li><strong>创建新标签：</strong>保存当前修改为新的标签版本</li>
              <li><strong>放弃修改：</strong>丢弃当前修改，直接切换到目标版本</li>
              <li><strong>取消操作：</strong>停留在当前版本继续编辑</li>
            </ul>
            {targetTag && (
              <p style={{ marginTop: '12px', color: '#8c8c8c', fontSize: '12px' }}>
                目标版本：<code>{targetTag}</code>
              </p>
            )}
            {currentMd5 && (
              <p style={{ color: '#8c8c8c', fontSize: '12px' }}>
                当前MD5：<code>{currentMd5.substring(0, 8)}...</code>
              </p>
            )}
          </div>
        }
        type="warning"
        showIcon
        style={{ marginBottom: '20px' }}
      />

      <Space style={{ width: '100%', justifyContent: 'flex-end' }} size="middle">
        <Button
          icon={<CloseOutlined />}
          onClick={onCancel}
          disabled={actionLoading.create || actionLoading.discard}
        >
          取消
        </Button>
        
        <Button
          type="default"
          danger
          icon={<DeleteOutlined />}
          onClick={() => handleAction('discard')}
          loading={actionLoading.discard}
          disabled={actionLoading.create}
        >
          放弃修改
        </Button>
        
        <Button
          type="primary"
          icon={<SaveOutlined />}
          onClick={() => handleAction('create')}
          loading={actionLoading.create}
          disabled={actionLoading.discard}
        >
          创建新标签
        </Button>
      </Space>
    </Modal>
  );
};

export default TagConfirmModal;
