import React from 'react';
import MeetingDocument from './meeting/MeetingDocument';

interface MeetingSummaryProps {
  taskId: string;
}

/**
 * 会议总结 - 使用统一的会议文档组件
 * 与项目/任务文档保持相同的 UI 风格
 */
export const MeetingSummary: React.FC<MeetingSummaryProps> = ({ taskId }) => {
  if (!taskId) {
    return (
      <div style={{ 
        height: '100%', 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'center',
        color: '#999'
      }}>
        请选择一个会议以查看会议总结
      </div>
    );
  }

  return (
    <MeetingDocument 
      meetingId={taskId} 
      slot="summary" 
      title="会议总结"
      color="#13c2c2"
    />
  );
};
