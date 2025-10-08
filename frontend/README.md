# Audio To Text UI

开发用前端。依赖后端服务(http://localhost:8000)。

## 启动

```bash
cd frontend
npm install
npm run dev
```

浏览: http://localhost:5173

## 功能
- 任务列表 / 创建 / 删除 / 启动 / 停止
- Chunk 5分钟块列表 + 状态标签 + 播放音频
- 右侧 Tabs: segments 可编辑，其余只读
- 自动轮询状态
- 登录鉴权：未登录显示登录页，登录后持久化 token (localStorage)

## 登录与鉴权
后端开启 JWT 保护后，前端登录方式：

1. 访问开发地址时出现登录页，输入管理员/已有用户账号：
	 - 默认管理员: `admin / neteye@123` (首次启动自动创建，务必修改密码)
2. 登录成功后 token、用户名、scopes 缓存于 localStorage `auth_info_v1`。
3. Axios 拦截器自动在每个 /api/v1 请求附加 Authorization 头。
4. 遇到 401 会清空本地 token 并回到登录页。

### 示例 (命令行获取 token)
```bash
curl -X POST http://localhost:8000/api/v1/login \
	-H 'Content-Type: application/json' \
	-d '{"Username":"admin","Password":"neteye@123"}'
```

### 安全建议
- 生产部署请：
	- 修改默认密码
	- 配置 USER_JWT_SECRET 环境变量
	- 考虑加入 Token 过期与刷新逻辑
	- 使用 HTTPS

## 注意
- 当前 TypeScript 严格模式下需要安装依赖后错误才会消失
- 生产需加鉴权、错误处理、token 隐藏

## 前端重建脚本 (跨平台)
为在新机器或复制后的代码目录中快速恢复可用构建，提供以下脚本：

### Bash / macOS / Linux
```bash
./rebuild-frontend.sh            # 安装依赖 + 构建
./rebuild-frontend.sh --skip-install  # 跳过安装直接构建
./rebuild-frontend.sh --install-only  # 只安装不构建
```
自动检测 pnpm > yarn > npm；安装失败会回退非 frozen 模式。

### PowerShell (Windows)
```powershell
./rebuild-frontend.ps1                # 安装依赖 + 构建
./rebuild-frontend.ps1 -SkipInstall   # 跳过安装直接构建
./rebuild-frontend.ps1 -InstallOnly   # 只安装不构建
```
同样自动检测 pnpm / yarn / npm。

### 构建产物
脚本会清理 dist 与 tsconfig.tsbuildinfo，然后执行 `build` 脚本，最后列出 dist 目录前若干文件。

### 环境要求
- Node.js >= 18 (建议与原开发环境保持一致)
- 对应包管理器 (若没有将自动 fallback 到 npm)

## 新同步 (Dispatch) 功能说明
前端已从旧的两步 (prepare + sync) 简化为单步 dispatch：
- 入口：Sync 面板中“一键 Dispatch”按钮
- API: `POST /api/v1/sync/dispatch`
- 参数：
  - target: 目标后端基础地址 (例如 http://10.0.0.5:8000)
  - mode: `client_overwrite` | `server_overwrite` | `merge_no_overwrite`
  - returnFiles: 可选布尔，是否让后端返回本次参与签名的文件摘要
- 安全：后端使用 HMAC-SHA256 签名 (summary|targetHost|timestamp)；接收端验证时间窗口 ±5 分钟。

### 兼容性
旧接口 `sync/prepare` 与 `sync` 暂时保留但后续可能移除；请尽快迁移到 dispatch。

## 常见问题 (FAQ)
| 问题 | 解决 | 备注 |
|------|------|------|
| 构建时报内存不足 | 增大 Node --max_old_space_size | 大型依赖树时出现 |
| 401 未授权 | 确认已登录且 token 未过期 | Dev 模式自动附加 Authorization |
| Dispatch 401 bad_signature | 检查 SYNC_SHARED_SECRET、目标 host:port 是否一致 | 反向代理需确保 Host 统一 |
| Dispatch 400 invalid_timestamp | 同步系统时间（NTP） | 时间偏差超过 ±5 分钟 |

## 后续改进建议
- 增加重放攻击防护（nonce 缓存）
- 对 dist 加 hash 及部署指引
- 去除遗留 prepare/sync 端点并更新文档
