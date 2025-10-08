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

## 2. 环境变量配置

### 2.1 Web Server 环境变量

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

### 2.2 MCP Server 环境变量

| 变量名 | 说明 | 默认值 | 必填 |
|-------|------|--------|------|
| `MCP_ENV` | 环境类型 | production | 是 |
| `MCP_HTTP_PORT` | 服务端口 | 8081 | 否 |
| `MCP_LOG_LEVEL` | 日志级别 | info | 否 |
| `MCP_SERVER_URL` | 后端服务地址 | http://localhost:8000 | 是 |
| `MCP_PASSWORD` | 认证密码 | - | 否 |

## 3. 日志管理

### 3.1 查看日志

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

### 3.2 日志格式

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

### 3.3 日志持久化

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

## 4. 监控和健康检查

### 4.1 健康检查端点

- **Liveness Probe**: `GET /health`
  - 检查服务是否存活
  - 始终返回 200 (除非服务宕机)

- **Readiness Probe**: `GET /readiness`
  - 检查服务是否就绪
  - 验证数据目录可访问性
  - 失败返回 503

### 4.2 容器健康检查

```bash
# 查看容器健康状态
docker inspect --format='{{.State.Health.Status}}' aidg-prod

# 查看健康检查日志
docker inspect --format='{{json .State.Health}}' aidg-prod | jq
```

## 5. 故障排查

### 5.1 服务无法启动

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

### 5.2 数据库连接失败

检查数据目录挂载：

```bash
docker-compose exec aidg ls -la /app/data/
```

### 5.3 内存不足

增加容器资源限制：

```yaml
deploy:
  resources:
    limits:
      memory: 4G
```

## 6. 安全加固

### 6.1 密码强度要求

- JWT Secret: 至少 32 字符
- Admin Password: 至少 8 字符，包含大小写字母、数字和特殊字符

### 6.2 网络隔离

使用 Docker 网络隔离：

```yaml
networks:
  frontend:
  backend:
    internal: true  # 仅内部访问
```

### 6.3 定期更新

```bash
# 拉取最新镜像
docker-compose pull

# 重新启动服务
docker-compose up -d
```

## 7. 升级和回滚

### 7.1 滚动升级

```bash
# 1. 拉取新版本镜像
docker pull aidg:1.1.0

# 2. 更新 docker-compose.prod.yml 中的版本号
VERSION=1.1.0

# 3. 滚动更新
docker-compose -f docker-compose.prod.yml up -d --no-deps --build aidg
```

### 7.2 回滚

```bash
# 回滚到上一个版本
VERSION=1.0.0 docker-compose -f docker-compose.prod.yml up -d web
```

## 8. 性能优化

### 8.1 资源分配建议

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

### 8.2 缓存优化

启用 HTTP 缓存头：

```nginx
# 静态资源缓存 (在反向代理中配置)
location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2)$ {
    expires 1y;
    add_header Cache-Control "public, immutable";
}
```

## 9. 备份策略

### 9.1 自动备份脚本

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

### 9.2 定时备份

```cron
# 每天凌晨2点备份
0 2 * * * /path/to/backup.sh
```

## 10. 生产检查清单

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
