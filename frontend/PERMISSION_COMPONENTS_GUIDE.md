# 权限组件使用指南

## 概述

本项目提供了两个核心权限组件用于前端权限控制:

- **PermissionGuard**: 权限守卫组件,用于根据权限条件渲染子组件
- **NoPermission**: 无权限提示组件,显示 403 错误页面

## PermissionGuard 组件

### 基本用法

```tsx
import { PermissionGuard } from '@/components/permission';
import { ScopeTaskWrite } from '@/constants/permissions';

// 单个权限检查
<PermissionGuard requiredPermission={ScopeTaskWrite}>
  <Button type="primary">编辑任务</Button>
</PermissionGuard>
```

### 多权限检查

```tsx
import { ScopeProjectDocWrite, ScopeTaskWrite } from '@/constants/permissions';

// 需要所有权限 (AND 逻辑)
<PermissionGuard 
  requiredPermission={[ScopeProjectDocWrite, ScopeTaskWrite]}
  requireAll={true}
>
  <Button>批量编辑</Button>
</PermissionGuard>

// 需要任意权限 (OR 逻辑)
<PermissionGuard 
  requiredPermission={[ScopeProjectDocRead, ScopeProjectDocWrite]}
  requireAll={false}
>
  <DocumentViewer />
</PermissionGuard>
```

### 自定义无权限显示

```tsx
<PermissionGuard 
  requiredPermission={ScopeTaskWrite}
  fallback={<Button disabled>无权限编辑</Button>}
>
  <Button type="primary">编辑任务</Button>
</PermissionGuard>
```

### 禁用 Loading 状态

```tsx
<PermissionGuard 
  requiredPermission={ScopeTaskWrite}
  showLoading={false}
>
  <ActionButton />
</PermissionGuard>
```

### API 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| requiredPermission | `string \| string[]` | 是 | - | 必需的权限范围 |
| requireAll | `boolean` | 否 | `true` | 数组权限时是否需要全部满足 |
| children | `React.ReactNode` | 是 | - | 有权限时渲染的内容 |
| fallback | `React.ReactNode` | 否 | `null` | 无权限时渲染的内容 |
| showLoading | `boolean` | 否 | `true` | 是否显示 loading 状态 |
| loadingComponent | `React.ReactNode` | 否 | `<Spin />` | 自定义 loading 组件 |

## NoPermission 组件

### 基本用法

```tsx
import { NoPermission } from '@/components/permission';
import { ScopeProjectDocWrite } from '@/constants/permissions';

// 单个权限提示
<NoPermission requiredPermission={ScopeProjectDocWrite} />

// 多个权限提示
<NoPermission 
  requiredPermission={[ScopeTaskWrite, ScopeTaskPlanApprove]}
/>
```

### 自定义描述文案

```tsx
<NoPermission 
  requiredPermission={ScopeTaskWrite}
  description="您需要任务编辑权限才能执行此操作。请联系管理员申请权限。"
/>
```

### 隐藏按钮

```tsx
<NoPermission 
  requiredPermission={ScopeTaskWrite}
  showBackButton={false}
  showViewPermissionButton={false}
/>
```

### 自定义跳转路径

```tsx
<NoPermission 
  requiredPermission={ScopeTaskWrite}
  backPath="/tasks"
  permissionPagePath="/settings/permissions"
/>
```

### API 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| requiredPermission | `string \| string[]` | 否 | - | 需要的权限范围 |
| description | `string` | 否 | 自动生成 | 自定义描述文案 |
| showBackButton | `boolean` | 否 | `true` | 是否显示返回按钮 |
| showViewPermissionButton | `boolean` | 否 | `true` | 是否显示查看权限按钮 |
| backPath | `string` | 否 | - | 自定义返回路径 |
| permissionPagePath | `string` | 否 | `/user-profile` | 查看权限页面路径 |

## 实际场景示例

### 场景 1: 任务列表操作按钮

```tsx
import { PermissionGuard } from '@/components/permission';
import { ScopeTaskWrite, ScopeTaskDelete } from '@/constants/permissions';

function TaskList() {
  return (
    <Table
      dataSource={tasks}
      columns={[
        // ... 其他列
        {
          title: '操作',
          render: (_, record) => (
            <Space>
              <PermissionGuard requiredPermission={ScopeTaskWrite}>
                <Button onClick={() => handleEdit(record)}>编辑</Button>
              </PermissionGuard>
              
              <PermissionGuard requiredPermission={ScopeTaskDelete}>
                <Button danger onClick={() => handleDelete(record)}>删除</Button>
              </PermissionGuard>
            </Space>
          ),
        },
      ]}
    />
  );
}
```

### 场景 2: 整页权限控制

```tsx
import { PermissionGuard, NoPermission } from '@/components/permission';
import { ScopeUserManage } from '@/constants/permissions';

function UserManagementPage() {
  return (
    <PermissionGuard 
      requiredPermission={ScopeUserManage}
      fallback={<NoPermission requiredPermission={ScopeUserManage} />}
    >
      <div>
        <h1>用户管理</h1>
        {/* 页面内容 */}
      </div>
    </PermissionGuard>
  );
}
```

### 场景 3: 菜单项权限控制

```tsx
import { usePermission } from '@/hooks/usePermission';
import { ScopeProjectDocRead, ScopeTaskRead } from '@/constants/permissions';

function AppMenu() {
  const { hasPermission } = usePermission();

  const menuItems = [
    {
      key: 'projects',
      label: '项目文档',
      visible: hasPermission(ScopeProjectDocRead),
    },
    {
      key: 'tasks',
      label: '任务管理',
      visible: hasPermission(ScopeTaskRead),
    },
  ].filter(item => item.visible);

  return <Menu items={menuItems} />;
}
```

### 场景 4: 表单字段权限控制

```tsx
import { PermissionGuard } from '@/components/permission';
import { ScopeTaskPlanApprove } from '@/constants/permissions';

function TaskForm() {
  return (
    <Form>
      <Form.Item label="任务名称" name="name">
        <Input />
      </Form.Item>
      
      <PermissionGuard requiredPermission={ScopeTaskPlanApprove}>
        <Form.Item label="审批状态" name="approvalStatus">
          <Select>
            <Option value="approved">已审批</Option>
            <Option value="rejected">已拒绝</Option>
          </Select>
        </Form.Item>
      </PermissionGuard>
    </Form>
  );
}
```

## 权限常量

所有权限常量定义在 `src/constants/permissions.ts`:

```typescript
// 项目文档权限
export const ScopeProjectDocRead = 'project.doc.read';
export const ScopeProjectDocWrite = 'project.doc.write';

// 任务权限
export const ScopeTaskRead = 'task.read';
export const ScopeTaskWrite = 'task.write';
export const ScopeTaskPlanApprove = 'task.plan.approve';

// 特性权限
export const ScopeFeatureRead = 'feature.read';
export const ScopeFeatureWrite = 'feature.write';

// 会议权限
export const ScopeMeetingRead = 'meeting.read';
export const ScopeMeetingWrite = 'meeting.write';

// 用户管理权限
export const ScopeUserManage = 'user.manage';
```

## 注意事项

1. **优先使用 PermissionGuard**: 对于组件级别的权限控制,优先使用 `PermissionGuard` 而不是手动调用 `usePermission`
2. **Loading 状态**: `PermissionGuard` 默认显示 loading 状态,对于不需要 loading 的场景(如按钮)可设置 `showLoading={false}`
3. **无权限提示**: 整页权限控制建议使用 `NoPermission` 组件作为 fallback,提供友好的用户体验
4. **权限缓存**: 权限信息有 5 分钟缓存,登录/登出时会自动刷新
5. **多权限检查**: 使用数组传入多个权限时,注意设置 `requireAll` 参数控制 AND/OR 逻辑
