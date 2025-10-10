import React, { createContext, useContext, useState, useCallback } from 'react';

interface TaskRefreshContextType {
  refreshTrigger: number;
  triggerRefresh: () => void;
}

const TaskRefreshContext = createContext<TaskRefreshContextType | undefined>(undefined);

export const TaskRefreshProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  const triggerRefresh = useCallback(() => {
    setRefreshTrigger(prev => prev + 1);
  }, []);

  return (
    <TaskRefreshContext.Provider value={{ refreshTrigger, triggerRefresh }}>
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
