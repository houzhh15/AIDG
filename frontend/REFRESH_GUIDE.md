# 页面数据刷新使用指南

## 📋 概述

项目使用全局刷新上下文 `TaskRefreshContext` 来管理页面数据刷新，支持：
- ✅ 全局刷新（向后兼容）
- ✅ 细粒度刷新（按事件类型）
- ✅ 批量刷新（同时刷新多个事件）

## 🎯 刷新事件类型

```typescript
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
```

## 📖 使用方法

### 1️⃣ 监听刷新事件（数据加载侧）

#### 方法A：监听单个事件

```tsx
import { useRefreshTrigger } from '../contexts/TaskRefreshContext';

function TaskList() {
  const taskListRefresh = useRefreshTrigger('task-list');
  
  useEffect(() => {
    loadTaskList(); // 重新加载数据
  }, [taskListRefresh]); // 当 task-list 事件触发时刷新
  
  return <div>...</div>;
}
```

#### 方法B：监听多个事件

```tsx
import { useRefreshTriggerMultiple } from '../contexts/TaskRefreshContext';

function TaskDocuments() {
  // 监听任务文档、任务详情、全局刷新
  const refresh = useRefreshTriggerMultiple(['task-document', 'task-detail', 'all']);
  
  useEffect(() => {
    loadDocuments();
  }, [refresh]);
  
  return <div>...</div>;
}
```

#### 方法C：使用原有的全局刷新（向后兼容）

```tsx
import { useTaskRefresh } from '../contexts/TaskRefreshContext';

function ProjectTaskSelector() {
  const { refreshTrigger } = useTaskRefresh();
  
  useEffect(() => {
    loadTasks();
  }, [refreshTrigger]);
  
  return <div>...</div>;
}
```

### 2️⃣ 触发刷新事件（数据变更侧）

#### 方法A：触发单个事件

```tsx
import { useTaskRefresh } from '../contexts/TaskRefreshContext';

function TaskEditor() {
  const { triggerRefreshFor } = useTaskRefresh();
  
  const handleSave = async () => {
    await saveTask();
    triggerRefreshFor('task-detail'); // 触发任务详情刷新
    message.success('保存成功');
  };
  
  return <Button onClick={handleSave}>保存</Button>;
}
```

#### 方法B：触发多个事件

```tsx
import { useTaskRefresh } from '../contexts/TaskRefreshContext';

function TaskDocumentEditor() {
  const { triggerRefreshForMultiple } = useTaskRefresh();
  
  const handleSave = async () => {
    await saveDocument();
    // 同时触发任务文档和任务详情刷新
    triggerRefreshForMultiple(['task-document', 'task-detail']);
    message.success('保存成功');
  };
  
  return <Button onClick={handleSave}>保存</Button>;
}
```

#### 方法C：触发全局刷新

```tsx
import { useTaskRefresh } from '../contexts/TaskRefreshContext';

function CreateProjectButton() {
  const { triggerRefreshFor } = useTaskRefresh();
  
  const handleCreate = async () => {
    await createProject();
    triggerRefreshFor('all'); // 触发全局刷新
    message.success('创建成功');
  };
  
  return <Button onClick={handleCreate}>创建项目</Button>;
}
```

#### 方法D：使用原有的全局刷新（向后兼容）

```tsx
import { useTaskRefresh } from '../contexts/TaskRefreshContext';

function TaskStatusButton() {
  const { triggerRefresh } = useTaskRefresh();
  
  const handleStatusChange = async () => {
    await updateTaskStatus();
    triggerRefresh(); // 原有方式仍然可用
    message.success('状态已更新');
  };
  
  return <Button onClick={handleStatusChange}>更新状态</Button>;
}
```

## 🎨 常见场景示例

### 场景1：任务CRUD操作

```tsx
// TaskSidebar.tsx - 创建/删除任务
const { triggerRefreshFor } = useTaskRefresh();

const handleCreateTask = async () => {
  await createTask();
  triggerRefreshFor('task-list'); // 刷新任务列表
};

const handleDeleteTask = async () => {
  await deleteTask();
  triggerRefreshFor('task-list'); // 刷新任务列表
};
```

### 场景2：文档编辑保存

```tsx
// SectionEditor.tsx - 保存章节
const { triggerRefreshForMultiple } = useTaskRefresh();

const handleSave = async () => {
  await saveSection();
  // 刷新任务文档和任务详情
  triggerRefreshForMultiple(['task-document', 'task-detail']);
  message.success('保存成功');
};
```

### 场景3：项目交付物更新

```tsx
// ProjectFeatureList.tsx - 保存特性列表
const { triggerRefreshFor } = useTaskRefresh();

const handleSave = async () => {
  await saveFeatureList();
  triggerRefreshFor('project-document'); // 刷新项目文档
  message.success('保存成功');
};
```

### 场景4：MCP资源管理

```tsx
// ResourceEditorModal.tsx - 创建/更新资源
const { triggerRefreshFor } = useTaskRefresh();

const handleSubmit = async () => {
  await saveResource();
  triggerRefreshFor('user-resource'); // 刷新用户资源
  message.success('保存成功');
  onClose();
};
```

### 场景5：执行计划更新

```tsx
// ExecutionPlanView.tsx - 更新执行计划
const { triggerRefreshFor } = useTaskRefresh();

const handleUpdateStep = async () => {
  await updatePlanStep();
  triggerRefreshFor('execution-plan'); // 刷新执行计划
  message.success('更新成功');
};
```

## 🔧 迁移现有代码

### 步骤1：识别数据变更点

找到所有会修改数据的操作：
- 创建/更新/删除操作
- 保存/提交操作
- 状态变更操作

### 步骤2：添加刷新触发

在数据变更成功后，调用 `triggerRefreshFor()` 或 `triggerRefreshForMultiple()`：

```tsx
// 修改前
const handleSave = async () => {
  await saveData();
  message.success('保存成功');
};

// 修改后
const handleSave = async () => {
  await saveData();
  triggerRefreshFor('task-document'); // 👈 添加刷新触发
  message.success('保存成功');
};
```

### 步骤3：添加刷新监听

在需要刷新的组件中，监听对应的刷新事件：

```tsx
// 修改前
useEffect(() => {
  loadData();
}, [projectId, taskId]);

// 修改后
const refresh = useRefreshTrigger('task-document'); // 👈 添加刷新监听
useEffect(() => {
  loadData();
}, [projectId, taskId, refresh]); // 👈 添加到依赖数组
```

## 📌 最佳实践

1. **精确匹配事件类型**：根据数据类型选择合适的事件，避免过度刷新
2. **批量刷新优化**：如果一个操作影响多个数据，使用 `triggerRefreshForMultiple`
3. **避免循环刷新**：不要在刷新回调中再次触发同一个刷新事件
4. **向后兼容**：现有的 `triggerRefresh()` 和 `refreshTrigger` 仍然可用
5. **调试技巧**：可以在触发刷新时添加 console.log 追踪刷新链路

## 🐛 常见问题

**Q: 为什么刷新没有生效？**
- 检查是否在 `useEffect` 的依赖数组中添加了刷新计数器
- 检查刷新事件类型是否匹配（触发侧和监听侧）

**Q: 页面刷新太频繁怎么办？**
- 使用更精确的事件类型，避免使用 'all'
- 检查是否有重复的刷新触发

**Q: 如何调试刷新逻辑？**
```tsx
const refresh = useRefreshTrigger('task-list');
useEffect(() => {
  console.log('task-list refresh triggered:', refresh);
  loadData();
}, [refresh]);
```

## 📚 参考

- 源码: `frontend/src/contexts/TaskRefreshContext.tsx`
- 现有使用示例: `frontend/src/components/ProjectTaskSidebar.tsx`
