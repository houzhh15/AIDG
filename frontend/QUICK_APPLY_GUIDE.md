# 快速应用示例

这是一份快速参考，展示如何在现有组件中快速应用刷新机制。

## 🚀 快速改造步骤

### 第1步：导入 Hook

```tsx
// 在文件顶部添加导入
import { useRefreshTrigger, useTaskRefresh } from '../contexts/TaskRefreshContext';
```

### 第2步：在组件中使用

```tsx
function YourComponent() {
  // 方案A：只需要监听刷新
  const refresh = useRefreshTrigger('task-document');
  
  // 方案B：需要触发刷新
  const { triggerRefreshFor } = useTaskRefresh();
  
  // 方案C：两者都需要
  const refresh = useRefreshTrigger('task-document');
  const { triggerRefreshFor } = useTaskRefresh();
  
  // ...rest of component
}
```

### 第3步：监听刷新（在 useEffect 中）

```tsx
useEffect(() => {
  loadData();
}, [projectId, taskId, refresh]); // 👈 添加 refresh 到依赖数组
```

### 第4步：触发刷新（在保存/更新后）

```tsx
const handleSave = async () => {
  await saveData();
  triggerRefreshFor('task-document'); // 👈 保存后触发刷新
  message.success('保存成功');
};
```

## 📝 常见组件改造模板

### 模板1：任务文档组件

```tsx
import { useRefreshTrigger, useTaskRefresh } from '../contexts/TaskRefreshContext';

function TaskDocumentEditor({ projectId, taskId }) {
  const refresh = useRefreshTrigger('task-document');
  const { triggerRefreshForMultiple } = useTaskRefresh();
  
  useEffect(() => {
    loadDocument();
  }, [projectId, taskId, refresh]);
  
  const handleSave = async () => {
    await saveDocument();
    triggerRefreshForMultiple(['task-document', 'task-detail']);
    message.success('保存成功');
  };
  
  return <div>...</div>;
}
```

### 模板2：任务列表组件

```tsx
import { useRefreshTrigger } from '../contexts/TaskRefreshContext';

function TaskList({ projectId }) {
  const refresh = useRefreshTrigger('task-list');
  
  useEffect(() => {
    loadTasks();
  }, [projectId, refresh]);
  
  return <div>...</div>;
}
```

### 模板3：任务CRUD操作组件

```tsx
import { useTaskRefresh } from '../contexts/TaskRefreshContext';

function TaskActions({ projectId, taskId }) {
  const { triggerRefreshFor } = useTaskRefresh();
  
  const handleCreate = async () => {
    await createTask();
    triggerRefreshFor('task-list');
    message.success('创建成功');
  };
  
  const handleDelete = async () => {
    await deleteTask(taskId);
    triggerRefreshFor('task-list');
    message.success('删除成功');
  };
  
  const handleStatusChange = async (status) => {
    await updateTaskStatus(taskId, status);
    triggerRefreshFor('task-detail');
    message.success('状态已更新');
  };
  
  return <div>...</div>;
}
```

### 模板4：执行计划组件

```tsx
import { useRefreshTrigger, useTaskRefresh } from '../contexts/TaskRefreshContext';

function ExecutionPlan({ projectId, taskId }) {
  const refresh = useRefreshTrigger('execution-plan');
  const { triggerRefreshFor } = useTaskRefresh();
  
  useEffect(() => {
    loadPlan();
  }, [projectId, taskId, refresh]);
  
  const handleUpdateStep = async (stepId, updates) => {
    await updatePlanStep(projectId, taskId, stepId, updates);
    triggerRefreshFor('execution-plan');
    message.success('更新成功');
  };
  
  return <div>...</div>;
}
```

### 模板5：项目文档组件

```tsx
import { useRefreshTrigger, useTaskRefresh } from '../../contexts/TaskRefreshContext';

function ProjectDocument({ projectId }) {
  const refresh = useRefreshTrigger('project-document');
  const { triggerRefreshFor } = useTaskRefresh();
  
  useEffect(() => {
    loadDocument();
  }, [projectId, refresh]);
  
  const handleSave = async () => {
    await saveProjectDocument();
    triggerRefreshFor('project-document');
    message.success('保存成功');
  };
  
  return <div>...</div>;
}
```

## 🎯 需要优先改造的组件列表

### 高优先级（数据变更频繁）

1. **TaskDocuments.tsx**
   - 监听: `task-document`, `task-detail`
   - 触发: 保存文档后触发 `task-document`

2. **SectionEditor.tsx** ✅ 已完成
   - 监听: 无需监听（Modal形式）
   - 触发: 保存后触发 `task-document`, `task-detail`

3. **ProjectTaskSidebar.tsx** ✅ 已部分完成
   - 监听: `task-list`
   - 触发: 创建/删除/更新任务后触发 `task-list`

4. **ExecutionPlanView.tsx**
   - 监听: `execution-plan`
   - 触发: 更新步骤后触发 `execution-plan`

### 中优先级（数据变更较少）

5. **ProjectFeatureList.tsx** ✅ 已完成
   - 监听: `project-document`
   - 触发: 保存后触发 `project-document`

6. **ProjectArchitectureDesign.tsx**
   - 监听: `project-document`
   - 触发: 保存后触发 `project-document`

7. **ResourcesManagement.tsx**
   - 监听: `user-resource`
   - 触发: 创建/更新/删除资源后触发 `user-resource`

8. **TaskSummaryPanel.tsx**
   - 监听: `task-summary`
   - 触发: 创建/更新总结后触发 `task-summary`

### 低优先级（读取为主）

9. **TaskDashboard.tsx**
   - 监听: `task-list`, `task-detail`, `all`
   - 触发: 无

10. **DocumentTOC.tsx**
    - 监听: `task-document`
    - 触发: 无

## ⚡ 一键批量搜索替换

### 查找需要改造的保存操作

使用 VS Code 全局搜索：

```regex
(message\.success|message\.info).*('保存成功'|'已保存'|'更新成功'|'创建成功')
```

### 查找需要监听的 useEffect

使用 VS Code 全局搜索：

```regex
useEffect.*\(\(\)\s*=>\s*\{[\s\S]*?load
```

## 📊 改造进度检查清单

- [x] TaskRefreshContext 升级
- [x] SectionEditor
- [x] ResourceEditorModal
- [x] ProjectFeatureList
- [ ] TaskDocuments
- [ ] ExecutionPlanView
- [ ] ProjectArchitectureDesign
- [ ] ProjectTechDesign
- [ ] TaskSummaryPanel
- [ ] ContextManagerDropdown

## 🔍 调试技巧

### 1. 追踪刷新事件

在组件中添加日志：

```tsx
const refresh = useRefreshTrigger('task-document');

useEffect(() => {
  console.log('[YourComponent] Refresh triggered:', refresh);
  loadData();
}, [refresh]);
```

### 2. 监控所有刷新事件

在 TaskRefreshContext 中临时添加：

```tsx
const triggerRefreshFor = useCallback((event: RefreshEvent) => {
  console.log('[TaskRefreshContext] Triggering refresh for:', event);
  // ...rest of code
}, []);
```

### 3. 检查刷新链路

```
用户操作 
  → handleSave 
    → API 调用 
      → triggerRefreshFor('task-document') 
        → 其他组件的 useEffect 触发 
          → loadData()
```

## 💡 小贴士

1. **优先使用细粒度事件**：避免使用 `'all'`，除非真的需要全局刷新
2. **避免循环刷新**：不要在 useEffect 回调中触发同一个事件
3. **批量操作使用 triggerRefreshForMultiple**：一次性触发多个事件
4. **保持向后兼容**：原有的 `triggerRefresh()` 仍然可用
5. **测试刷新逻辑**：保存后检查其他组件是否正确刷新
