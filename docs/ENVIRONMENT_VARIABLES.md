# AIDG 环境变量配置手册 🔧

> 完整的环境变量说明，帮你理解每个配置的作用和如何正确设置。

---

## 📋 目录

1. [基础配置](#基础配置)
2. [安全配置](#安全配置)
3. [服务端口配置](#服务端口配置)
4. [依赖服务配置](#依赖服务配置)
5. [音频处理配置](#音频处理配置)
6. [日志配置](#日志配置)
7. [数据目录配置](#数据目录配置)
8. [完整示例](#完整示例)

---

## 基础配置

### ENV
**作用**：设置运行环境  
**可选值**：`development` | `staging` | `production`  
**默认值**：`development`  
**示例**：
```yaml
ENV=development
```

**说明**：
- `development`：开发环境，会输出详细日志，允许跨域访问
- `staging`：测试环境，介于开发和生产之间
- `production`：生产环境，最严格的安全设置

### PORT
**作用**：Web 服务器监听端口  
**默认值**：`8000`  
**示例**：
```yaml
PORT=8000
```

**说明**：
- 这是你在浏览器中访问的端口
- 访问地址：`http://localhost:8000`
- 如果被占用，可以改成其他端口（如 9000）

### MCP_HTTP_PORT
**作用**：MCP 服务器监听端口  
**默认值**：`8081`  
**示例**：
```yaml
MCP_HTTP_PORT=8081
```

**说明**：
- MCP 是给 AI 助手用的接口
- 访问地址：`http://localhost:8081`
- Claude Desktop 等工具会连接这个端口

---

## 安全配置

### JWT_SECRET
**作用**：JWT 令牌签名密钥（用于会话管理）  
**必需**：✅ 是  
**最小长度**：32字符  
**示例**：
```yaml
JWT_SECRET=dev-secret-change-me-in-production-at-least-32-chars
```

**生成方法**：
```bash
# Linux/Mac
openssl rand -base64 32

# 示例输出
k8ZtV4mN2pQ9xR5wS1uY7oE3hG6jL0bA
```

**⚠️ 安全提示**：
- 开发环境可以用默认值
- **生产环境必须改**，否则别人可以伪造登录令牌
- 改了这个值后，所有用户需要重新登录

### USER_JWT_SECRET
**作用**：用户认证 JWT 密钥（独立于会话）  
**必需**：✅ 是  
**最小长度**：32字符  
**示例**：
```yaml
USER_JWT_SECRET=dev-user-jwt-secret-at-least-32-chars-long
```

**说明**：
- 和 `JWT_SECRET` 类似，但用于用户身份验证
- 必须和 `JWT_SECRET` 不同
- 生产环境务必修改

### ADMIN_DEFAULT_PASSWORD
**作用**：管理员初始密码  
**必需**：✅ 是  
**最小长度**：8字符（生产环境）  
**示例**：
```yaml
ADMIN_DEFAULT_PASSWORD=admin123
```

**⚠️ 安全提示**：
- 开发环境可以用 `admin123`
- **生产环境必须改成强密码**
- 首次登录后立即修改密码
- 禁止使用：`admin123`、`changeme`、`neteye@123` 等弱密码

### MCP_PASSWORD
**作用**：MCP 服务访问密码  
**必需**：⚠️ 生产环境必需  
**示例**：
```yaml
MCP_PASSWORD=dev-mcp-password
```

**说明**：
- AI 助手连接 MCP 服务时需要这个密码
- 开发环境可选
- 生产环境强制要求

### CORS_ALLOWED_ORIGINS
**作用**：允许跨域访问的源  
**格式**：逗号分隔的 URL 列表  
**示例**：
```yaml
CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:8000
```

**说明**：
- 开发环境通常允许 localhost
- 生产环境改成你的实际域名
- 多个域名用逗号分隔

---

## 服务端口配置

### 端口映射总览

| 服务 | 容器内端口 | 主机端口 | 环境变量 | 访问地址 |
|------|----------|---------|---------|---------|
| Web Server | 8000 | 8000 | `PORT` | http://localhost:8000 |
| MCP Server | 8081 | 8081 | `MCP_HTTP_PORT` | http://localhost:8081 |
| Whisper | 80 | 8082 | - | http://localhost:8082 |
| Deps-Service | 8080 | 8080 | - | http://localhost:8080 |

### 修改端口示例

如果你的 8000 端口被占用了：

```yaml
# docker-compose.yml
services:
  aidg:
    ports:
      - "9000:8000"  # 主机端口:容器端口
    environment:
      - PORT=8000    # 保持不变，这是容器内的端口
```

访问地址变成：`http://localhost:9000`

---

## 依赖服务配置

### DEPENDENCY_MODE
**作用**：依赖执行模式  
**可选值**：`remote` | `fallback` | `local`  
**默认值**：`fallback`  
**示例**：
```yaml
DEPENDENCY_MODE=remote
```

**说明**：
- `remote`：总是使用远程 deps-service
- `fallback`：优先使用远程，失败时降级到本地
- `local`：总是使用本地执行（仅限完整镜像）

**推荐配置**：
- 基础版：`fallback`（因为没有本地能力）
- 完整版：`remote`（明确依赖外部服务）

### DEPS_SERVICE_URL
**作用**：deps-service 服务地址  
**示例**：
```yaml
DEPS_SERVICE_URL=http://aidg-deps-service:8080
```

**说明**：
- 在 Docker Compose 中使用容器名：`aidg-deps-service`
- 独立部署时使用 IP 或域名：`http://192.168.1.100:8080`

### WHISPER_API_URL
**作用**：Whisper 服务地址  
**示例**：
```yaml
WHISPER_API_URL=http://whisper:80
```

**说明**：
- 在 Docker Compose 中使用容器名：`whisper`
- 独立部署时使用 IP 或域名

### WHISPER_MODE
**作用**：Whisper 实现模式  
**可选值**：`go-whisper` | `faster-whisper` | `local-whisper`  
**默认值**：`go-whisper`  
**示例**：
```yaml
WHISPER_MODE=go-whisper
```

**说明**：
- `go-whisper`：使用 go-whisper HTTP API（推荐）
- `faster-whisper`：使用 faster-whisper HTTP API
- `local-whisper`：使用本地 Whisper 命令行工具

---

## 音频处理配置

### ENABLE_AUDIO_CONVERSION
**作用**：启用音频格式转换（使用 FFmpeg）  
**可选值**：`true` | `false`  
**默认值**：`true`  
**示例**：
```yaml
ENABLE_AUDIO_CONVERSION=true
```

### ENABLE_SPEAKER_DIARIZATION
**作用**：启用说话人识别（使用 PyAnnote）  
**可选值**：`true` | `false`  
**默认值**：`true`  
**示例**：
```yaml
ENABLE_SPEAKER_DIARIZATION=true
```

### ENABLE_DEGRADATION
**作用**：启用服务降级（失败时自动切换到 Mock 模式）  
**可选值**：`true` | `false`  
**默认值**：`true`  
**示例**：
```yaml
ENABLE_DEGRADATION=true
```

**说明**：
- 开启后，如果 Whisper 服务不可用，会使用模拟数据
- 开发环境建议开启，生产环境视需求决定

### HEALTH_CHECK_INTERVAL
**作用**：健康检查间隔  
**格式**：时间字符串（如 `5m`, `30s`）  
**默认值**：`5m`  
**示例**：
```yaml
HEALTH_CHECK_INTERVAL=5m
```

### HEALTH_CHECK_FAIL_THRESHOLD
**作用**：健康检查失败次数阈值  
**默认值**：`3`  
**示例**：
```yaml
HEALTH_CHECK_FAIL_THRESHOLD=3
```

**说明**：
- 连续失败 3 次后触发降级
- 降级后会切换到 Mock 模式

### ENABLE_OFFLINE
**作用**：启用离线模式（不从网络下载模型）  
**可选值**：`true` | `false`  
**默认值**：`false`  
**示例**：
```yaml
ENABLE_OFFLINE=true
```

**说明**：
- 开启后，PyAnnote 不会从 HuggingFace 下载模型
- 需要提前下载好模型文件

### HUGGINGFACE_TOKEN
**作用**：HuggingFace 访问令牌  
**必需**：⚠️ 使用 PyAnnote 时必需  
**示例**：
```yaml
HUGGINGFACE_TOKEN=hf_xxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**获取方法**：
1. 访问 https://huggingface.co/settings/tokens
2. 登录或注册（免费）
3. 创建 Read 权限的 token
4. 复制 token

### HF_HOME
**作用**：HuggingFace 模型缓存目录  
**默认值**：`/models/huggingface`  
**示例**：
```yaml
HF_HOME=/models/huggingface
```

---

## 日志配置

### LOG_LEVEL
**作用**：日志级别  
**可选值**：`debug` | `info` | `warn` | `error`  
**默认值**：`info`  
**示例**：
```yaml
LOG_LEVEL=debug
```

**说明**：
- `debug`：最详细，包含所有调试信息（开发环境）
- `info`：标准信息，记录重要事件
- `warn`：警告信息
- `error`：只记录错误（生产环境推荐 info）

### LOG_FORMAT
**作用**：日志输出格式  
**可选值**：`console` | `json`  
**默认值**：`console`  
**示例**：
```yaml
LOG_FORMAT=console
```

**说明**：
- `console`：人类可读格式（开发环境）
- `json`：结构化 JSON 格式（生产环境，方便日志分析）

---

## 数据目录配置

### PROJECTS_DIR
**作用**：项目数据存储目录  
**默认值**：`/app/data/projects`  
**示例**：
```yaml
PROJECTS_DIR=/app/data/projects
```

### USERS_DIR
**作用**：用户数据存储目录  
**默认值**：`/app/data/users`  
**示例**：
```yaml
USERS_DIR=/app/data/users
```

### MEETINGS_DIR
**作用**：会议数据存储目录  
**默认值**：`/app/data/meetings`  
**示例**：
```yaml
MEETINGS_DIR=/app/data/meetings
```

### AUDIT_LOGS_DIR
**作用**：审计日志存储目录  
**默认值**：`/app/data/audit_logs`  
**示例**：
```yaml
AUDIT_LOGS_DIR=/app/data/audit_logs
```

**⚠️ 重要提示**：
- 这些是容器内的路径
- 通过 Volume 映射到主机目录：
  ```yaml
  volumes:
    - ./data/projects:/app/data/projects
  ```

---

## 完整示例

### 基础版配置（开发环境）

```yaml
# docker-compose.yml
services:
  aidg:
    environment:
      # 基础配置
      - ENV=development
      - PORT=8000
      - MCP_HTTP_PORT=8081
      
      # 日志配置
      - LOG_LEVEL=debug
      - LOG_FORMAT=console
      
      # 安全配置（开发环境）
      - JWT_SECRET=dev-secret-change-me-in-production-at-least-32-chars
      - USER_JWT_SECRET=dev-user-jwt-secret-at-least-32-chars
      - ADMIN_DEFAULT_PASSWORD=admin123
      - MCP_PASSWORD=dev-mcp-password
      
      # CORS 配置
      - CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:8000
      
      # 数据目录
      - PROJECTS_DIR=/app/data/projects
      - USERS_DIR=/app/data/users
      - MEETINGS_DIR=/app/data/meetings
      - AUDIT_LOGS_DIR=/app/data/audit_logs
      
      # MCP 配置
      - MCP_SERVER_URL=http://localhost:8000
```

### 完整版配置（开发环境）

```yaml
# docker-compose.deps.yml
services:
  aidg:
    environment:
      # 基础配置（同上）
      - ENV=development
      - PORT=8000
      - MCP_HTTP_PORT=8081
      
      # 依赖服务配置
      - DEPENDENCY_MODE=remote
      - DEPS_SERVICE_URL=http://aidg-deps-service:8080
      - WHISPER_API_URL=http://whisper:80
      
      # 音频处理配置
      - ENABLE_AUDIO_CONVERSION=true
      - ENABLE_SPEAKER_DIARIZATION=true
      - ENABLE_DEGRADATION=true
      - WHISPER_MODE=go-whisper
      
      # 健康检查配置
      - HEALTH_CHECK_INTERVAL=5m
      - HEALTH_CHECK_FAIL_THRESHOLD=3
      
      # HuggingFace 配置
      - HF_HOME=/models/huggingface
      - ENABLE_OFFLINE=false
      
      # 其他配置（同基础版）
      ...
  
  deps-service:
    environment:
      - HUGGINGFACE_TOKEN=${HUGGINGFACE_TOKEN}
      - HF_HOME=/models/huggingface
      - LOG_LEVEL=debug
```

### 生产环境配置

```yaml
# docker-compose.prod.yml
services:
  aidg:
    environment:
      # 基础配置
      - ENV=production
      - PORT=8000
      - MCP_HTTP_PORT=8081
      
      # 日志配置（生产环境）
      - LOG_LEVEL=info
      - LOG_FORMAT=json
      
      # ⚠️ 安全配置（必须修改！）
      - JWT_SECRET=你生成的32位以上随机字符串
      - USER_JWT_SECRET=另一个32位以上随机字符串
      - ADMIN_DEFAULT_PASSWORD=你的强密码（12位以上）
      - MCP_PASSWORD=MCP强密码
      
      # CORS 配置（生产环境）
      - CORS_ALLOWED_ORIGINS=https://yourdomain.com
      
      # 依赖服务配置
      - DEPENDENCY_MODE=remote
      - DEPS_SERVICE_URL=http://aidg-deps-service:8080
      - WHISPER_API_URL=http://whisper:80
      
      # 音频处理配置
      - ENABLE_AUDIO_CONVERSION=true
      - ENABLE_SPEAKER_DIARIZATION=true
      - ENABLE_DEGRADATION=false  # 生产环境可能不需要降级
      
      # 其他配置...
```

---

## 🔐 安全检查清单

部署到生产环境前，请确认：

- [ ] 修改了 `JWT_SECRET`（至少32位）
- [ ] 修改了 `USER_JWT_SECRET`（至少32位）
- [ ] 修改了 `ADMIN_DEFAULT_PASSWORD`（强密码）
- [ ] 修改了 `MCP_PASSWORD`（强密码）
- [ ] 设置了正确的 `CORS_ALLOWED_ORIGINS`
- [ ] 使用了 HTTPS（配置反向代理）
- [ ] 设置了 `LOG_FORMAT=json`（便于日志分析）
- [ ] 设置了 `ENV=production`
- [ ] 备份了数据目录

---

## 📝 配置文件模板

### 创建 .env 文件

你可以创建一个 `.env` 文件来管理环境变量：

```bash
# .env
ENV=development
PORT=8000
MCP_HTTP_PORT=8081

# 安全配置
JWT_SECRET=你的JWT密钥
USER_JWT_SECRET=你的用户JWT密钥
ADMIN_DEFAULT_PASSWORD=你的管理员密码
MCP_PASSWORD=你的MCP密码

# HuggingFace
HUGGINGFACE_TOKEN=hf_xxxxxxxxxxxxx

# 依赖服务
DEPS_SERVICE_URL=http://aidg-deps-service:8080
WHISPER_API_URL=http://whisper:80
```

然后在 `docker-compose.yml` 中引用：

```yaml
services:
  aidg:
    env_file:
      - .env
```

**⚠️ 重要**：把 `.env` 加入 `.gitignore`，不要提交到 Git！

---

## 🆘 故障排查

### 配置验证命令

```bash
# 检查环境变量是否生效
docker-compose exec aidg env | grep JWT_SECRET

# 检查配置文件语法
docker-compose config

# 查看实际使用的配置
docker-compose config --services
```

### 常见配置错误

1. **JWT_SECRET 太短**
   ```
   Error: JWT_SECRET must be at least 32 characters
   ```
   解决：生成更长的密钥

2. **端口冲突**
   ```
   Error: port is already allocated
   ```
   解决：修改端口映射或停止占用端口的程序

3. **HuggingFace Token 无效**
   ```
   Error: Could not authenticate with HuggingFace
   ```
   解决：检查 token 是否正确，是否有 Read 权限

---

## 📚 相关文档

- 📖 [部署指南](DEPLOYMENT_GUIDE_FRIENDLY.md)
- 🚀 [快速开始](QUICK_START.md)
- 🐳 [Docker 配置](DOCKER_DEPLOYMENT.md)
- 🔒 [安全最佳实践](SECURITY.md)

---

*最后更新：2025-01-14*
*有任何问题？查看 [GitHub Issues](https://github.com/houzhh15/AIDG/issues)*
