# Docker Compose 启动后无法访问服务故障排查

## 问题现象

容器启动成功（`docker compose up -d` 无错误），但 `http://localhost:8000` 无法访问。

---

## 故障排查步骤

### 1. 检查容器状态

```bash
# 查看所有容器状态
docker compose ps

# 或使用 docker ps
docker ps -a | grep aidg
```

**期望结果**：
```
NAME           STATUS         PORTS
aidg-unified   Up X minutes   0.0.0.0:8000->8000/tcp, 0.0.0.0:8081->8081/tcp
```

**问题判断**：
- ❌ `Exited` 或 `Restarting` - 容器启动失败或反复重启
- ❌ 没有显示 - 容器未创建
- ✅ `Up` - 容器正在运行（继续下一步）

---

### 2. 检查容器日志

```bash
# 查看实时日志
docker compose logs -f aidg

# 查看最后 100 行日志
docker compose logs --tail=100 aidg

# 查看特定服务的日志
docker compose logs aidg | grep "web-server"
docker compose logs aidg | grep "mcp-server"
```

**常见错误信息**：

#### 错误 1: 端口已被占用
```
Error starting userland proxy: listen tcp4 0.0.0.0:8000: bind: address already in use
```

**解决方案**：
```bash
# 查找占用端口的进程
lsof -i :8000
lsof -i :8081

# 杀死进程（替换 PID）
kill -9 <PID>

# 或修改 docker-compose.yml 使用不同端口
ports:
  - "8001:8000"  # 本地 8001 映射到容器 8000
  - "8082:8081"
```

#### 错误 2: 环境变量缺失或无效

**错误信息 A - 环境变量无效**：
```
Invalid config: invalid environment: dev (must be 'development' or 'production')
```

**错误信息 B - 环境变量缺失**：
```
invalid configuration: USER_JWT_SECRET is required
```

**解决方案**：

检查并修复 `docker-compose.yml` 中的环境变量：

```yaml
environment:
  # ENV 必须是 'development' 或 'production'，不能是 'dev' 或 'prod'
  - ENV=development  # ✅ 正确
  # - ENV=dev        # ❌ 错误
  
  # JWT_SECRET 用于 MCP 认证，至少 32 字符
  - JWT_SECRET=your-secret-at-least-32-characters
  
  # USER_JWT_SECRET 用于用户认证，至少 32 字符
  - USER_JWT_SECRET=your-user-jwt-secret-at-least-32-chars
  
  # 管理员密码，至少 8 字符
  - ADMIN_DEFAULT_PASSWORD=your-password
```

**完整的必需环境变量列表**：
- `ENV`: `development` 或 `production`
- `JWT_SECRET`: MCP Server 认证密钥（≥32 字符）
- `USER_JWT_SECRET`: 用户认证密钥（≥32 字符）
- `ADMIN_DEFAULT_PASSWORD`: 管理员密码（≥8 字符）

#### 错误 2 (原): 环境变量缺失

**旧的错误信息**：
```
panic: JWT_SECRET is required
```

**解决方案**：
```bash
# 检查 .env 文件
cat .env

# 确保包含必需的环境变量
JWT_SECRET=your-secret-at-least-32-characters
USER_JWT_SECRET=your-user-jwt-secret
ADMIN_DEFAULT_PASSWORD=your-password
```

#### 错误 3: 数据目录权限问题
```
mkdir: cannot create directory '/app/data/projects': Permission denied
```

**解决方案**：
```bash
# 检查并修复权限
sudo chown -R $(id -u):$(id -g) ./data
chmod -R 755 ./data
```

#### 错误 4: Supervisor 进程启动失败
```
Error: cannot find command 'web-server'
Error: cannot find command 'mcp-server'
```

**解决方案**：
- 重新构建镜像：`docker compose build --no-cache`
- 检查 Dockerfile 中的 COPY 命令是否正确

---

### 3. 检查进程状态（在容器内）

```bash
# 进入容器
docker compose exec aidg sh

# 检查 supervisor 状态
supervisorctl status

# 期望输出：
# web-server    RUNNING   pid 123, uptime 0:01:00
# mcp-server    RUNNING   pid 124, uptime 0:01:00

# 手动测试服务
wget -O- http://localhost:8000/health
wget -O- http://localhost:8081/health
```

**问题判断**：
- ❌ `FATAL` - 进程启动失败，检查日志
- ❌ `STOPPED` - 进程已停止
- ⚠️  `BACKOFF` - 进程反复重启
- ✅ `RUNNING` - 进程正常运行

**查看详细日志**：
```bash
# 在容器内
cat /var/log/supervisor/web-server-stdout.log
cat /var/log/supervisor/web-server-stderr.log
cat /var/log/supervisor/mcp-server-stdout.log
cat /var/log/supervisor/mcp-server-stderr.log
```

---

### 4. 检查网络连接

```bash
# 从主机测试容器端口
nc -zv localhost 8000
nc -zv localhost 8081

# 或使用 telnet
telnet localhost 8000

# 或使用 curl 详细模式
curl -v http://localhost:8000/health
```

**期望结果**：
```
* Connected to localhost (127.0.0.1) port 8000
< HTTP/1.1 200 OK
```

**问题判断**：
- ❌ `Connection refused` - 服务未监听该端口
- ❌ `Connection timeout` - 防火墙或网络问题
- ✅ `200 OK` - 服务正常

---

### 5. 检查健康检查状态

```bash
# 查看容器健康状态
docker inspect aidg-unified | grep -A 10 "Health"

# 或使用
docker ps --format "table {{.Names}}\t{{.Status}}"
```

**健康状态**：
- `health: starting` - 健康检查进行中
- `healthy` - 服务健康
- `unhealthy` - 服务不健康（健康检查失败）

---

### 6. 验证容器内服务配置

```bash
# 进入容器
docker compose exec aidg sh

# 检查环境变量
env | grep -E "PORT|HOST|JWT"

# 检查进程监听端口
netstat -tlnp | grep -E "8000|8081"
# 或
ss -tlnp | grep -E "8000|8081"

# 期望输出：
# tcp    0    0 0.0.0.0:8000    0.0.0.0:*    LISTEN    123/server
# tcp    0    0 0.0.0.0:8081    0.0.0.0:*    LISTEN    124/mcp-server
```

---

## 常见问题及解决方案

### 问题 A: 容器一直重启

**症状**：`docker compose ps` 显示 `Restarting`

**排查**：
```bash
# 查看重启次数和时间
docker compose ps

# 查看启动日志
docker compose logs --tail=50 aidg
```

**可能原因**：
1. 配置文件错误（如 supervisord.conf）
2. 二进制文件缺失或损坏
3. 环境变量配置错误
4. 依赖服务未准备好

**解决方案**：
```bash
# 重新构建（无缓存）
docker compose down
docker compose build --no-cache
docker compose up -d
```

---

### 问题 B: 健康检查失败

**症状**：容器 `unhealthy` 状态

**排查**：
```bash
# 手动执行健康检查命令
docker compose exec aidg wget --spider http://localhost:8000/health
docker compose exec aidg wget --spider http://localhost:8081/health
```

**可能原因**：
1. 服务启动时间过长（超过健康检查的 start-period）
2. `/health` 端点未实现或返回错误
3. 服务监听在错误的地址（如 127.0.0.1 而不是 0.0.0.0）

**解决方案**：
修改 `Dockerfile` 中的健康检查配置：
```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8000/health && \
        wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1
```

---

### 问题 C: 端口未绑定到 0.0.0.0

**症状**：容器内可以访问，主机无法访问

**排查**：
```bash
# 在容器内检查监听地址
docker compose exec aidg netstat -tlnp

# 错误示例（只监听 127.0.0.1）：
# tcp    0    0 127.0.0.1:8000    0.0.0.0:*    LISTEN

# 正确示例（监听所有接口）：
# tcp    0    0 0.0.0.0:8000      0.0.0.0:*    LISTEN
```

**解决方案**：
确保服务绑定到 `0.0.0.0` 而不是 `localhost` 或 `127.0.0.1`。

在 Go 代码中：
```go
// 错误
server.Run("localhost:8000")

// 正确
server.Run("0.0.0.0:8000")
// 或
server.Run(":8000")
```

---

### 问题 D: Docker Desktop 端口转发问题（macOS/Windows）

**症状**：Linux 容器在 macOS 上无法访问

**解决方案**：

1. **重启 Docker Desktop**
   ```bash
   # macOS
   killall Docker && open /Applications/Docker.app
   
   # 或通过 GUI 重启
   ```

2. **检查 Docker Desktop 设置**
   - Resources → Network
   - 确保未启用 VPN 或代理干扰

3. **使用 host.docker.internal**
   在某些情况下，使用：
   ```
   http://host.docker.internal:8000
   ```

---

## 完整诊断脚本

创建并运行此脚本进行全面检查：

```bash
#!/bin/bash
# docker-diagnose.sh

echo "=== Docker Compose 诊断 ==="
echo ""

echo "1. 检查容器状态"
docker compose ps
echo ""

echo "2. 检查端口占用"
lsof -i :8000 || echo "端口 8000 未被占用"
lsof -i :8081 || echo "端口 8081 未被占用"
echo ""

echo "3. 检查最近的日志（最后 20 行）"
docker compose logs --tail=20 aidg
echo ""

echo "4. 检查容器健康状态"
docker inspect aidg-unified --format='{{.State.Health.Status}}' 2>/dev/null || echo "无健康检查信息"
echo ""

echo "5. 测试端口连接"
nc -zv localhost 8000 2>&1 || echo "无法连接到 8000"
nc -zv localhost 8081 2>&1 || echo "无法连接到 8081"
echo ""

echo "6. 检查 Supervisor 状态（如果容器在运行）"
docker compose exec aidg supervisorctl status 2>/dev/null || echo "无法获取 supervisor 状态"
echo ""

echo "诊断完成"
```

使用方法：
```bash
chmod +x docker-diagnose.sh
./docker-diagnose.sh
```

---

## 快速修复命令

```bash
# 方案 1: 重启容器
docker compose restart

# 方案 2: 完全重建
docker compose down
docker compose up -d --build

# 方案 3: 无缓存重建
docker compose down
docker compose build --no-cache
docker compose up -d

# 方案 4: 清理并重建
docker compose down -v  # 删除卷
docker system prune -a  # 清理所有未使用的资源
docker compose up -d --build
```

---

## 参考日志位置

**容器日志**：
- `docker compose logs aidg`

**Supervisor 日志**（容器内）：
- `/var/log/supervisor/supervisord.log`
- `/var/log/supervisor/web-server-stdout.log`
- `/var/log/supervisor/web-server-stderr.log`
- `/var/log/supervisor/mcp-server-stdout.log`
- `/var/log/supervisor/mcp-server-stderr.log`

**应用日志**（如果配置了）：
- `/app/data/audit_logs/`

---

## 成功标志

当一切正常时，你应该看到：

```bash
# 1. 容器状态
$ docker compose ps
NAME           STATUS         PORTS
aidg-unified   Up 2 minutes   0.0.0.0:8000->8000/tcp, 0.0.0.0:8081->8081/tcp

# 2. 健康检查
$ curl http://localhost:8000/health
{"status":"ok"}

$ curl http://localhost:8081/health
{"status":"ok"}

# 3. Supervisor 状态
$ docker compose exec aidg supervisorctl status
web-server    RUNNING   pid 123, uptime 0:02:00
mcp-server    RUNNING   pid 124, uptime 0:02:00

# 4. 前端可访问
$ curl -I http://localhost:8000/
HTTP/1.1 200 OK
```

---

**如果仍然无法解决，请提供以下信息：**
1. `docker compose ps` 的输出
2. `docker compose logs --tail=50 aidg` 的输出
3. `docker compose exec aidg supervisorctl status` 的输出
4. 操作系统和 Docker 版本信息
