# MCP Resources 手动刷新功能

## 功能概述

在页面 Header 的 MCP Resources 管理器中添加了一个"刷新资源"按钮，用户可以点击该按钮手动刷新当前任务的 MCP 资源。

## 实现说明

### 后端实现

1. **新增 API 接口**: `POST /api/v1/user/resources/refresh`
   - 文件: `cmd/server/internal/api/resources_refresh.go`
   - 功能: 清除旧的自动资源，并根据当前任务重新添加资源
   - 权限: 需要 `task.read` 权限

2. **路由注册**: 
   - 文件: `cmd/server/main.go` (第 813 行)
   - 权限配置: `cmd/server/main.go` (第 420 行)

### 前端实现

1. **API 调用函数**: 
   - 文件: `frontend/src/api/resourceApi.ts`
   - 函数: `refreshUserResources()`

2. **UI 组件更新**:
   - 文件: `frontend/src/components/project/ContextManagerDropdown.tsx`
   - 添加了刷新按钮图标 `ReloadOutlined`
   - 实现了点击刷新的处理函数 `handleRefreshClick()`
   - 在下拉菜单中添加了刷新按钮（位于"新增资源"按钮下方）

## 使用方法

1. 在项目页面 Header 中找到"上下文管理 (MCP Resources)"按钮
2. 点击按钮展开下拉菜单
3. 在菜单顶部可以看到两个按钮：
   - **新增资源**: 手动添加自定义资源
   - **刷新资源**: 根据当前任务重新加载自动资源
4. 点击"刷新资源"按钮
5. 系统会：
   - 清除旧的自动添加的资源
   - 重新加载当前任务的所有文档资源（需求、设计、测试、执行计划等）
   - 重新加载项目级资源（架构设计、特性列表）
   - 重新加载任务关联的引用文档
6. 刷新成功后显示提示消息

## 使用场景

- 编辑或新增任务的需求文档后，点击刷新以更新 MCP Resources
- 编辑或新增任务的设计文档后，点击刷新以更新 MCP Resources
- 修改任务关联的引用文档后，点击刷新以更新 MCP Resources
- 任何时候觉得 MCP Resources 内容不是最新时，都可以手动刷新

## 注意事项

1. 刷新操作只对自动添加的资源有效（`auto_added=true`），手动添加的自定义资源不会被清除
2. 必须先选择当前任务才能进行刷新操作
3. 刷新操作会从文件系统重新读取文档内容，确保资源内容是最新的
4. 如果当前任务没有设置，刷新操作会返回错误提示

## 技术细节

### 后端处理流程

1. 获取当前用户的当前任务信息
2. 验证当前任务是否存在
3. 清除用户的所有自动添加的资源
4. 调用 `addTaskResources()` 重新添加任务相关资源：
   - 任务级文档: requirements, design, test, execution_plan
   - 项目级文档: architecture_design, feature_list
   - 任务引用文档: 通过 docHandler 获取关联的文档

### 前端处理流程

1. 用户点击刷新按钮
2. 调用 `refreshUserResources()` API
3. 显示加载状态
4. 等待后端处理完成
5. 重新加载资源列表以显示最新内容
6. 显示成功/失败提示消息

## 测试建议

1. 编辑一个任务的需求文档
2. 打开 MCP Resources 下拉菜单，查看当前资源内容
3. 点击"刷新资源"按钮
4. 再次查看资源内容，确认已更新为最新版本
5. 检查浏览器控制台和服务器日志，确认没有错误

## 相关文件

### 后端
- `cmd/server/internal/api/resources_refresh.go` - 新增的刷新接口
- `cmd/server/internal/api/resource_utils.go` - 资源管理工具函数
- `cmd/server/main.go` - 路由和权限配置

### 前端
- `frontend/src/api/resourceApi.ts` - API 调用函数
- `frontend/src/components/project/ContextManagerDropdown.tsx` - UI 组件
