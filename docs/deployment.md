# AIDG 部署指南

## 架构概述

AIDG 采用**统一容器架构**，在单个 Docker 镜像中集成了：

- **Web Server** (端口 8000): 提供 REST API 和人机交互界面
- **MCP Server** (端口 8081): 提供 AI 工具接口（Model Context Protocol）
- **Frontend**: React + TypeScript 构建的前端应用

这两个服务通过 **Supervisor** 进程管理器在同一容器内运行，确保：
- ✅ 版本同步：两个服务始终使用相同版本
- ✅ 低延迟：localhost 通信，零网络开销
- ✅ 简化部署：单个镜像，一次构建，统一配置
- ✅ 一致健康检查：验证两个服务的可用性

## 1. Docker 部署

### 1.1 前置要求

- Docker 20.10+
- Docker Compose 2.0+
- 至少 2GB 可用内存
- 至少 5GB 可用磁盘空间

### 1.2 快速开始（开发环境）

```bash
# 1. 克隆仓库
git clone https://github.com/houzhh15-hub/AIDG.git
cd AIDG

# 2. 启动服务
docker-compose up -d

# 3. 查看日志
docker-compose logs -f

# 4. 访问服务
# Web UI: http://localhost:8000
# MCP Server: http://localhost:8081
# Frontend Dev (可选): http://localhost:5173
```

### 1.3 生产环境部署

#### 步骤 1: 准备环境变量

创建 `.env.prod` 文件：

```bash
# Server Configuration
VERSION=1.0.0
LOG_LEVEL=info

# Security (必须修改)
JWT_SECRET=your-strong-jwt-secret-at-least-32-chars
ADMIN_DEFAULT_PASSWORD=your-secure-admin-password
MCP_PASSWORD=your-mcp-password

# CORS Configuration
CORS_ALLOWED_ORIGINS=https://yourdomain.com
```

#### 步骤 2: 构建镜像

```bash
# 构建统一镜像（包含 Web Server + MCP Server + Frontend）
docker build -t aidg:1.0.0 .
```

#### 步骤 3: 启动生产服务

```bash
# 使用生产配置启动
docker-compose -f docker-compose.prod.yml --env-file .env.prod up -d
```

#### 步骤 4: 验证部署

```bash
# 检查服务状态
docker-compose -f docker-compose.prod.yml ps

# 健康检查（两个服务都在同一容器中）
curl http://localhost:8000/health  # Web Server
curl http://localhost:8081/health  # MCP Server

# 就绪检查
curl http://localhost:8000/readiness
```

### 1.4 数据持久化

生产环境使用 Docker 命名卷持久化数据：

```yaml
volumes:
  projects_data:   # 项目数据
  users_data:      # 用户数据
  meetings_data:   # 会议数据
  audit_logs_data: # 审计日志
```

备份数据：

```bash
# 备份所有数据卷
docker run --rm -v aidg_projects_data:/data -v $(pwd):/backup \
  alpine tar czf /backup/projects_backup.tar.gz -C /data .

# 恢复数据
docker run --rm -v aidg_projects_data:/data -v $(pwd):/backup \
  alpine tar xzf /backup/projects_backup.tar.gz -C /data
```

## 2. Docker 环境配置（音频处理工具链）

### 2.1 必需环境变量

AIDG 集成了 Whisper 转录、PyAnnote 说话人识别和音频处理工具链，以下环境变量用于配置这些功能：

| 变量名 | 说明 | 默认值 | 必填 |
|-------|------|--------|------|
| `WHISPER_API_URL` | Whisper 转录服务地址 | http://whisper:8082 | 是 |
| `HUGGINGFACE_TOKEN` | Hugging Face API Token (下载 PyAnnote 模型) | - | 首次运行必填 |
| `HF_HOME` | Hugging Face 模型缓存目录 | /models/huggingface | 否 |
| `LOG_LEVEL` | 日志级别 (debug/info/warn/error) | info | 否 |
| `ENABLE_OFFLINE` | 离线模式（使用本地缓存模型） | true | 否 |

**重要**: `HUGGINGFACE_TOKEN` 用于首次下载 PyAnnote 模型，获取方式：
1. 注册 [Hugging Face](https://huggingface.co/) 账号
2. 访问 [Settings > Access Tokens](https://huggingface.co/settings/tokens)
3. 创建 Read 权限的 Token
4. 接受 [PyAnnote 模型协议](https://huggingface.co/pyannote/speaker-diarization-3.1)

### 2.2 模型下载和缓存

AIDG 使用两类 AI 模型，需要预先下载或通过挂载卷提供：

#### Whisper 模型缓存

Whisper 模型存储在 `./models/whisper/` 目录：

```bash
# 创建 Whisper 模型目录
mkdir -p ./models/whisper

# Whisper 容器首次启动时会自动下载模型到 /models/ 目录
# 常用模型: tiny, base, small, medium, large-v2, large-v3
```

**推荐配置**:
- 中文转录：`medium` 或 `large-v2`
- 英文转录：`small` 或 `medium`
- 快速测试：`base` 或 `tiny`

#### PyAnnote 模型缓存

PyAnnote 模型存储在 `./models/huggingface/` 目录：

```bash
# 创建 PyAnnote 模型目录
mkdir -p ./models/huggingface

# 首次启动时，需要提供 HUGGINGFACE_TOKEN 环境变量
# PyAnnote 会自动下载模型到 HF_HOME (/models/huggingface/)
```

启动后，模型会缓存在以下路径：
```
./models/huggingface/
├── hub/
│   └── models--pyannote--speaker-diarization-3.1/
└── ...
```

**离线模式**: 设置 `ENABLE_OFFLINE=true` 后，系统会优先使用本地缓存的模型，避免每次启动时联网验证。

### 2.3 Docker Compose 使用指南

#### 构建镜像

```bash
# 构建所有服务（包括 Whisper 和 AIDG）
docker-compose build

# 仅构建 AIDG 服务
docker-compose build aidg

# 无缓存重新构建（解决依赖问题时使用）
docker-compose build --no-cache aidg
```

#### 启动服务

```bash
# 前台启动（查看实时日志）
docker-compose up

# 后台启动
docker-compose up -d

# 仅启动特定服务
docker-compose up -d whisper

# 启动并重新构建
docker-compose up -d --build
```

#### 查看日志

```bash
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f aidg
docker-compose logs -f whisper

# 查看最近 100 行日志
docker-compose logs --tail=100 aidg

# 仅查看错误日志
docker-compose logs aidg | grep ERROR
```

#### 停止和清理

```bash
# 停止所有服务（保留容器）
docker-compose stop

# 停止并删除容器（保留数据卷）
docker-compose down

# 删除容器和数据卷（危险操作！）
docker-compose down -v

# 删除容器、数据卷和镜像
docker-compose down -v --rmi all
```

### 2.4 环境检查

AIDG 提供环境检查 API，用于验证所有依赖项就绪：

```bash
# 检查环境状态
curl http://localhost:8000/api/v1/environment/status

# 强制重新检查（忽略缓存）
curl http://localhost:8000/api/v1/environment/status?force=true
```

**返回示例**:
```json
{
  "ready": true,
  "details": {
    "token": {
      "present": true,
      "masked": "hf_****abcd",
      "offline_mode": true
    },
    "models": {
      "pyannote": {
        "available": true,
        "path": "/models/huggingface/pyannote/speaker-diarization-3.1",
        "size_mb": 485
      }
    },
    "services": {
      "whisper": {
        "available": true,
        "url": "http://whisper:8082",
        "response_time_ms": 15
      }
    },
    "tools": {
      "ffmpeg": {
        "available": true,
        "version": "6.0"
      }
    }
  },
  "issues": [],
  "warnings": [
    "Whisper 服务响应时间较长 (1500ms)，建议检查资源配置"
  ]
}
```

### 2.5 故障排查方法

#### 问题 1: Whisper 服务无法连接

**症状**: `environment/status` 返回 `whisper.available = false`

**检查步骤**:
```bash
# 1. 检查 Whisper 容器状态
docker-compose ps whisper

# 2. 查看 Whisper 日志
docker-compose logs whisper

# 3. 手动测试 Whisper API
curl http://localhost:8082/health

# 4. 检查网络连通性
docker-compose exec aidg ping whisper
```

**解决方案**:
- 确保 `docker-compose.yml` 中 Whisper 服务已启动
- 检查 `WHISPER_API_URL` 环境变量配置
- 等待 Whisper 健康检查通过（约 30 秒）

#### 问题 2: PyAnnote 模型下载失败

**症状**: `environment/status` 返回 `models.pyannote.available = false`

**检查步骤**:
```bash
# 1. 检查 HUGGINGFACE_TOKEN 是否设置
docker-compose exec aidg printenv | grep HUGGINGFACE_TOKEN

# 2. 检查模型目录挂载
docker-compose exec aidg ls -la /models/huggingface/

# 3. 查看下载日志
docker-compose logs aidg | grep pyannote
```

**解决方案**:
- 在 `docker-compose.yml` 中添加 `HUGGINGFACE_TOKEN` 环境变量
- 确保已接受 [PyAnnote 模型协议](https://huggingface.co/pyannote/speaker-diarization-3.1)
- 手动下载模型：
  ```bash
  # 在宿主机下载
  pip install huggingface_hub
  huggingface-cli login  # 输入 Token
  huggingface-cli download pyannote/speaker-diarization-3.1 \
    --local-dir ./models/huggingface/pyannote/speaker-diarization-3.1
  ```

#### 问题 3: FFmpeg 录制失败

**症状**: 音频处理任务卡在 FFmpeg 录制阶段

**检查步骤**:
```bash
# 1. 检查 FFmpeg 版本
docker-compose exec aidg ffmpeg -version

# 2. 检查音频设备权限
docker-compose exec aidg ls -la /dev/snd/

# 3. 测试录制
docker-compose exec aidg ffmpeg -f alsa -i default -t 5 test.wav
```

**解决方案**:
- 确保 Dockerfile 已安装 `alsa-lib` 和 `alsa-utils`
- 在 `docker-compose.yml` 中添加设备挂载：
  ```yaml
  devices:
    - /dev/snd:/dev/snd
  ```

#### 问题 4: 磁盘空间不足

**症状**: `environment/status` 返回 `DISK_FULL` 错误

**检查步骤**:
```bash
# 1. 检查数据卷大小
docker system df -v

# 2. 检查模型目录大小
du -sh ./models/

# 3. 检查临时文件
docker-compose exec aidg du -sh /tmp /app/tmp
```

**解决方案**:
- 清理未使用的 Docker 资源：
  ```bash
  docker system prune -a --volumes
  ```
- 删除旧模型或音频文件
- 扩展宿主机磁盘空间

## 3. 环境变量配置

### 3.1 Web Server 环境变量

| 变量名 | 说明 | 默认值 | 必填 |
|-------|------|--------|------|
| `ENV` | 环境类型 (dev/staging/production) | production | 是 |
| `PORT` | 服务端口 | 8000 | 否 |
| `LOG_LEVEL` | 日志级别 (debug/info/warn/error) | info | 否 |
| `LOG_FORMAT` | 日志格式 (console/json) | json | 否 |
| `JWT_SECRET` | JWT 密钥 (至少32字符) | - | 是 |
| `ADMIN_DEFAULT_PASSWORD` | 默认管理员密码 | - | 生产必填 |
| `PROJECTS_DIR` | 项目数据目录 | /app/data/projects | 否 |
| `USERS_DIR` | 用户数据目录 | /app/data/users | 否 |
| `MEETINGS_DIR` | 会议数据目录 | /app/data/meetings | 否 |
| `AUDIT_LOGS_DIR` | 审计日志目录 | /app/data/audit_logs | 否 |
| `MCP_SERVER_URL` | MCP 服务地址 | http://localhost:8081 | 否 |
| `MCP_PASSWORD` | MCP 认证密码 | - | 否 |
| `CORS_ALLOWED_ORIGINS` | CORS 允许的源 (逗号分隔) | http://localhost:8000 | 否 |

### 3.2 MCP Server 环境变量

| 变量名 | 说明 | 默认值 | 必填 |
|-------|------|--------|------|
| `MCP_ENV` | 环境类型 | production | 是 |
| `MCP_HTTP_PORT` | 服务端口 | 8081 | 否 |
| `MCP_LOG_LEVEL` | 日志级别 | info | 否 |
| `MCP_SERVER_URL` | 后端服务地址 | http://localhost:8000 | 是 |
| `MCP_PASSWORD` | 认证密码 | - | 否 |

## 4. 日志管理

### 4.1 查看日志

```bash
# 查看所有服务日志
docker-compose logs -f

# 查看最近100行日志
docker-compose logs --tail=100 aidg

# 仅查看 Web Server 日志
docker-compose logs -f aidg | grep "web-server"

# 仅查看 MCP Server 日志
docker-compose logs -f aidg | grep "mcp-server"
```

### 4.2 日志格式

生产环境使用 JSON 格式日志：

```json
{
  "time": "2025-01-08T07:00:00Z",
  "level": "INFO",
  "msg": "server starting",
  "component": "web-server",
  "addr": ":8000",
  "env": "production"
}
```

### 4.3 日志持久化

配置日志驱动：

```yaml
services:
  aidg:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

## 5. 监控和健康检查

### 5.1 健康检查端点

- **Liveness Probe**: `GET /health`
  - 检查服务是否存活
  - 始终返回 200 (除非服务宕机)

- **Readiness Probe**: `GET /readiness`
  - 检查服务是否就绪
  - 验证数据目录可访问性
  - 失败返回 503

### 5.2 容器健康检查

```bash
# 查看容器健康状态
docker inspect --format='{{.State.Health.Status}}' aidg-prod

# 查看健康检查日志
docker inspect --format='{{json .State.Health}}' aidg-prod | jq
```

## 6. 故障排查

### 6.1 服务无法启动

```bash
# 检查容器状态
docker-compose ps

# 查看启动日志
docker-compose logs aidg

# 常见问题：
# 1. 端口冲突 - 修改 docker-compose.yml 中的端口映射
# 2. 环境变量缺失 - 检查 .env 文件
# 3. 权限问题 - 确保数据目录有写权限
```

### 6.2 数据库连接失败

检查数据目录挂载：

```bash
docker-compose exec aidg ls -la /app/data/
```

### 6.3 内存不足

增加容器资源限制：

```yaml
deploy:
  resources:
    limits:
      memory: 4G
```

## 7. 安全加固

### 7.1 密码强度要求

- JWT Secret: 至少 32 字符
- Admin Password: 至少 8 字符，包含大小写字母、数字和特殊字符

### 7.2 网络隔离

使用 Docker 网络隔离：

```yaml
networks:
  frontend:
  backend:
    internal: true  # 仅内部访问
```

### 7.3 定期更新

```bash
# 拉取最新镜像
docker-compose pull

# 重新启动服务
docker-compose up -d
```

## 8. 升级和回滚

### 8.1 滚动升级

```bash
# 1. 拉取新版本镜像
docker pull aidg:1.1.0

# 2. 更新 docker-compose.prod.yml 中的版本号
VERSION=1.1.0

# 3. 滚动更新
docker-compose -f docker-compose.prod.yml up -d --no-deps --build aidg
```

### 8.2 回滚

```bash
# 回滚到上一个版本
VERSION=1.0.0 docker-compose -f docker-compose.prod.yml up -d web
```

## 9. 性能优化

### 9.1 资源分配建议

**小型部署** (< 100 用户):
- Web Server: 1 CPU, 1GB RAM
- MCP Server: 0.5 CPU, 512MB RAM

**中型部署** (100-1000 用户):
- Web Server: 2 CPU, 2GB RAM
- MCP Server: 1 CPU, 1GB RAM

**大型部署** (> 1000 用户):
- Web Server: 4+ CPU, 4GB+ RAM
- MCP Server: 2 CPU, 2GB RAM
- 考虑使用负载均衡和多实例部署

### 9.2 缓存优化

启用 HTTP 缓存头：

```nginx
# 静态资源缓存 (在反向代理中配置)
location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2)$ {
    expires 1y;
    add_header Cache-Control "public, immutable";
}
```

## 10. 备份策略

### 10.1 自动备份脚本

```bash
#!/bin/bash
# backup.sh - 自动备份数据卷

BACKUP_DIR="/backup/aidg/$(date +%Y%m%d)"
mkdir -p "$BACKUP_DIR"

# 备份所有数据卷
for volume in projects_data users_data meetings_data audit_logs_data; do
  docker run --rm \
    -v "aidg_${volume}:/data" \
    -v "$BACKUP_DIR:/backup" \
    alpine tar czf "/backup/${volume}.tar.gz" -C /data .
done

echo "Backup completed: $BACKUP_DIR"
```

### 10.2 定时备份

```cron
# 每天凌晨2点备份
0 2 * * * /path/to/backup.sh
```

## 11. 生产检查清单

部署前确认：

- [ ] 所有密码已修改为强密码
- [ ] 环境变量配置正确
- [ ] CORS 配置符合域名要求
- [ ] 数据卷配置正确
- [ ] 健康检查端点正常
- [ ] 日志输出正常
- [ ] 资源限制已配置
- [ ] 备份策略已就位
- [ ] 监控告警已配置

## 支持

如遇问题，请查看：
- [故障排查文档](./troubleshooting.md)
- [开发文档](./development.md)
- [GitHub Issues](https://github.com/houzhh15-hub/AIDG/issues)
