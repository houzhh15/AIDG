# AIDG 开发指南

## 1. 环境准备

### 必需工具
- Go 1.22+
- Node.js 18+
- Make

### 安装依赖

```bash
# Go 依赖
go mod download

# 前端依赖
cd frontend && npm install
```

## 2. 本地开发

### 启动后端

```bash
# 方式1: Make
make dev

# 方式2: 直接运行
go run ./cmd/server/main.go
```

### 启动前端

```bash
cd frontend
npm run dev
```

### 启动 MCP Server

```bash
go run ./cmd/mcp-server/main.go
```

## 3. 代码规范

### Go 代码风格
- 使用 `gofmt` 格式化
- 遵循 Go 官方代码规范
- 包名使用小写单数形式

### TypeScript 代码风格
- 使用 2 空格缩进
- 使用 ESLint 检查

```bash
npm run lint
```

## 4. 测试

```bash
# 单元测试
make test

# 覆盖率
make test-coverage
```

## 5. 构建

```bash
# 构建所有
make build

# 仅构建服务器
make build-server

# 仅构建前端
make build-frontend
```

## 6. 调试技巧

### VSCode 配置

创建 `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Server",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/server",
      "env": {
        "ENV": "dev",
        "LOG_LEVEL": "debug"
      }
    }
  ]
}
```

## 7. 提交规范

### Commit Message 格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Type:**
- feat: 新功能
- fix: 修复
- docs: 文档
- style: 格式
- refactor: 重构
- test: 测试
- chore: 构建/工具

**示例:**
```
feat(server): 添加健康检查端点

- 添加 GET /health 端点
- 添加 GET /readiness 端点
- 支持 Kubernetes 健康检查

Closes #123
```

## 8. 目录结构

```
AIDG/
├── cmd/                # 可执行文件入口
│   ├── server/        # Web Server
│   └── mcp-server/ # MCP Server
├── pkg/               # 公共库
├── frontend/          # 前端代码
├── docs/              # 文档
├── data/              # 数据目录
└── deployments/       # 部署配置
```

## 9. 常用命令

```bash
# 开发
make dev              # 启动开发环境
make watch            # 监听文件变化自动重启

# 构建
make build            # 构建所有
make clean            # 清理构建文件

# 测试
make test             # 运行测试
make test-coverage    # 测试覆盖率

# Docker
make docker-build     # 构建 Docker 镜像
make docker-run       # 运行 Docker 容器
```
