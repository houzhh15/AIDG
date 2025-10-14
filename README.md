# AIDG (AI-Dev-Gov)

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Node Version](https://img.shields.io/badge/Node.js-18+-339933?style=flat&logo=node.js)](https://nodejs.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

---

## 📖 简介

这是作者在实际开发中使用的自用工具，其核心理念并非提升 AI 的智能，而是规范其行为，确保人机协作的有效性。它通过为项目上下文建立“单一事实来源”，并记录 AI 的每一次决策依据，致力于让 AI 开发过程变得更可控、透明和可追溯。


## ⚡ 关键特性 (Key Features)


1. **上下文不再缺失**  
   AI 必须先从"单一事实来源"获取项目上下文，不能再"自由发挥"。

2. **决策过程可追溯**  
   每次 AI 的推理依据（最终提示词）都被完整记录，问题可以快速定位。

3. **人机协作可控**  
   AI 提交执行计划，人工审批后再执行，关键环节始终由人类把关。

---

## 👥 使用场景


#### 1. **独立开发者/全栈工程师**

- **特点**: 同时负责前后端、运维、产品设计，时间和精力有限
- **需求**: 希望 AI 能尽可能独立完成任务，人工只在关键环节干预
- **痛点**: 项目规模变大后，上下文爆炸、可维护性下降、AI 幻觉频发

#### 2. **项目管理者/技术主管**

- **特点**: 需要掌控项目进度、技术决策和团队协作
- **需求**: 需要清晰的任务跟踪、文档管理、进度统计和决策依据
- **痛点**: AI 辅助开发的过程不透明，难以审查和追溯

#### 3. **产品经理/需求分析师**

- **特点**: 需要快速验证想法、完成原型、管理需求变更
- **需求**: 需要将会议讨论、需求想法快速转化为结构化文档
- **痛点**: 需求散落在会议记录、聊天记录、邮件中，整理成本高

---


## 🚀 快速开始 (Getting Started)



### 📦 1. 快速部署（5 分钟）

详细步骤请参考：[**快速开始指南 (QUICK_START.md)**](docs/QUICK_START.md)

**超简版**:

```bash
# 1. 下载配置文件
curl -O https://raw.githubusercontent.com/houzhh15-hub/AIDG/main/docker-compose.ghcr.yml

# 2. 创建配置文件
cat > .env << 'EOF'
JWT_SECRET=your-super-secret-key-change-me
EOF

# 3. 启动服务（基础版，100MB）
docker compose -f docker-compose.ghcr.yml up -d

# 4. 打开浏览器访问
open http://localhost:8000
```

**就这么简单！** 🎉

> 💡 **提示**: 基础版（100MB）已包含核心功能。如需会议录音转写功能，请参考完整部署方案。

---

### 🌐 2. Web 界面使用



#### 📝 创建用户、项目和任务

1. **创建用户** (首次使用)
   - 访问 http://localhost:8000
   - 点击"注册"创建管理员账号
   - 使用用户名/密码登录

2. **创建项目**
   - 导航至"项目管理"
   - 点击"新建项目"，填写项目名称和描述
   - 配置特性列表和架构文档

3. **创建任务**
   - 进入项目详情页
   - 点击"新建任务"，填写任务信息
   - 为任务生成需求文档和设计文档

4. **绑定当前任务**
   - 在任务列表中点击"选择为当前任务"
   - 这样 AI 工具就能自动获取该任务的上下文

---

### 🔌 3. MCP 服务器配置



#### 在 AI 开发工具中接入 AIDG

AIDG 基于 [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) 提供标准化接口，支持任何兼容 MCP 的 AI 工具接入。

**推荐工具**:
- **Cursor** - 强大的 AI 代码编辑器
- **Continue** - VS Code 的 AI 辅助插件
- **Cline** - 命令行 AI 助手

**配置步骤** (以 Cursor 为例):

1. **打开 MCP 配置文件**:
   ```bash
   # macOS/Linux
   code ~/.cursor/mcp.json
   
   # Windows
   code %APPDATA%\Cursor\mcp.json
   ```
2. **添加 AIDG MCP Server**:
   ```json
   {
     "mcpServers": {
       "aidg": {
         "url": "http://localhost:8081",
         "apiKey": "your-jwt-token-here"
       }
     }
   }
   ```

3. **获取 API Key**:
   - 在 AIDG Web 界面登录
   - 导航至"用户设置" → "API 令牌"
   - 复制 JWT Token 并填入配置文件

4. **重启 AI 工具**，配置生效 ✅

**详细配置说明**: [MCP 配置文档](docs/MCP_PROMPTS_CONFIGURATION.md)

---
### 🔄 4. 完整开发流程

AIDG 支持完整的任务驱动开发流程：

```
需求阶段 → 设计阶段 → 执行阶段 → 测试阶段 → 集成阶段 → 知识回流
```

#### 📋 典型工作流

AIDG 的工作流与 MCP（Model Context Protocol）内置的提示词模板紧密集成，以实现引导式的、结构化的开发流程。用户可以基于这些模板进行补充、编辑或创建新的提示词。

1.  **需求文档生成**
    *   在 Web 界面创建任务后，可使用以下提示词模板引导 AI 生成需求文档。
    *   `mcp/requirements/generate`: 基于项目特性列表和任务描述，生成初步的需求文档。
    *   `mcp/requirements/refine`: 对已有的需求文档进行优化、补充或调整。

2.  **设计文档生成**
    *   基于已确定的需求文档，引导 AI 生成技术设计。
    *   `mcp/design/generate`: 根据需求文档，创建技术设计，包括模块划分、接口定义等。
    *   `mcp/design/refine`: 迭代和完善现有的设计文档。

3.  **执行计划提交**
    *   AI 根据设计文档，生成一份详细的、可分步执行的计划。
    *   `mcp/plan/generate`: 从设计文档生成编码或操作步骤的执行计划。
    *   人工在 Web 界面对该计划进行审批，确保执行方向的正确性。

4.  **自主执行与追踪**
    *   计划经审批后，AI 通过调用 `get_next_executable_step` 获取下一步操作。
    *   完成每一步后，通过 `update_plan_step_status` 回写执行状态和结果。
    *   整个过程在 Web 界面上可被实时追踪，确保过程可控。

## 📚 详细部署方案

想了解更多部署细节？我们为你准备了三份文档：

### 📖 文档导航

1. **[快速开始 (QUICK_START.md)](docs/QUICK_START.md)**  
   ⏱️ 5 分钟快速部署，适合想快速体验的用户

2. **[友好部署指南 (DEPLOYMENT_GUIDE_FRIENDLY.md)](docs/DEPLOYMENT_GUIDE_FRIENDLY.md)**  
   📘 完整的部署指南，包含两种方案：
   - **方案一**: 基础版部署（100MB，核心功能）
   - **方案二**: 完整版部署（会议录音转写功能）
   
   使用友好的语言，适合非技术人员阅读。

3. **[环境变量配置手册 (ENVIRONMENT_VARIABLES.md)](docs/ENVIRONMENT_VARIABLES.md)**  
   ⚙️ 所有环境变量的详细说明、默认值和最佳实践

### 🏗️ 架构说明

AIDG 采用统一容器架构，提供三个 Docker 镜像：

| **aidg-aidg** | ~100MB | Web Server + MCP Server + Frontend | 基础版，适合大部分场景 |
| **aidg-deps-service** | ~2GB | 说话人识别服务 (PyAnnote) | 完整版，需要会议功能 |
| **go-whisper** | ~500MB | Whisper 语音转写服务 | 完整版，需要会议功能 |

#### 🎨 三个镜像的关系

```
┌─────────────────────────────────────────────────────────────┐
│                       基础版部署                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  aidg-aidg (100MB)                                    │ │
│  │  ├── Web Server (REST API + 人机交互界面)              │ │
│  │  ├── MCP Server (AI 工具接口)                         │ │
│  │  └── Frontend (React 单页应用)                        │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                               │
│  ✅ 支持: 项目管理、任务管理、文档管理、AI 治理、进度追踪      │
│  ❌ 不支持: 会议录音转写、说话人识别                          │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                       完整版部署                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  aidg-aidg (100MB)                                    │ │
│  │  ├── Web Server                                       │ │
│  │  ├── MCP Server                                       │ │
│  │  └── Frontend                                         │ │
│  └───────────┬────────────────────────────────────────────┘ │
│              │                                               │
│              │ HTTP 调用                                     │
│              │                                               │
│              ↓                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  aidg-deps-service (2GB)                              │ │
│  │  └── PyAnnote (说话人识别 AI 模型)                     │ │
│  └────────────────────────────────────────────────────────┘ │
│              ↓                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  go-whisper (500MB)                                   │ │
│  │  └── Whisper (语音转写 AI 模型)                        │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                               │
│  ✅ 支持: 所有功能 + 会议录音转写 + 说话人识别                 │
└─────────────────────────────────────────────────────────────┘
```

**选择建议**:
- 🎯 **只需要 AI 治理、任务管理、文档管理** → 基础版（100MB）
- 🎙️ **需要会议录音自动转写功能** → 完整版（~2.5GB）

更多架构细节，请参考 [架构迁移文档](docs/ARCHITECTURE_MIGRATION.md)。

---

## 🏗️ 技术栈



### 后端技术

- **Go 1.22+** - 高性能后端服务
- **Gin Web Framework** - RESTful API 框架
- **JWT 认证** - 安全的用户认证机制
- **JSON 文件存储** - 轻量级数据持久化
- **Supervisor** - 进程管理和服务协调

### 前端技术

- **React 18** - 现代化前端框架
- **TypeScript 5** - 类型安全的 JavaScript
- **Ant Design** - 企业级 UI 组件库
- **Vite 5** - 极速的构建工具
- **React Router 6** - 客户端路由

### AI 集成

- **Model Context Protocol (MCP)** - 标准化 AI 工具接口

### 部署技术

- **Docker** - 容器化部署
- **Docker Compose** - 多服务编排
- **Alpine Linux** - 轻量级基础镜像
- **GitHub Actions** - CI/CD 自动化

---

## 🔧 开发指南



### 本地开发环境



#### 前置要求

- **Go 1.22+** - [安装指南](https://golang.org/doc/install)
- **Node.js 18+** - [安装指南](https://nodejs.org/)
- **Make** - 构建工具

#### 启动开发环境

```bash
# 1. 克隆仓库
git clone https://github.com/houzhh15-hub/AIDG.git
cd AIDG

# 2. 安装依赖
make install

# 3. 配置环境变量
cp .env.example .env
# 编辑 .env，设置 JWT_SECRET、ADMIN_DEFAULT_PASSWORD 等

# 4. 启动开发服务器
make dev
# 这会同时启动 Web Server、MCP Server 和前端开发服务器

# 或者分别启动（推荐，便于调试）
# Terminal 1: 启动后端
./bin/server

# Terminal 2: 启动 MCP Server
./bin/mcp-server

# Terminal 3: 启动前端
cd frontend && npm run dev
```

#### 构建生产版本

```bash
# 构建所有组件
make build

# 仅构建后端
make build-backend

# 仅构建前端
make build-frontend

# 构建 Docker 镜像
make docker-build
```

## 🔐 安全建议



### 生产环境配置检查清单

在生产环境部署前，请确保完成以下配置：

- [ ] **JWT_SECRET**: 使用至少 32 字符的随机字符串
- [ ] **ADMIN_DEFAULT_PASSWORD**: 设置强密码（包含大小写、数字、特殊字符）
- [ ] **MCP_PASSWORD**: 设置 MCP Server 访问密码
- [ ] **CORS_ALLOWED_ORIGINS**: 只允许信任的域名
- [ ] **LOG_FORMAT**: 设置为 `json` 便于日志分析
- [ ] **数据备份**: 定期备份 `data/` 目录
- [ ] **HTTPS**: 使用反向代理（Nginx/Traefik）启用 HTTPS
- [ ] **防火墙**: 限制只有必要的端口对外开放

详细安全配置，请参考 [环境变量配置手册](docs/ENVIRONMENT_VARIABLES.md)。

---

## 🤝 社区与支持



### 获取帮助

遇到问题？以下是获取帮助的途径：

1. **📖 查看文档**  
   - [快速开始](docs/QUICK_START.md)
   - [友好部署指南](docs/DEPLOYMENT_GUIDE_FRIENDLY.md)
   - [故障排查](docs/troubleshooting.md)

2. **🐛 提交 Issue**  
   在 [GitHub Issues](https://github.com/houzhh15-hub/AIDG/issues) 报告 Bug 或提出功能建议

3. **💬 加入讨论**  
   在 [GitHub Discussions](https://github.com/houzhh15-hub/AIDG/discussions) 参与社区讨论

### 常见问题 (FAQ)

**Q: 基础版和完整版有什么区别？**  
A: 基础版（100MB）包含核心的项目管理、任务管理、文档管理和 AI 治理功能。完整版（~2.5GB）额外包含会议录音自动转写和说话人识别功能。

**Q: 支持哪些 AI 工具接入？**  
A: AIDG 基于标准的 Model Context Protocol (MCP) 提供接口，理论上支持任何兼容 MCP 的 AI 工具，包括 Cursor、Continue、Cline 等。

**Q: 数据存储在哪里？如何备份？**  
A: 所有数据存储在 `data/` 目录下，使用 JSON 文件格式。建议定期备份该目录，可以使用 `tar` 或 `rsync` 等工具。

**Q: 如何升级到新版本？**  
A: 拉取最新镜像后重启服务即可：
```bash
docker compose pull
docker compose up -d
```

**Q: 可以在生产环境使用吗？**  
A: 可以。但请务必完成"安全建议"章节中的配置检查清单，特别是密钥、密码和 CORS 配置。

**Q: 支持多用户和权限管理吗？**  
A: 支持。AIDG 提供基于角色的访问控制 (RBAC)，可以创建多个用户并分配不同的权限。

更多问题，请查看 [故障排查文档](docs/troubleshooting.md)。

---

## 📜 许可证

本项目采用 [Apache-2.0 许可证](LICENSE)。

## 🙏 致谢

AIDG 的开发过程中受益于以下开源项目和社区：

- [Model Context Protocol](https://modelcontextprotocol.io/) - 标准化的 AI 工具协议
- [Gin Web Framework](https://gin-gonic.com/) - 高性能 Go Web 框架
- [React](https://react.dev/) - 现代化前端框架
- [Ant Design](https://ant.design/) - 企业级 UI 组件库
- [Whisper](https://github.com/openai/whisper) - OpenAI 语音识别模型
- [PyAnnote](https://github.com/pyannote/pyannote-audio) - 说话人识别库

---

<div align="center">

**让 AI 开发变得可控、透明、可追溯** ✨

Made with ❤️ by the AIDG Community

[⭐ Star on GitHub](https://github.com/houzhh15-hub/AIDG) | [📖 Documentation](docs/) | [💬 Discussions](https://github.com/houzhh15-hub/AIDG/discussions)

</div>