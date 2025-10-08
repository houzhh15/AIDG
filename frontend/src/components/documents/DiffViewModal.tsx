import React, { useState, useEffect } from 'react';
import { Modal, Typography, Spin, Alert, Tabs, message } from 'antd';
import { SwapOutlined } from '@ant-design/icons';
import { DiffViewModalProps } from '../../types/documents';

const { Text, Title } = Typography;
const { TabPane } = Tabs;

interface DiffLine {
  type: 'add' | 'remove' | 'normal';
  lineNumber: number;
  content: string;
}

const buildFallbackDiff = (fromContent: string, toContent: string): DiffLine[] => {
  const fromLines = fromContent.split('\n');
  const toLines = toContent.split('\n');
  const diff: DiffLine[] = [];

  let fromIndex = 0;
  let toIndex = 0;

  while (fromIndex < fromLines.length || toIndex < toLines.length) {
    const fromLine = fromLines[fromIndex];
    const toLine = toLines[toIndex];

    if (fromLine === toLine) {
      diff.push({
        type: 'normal',
        lineNumber: (toIndex ?? 0) + 1,
        content: toLine ?? ''
      });
      fromIndex++;
      toIndex++;
    } else if (fromIndex < fromLines.length && toIndex < toLines.length) {
      if (fromLine) {
        diff.push({
          type: 'remove',
          lineNumber: fromIndex + 1,
          content: fromLine
        });
      }
      if (toLine) {
        diff.push({
          type: 'add',
          lineNumber: toIndex + 1,
          content: toLine
        });
      }
      fromIndex++;
      toIndex++;
    } else if (fromIndex < fromLines.length) {
      diff.push({
        type: 'remove',
        lineNumber: fromIndex + 1,
        content: fromLine ?? ''
      });
      fromIndex++;
    } else {
      diff.push({
        type: 'add',
        lineNumber: toIndex + 1,
        content: toLine ?? ''
      });
      toIndex++;
    }
  }

  return diff;
};

const DiffViewModal: React.FC<DiffViewModalProps> = ({
  projectId,
  nodeId,
  fromVersion,
  toVersion,
  visible,
  onClose,
  currentVersion
}) => {
  const [loading, setLoading] = useState<boolean>(false);
  const [diffData, setDiffData] = useState<DiffLine[]>([]);
  const [fromContent, setFromContent] = useState<string>('');
  const [toContent, setToContent] = useState<string>('');
  const [error, setError] = useState<string>('');

  useEffect(() => {
    if (visible && fromVersion && toVersion) {
      loadDiffData();
    }
  }, [visible, projectId, nodeId, fromVersion, toVersion]);

  const loadDiffData = async () => {
    setLoading(true);
    setError('');
    try {
      const { default: documentsAPI } = await import('../../api/documents');
      try {
        await documentsAPI.compareVersions(projectId, nodeId, fromVersion, toVersion);
      } catch (compareError: any) {
        const status = compareError?.response?.status;
        console.error('差异对比API失败:', compareError);
        if (status === 500) {
          message.warning('后端暂不支持该版本组合的差异对比，已使用前端计算结果。');
        } else if (status === 404) {
          message.warning('找不到针对该版本组合的差异数据，已使用前端计算结果。');
        } else {
          message.warning('获取差异对比失败，已使用前端计算结果。');
        }
      }

      const resolveContent = async (version: number) => {
        if (typeof currentVersion === 'number' && version === currentVersion) {
          const current = await documentsAPI.getContent(projectId, nodeId);
          return current.content;
        }
        const historical = await documentsAPI.getVersionContent(projectId, nodeId, version);
        return historical.content;
      };

      const [fromContentValue, toContentValue] = await Promise.all([
        resolveContent(fromVersion),
        resolveContent(toVersion)
      ]);

      setFromContent(fromContentValue);
      setToContent(toContentValue);

      const fallbackDiff = buildFallbackDiff(fromContentValue, toContentValue);
      setDiffData(fallbackDiff);
    } catch (error) {
      console.error('加载差异对比失败:', error);
      setError('加载差异对比失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  const renderDiffLine = (line: DiffLine, index: number) => {
    const getLineStyle = () => {
      switch (line.type) {
        case 'add':
          return { backgroundColor: '#f6ffed', borderLeft: '3px solid #52c41a' };
        case 'remove':
          return { backgroundColor: '#fff2f0', borderLeft: '3px solid #ff4d4f' };
        default:
          return { backgroundColor: 'transparent' };
      }
    };

    const getLinePrefix = () => {
      switch (line.type) {
        case 'add':
          return '+ ';
        case 'remove':
          return '- ';
        default:
          return '  ';
      }
    };

    return (
      <div
        key={index}
        style={{
          ...getLineStyle(),
          padding: '4px 8px',
          fontFamily: 'Monaco, Consolas, "Courier New", monospace',
          fontSize: 13,
          lineHeight: 1.4,
          whiteSpace: 'pre-wrap'
        }}
      >
        <span style={{ color: '#666', marginRight: 8, minWidth: 40, display: 'inline-block' }}>
          {line.lineNumber}
        </span>
        <span style={{ color: line.type === 'add' ? '#52c41a' : line.type === 'remove' ? '#ff4d4f' : '#333' }}>
          {getLinePrefix()}{line.content}
        </span>
      </div>
    );
  };

  const renderContentTab = (content: string, title: string) => (
    <div style={{ maxHeight: 500, overflowY: 'auto', border: '1px solid #d9d9d9', borderRadius: 4 }}>
      <pre style={{ 
        padding: 16, 
        margin: 0, 
        fontFamily: 'Monaco, Consolas, "Courier New", monospace',
        fontSize: 13,
        lineHeight: 1.4,
        whiteSpace: 'pre-wrap'
      }}>
        {content}
      </pre>
    </div>
  );

  return (
    <Modal
      title={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <SwapOutlined />
          <span>版本对比: v{fromVersion} → v{toVersion}</span>
        </div>
      }
      open={visible}
      onCancel={onClose}
      width={900}
      footer={null}
      destroyOnClose
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: '40px 0' }}>
          <Spin size="large" />
          <div style={{ marginTop: 16 }}>
            <Text type="secondary">加载差异对比中...</Text>
          </div>
        </div>
      ) : error ? (
        <Alert
          message="加载失败"
          description={error}
          type="error"
          showIcon
        />
      ) : (
        <Tabs defaultActiveKey="diff">
          <TabPane tab={`差异对比 (${diffData.filter(d => d.type !== 'normal').length} 处变更)`} key="diff">
            <div style={{ maxHeight: 500, overflowY: 'auto', border: '1px solid #d9d9d9', borderRadius: 4 }}>
              {diffData.map(renderDiffLine)}
            </div>
            <div style={{ marginTop: 16, fontSize: 12, color: '#666' }}>
              <Text type="secondary">
                绿色行表示新增内容，红色行表示删除内容
              </Text>
            </div>
          </TabPane>
          <TabPane tab={`版本 ${fromVersion}`} key="from">
            {renderContentTab(fromContent, `版本 ${fromVersion}`)}
          </TabPane>
          <TabPane tab={`版本 ${toVersion}`} key="to">
            {renderContentTab(toContent, `版本 ${toVersion}`)}
          </TabPane>
        </Tabs>
      )}
    </Modal>
  );
};

export default DiffViewModal;