# Docker 构建快速参考

## ✅ 所有已修复的问题

### 问题 1: Go embed 文件未找到 ✅
```
错误: pattern task.prompt.md: no matching files found
修复: 删除未使用的 template.go
```

### 问题 2: Go 版本不匹配 ✅
```
错误: go.mod requires go >= 1.22 (running go 1.21.13)
修复: Dockerfile 使用 golang:1.22-alpine
```

### 问题 3: supervisord.conf 未找到 ✅
```
错误: COPY deployments/docker/supervisord.conf 失败
修复: .dockerignore 添加 !deployments/docker/supervisord.conf
```

### 问题 4: npm 平台依赖错误 ✅
```
错误: @rollup/rollup-darwin-arm64 平台不匹配
修复: 从 package.json 删除平台特定依赖
```

### 问题 5: TypeScript 类型错误 ✅
```
错误: Property 'env' does not exist on type 'ImportMeta'
修复: 创建 frontend/src/vite-env.d.ts
```

### 问题 6: Vite 缺少 terser 依赖 ✅
```
错误: terser not found. Since Vite v3, terser has become an optional dependency
修复: package.json 添加 "terser": "^5.36.0"
```

### 问题 7: docker-compose 引用错误 ✅
```
错误: open Dockerfile.unified: no such file or directory
修复: docker-compose.yml 改为 dockerfile: Dockerfile
```

---

## 🚀 构建命令

```bash
# 方式 1: 使用 Makefile
make docker-build VERSION=1.0.0

# 方式 2: 直接构建
docker build -t aidg:1.0.0 .

# 方式 3: 无缓存构建
docker build --no-cache -t aidg:1.0.0 .
```

---

## 📋 关键配置

### Dockerfile
```dockerfile
# Go 版本: 1.22
FROM golang:1.22-alpine AS backend-builder

# 前端构建: 删除 lock 文件
RUN rm -f package-lock.json && npm install --production=false
```

### go.mod
```
go 1.22
```

### .dockerignore
```
deployments/kubernetes/
!deployments/docker/supervisord.conf
**/package-lock.json
```

---

## 🔍 验证

```bash
# 1. 运行测试脚本
./test-docker-build.sh

# 2. 检查 Go 版本
grep "FROM golang" Dockerfile

# 3. 检查 npm 命令
grep "npm install" Dockerfile
```

---

## 📚 详细文档

- [DOCKER_BUILD_TROUBLESHOOTING.md](./DOCKER_BUILD_TROUBLESHOOTING.md) - 详细故障排查
- [DOCKER_BUILD_FIX_SUMMARY.md](./DOCKER_BUILD_FIX_SUMMARY.md) - 完整修复记录
- [deployment.md](./deployment.md) - 部署指南

---

**最后更新**: 2025-10-08  
**状态**: ✅ 所有问题已修复
