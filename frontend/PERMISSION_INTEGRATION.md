# 前端权限系统集成说明

## ✅ 已完成集成

### 1. 主入口集成 (main.tsx)
```tsx
import { PermissionProvider } from './contexts/PermissionContext';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <PermissionProvider>
    <App />
  </PermissionProvider>
);
```

**作用**: 
- 在应用最外层包裹 `PermissionProvider`
- 全局提供权限上下文
- 自动加载用户权限档案
- 监听登录/登出事件自动刷新权限

---

### 2. 主页面集成 (App.tsx)

#### 2.1 导入权限组件
```tsx
import { RoleManagement } from './components/role/RoleManagement';
import { usePermission } from './hooks/usePermission';
import { ScopeUserManage } from './constants/permissions';
```

#### 2.2 用户管理视图升级为 Tabs
原来:
```tsx
<UserManagement />
```

现在:
```tsx
<Tabs
  items={[
    {
      key: 'users',
      label: <span><UserOutlined /> 用户管理</span>,
      children: <UserManagement />,
    },
    {
      key: 'roles',
      label: <span><SafetyOutlined /> 角色管理</span>,
      children: <RoleManagement />,
    },
  ]}
/>
```

**新增功能**:
- 用户管理页签 (原有功能)
- 角色管理页签 (新增)
  - 项目选择器
  - 角色列表
  - 创建/编辑/删除角色
  - 权限配置

---

## 📦 已集成的组件

### 权限基础设施
- ✅ `PermissionProvider` - 权限上下文提供者
- ✅ `usePermission` - 权限检查 Hook
- ✅ `PermissionGuard` - 权限守卫组件
- ✅ `NoPermission` - 无权限提示页面

### 角色管理
- ✅ `RoleManagement` - 角色管理主页面
- ✅ `RoleFormModal` - 角色创建/编辑表单
- ✅ `PermissionSelector` - 权限选择器

### 用户中心
- ✅ `UserProfile` - 个人中心页面 (独立组件,可添加到导航)
- ✅ `UserProjectRolesPanel` - 用户项目角色面板

---

## 🔧 如何使用

### 1. 在组件中检查权限

```tsx
import { usePermission } from './hooks/usePermission';
import { ScopeTaskWrite } from './constants/permissions';

function MyComponent() {
  const { hasPermission, loading } = usePermission();
  
  if (loading) return <Spin />;
  
  if (!hasPermission(ScopeTaskWrite)) {
    return <div>无权限</div>;
  }
  
  return <div>有权限的内容</div>;
}
```

### 2. 使用 PermissionGuard 组件

```tsx
import { PermissionGuard } from './components/permission/PermissionGuard';
import { ScopeTaskWrite } from './constants/permissions';

function MyComponent() {
  return (
    <PermissionGuard 
      requiredPermission={ScopeTaskWrite}
      fallback={<Button disabled>无权限编辑</Button>}
    >
      <Button type="primary">编辑任务</Button>
    </PermissionGuard>
  );
}
```

### 3. 在菜单中使用权限

```tsx
import { usePermission } from './hooks/usePermission';
import { ScopeUserManage } from './constants/permissions';

function Navigation() {
  const { hasPermission } = usePermission();
  
  const menuItems = [
    { key: 'home', label: '首页' },
    // 只有有权限的用户才能看到
    hasPermission(ScopeUserManage) && { 
      key: 'users', 
      label: '用户管理' 
    },
  ].filter(Boolean);
  
  return <Menu items={menuItems} />;
}
```

---

## 🎯 可选扩展

### 1. 添加个人中心入口

在 Header 添加用户菜单:

```tsx
import { UserProfile } from './components/UserProfile';

// 在 Header 右侧添加
<Dropdown
  menu={{
    items: [
      {
        key: 'profile',
        label: '个人中心',
        onClick: () => navigate('/profile'),
      },
      {
        key: 'logout',
        label: '退出登录',
        onClick: handleLogout,
      },
    ],
  }}
>
  <Avatar icon={<UserOutlined />} />
</Dropdown>
```

### 2. 在现有组件中添加权限控制

#### TaskSidebar (会议管理)
```tsx
import { PermissionGuard } from './permission/PermissionGuard';
import { ScopeMeetingWrite } from '../constants/permissions';

// 创建会议按钮
<PermissionGuard requiredPermission={ScopeMeetingWrite} showLoading={false}>
  <Button onClick={onCreate}>创建会议</Button>
</PermissionGuard>
```

#### TaskDocuments (任务文档)
```tsx
import { usePermission } from '../hooks/usePermission';
import { ScopeTaskWrite } from '../constants/permissions';

function TaskDocuments() {
  const { hasPermission } = usePermission();
  const canEdit = hasPermission(ScopeTaskWrite);
  
  return (
    <div>
      {canEdit && <Button>编辑文档</Button>}
    </div>
  );
}
```

### 3. 扩展 UserManagement 组件

在 `UserManagement.tsx` 中添加用户角色分配:

```tsx
import { UserProjectRolesPanel } from './UserProjectRolesPanel';

// 在用户详情面板添加
{selectedUser && (
  <>
    {/* 原有权限设置 */}
    <div>权限设置...</div>
    
    {/* 新增: 项目角色管理 */}
    <UserProjectRolesPanel username={selectedUser.username} />
  </>
)}
```

---

## 📊 权限常量

所有可用的权限 scope (在 `src/constants/permissions.ts`):

```typescript
// 项目文档
ScopeProjectDocRead = 'project.doc.read'
ScopeProjectDocWrite = 'project.doc.write'
// 项目管理
ScopeProjectAdmin = 'project.admin'

// 任务
ScopeTaskRead = 'task.read'
ScopeTaskWrite = 'task.write'
ScopeTaskPlanApprove = 'task.plan.approve'

// 特性
ScopeFeatureRead = 'feature.read'
ScopeFeatureWrite = 'feature.write'

// 会议
ScopeMeetingRead = 'meeting.read'
ScopeMeetingWrite = 'meeting.write'

// 用户管理
ScopeUserManage = 'user.manage'
```

---

## 🔍 验证集成

### 1. 编译验证
```bash
cd frontend && npm run build
# ✓ 6202 modules transformed.
# ✓ built in 10.11s
```

### 2. 功能验证清单
- ✅ 主页加载时自动获取用户权限
- ✅ 用户管理页面显示 "用户管理" 和 "角色管理" 两个页签
- ✅ 角色管理页面可以创建/编辑/删除角色
- ✅ 权限选择器正常工作
- ✅ 登录/登出时权限自动刷新
- ✅ 5分钟权限缓存机制生效

### 3. API 调用流程
1. 用户登录 → `onAuthChange` 触发
2. `PermissionProvider` 自动调用 `getUserProfile()`
3. 后端返回 `{ username, roles[], default_permissions[] }`
4. 提取所有 scopes 并缓存
5. 组件通过 `usePermission()` 获取权限
6. 5分钟后或登出时缓存失效

---

## 🎉 集成完成!

✅ **PermissionProvider** 已包裹整个应用
✅ **角色管理页面** 已添加到用户管理 Tabs
✅ **权限 Hook** 可在任何组件中使用
✅ **编译验证** 通过

下一步建议:
1. 在关键操作按钮添加 `PermissionGuard`
2. 在导航菜单中根据权限显示/隐藏菜单项
3. 添加个人中心入口
4. 扩展 UserManagement 组件添加角色分配功能
