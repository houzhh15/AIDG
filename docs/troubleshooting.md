# AIDG 故障排查指南

## 1. 编译错误

### 1.1 Go 模块问题

**错误**: `package xxx is not in std`

**解决方案**:
```bash
go mod tidy
go mod download
```

### 1.2 导入路径错误

**错误**: `undefined: config`

**解决方案**: 检查导入语句是否正确
```go
import "github.com/houzhh15-hub/AIDG/cmd/server/internal/config"
```

## 2. 运行时问题

### 2.1 端口占用

**错误**: `address already in use`

**解决方案**:
```bash
# 查找占用端口的进程
lsof -i :8000

# 终止进程
kill -9 <PID>
```

### 2.2 权限问题

**错误**: `permission denied`

**解决方案**:
```bash
# 修改数据目录权限
chmod -R 755 data/
chown -R $(whoami) data/
```

### 2.3 配置加载失败

**错误**: `failed to load config`

**解决方案**: 检查环境变量是否设置
```bash
# 必需环境变量
export JWT_SECRET="your-secret"
export ADMIN_DEFAULT_PASSWORD="your-password"
```

## 3. Docker 问题

### 3.1 构建失败

**解决方案**:
```bash
# 清理 Docker 缓存
docker system prune -a

# 重新构建
docker-compose build --no-cache
```

### 3.2 容器无法启动

**解决方案**:
```bash
# 查看日志
docker-compose logs web

# 检查配置
docker-compose config
```

### 3.3 数据卷问题

**解决方案**:
```bash
# 列出数据卷
docker volume ls

# 删除未使用的卷
docker volume prune
```

## 4. 前端问题

### 4.1 npm 安装失败

**解决方案**:
```bash
# 清理缓存
npm cache clean --force

# 删除 node_modules
rm -rf node_modules package-lock.json

# 重新安装
npm install
```

### 4.2 构建失败

**错误**: `JavaScript heap out of memory`

**解决方案**:
```bash
# 增加内存限制
NODE_OPTIONS="--max-old-space-size=4096" npm run build
```

## 5. API 问题

### 5.1 CORS 错误

**解决方案**: 检查 CORS 配置
```bash
# 设置允许的源
export CORS_ALLOWED_ORIGINS="http://localhost:5173,http://localhost:8000"
```

### 5.2 认证失败

**解决方案**: 检查 JWT Secret
```bash
# 确保 JWT_SECRET 已设置且长度 >= 32
export JWT_SECRET="your-strong-secret-at-least-32-characters"
```

## 6. 性能问题

### 6.1 内存占用高

**解决方案**:
- 检查是否有内存泄漏
- 增加容器内存限制
- 使用 pprof 分析

```bash
# 启用 pprof
go tool pprof http://localhost:8000/debug/pprof/heap
```

### 6.2 响应慢

**排查步骤**:
1. 检查网络延迟
2. 检查数据库查询
3. 查看日志中的慢请求

## 7. 日志分析

### 7.1 查找错误日志

```bash
# Docker 环境
docker-compose logs web | grep ERROR

# 本地环境
grep ERROR logs/app.log
```

### 7.2 日志级别调整

```bash
# 开发环境使用 debug
export LOG_LEVEL=debug

# 生产环境使用 info
export LOG_LEVEL=info
```

## 8. 健康检查失败

### 8.1 /health 返回 404

**原因**: 路由未注册或服务未启动

**解决方案**: 检查路由配置和服务状态

### 8.2 /readiness 返回 503

**原因**: 数据目录不可访问

**解决方案**:
```bash
# 检查目录存在性
ls -la data/projects data/users data/meetings

# 创建缺失目录
mkdir -p data/{projects,users,meetings,audit_logs}
```

## 9. 常见错误码

| 错误码 | 含义 | 解决方案 |
|-------|------|---------|
| 401 | 未授权 | 检查 JWT token |
| 403 | 禁止访问 | 检查用户权限 |
| 404 | 资源不存在 | 检查路由配置 |
| 500 | 服务器错误 | 查看服务器日志 |
| 503 | 服务不可用 | 检查健康检查端点 |

## 10. 支持渠道

- GitHub Issues: https://github.com/houzhh15-hub/AIDG/issues
- 文档: [README.md](../README.md)
- 部署指南: [deployment.md](./deployment.md)
