# AIDG (AI-Dev-Gov)

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Node Version](https://img.shields.io/badge/Node.js-18+-339933?style=flat&logo=node.js)](https://nodejs.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![License](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)

> **让 AI 开发变得可控、透明、可追溯** ✨

---

## 📖 简介

"AI-Dev-Gov 不是让 AI 更聪明，而是为'无状态'的 AI 构建'有状态'的外部记忆与治理系统，通过'先取证、后推理、必审批、可追溯'的强制工作流，将 AI 开发从'黑盒魔法'转变为'透明工程'，重新定义了 AI 时代开发者的核心价值：从'编写代码'到'定义问题、构建上下文、审查结果'的架构师角色。"

### 关键特性

1. 治理闭环：先取证后推理、审批把关、提示词与执行步骤前置记录，形成“证据—决策—产出—审计”链条。
2. 上下文结构：多层级文档 + 任务级需求/设计/测试 + 执行计划工件，使信息从会议/原始素材 → 结构化知识 → 精准注入。
3. 可追溯执行：执行计划与提示词日志，双轴记录 AI 所“依据什么做了什么”。
4. 可视化：项目状态页、步骤状态、依赖图、时间维度进展树、特性列表与架构一体化展示，降低认知负荷与协调成本。

### 核心理念


#### AI辅助开发时代的挑战：

1. AI 开发的瓶颈不在于模型能力，而在于上下文管理和过程治理
- **误区**: 认为 AI 开发问题源于"模型不够聪明"，期待 GPT-5、Claude 4 来解决一切
- **真相**: 即使模型能力再强，如果缺乏上下文管理和过程治理，仍会导致"垃圾进、垃圾出"
**深层原因**: 人类倾向于相信"技术奇点"能解决一切问题，而不愿意承认"管理和规范"才是关键。

2. 真正的 AI 驱动开发，不是让 AI 代替人思考，而是构建一个能让人和 AI 的思考过程"相互对齐"并"留下痕迹"的系统
- **误区**: 期望 AI 能"一步到位"地解决问题，追求"无人干预的全自动化"
- **真相**: AI 的价值在于"辅助"而非"替代"，人机协同的关键是"思考过程的透明化"
**深层原因**: 过度乐观主义（Techno-optimism）导致人们忽视了 AI 的局限性和不可预测性。

3. 在 AI 时代，开发者的核心价值正在从"编写代码"转向"定义问题、构建上下文和审查结果"
- **误区**: 认为 AI 会"取代程序员"，引发职业焦虑
- **真相**: AI 只会淘汰"打字员式的程序员"，而提升"架构师式的程序员"的价值
**深层原因**: 人们低估了"问题定义"和"质量控制"的难度，高估了"代码编写"的价值。
—

#### 洞察与机遇

1. “AI 的 ‘无状态’ 与 项目开发 ‘有状态’ 的结构性矛盾，必须靠外部治理层弥合。”
2. “治理提升不是限制 AI，而是把 AI 的能力从‘瞬时灵感’固化为‘可迭代资产’。”
3. “提示词模板是把最佳实践从‘经验’升级为‘协议’的媒介，策略即代码。”
4. “导航算法让计划成为‘活文档’，执行状态本身就是对计划质量的持续验证。”

#### AIDG 的核心理念：

1. 复杂系统必须支持"发散-收敛"的知识迭代模式
2. 过程的透明化是质量保证和持续改进的基础
3. 单一事实来源 (SSoT) 是避免信息不一致的唯一可靠方法

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

1. **获取 API Key**:
   - 在 AIDG Web 界面登录
   - 导航至"用户设置" → "API 令牌"
   - 复制 JWT Token 并填入配置文件

2. **添加 AIDG MCP Server**:
   在 `config.json` 中添加 MCP 服务器配置：
   ```json
   {
     "mcpServers": {
       "aidg": {
         "url": "http://localhost:8081/mcp",
         "headers": {
		"Authorization": "Bearer your-jwt-token-here"
	 }
       }
     }
   }	
   ```

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

## 📜 许可证

本项目采用 [Apache-2.0 许可证](LICENSE)。

## 感谢

AIDG 的开发过程中受益于以下开源项目和社区：

- [Model Context Protocol](https://modelcontextprotocol.io/) - 标准化的 AI 工具协议
- [Gin Web Framework](https://gin-gonic.com/) - 高性能 Go Web 框架
- [React](https://react.dev/) - 现代化前端框架
- [Ant Design](https://ant.design/) - 企业级 UI 组件库
- [Whisper](https://github.com/openai/whisper) - OpenAI 语音识别模型
- [PyAnnote](https://github.com/pyannote/pyannote-audio) - 说话人识别库

---