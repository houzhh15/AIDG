# 服务状态感知的 UI 动态隐藏功能

## 📋 需求说明

如果用户未部署 `go-whisper` 容器和 `aidg-deps-service` 容器，则在前端页面的会议视图中：
- 隐藏**中间栏**（Chunk 列表栏）
- 隐藏**右侧区域**中的 **Chunk 详情 Tab 页**

## ✅ 实现方案

### 1. 后端实现

#### 1.1 新增服务状态 API

**文件**: `cmd/server/internal/api/services_status.go`

```go
// ServicesStatusResponse 服务状态响应
type ServicesStatusResponse struct {
	WhisperAvailable    bool   `json:"whisper_available"`
	DepsServiceAvailable bool   `json:"deps_service_available"`
	WhisperMode         string `json:"whisper_mode,omitempty"`
	DependencyMode      string `json:"dependency_mode,omitempty"`
}

// HandleServicesStatus 返回当前服务的部署状态
// GET /api/v1/services/status
func HandleServicesStatus(orch *orchestrator.Orchestrator) gin.HandlerFunc
```

**API 端点**: `GET /api/v1/services/status`

**响应示例**:
```json
{
  "whisper_available": true,
  "deps_service_available": true
}
```

#### 1.2 注册 API 路由

**文件**: `cmd/server/main.go` (约第 797 行)

```go
// ========== Services Status API ==========
// 检查服务部署状态（whisper 和 deps-service）
r.GET("/api/v1/services/status", func(c *gin.Context) {
    // 从meetingsReg获取任意一个orchestrator实例
    var activeOrch *orchestrator.Orchestrator
    for _, task := range meetingsReg.List() {
        if task.Orch != nil {
            activeOrch = task.Orch
            break
        }
    }

    // 调用status handler
    handler := api.HandleServicesStatus(activeOrch)
    handler(c)
})
```

**检测逻辑**:
- 通过 `Orchestrator.GetHealthChecker()` 检测 Whisper 服务
- 通过 `Orchestrator.GetDependencyClient()` 检测 deps-service

---

### 2. 前端实现

#### 2.1 API 客户端

**文件**: `frontend/src/api/client.ts`

```typescript
// Services status
export interface ServicesStatus {
  whisper_available: boolean;
  deps_service_available: boolean;
  whisper_mode?: string;
  dependency_mode?: string;
}

export async function getServicesStatus(): Promise<ServicesStatus> {
  const r = await api.get('/services/status');
  return r.data;
}
```

#### 2.2 App 组件状态管理

**文件**: `frontend/src/App.tsx`

**新增 State**:
```typescript
const [servicesStatus, setServicesStatus] = useState<ServicesStatus | null>(null);
```

**新增获取服务状态函数**:
```typescript
async function refreshServicesStatus(){
  if(!auth) return;
  try {
    const status = await getServicesStatus();
    setServicesStatus(status);
  } catch(e:any){ 
    console.error('Failed to get services status:', e);
    // 如果获取失败，设置默认值（假设服务不可用）
    setServicesStatus({
      whisper_available: false,
      deps_service_available: false
    });
  }
}
```

**生命周期**:
```typescript
useEffect(()=>{ if(auth) refreshServicesStatus(); },[auth]);
```

#### 2.3 MeetingView 组件

**新增 Props**:
```typescript
const MeetingView: React.FC<{
  // ... 其他 props
  servicesStatus: ServicesStatus | null;
}>
```

**条件渲染逻辑**:
```typescript
// 检查是否应该显示 chunk 相关功能
// 只有当 whisper 和 deps-service 都可用时才显示
const showChunkFeatures = servicesStatus?.whisper_available && servicesStatus?.deps_service_available;
```

**隐藏 Chunk 列表**:
```typescript
{canReadMeeting && showChunkFeatures && (
  <div className="scroll-region" style={{ width:280, borderRight:'1px solid #f0f0f0', height: '100%' }}>
    <ChunkList ... />
  </div>
)}
```

**传递状态到 RightPanel**:
```typescript
<RightPanel 
  taskId={currentTask||''} 
  chunkId={canReadMeeting && showChunkFeatures ? currentChunk : undefined} 
  canWriteMeeting={canWriteMeeting} 
  canReadMeeting={canReadMeeting}
  showChunkDetails={showChunkFeatures}
/>
```

#### 2.4 RightPanel 组件

**文件**: `frontend/src/components/RightPanel.tsx`

**新增 Props**:
```typescript
interface RightPanelProps {
  taskId: string;
  chunkId?: string;
  canWriteMeeting?: boolean;
  canReadMeeting?: boolean;
  showChunkDetails?: boolean; // 是否显示 Chunk 详情（基于服务部署状态）
}
```

**条件渲染 Chunk 详情 Tab**:
```typescript
// 只有当服务可用且有读取权限时才显示 Chunk 详情
(allowRead && showChunkDetails) ? {
  key: 'chunks',
  label: (
    <span>
      <DatabaseOutlined />
      Chunk详情
    </span>
  ),
  children: (
    <div style={{ height: '100%', overflow: 'hidden' }}>
      <ChunkDetailTabs ... />
    </div>
  ),
} : null,
```

---

## 🎯 功能效果

### 场景 1: 完整部署（Whisper + Deps-Service）

**部署配置**:
```yaml
services:
  aidg:
    ...
  whisper:
    image: ghcr.io/mutablelogic/go-whisper:latest
  deps-service:
    image: aidg-deps-service:latest
```

**API 响应**:
```json
{
  "whisper_available": true,
  "deps_service_available": true
}
```

**前端表现**:
- ✅ 显示 Chunk 列表栏（中间栏）
- ✅ 显示 Chunk 详情 Tab 页

---

### 场景 2: 基础部署（仅 AIDG）

**部署配置**:
```yaml
services:
  aidg:
    image: ghcr.io/houzhh15-hub/aidg:latest
    # 没有 whisper 和 deps-service
```

**API 响应**:
```json
{
  "whisper_available": false,
  "deps_service_available": false
}
```

**前端表现**:
- ❌ **隐藏** Chunk 列表栏（中间栏）
- ❌ **隐藏** Chunk 详情 Tab 页
- ✅ 仍然显示：
  - 会议背景
  - 会议详情
  - 会议总结
  - 成果物

---

## 🔧 技术细节

### 服务检测机制

1. **Whisper 检测**:
   - 通过 `Orchestrator.GetHealthChecker()` 非 nil 判断
   - 健康检查器的存在表明 Whisper 服务已配置

2. **Deps-Service 检测**:
   - 通过 `Orchestrator.GetDependencyClient()` 非 nil 判断
   - 依赖客户端的存在表明 deps-service 已配置

3. **容错处理**:
   - 如果 API 调用失败，默认认为服务不可用
   - 不会阻塞页面加载，仅影响特定 UI 元素

### 兼容性

- **向后兼容**: `showChunkDetails` 默认为 `true`
- **权限检查**: 仍然尊重 `meeting.read` 和 `meeting.write` 权限
- **渐进式降级**: 服务不可用时自动隐藏相关功能

---

## 📝 测试验证

### 1. 测试完整部署

```bash
# 启动完整服务
docker compose -f docker-compose.yml up -d

# 检查 API
curl http://localhost:8000/api/v1/services/status
# 预期输出: {"whisper_available":true,"deps_service_available":true}
```

### 2. 测试基础部署

```bash
# 仅启动 AIDG
docker compose -f docker-compose.ghcr.yml up -d

# 检查 API
curl http://localhost:8000/api/v1/services/status
# 预期输出: {"whisper_available":false,"deps_service_available":false}
```

### 3. 前端验证

1. 登录系统
2. 切换到"会议"视图
3. 观察中间栏和 Chunk 详情 Tab 的显示状态

---

## ✨ 优势

1. **自动适配**: 根据实际部署自动调整 UI
2. **用户友好**: 避免显示无法使用的功能
3. **清晰反馈**: 用户一眼就能看出哪些功能可用
4. **零配置**: 无需手动配置，自动检测
5. **性能优化**: 不加载不需要的组件

---

## 🚀 部署建议

### 基础版用户（100MB）

适合以下场景：
- 只需要项目管理和文档功能
- 不需要会议录音转写
- 资源受限的环境

**部署命令**:
```bash
docker compose -f docker-compose.ghcr.yml up -d
```

### 完整版用户（~2.5GB）

适合以下场景：
- 需要完整的会议录音功能
- 需要自动转写和说话人识别
- 有充足的资源

**部署命令**:
```bash
docker compose -f docker-compose.yml up -d
```

---

## 📚 相关文件

### 后端
- `cmd/server/internal/api/services_status.go` - 服务状态 API
- `cmd/server/main.go` - API 路由注册

### 前端
- `frontend/src/api/client.ts` - API 客户端
- `frontend/src/App.tsx` - 主应用组件
- `frontend/src/components/RightPanel.tsx` - 右侧面板组件

---

## 🎉 完成状态

- ✅ 后端 API 实现
- ✅ 前端状态管理
- ✅ UI 条件渲染
- ✅ 编译测试通过
- ✅ 向后兼容性保证
