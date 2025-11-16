import React, { createContext, useContext, useState, useCallback } from 'react';

/**
 * 刷新事件类型
 * 用于精细控制不同类型的数据刷新
 */
export type RefreshEvent = 
  | 'task-list'           // 任务列表变更
  | 'task-detail'         // 任务详情变更
  | 'task-document'       // 任务文档变更
  | 'project-list'        // 项目列表变更
  | 'project-document'    // 项目文档变更
  | 'user-resource'       // 用户资源变更
  | 'execution-plan'      // 执行计划变更
  | 'task-summary'        // 任务总结变更
  | 'all';                // 全局刷新

interface RefreshTriggers {
  [key: string]: number;
}

interface TaskRefreshContextType {
  // 向后兼容：保留原有的全局刷新
  refreshTrigger: number;
  triggerRefresh: () => void;
  
  // 新增：细粒度刷新
  refreshTriggers: RefreshTriggers;
  triggerRefreshFor: (event: RefreshEvent) => void;
  
  // 批量刷新
  triggerRefreshForMultiple: (events: RefreshEvent[]) => void;
}

const TaskRefreshContext = createContext<TaskRefreshContextType | undefined>(undefined);

export const TaskRefreshProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [refreshTrigger, setRefreshTrigger] = useState(0);
  const [refreshTriggers, setRefreshTriggers] = useState<RefreshTriggers>({});

  // 全局刷新（向后兼容）
  const triggerRefresh = useCallback(() => {
    setRefreshTrigger(prev => prev + 1);
    // 同时触发 'all' 事件
    setRefreshTriggers(prev => ({
      ...prev,
      all: (prev.all || 0) + 1
    }));
  }, []);

  // 细粒度刷新
  const triggerRefreshFor = useCallback((event: RefreshEvent) => {
    setRefreshTriggers(prev => ({
      ...prev,
      [event]: (prev[event] || 0) + 1,
      // 如果是 'all' 事件，也触发全局刷新
      ...(event === 'all' ? { all: (prev.all || 0) + 1 } : {})
    }));
    
    // 如果是 'all' 事件，同时触发全局刷新计数器
    if (event === 'all') {
      setRefreshTrigger(prev => prev + 1);
    }
  }, []);

  // 批量刷新
  const triggerRefreshForMultiple = useCallback((events: RefreshEvent[]) => {
    setRefreshTriggers(prev => {
      const updates: RefreshTriggers = { ...prev };
      events.forEach(event => {
        updates[event] = (updates[event] || 0) + 1;
      });
      return updates;
    });
  }, []);

  return (
    <TaskRefreshContext.Provider value={{ 
      refreshTrigger, 
      triggerRefresh,
      refreshTriggers,
      triggerRefreshFor,
      triggerRefreshForMultiple
    }}>
      {children}
    </TaskRefreshContext.Provider>
  );
};

export const useTaskRefresh = () => {
  const context = useContext(TaskRefreshContext);
  if (context === undefined) {
    throw new Error('useTaskRefresh must be used within a TaskRefreshProvider');
  }
  return context;
};

/**
 * 自定义 Hook：监听特定事件的刷新
 * @param event 要监听的刷新事件
 * @returns 刷新计数器
 * 
 * @example
 * const taskListRefresh = useRefreshTrigger('task-list');
 * useEffect(() => {
 *   loadTaskList();
 * }, [taskListRefresh]);
 */
export const useRefreshTrigger = (event: RefreshEvent) => {
  const { refreshTriggers } = useTaskRefresh();
  return refreshTriggers[event] || 0;
};

/**
 * 自定义 Hook：监听多个事件的刷新
 * @param events 要监听的刷新事件数组
 * @returns 合并的刷新计数器
 * 
 * @example
 * const refresh = useRefreshTriggerMultiple(['task-list', 'task-detail', 'all']);
 * useEffect(() => {
 *   loadData();
 * }, [refresh]);
 */
export const useRefreshTriggerMultiple = (events: RefreshEvent[]) => {
  const { refreshTriggers } = useTaskRefresh();
  // 返回所有事件计数器的总和
  return events.reduce((sum, event) => sum + (refreshTriggers[event] || 0), 0);
};
