import React, { useState } from 'react';
import { Tabs } from 'antd';
import { FileTextOutlined, DatabaseOutlined, BookOutlined, FileSearchOutlined, CheckCircleOutlined } from '@ant-design/icons';
import { ChunkDetailTabs } from './ChunkDetailTabs';
import { MeetingContext } from './MeetingContext';
import { MeetingDetails } from './MeetingDetails';
import { MeetingSummary } from './MeetingSummary';
import { Deliverables } from './Deliverables';

const { TabPane } = Tabs;

interface RightPanelProps {
  taskId: string;
  chunkId?: string;
  canWriteMeeting?: boolean; // 是否有 meeting.write 权限
  canReadMeeting?: boolean; // 是否拥有 meeting.read 权限（write 隐式包含）
}

export const RightPanel: React.FC<RightPanelProps> = ({ taskId, chunkId, canWriteMeeting, canReadMeeting }) => {
  const [activeTab, setActiveTab] = useState<string>('context');
  const allowWrite = !!canWriteMeeting;
  const allowRead = canReadMeeting ?? allowWrite;

  const items = [
    {
      key: 'context',
      label: (
        <span>
          <FileTextOutlined />
          会议背景
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'hidden' }}>
          <MeetingContext taskId={taskId} />
        </div>
      ),
    },
    {
      key: 'details',
      label: (
        <span>
          <BookOutlined />
          会议详情
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'hidden' }}>
          <MeetingDetails taskId={taskId} />
        </div>
      ),
    },
    {
      key: 'summary',
      label: (
        <span>
          <FileSearchOutlined />
          会议总结
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'hidden' }}>
          <MeetingSummary taskId={taskId} />
        </div>
      ),
    },
    {
      key: 'features',
      label: (
        <span>
          <CheckCircleOutlined />
          成果物
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'hidden' }}>
          <Deliverables taskId={taskId} />
        </div>
      ),
    },
    allowRead ? {
      key: 'chunks',
      label: (
        <span>
          <DatabaseOutlined />
          Chunk详情
        </span>
      ),
      children: (
        <div style={{ height: '100%', overflow: 'hidden' }}>
          <ChunkDetailTabs taskId={taskId} chunkId={chunkId} canWriteMeeting={allowWrite} canReadMeeting={allowRead} />
        </div>
      ),
    } : null,
  ];

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={items.filter(Boolean) as any}
        style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}
        tabBarStyle={{ marginBottom: 16, paddingLeft: 8, paddingRight: 8, flexShrink: 0 }}
      />
    </div>
  );
};
