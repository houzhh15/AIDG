# AIDG Docker Deployment Guide

本指南介绍如何使用 Docker 部署 AIDG 统一平台。

## 📋 文件说明

### Docker Compose 配置文件

- **`docker-compose.yml`** - 开发环境（本地构建）
  - 从源码构建镜像
  - 适合开发和调试
  
- **`docker-compose.ghcr.yml`** - 开发环境（预构建镜像）
  - 使用 GitHub Container Registry 的预构建镜像
  - 快速启动，无需本地构建
  - 默认使用 `0.1.0-alpha` 版本
  
- **`docker-compose.prod.yml`** - 生产环境
  - 使用 GHCR 预构建镜像
  - 从 `.env` 文件加载配置
  - 包含资源限制和日志配置
  - 默认使用 `latest` 版本

## 🚀 快速开始

### 方式 1：使用预构建镜像（推荐）

```bash
# 1. 启动服务（使用默认 0.1.0-alpha 版本）
docker-compose -f docker-compose.ghcr.yml up -d

# 2. 查看日志
docker-compose -f docker-compose.ghcr.yml logs -f

# 3. 停止服务
docker-compose -f docker-compose.ghcr.yml down
```

### 方式 2：本地构建

```bash
# 1. 构建并启动
docker-compose up -d

# 2. 查看日志
docker-compose logs -f

# 3. 停止服务
docker-compose down
```

## 🔧 版本管理

### 使用特定版本

```bash
# 使用特定版本
IMAGE_TAG=0.1.0-alpha docker-compose -f docker-compose.ghcr.yml up -d

# 使用最新版本
IMAGE_TAG=latest docker-compose -f docker-compose.ghcr.yml up -d

# 使用特定语义版本
IMAGE_TAG=0.1 docker-compose -f docker-compose.ghcr.yml up -d
```

### 可用的镜像标签

查看所有可用版本：https://github.com/houzhh15-hub/AIDG/pkgs/container/aidg

常用标签：
- `latest` - 最新稳定版本
- `0.1.0-alpha` - Alpha 测试版本
- `0.1` - 0.1.x 系列最新版本
- `v0.1.0-alpha` - 完整版本标签

## 🔐 生产环境部署

### 1. 准备环境配置

```bash
# 复制环境变量模板
cp .env.example .env

# 编辑 .env 文件，更新所有密钥和密码
nano .env  # 或使用你喜欢的编辑器
```

### 2. 必须修改的配置项

在 `.env` 文件中，必须修改以下值：

```bash
# 生产环境标识
ENV=production

# JWT 密钥（至少 32 字符）
JWT_SECRET=your-super-secret-jwt-key-at-least-32-characters-long
USER_JWT_SECRET=your-user-jwt-secret-at-least-32-characters-long

# 管理员密码（强密码）
ADMIN_DEFAULT_PASSWORD=your-strong-admin-password

# MCP 密码
MCP_PASSWORD=your-mcp-password

# 日志配置
LOG_LEVEL=info
LOG_FORMAT=json

# CORS 配置（更新为你的域名）
CORS_ALLOWED_ORIGINS=https://yourdomain.com,https://www.yourdomain.com
```

### 3. 启动生产环境

```bash
# 启动服务
docker-compose -f docker-compose.prod.yml up -d

# 查看健康状态
docker-compose -f docker-compose.prod.yml ps

# 查看日志
docker-compose -f docker-compose.prod.yml logs -f
```

## 📍 访问服务

服务启动后，可以通过以下地址访问：

- **Web 界面（人类界面）**: http://localhost:8000
- **MCP 服务（AI 界面）**: http://localhost:8081
- **健康检查（Web）**: http://localhost:8000/health
- **健康检查（MCP）**: http://localhost:8081/health

## 🗂️ 数据持久化

数据存储在本地目录：

```
data/
├── projects/      # 项目数据
├── users/         # 用户数据
├── meetings/      # 会议数据
└── audit_logs/    # 审计日志
```

**重要提示**：
- 确保定期备份 `data/` 目录
- 生产环境建议使用 Docker volumes 或外部存储

## 🛠️ 常用命令

### 查看服务状态

```bash
# 使用 GHCR 镜像
docker-compose -f docker-compose.ghcr.yml ps

# 使用生产配置
docker-compose -f docker-compose.prod.yml ps
```

### 重启服务

```bash
docker-compose -f docker-compose.ghcr.yml restart
```

### 更新到新版本

```bash
# 1. 拉取新镜像
IMAGE_TAG=0.2.0 docker-compose -f docker-compose.ghcr.yml pull

# 2. 重新创建容器
IMAGE_TAG=0.2.0 docker-compose -f docker-compose.ghcr.yml up -d

# 3. 清理旧镜像（可选）
docker image prune -f
```

### 查看日志

```bash
# 实时查看所有日志
docker-compose -f docker-compose.ghcr.yml logs -f

# 查看最近 100 行
docker-compose -f docker-compose.ghcr.yml logs --tail=100

# 只查看 aidg 服务的日志
docker-compose -f docker-compose.ghcr.yml logs -f aidg
```

### 进入容器

```bash
# 进入容器 shell
docker-compose -f docker-compose.ghcr.yml exec aidg sh

# 或使用 docker 命令
docker exec -it aidg-unified sh
```

### 完全清理

```bash
# 停止并删除容器、网络
docker-compose -f docker-compose.ghcr.yml down

# 同时删除 volumes（警告：会删除所有数据！）
docker-compose -f docker-compose.ghcr.yml down -v
```

## 🔍 故障排查

### 检查容器状态

```bash
docker-compose -f docker-compose.ghcr.yml ps
```

### 查看健康检查

```bash
docker inspect aidg-unified | grep -A 10 Health
```

### 测试健康端点

```bash
# 测试 Web 服务
curl http://localhost:8000/health

# 测试 MCP 服务
curl http://localhost:8081/health
```

### 查看详细日志

```bash
# 查看启动日志
docker-compose -f docker-compose.ghcr.yml logs aidg | head -50

# 查看错误日志
docker-compose -f docker-compose.ghcr.yml logs aidg | grep -i error
```

## ⚠️ 安全提醒

1. **永远不要**将 `.env` 文件提交到版本控制
2. 生产环境**必须**修改所有默认密码和密钥
3. JWT 密钥**必须**至少 32 字符长
4. 定期更新到最新版本以获得安全补丁
5. 限制端口访问，考虑使用反向代理（如 nginx）
6. 定期备份数据目录

## 📚 更多信息

- **项目仓库**: https://github.com/houzhh15-hub/AIDG
- **镜像仓库**: https://github.com/houzhh15-hub/AIDG/pkgs/container/aidg
- **问题反馈**: https://github.com/houzhh15-hub/AIDG/issues

## 📝 版本历史

- **v0.1.0-alpha** (2025-10-09)
  - 初始 Alpha 版本发布
  - 基础后端服务（Go/Gin）
  - MCP 服务器集成
  - React 前端应用
  - Docker 支持
  - CI/CD 流水线
