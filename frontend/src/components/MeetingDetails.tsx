import React from 'react';
import MeetingDocument from './meeting/MeetingDocument';

interface MeetingDetailsProps {
  taskId: string;
}

/**
 * 会议详情（润色记录）- 使用统一的会议文档组件
 * 与项目/任务文档保持相同的 UI 风格
 */
export const MeetingDetails: React.FC<MeetingDetailsProps> = ({ taskId }) => {
  if (!taskId) {
    return (
      <div style={{ 
        height: '100%', 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'center',
        color: '#999'
      }}>
        请选择一个会议以查看会议详情
      </div>
    );
  }

  return (
    <MeetingDocument 
      meetingId={taskId} 
      slot="polish" 
      title="会议详情"
      color="#722ed1"
    />
  );
};
