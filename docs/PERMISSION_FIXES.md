# 权限系统修复说明

## 修复日期
2025年10月8日

## 问题描述

### 问题1：Admin 用户权限显示不全
- **现象**：Admin 用户实际拥有 14 个权限，但前端创建角色时只能选择 3 个权限类别
- **影响**：无法为新角色分配会议管理和用户管理权限

### 问题2：无项目时显示误导性 403 错误
- **现象**：当系统中没有任何项目时，admin 用户访问项目页面和角色页面会显示 403 权限错误
- **影响**：用户误以为是权限问题，实际上是因为没有项目数据

## Admin 用户创建机制

### 创建流程
1. **代码位置**：`cmd/server/internal/users/manager.go` 第 136 行
2. **函数**：`EnsureDefaultAdmin(defaultPassword string)`
3. **触发时机**：服务器启动时（`cmd/server/main.go` 第 118 行）
4. **条件**：仅在用户表为空时创建

### 密码来源
1. **优先级**：环境变量 `ADMIN_DEFAULT_PASSWORD`（docker-compose.yml 中设置为 `admin123`）
2. **备用值**：代码中硬编码的 `neteye@123`（仅在创建其他用户时使用）

### Admin 权限
Admin 用户自动获得所有权限（`allScopes`），包括：

**新权限系统（8个有效权限）：**
- `project.doc.read` - 项目文档读取
- `project.doc.write` - 项目文档编辑
- `task.read` - 任务读取
- `task.write` - 任务编辑
- `task.plan.approve` - 执行计划审批
- `meeting.read` - 会议读取
- `meeting.write` - 会议编辑
- `user.manage` - 用户管理

**旧权限（6个，向后兼容）：**
- `feature.read` / `feature.write` - 特性管理（已弃用，使用 project.doc.* 代替）
- `architecture.read` / `architecture.write` - 架构设计（已弃用）
- `tech.read` / `tech.write` - 技术设计（已弃用）

## 修复方案

### 修复1：完善前端权限分组显示

**文件**：`frontend/src/constants/permissions.ts`

**修改内容**：
在 `PermissionGroups` 数组中添加了"用户管理"分组：

```typescript
{
  title: '用户管理',
  scopes: [
    {
      label: '用户管理',
      value: ScopeUserManage,
      description: '管理用户、角色、权限（系统级权限）',
    },
  ],
}
```

**效果**：
- 前端现在可以显示所有 10 个有效权限（包括 8 个新权限 + 2 个保留的特性管理权限）
- 创建角色时可以分配会议管理和用户管理权限

### 修复2：优化角色页面空状态处理

**文件**：`frontend/src/components/role/RoleManagement.tsx`

**修改1：错误处理**
```typescript
// 如果是 403 权限错误，不显示错误提示，让页面显示空状态
if (error?.response?.status !== 403) {
  message.error('加载项目列表失败: ' + error.message);
}
// 权限不足时设置空项目列表
setProjects([]);
```

**修改2：左侧项目树空状态**
```typescript
{projects.length === 0 && !loadingProjects ? (
  <div style={{ padding: '16px', textAlign: 'center', color: '#999' }}>
    <Empty 
      description="暂无项目" 
      image={Empty.PRESENTED_IMAGE_SIMPLE}
    />
  </div>
) : (
  <Tree ... />
)}
```

**修改3：右侧内容区空状态**
```typescript
{!selectedProjectId ? (
  <Empty 
    description={projects.length === 0 ? "暂无可用项目，请先创建项目" : "请先在左侧选择项目"} 
    image={Empty.PRESENTED_IMAGE_SIMPLE}
  />
) : (
  <Table ... />
)}
```

**效果**：
- 没有项目时显示友好的"暂无项目"提示
- 不再显示误导性的 403 错误
- 引导用户先创建项目

### 修复3：将项目文档权限添加到全局权限白名单 ⭐ 根本原因修复

**文件**：`cmd/server/main.go`

**问题根源**：
后端权限检查逻辑中有一个全局权限白名单（第 453-459 行），只有白名单中的权限才能在没有项目上下文时使用。原来的白名单只包含 `user.manage`、`meeting.read`、`meeting.write` 三个权限，而 `project.doc.read` 和 `project.doc.write` 不在白名单中。

这导致：
1. Admin 用户访问 `/api/v1/projects` API 时，即使 JWT token 中有 `project.doc.read` 权限，也会被过滤掉
2. 后端返回 403 Forbidden 错误
3. 前端虽然不显示错误消息，但浏览器控制台仍会显示 HTTP 403 错误

**修改内容**：
```go
// 权限计算：区分全局权限和项目权限
// 全局权限白名单：只有这些权限可以跨项目使用
globalScopesWhitelist := map[string]bool{
	"user.manage":        true, // 用户管理
	"meeting.read":       true, // 会议记录读取
	"meeting.write":      true, // 会议记录写入
	"project.doc.read":   true, // 项目文档读取（允许查看项目列表）✅ 新增
	"project.doc.write":  true, // 项目文档写入（允许创建项目）✅ 新增
	"feature.read":       true, // 旧版特性读取（向后兼容）✅ 新增
	"feature.write":      true, // 旧版特性写入（向后兼容）✅ 新增
}
```

**效果**：
- Admin 用户可以正常访问 `/api/v1/projects` API
- **不再有任何 403 错误**（包括浏览器控制台）
- 拥有 `project.doc.read` 权限的用户可以查看项目列表
- 拥有 `project.doc.write` 权限的用户可以创建新项目

## 权限系统架构说明

### 两层权限体系

1. **系统级权限（User Scopes）**
   - 存储在 `users/users.json` 中
   - 通过 JWT token 传递
   - 用于全局功能访问控制（如用户管理、系统设置）
   - Admin 用户拥有所有系统级权限

2. **项目级权限（Role-based Permissions）**
   - 存储在 `projects/{project_id}/roles/` 目录
   - 通过用户-角色-权限映射实现
   - 用于项目内资源访问控制
   - 需要先创建项目和角色

### 权限检查逻辑

**后端**（`cmd/server/main.go`）：
```go
// 系统级权限检查
routeScopes := map[string][]string{
  "POST /api/v1/roles": {users.ScopeUserManage},
  "GET /api/v1/roles": {users.ScopeUserManage},
  // ...
}

// 项目级权限检查
projectScopes, err := userRoleService.ComputeEffectiveScopes(username, projectID)
```

**前端**（`frontend/src/hooks/usePermission.ts`）：
```typescript
const { hasPermission, hasAnyPermission, hasAllPermissions } = usePermission();
```

## 测试验证

### 使用 Playwright 测试（已验证 ✅）

**测试环境**：http://localhost:8000

**测试步骤**：

1. ✅ **登录测试**
   - 用户名：`admin`
   - 密码：`admin123`
   - 结果：登录成功，无错误提示

2. ✅ **角色管理页面测试**
   - 进入"用户" > "角色管理"
   - 验证左侧显示"暂无项目"空状态（而非错误）
   - 验证右侧显示"暂无可用项目，请先创建项目"
   - **验证浏览器控制台无 403 错误** ⭐

3. ✅ **项目页面测试**
   - 切换到"项目"标签
   - 验证显示"请选择或创建一个项目"
   - 验证侧边栏显示"No data"空状态
   - **验证浏览器控制台无 403 错误** ⭐

4. ✅ **网络请求测试**
   - 检查 `/api/v1/projects` 请求
   - 之前：返回 403 Forbidden
   - 现在：返回 200 OK（空数组）

### 登录凭据
- **用户名**：`admin`
- **密码**：`admin123`

### 手动验证步骤

1. **权限显示验证**
   - 进入"系统管理" > "用户管理"
   - 点击"创建用户"或"分配角色"
   - 验证权限选择器显示 4 个分组：项目文档、任务管理、特性管理、会议管理
   - 如果创建全局角色，应该能看到"用户管理"权限

2. **空状态验证**
   - 进入"系统管理" > "角色管理"
   - 如果没有项目，应该显示"暂无项目，请先创建项目"
   - 不应该显示 403 错误或红色错误提示
   - **打开浏览器开发者工具 Console，验证无 403 错误**

3. **创建项目后验证**
   - 在"项目"标签页创建一个新项目
   - 返回"角色管理"页面
   - 左侧项目树应该显示新创建的项目
   - 选择项目后可以创建和管理角色

## 已知限制

1. **旧权限兼容性**
   - 代码中保留了 6 个已弃用的权限（feature.*, architecture.*, tech.*）
   - 这些权限不在前端权限选择器中显示
   - 已存在的使用这些权限的角色仍然有效

2. **项目级权限依赖**
   - 角色管理、项目权限等功能需要先创建项目
   - Admin 的系统级权限不会自动赋予项目级权限
   - 需要为 admin 用户在每个项目中分配角色

3. **权限粒度**
   - 当前权限是粗粒度的（读/写级别）
   - 未来可能需要更细粒度的权限控制（如按功能模块）

## 部署说明

### 本次修复涉及的文件
1. `frontend/src/constants/permissions.ts` - 权限分组定义
2. `frontend/src/components/role/RoleManagement.tsx` - 角色管理页面
3. `cmd/server/main.go` - 后端权限检查逻辑 ⭐ **关键修复**

### 部署步骤
```bash
# 1. 重新构建前端（如果只修改了前端）
cd frontend
npm run build

# 2. 重新构建后端并部署（必须，因为修改了权限逻辑）
cd ..
docker compose build aidg

# 3. 重启容器
docker compose up -d
```

### 验证部署
```bash
# 检查容器状态
docker compose ps

# 查看日志
docker compose logs -f aidg

# 访问应用
# http://localhost:8000

# 测试 API（应该返回 200）
curl -H "Authorization: Bearer YOUR_TOKEN" http://localhost:8000/api/v1/projects
```

### 部署检查清单
- [ ] 后端编译成功
- [ ] 容器启动成功
- [ ] 登录页面可访问
- [ ] 使用 admin/admin123 登录成功
- [ ] 访问角色管理页面无 403 错误
- [ ] 浏览器控制台无 403 错误
- [ ] 项目列表页面正常显示空状态

## 后续优化建议

1. **创建示例项目**
   - 考虑在首次部署时自动创建一个示例项目
   - 避免新用户看到空状态

2. **权限预设模板**
   - 提供常用角色模板（管理员、开发者、查看者等）
   - 简化角色创建流程

3. **权限文档**
   - 在前端添加权限说明的帮助文档
   - 解释每个权限的具体作用和影响范围

4. **默认角色**
   - 为每个新项目自动创建默认角色（如 Owner、Member）
   - 自动为项目创建者分配 Owner 角色

## 相关文档

- [权限组件集成指南](./PERMISSION_INTEGRATION.md)
- [权限组件开发指南](./PERMISSION_COMPONENTS_GUIDE.md)
- [合规报告](./COMPLIANCE_REPORT.md)

## 更新历史

- **2025-10-08 17:00**：修复 403 错误根本原因 - 将项目文档权限添加到全局权限白名单 ⭐
  - 修改了后端权限检查逻辑（`cmd/server/main.go`）
  - 使用 Playwright 测试验证，确认浏览器控制台无任何 403 错误
  - Admin 用户可以正常访问项目列表 API
  
- **2025-10-08 14:00**：修复权限显示和空状态问题
  - 前端权限分组完善
  - 角色管理页面空状态优化
  
- **2025-10-07**：创建初始 admin 用户
