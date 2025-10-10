# AIDG (AI-Dev-Gov)

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Node Version](https://img.shields.io/badge/Node.js-18+-339933?style=flat&logo=node.js)](https://nodejs.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

AI 辅助开发治理平台 - 智能化项目管理与协作系统

## ✨ 特性

- 🤖 **AI 辅助开发**: 集成 MCP (Model Context Protocol) 协议，提供智能代码辅助
- 📋 **项目管理**: 完整的项目、任务、需求和设计文档管理
- 🔄 **进度跟踪**: 周/月/季度进度自动统计和可视化
- 📝 **文档协作**: 结构化文档编辑，支持实时协作和版本控制
- 🎯 **执行计划**: 自动生成和跟踪任务执行步骤
- 🎙️ **音频录制**: 浏览器录音和文件上传，解决Docker环境音频设备限制
- 🔐 **权限管理**: 基于角色的访问控制 (RBAC)
- 📊 **数据统计**: 实时项目数据统计和分析
- 🚀 **生产就绪**: Docker 容器化部署，健康检查，优雅关闭

## 🏗️ 架构设计

### 统一容器架构

AIDG 采用统一容器架构，在单个 Docker 镜像中集成：

- **Web Server** (端口 8000): REST API + 人机交互界面
- **MCP Server** (端口 8081): AI 工具接口 (Model Context Protocol)
- **Frontend**: React + TypeScript 单页应用

通过 **Supervisor** 进程管理器协调两个服务，确保版本同步、低延迟通信和简化部署。

详见 [架构迁移文档](docs/ARCHITECTURE_MIGRATION.md)

### 技术栈

**后端**:
- Go 1.22+ (Gin Web Framework)
- JSON 文件存储系统
- JWT 认证

**前端**:
- React 18
- TypeScript 5
- Ant Design
- Vite 5
- React Router 6

**部署**:
- Docker + Docker Compose
- Supervisor 进程管理
- Alpine Linux 基础镜像
- GitHub Actions CI/CD

## 📁 项目结构

```
AIDG/
├── cmd/                      # 命令行程序入口
│   ├── server/               # Web Server (REST API + 前端服务)
│   │   └── main.go
│   └── mcp-server/           # MCP Server (AI 工具接口)
│       └── main.go
├── pkg/                      # 可导出的公共包
│   └── logger/               # 日志工具
├── frontend/                 # Web 前端应用
│   ├── src/
│   │   ├── components/       # React 组件
│   │   ├── api/              # API 客户端
│   │   ├── contexts/         # React Context
│   │   └── hooks/            # 自定义 Hooks
│   └── dist/                 # 构建输出
├── deployments/              # 部署配置
│   ├── docker/
│   │   └── supervisord.conf  # 进程管理配置
│   └── kubernetes/           # K8s 配置 (待实现)
├── scripts/                  # 工具脚本
│   └── dev.sh                # 开发环境启动脚本
├── docs/                     # 文档目录
│   ├── deployment.md         # 部署指南
│   ├── ARCHITECTURE_MIGRATION.md  # 架构迁移说明
│   ├── acceptance.md         # 验收文档
│   └── COMPLIANCE_REPORT.md  # 合规报告
├── data/                     # 数据目录 (运行时生成)
│   ├── projects/             # 项目数据
│   ├── meetings/             # 会议记录
│   └── users/                # 用户数据
├── Dockerfile                # 统一镜像配置
├── docker-compose.yml        # 开发环境配置
├── docker-compose.prod.yml   # 生产环境配置
├── Makefile                  # 构建脚本
└── go.mod                    # Go 依赖管理
```

## 🚀 快速开始

### 方式一：Docker 部署 (推荐)

#### 前置要求

- Docker 20.10+
- Docker Compose 2.0+

#### 启动服务

```bash
# 1. 克隆仓库
git clone https://github.com/houzhh15-hub/AIDG.git
cd AIDG

# 2. 启动开发环境
docker-compose up -d

# 3. 查看日志
docker-compose logs -f

# 4. 访问服务
# Web UI: http://localhost:8000
# MCP Server: http://localhost:8081
```

#### 健康检查

```bash
curl http://localhost:8000/health   # Web Server
curl http://localhost:8081/health   # MCP Server
```

### 方式二：本地开发

#### 前置要求

- Go 1.22+
- Node.js 18+
- make

#### 步骤

```bash
# 1. 安装依赖
make install

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env 文件，配置必要参数：
# - JWT_SECRET (至少 32 字符)
# - ADMIN_DEFAULT_PASSWORD (至少 8 字符)
# - MCP_PASSWORD

# 3. 构建项目
make build

# 4. 启动开发服务器
make dev
# 或分别启动：
# Terminal 1: ./bin/server
# Terminal 2: ./bin/mcp-server
# Terminal 3: cd frontend && npm run dev
```

## 🔧 配置说明

### 环境变量

创建 `.env` 文件并配置以下变量：

```bash
# Server Configuration
VERSION=1.0.0
LOG_LEVEL=info              # debug, info, warn, error
LOG_FORMAT=console          # console (开发), json (生产)

# Security (必须修改!)
JWT_SECRET=your-strong-jwt-secret-at-least-32-characters-long
ADMIN_DEFAULT_PASSWORD=your-secure-admin-password
MCP_PASSWORD=your-mcp-password

# CORS Configuration
CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:8000

# Server Ports
WEB_SERVER_PORT=8000
MCP_SERVER_PORT=8081
```

### 安全建议

⚠️ **生产环境必须修改以下配置**:

- `JWT_SECRET`: 至少 32 字符的随机字符串
- `ADMIN_DEFAULT_PASSWORD`: 强密码（包含大小写、数字、特殊字符）
- `MCP_PASSWORD`: 用于 MCP 服务认证的密码
- `CORS_ALLOWED_ORIGINS`: 仅允许信任的域名

## 🛠️ 开发指南

### 可用命令

```bash
# 安装依赖
make install              # 安装 Go 和 Node.js 依赖

# 构建
make build                # 开发构建（保留调试信息）
make build-prod           # 生产构建（优化和压缩）
make docker-build         # 构建 Docker 镜像

# 运行
make dev                  # 启动开发服务器
make run                  # 运行编译后的程序

# 测试
make test                 # 运行所有测试
make test-coverage        # 运行测试并生成覆盖率报告

# 清理
make clean                # 清理构建产物
```

### 项目约定

- **代码风格**: 遵循 Go 官方代码规范和 TypeScript/React 最佳实践
- **提交信息**: 使用语义化提交信息 (feat/fix/docs/refactor 等)
- **分支策略**: main (生产) / develop (开发) / feature/* (特性)
- **API 规范**: RESTful API 设计，统一错误码

### API 文档

启动服务后访问:
- API 文档: `http://localhost:8000/api/docs` (待实现)
- Swagger UI: `http://localhost:8000/swagger` (待实现)

## 📦 生产部署

### Docker 部署

```bash
# 1. 准备环境变量
cp .env.example .env.prod
# 编辑 .env.prod，配置生产环境参数

# 2. 构建镜像
docker build -t aidg:1.0.0 .

# 3. 启动生产服务
docker-compose -f docker-compose.prod.yml --env-file .env.prod up -d

# 4. 验证部署
docker-compose -f docker-compose.prod.yml ps
curl http://localhost:8000/health
curl http://localhost:8081/health
```

### 资源要求

**最低配置**:
- CPU: 2 核
- 内存: 2GB
- 磁盘: 5GB

**推荐配置**:
- CPU: 4 核
- 内存: 4GB
- 磁盘: 20GB

详细部署指南请参考 [docs/deployment.md](docs/deployment.md)

## 📊 功能模块

### 核心功能

- ✅ 用户认证与授权
- ✅ 项目管理（创建、编辑、归档）
- ✅ 任务管理（需求、设计、测试文档）
- ✅ 进度跟踪（周/月/季度统计）
- ✅ 文档协作（结构化编辑、版本控制）
- ✅ 会议管理（记录、总结、特性提取）
- ✅ 音频录制（浏览器录音、文件上传）
- ✅ MCP 工具集成（AI 代码辅助）
- ✅ 审计日志（操作记录）

### 音频录制功能 🎙️

AIDG 提供完整的音频录制和上传功能，专为 Docker 环境设计，解决容器无法访问宿主机音频设备的限制。

**功能特性**:
- 🎤 **浏览器录音**: 使用 MediaRecorder API 直接在浏览器中录音
- 📁 **文件上传**: 支持 WAV/MP3/M4A/FLAC/OGG 格式音频文件
- 🔄 **自动分片**: 录音超过5分钟自动分片上传，避免内存溢出
- ⏸️ **暂停/恢复**: 录音过程中可随时暂停和恢复
- 📊 **实时进度**: 显示录音时长、已上传分片数和文件大小
- 🔁 **自动重试**: 上传失败自动重试3次（间隔2秒）
- 🔒 **权限管理**: 麦克风权限请求和错误处理
- 📝 **格式验证**: 自动验证文件格式和大小（最大500MB）

**使用方法**:
1. 登录系统，进入任务详情页
2. 点击右侧"设置"按钮打开抽屉
3. 滚动到"音频录制"区域
4. 选择"🎙️ 浏览器录音"或"📁 文件上传"标签
5. 开始录音或上传文件

**浏览器要求**:
- Chrome 60+ (推荐)
- Firefox 55+
- Safari 11+ (需要 HTTPS)

**Docker 环境说明**:
在 Docker 容器中运行时，后端无法访问宿主机的音频设备。音频录制功能通过前端浏览器实现，绕过了这一限制。录制的音频通过 HTTP API 上传到后端存储。

详细测试指南请参考 [docs/AUDIO_RECORDING_TEST_GUIDE.md](docs/AUDIO_RECORDING_TEST_GUIDE.md)

### 待实现功能

- ⏳ Kubernetes 部署支持
- ⏳ 实时协作编辑
- ⏳ 消息通知系统
- ⏳ 数据备份与恢复
- ⏳ API 文档自动生成
- ⏳ 性能监控与告警

## 🔐 安全特性

- ✅ JWT Token 认证
- ✅ 基于角色的访问控制 (RBAC)
- ✅ CORS 跨域保护
- ✅ 密码加密存储
- ✅ 审计日志记录
- ✅ 非 root 容器运行
- ✅ 健康检查与优雅关闭

## 🐛 故障排查

### 服务无法启动

```bash
# 查看容器状态
docker-compose ps

# 查看日志
docker-compose logs -f aidg

# 检查端口占用
lsof -i :8000
lsof -i :8081
```

### 健康检查失败

```bash
# 手动测试健康检查
docker exec aidg wget --spider http://localhost:8000/health
docker exec aidg wget --spider http://localhost:8081/health

# 查看进程状态
docker exec aidg supervisorctl status
```

### 数据目录权限问题

```bash
# 确保数据目录有写权限
docker-compose exec aidg ls -la /app/data/

# 如有问题，调整宿主机权限
sudo chown -R $(id -u):$(id -g) ./data/
```

更多问题请参考 [部署指南](docs/deployment.md#5-故障排查)

## 📚 文档

- [部署指南](docs/deployment.md) - 完整的部署和运维指南
- [架构迁移](docs/ARCHITECTURE_MIGRATION.md) - 统一镜像架构说明
- [验收文档](docs/acceptance.md) - 功能验收和测试标准
- [合规报告](docs/COMPLIANCE_REPORT.md) - 设计文档合规性检查

## 🤝 贡献指南

欢迎贡献代码、报告问题或提出建议！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 提交 Pull Request

## 📝 更新日志

查看 [CHANGELOG.md](CHANGELOG.md) 了解版本更新历史。

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 👥 团队

- **开发团队**: AIDG Development Team
- **维护者**: [@houzhh15-hub](https://github.com/houzhh15-hub)

## 🔗 相关链接

- [GitHub 仓库](https://github.com/houzhh15-hub/AIDG)
- [问题反馈](https://github.com/houzhh15-hub/AIDG/issues)
- [MCP 协议文档](https://modelcontextprotocol.io)

---

**Made with ❤️ by AIDG Team**
