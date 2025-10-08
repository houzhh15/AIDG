# AIDG 架构迁移：双镜像 → 统一镜像

## 迁移概述

本项目已从**双镜像架构**迁移至**统一镜像架构**，将 Web Server 和 MCP Server 整合到单个 Docker 镜像中。

## 迁移原因

### 问题分析

双镜像架构存在以下问题：

1. **版本同步复杂**: 两个服务必须保持相同版本，独立部署容易出现版本不一致
2. **网络依赖**: MCP Server 需要配置 Web Server 的 URL，增加配置复杂度
3. **部署复杂**: 需要管理两个镜像的构建、推送和部署流程
4. **资源浪费**: 两个容器各自独立，无法共享内存和资源

### 解决方案

采用统一镜像架构，使用 **Supervisor** 进程管理器在单个容器内运行两个服务：

- ✅ **版本同步**: 单个镜像确保两个服务版本一致
- ✅ **低延迟通信**: localhost 通信，零网络开销
- ✅ **简化部署**: 单个镜像，一次构建
- ✅ **统一配置**: 共享环境变量和配置文件
- ✅ **一致性健康检查**: 验证两个服务的整体可用性

## 架构对比

### 旧架构（双镜像）

```yaml
services:
  web:
    image: aidg-web:1.0.0
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8000:8000"
  
  mcp:
    image: aidg-mcp:1.0.0
    build:
      context: .
      dockerfile: Dockerfile.mcp
    ports:
      - "8081:8081"
    environment:
      - WEB_SERVER_URL=http://web:8000  # 跨容器网络配置
```

### 新架构（统一镜像）

```yaml
services:
  aidg:
    image: aidg:1.0.0
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8000:8000"  # Web Server
      - "8081:8081"  # MCP Server
    # 单个容器，localhost 通信，无需配置 URL
```

## 技术实现

### Supervisor 配置

使用 Supervisor 管理两个进程：

```ini
[program:web-server]
command=/app/server
stdout_logfile=/dev/stdout
stderr_logfile=/dev/stderr

[program:mcp-server]
command=/app/mcp-server
stdout_logfile=/dev/stdout
stderr_logfile=/dev/stderr
```

### Dockerfile 结构

多阶段构建：

1. **Go Builder**: 编译 Web Server 和 MCP Server
2. **Node Builder**: 构建前端应用
3. **Runtime**: Alpine + Supervisor + 两个可执行文件 + 前端静态资源

### 健康检查

统一健康检查脚本验证两个服务：

```bash
wget --no-verbose --tries=1 --spider http://localhost:8000/health && \
wget --no-verbose --tries=1 --spider http://localhost:8081/health
```

## 迁移变更清单

### 删除的文件

- ❌ `Dockerfile.mcp` (MCP Server 独立镜像)
- ❌ `Dockerfile.unified` (临时统一镜像，已重命名)
- ❌ `docker-compose.unified.yml` (临时配置，已重命名)
- ❌ `docker-compose.unified.prod.yml` (临时配置，已重命名)

### 新增的文件

- ✅ `deployments/docker/supervisord.conf` (进程管理配置)

### 修改的文件

- 📝 `Dockerfile` (从独立 Web Server 镜像改为统一镜像)
- 📝 `docker-compose.yml` (服务名从 web/mcp 改为 aidg)
- 📝 `docker-compose.prod.yml` (同上)
- 📝 `.github/workflows/deploy.yml` (构建单个镜像)
- 📝 `Makefile` (docker-build 目标更新)
- 📝 `docs/deployment.md` (部署文档更新)
- 📝 `docs/acceptance.md` (验收文档更新)
- 📝 `docs/COMPLIANCE_REPORT.md` (合规报告更新)
- 📝 `CHANGELOG.md` (更新日志)

## 使用指南

### 开发环境

```bash
# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f aidg

# 查看特定服务日志
docker-compose logs -f aidg | grep "web-server"
docker-compose logs -f aidg | grep "mcp-server"

# 健康检查
curl http://localhost:8000/health  # Web Server
curl http://localhost:8081/health  # MCP Server
```

### 生产环境

```bash
# 构建镜像
docker build -t aidg:1.0.0 .

# 启动服务
docker-compose -f docker-compose.prod.yml up -d

# 查看状态
docker-compose -f docker-compose.prod.yml ps

# 健康检查
curl http://localhost:8000/health
curl http://localhost:8081/health
```

### Makefile 使用

```bash
# 构建 Docker 镜像
make docker-build VERSION=1.0.0

# 构建二进制文件
make build

# 运行测试
make test
```

## CI/CD 变更

### GitHub Actions

构建流程简化：

```yaml
# 旧方式（构建两个镜像）
- docker build -t aidg-server:${{ github.sha }} .
- docker build -t aidg-mcp:${{ github.sha }} -f Dockerfile.mcp .

# 新方式（构建单个镜像）
- docker build -t aidg:${{ github.sha }} .
```

## 兼容性说明

### 向后兼容

- ✅ API 端点保持不变
- ✅ 端口映射保持不变 (8000, 8081)
- ✅ 环境变量配置保持不变
- ✅ 数据卷挂载保持不变

### 不兼容变更

- ❌ Docker Compose 服务名从 `web`/`mcp` 改为 `aidg`
- ❌ 镜像名从 `aidg-web`/`aidg-mcp` 改为 `aidg`
- ❌ 不再支持独立部署 Web Server 或 MCP Server

## 性能影响

### 优势

- ✅ **启动速度**: 单容器启动，减少容器间依赖等待
- ✅ **内存占用**: 共享基础镜像层，节省内存
- ✅ **通信延迟**: localhost 通信，延迟接近零
- ✅ **构建速度**: 单次多阶段构建，避免重复步骤

### 监控建议

```bash
# 查看容器资源使用
docker stats aidg

# 查看进程状态
docker exec aidg supervisorctl status

# 重启特定进程
docker exec aidg supervisorctl restart web-server
docker exec aidg supervisorctl restart mcp-server
```

## 故障排查

### 服务启动失败

```bash
# 查看 Supervisor 日志
docker exec aidg cat /var/log/supervisor/supervisord.log

# 查看特定服务日志
docker exec aidg cat /var/log/supervisor/web-server-stdout.log
docker exec aidg cat /var/log/supervisor/mcp-server-stdout.log
```

### 重启服务

```bash
# 重启整个容器
docker-compose restart aidg

# 重启特定进程（不重启容器）
docker exec aidg supervisorctl restart web-server
docker exec aidg supervisorctl restart mcp-server
```

### 健康检查失败

```bash
# 手动测试健康检查
docker exec aidg wget --spider http://localhost:8000/health
docker exec aidg wget --spider http://localhost:8081/health

# 查看进程状态
docker exec aidg supervisorctl status
```

## 迁移日期

- **决策日期**: 2025-01-XX
- **实施日期**: 2025-01-XX
- **完成日期**: 2025-01-XX
- **状态**: ✅ 已完成

## 相关文档

- [部署指南](./deployment.md) - 包含统一镜像架构说明
- [验收文档](./acceptance.md) - 更新的测试流程
- [合规报告](./COMPLIANCE_REPORT.md) - Docker 配置合规状态

## 总结

统一镜像架构简化了 AIDG 的部署和运维流程，消除了版本同步问题，提高了系统的可靠性和可维护性。这一变更是基于 Web Server 和 MCP Server 紧密耦合的特性做出的合理架构决策。
