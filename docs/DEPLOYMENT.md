# AIDG 部署指南

本文档提供 AIDG 的完整部署说明，包括快速开始、部署方案选择、环境变量配置和 HTTPS 设置。

---

## 目录

- [快速开始](#快速开始)
- [部署方案](#部署方案)
- [本地构建](#本地构建)
- [生产部署](#生产部署)
- [环境变量](#环境变量)
- [HTTPS 配置](#https-配置)
- [故障排除](#故障排除)

---

## 快速开始

### 最简部署（5 分钟）

```bash
# 1. 克隆仓库
git clone https://github.com/houzhh15/AIDG.git
cd AIDG

# 2. 配置环境变量
cp deployments/production/.env.example deployments/production/.env
# 编辑 .env 文件，修改必要的密钥

# 3. 启动 Lite 方案（仅主服务）
cd deployments/production
docker compose -f docker-compose.lite.yml up -d

# 4. 访问服务
# Web 界面: http://localhost:8000
# MCP 服务: http://localhost:8001
```

---

## 部署方案

AIDG 提供三种部署方案，根据功能需求选择：

### 方案对比

| 方案 | 镜像大小 | 服务组成 | 适用场景 |
|------|---------|---------|---------|
| **Lite** | ~250MB | aidg | 项目管理、任务管理、文档管理 |
| **Standard** | ~2.3GB | aidg + deps + whisper | + 音频转换、说话人分离、语音转录 |
| **Full** | ~4.5GB | aidg + deps + whisper + nlp | + 语义搜索 |

### 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| server | 8000 | Web API + 前端界面 |
| mcp-server | 8001 | MCP 协议服务（AI 工具接入） |
| deps-service | 8080 | 内部服务，不对外暴露 |
| whisper | 80 | 内部服务，不对外暴露 |
| nlp-service | 5001 | 内部服务，不对外暴露 |

### 功能矩阵

| 功能 | Lite | Standard | Full |
|------|:----:|:--------:|:----:|
| 项目/任务管理 | ✅ | ✅ | ✅ |
| 文档管理 | ✅ | ✅ | ✅ |
| MCP 协议接入 | ✅ | ✅ | ✅ |
| 执行计划导航 | ✅ | ✅ | ✅ |
| 文件转换 (OCR) | ✅ | ✅ | ✅ |
| 音频格式转换 | ❌ | ✅ | ✅ |
| 说话人分离 | ❌ | ✅ | ✅ |
| 语音转文字 | ❌ | ✅ | ✅ |
| 语义搜索 | ❌ | ❌ | ✅ |

---

## 本地构建

适用于开发测试或需要自定义镜像的场景。

### 构建镜像

```bash
cd build

# 构建所有镜像
./build-images.sh all

# 构建单个镜像
./build-images.sh main      # 主服务镜像
./build-images.sh deps      # 依赖服务镜像
./build-images.sh nlp       # NLP 服务镜像
./build-images.sh lite      # 轻量镜像（无 Python）

# 使用自定义标签
./build-images.sh --tag v1.0.0 main

# 多架构构建
./build-images.sh --platform linux/amd64,linux/arm64 main
```

### 启动本地构建服务

```bash
cd build/local

# 配置环境变量
cp .env.example .env

# 选择方案启动
docker compose -f docker-compose.lite.yml up -d      # Lite
docker compose -f docker-compose.standard.yml up -d  # Standard
docker compose -f docker-compose.full.yml up -d      # Full
```

---

## 生产部署

使用 GHCR 预构建镜像进行快速部署。

### 部署步骤

```bash
cd deployments/production

# 1. 配置环境变量（必须修改安全密钥！）
cp .env.example .env
vi .env
```

**⚠️ 重要：必须修改以下安全配置**

```bash
# .env 文件示例
IMAGE_TAG=latest

# 安全配置（必须修改！）
JWT_SECRET=your-secure-32-char-secret-key-here
USER_JWT_SECRET=another-secure-32-char-key-here
IDP_ENCRYPTION_KEY=32-char-encryption-key-here
ADMIN_DEFAULT_PASSWORD=YourStrongPassword123!
MCP_PASSWORD=your-mcp-password-here

# HuggingFace Token（Standard/Full 方案需要）
HUGGINGFACE_TOKEN=hf_xxxxxxxxxxxxxxxxxxxxx
```

```bash
# 2. 下载 AI 模型（Standard/Full 方案需要，离线环境必须提前准备）
# 参见下方 "离线模型准备" 章节
cd ../..
python scripts/download_pyannote_models.py --cache_dir ./models/huggingface/hub --token $HUGGINGFACE_TOKEN

# 3. 启动服务
cd deployments/production
docker compose -f docker-compose.lite.yml up -d      # Lite 方案
docker compose -f docker-compose.standard.yml up -d  # Standard 方案
docker compose -f docker-compose.full.yml up -d      # Full 方案

# 4. 查看服务状态
docker compose -f docker-compose.lite.yml ps
docker compose -f docker-compose.lite.yml logs -f

# 5. 停止服务
docker compose -f docker-compose.lite.yml down
```

### 更新镜像版本

```bash
# 拉取最新镜像
docker compose -f docker-compose.standard.yml pull

# 重启服务
docker compose -f docker-compose.standard.yml up -d
```

---

## 离线模型准备

Standard 和 Full 方案需要 AI 模型，首次启动时会自动下载。如果部署环境无法访问互联网，需要提前下载模型。

### PyAnnote 说话人分离模型

#### 在联网环境下载

```bash
# 1. 安装依赖
pip install huggingface_hub

# 2. 设置 HuggingFace Token（需要先在 huggingface.co 接受模型许可）
export HUGGINGFACE_TOKEN=hf_xxxxxxxxxxxxxxxxxxxxx

# 3. 下载模型到指定目录
python scripts/download_pyannote_models.py \
  --cache_dir ./models/huggingface/hub \
  --token $HUGGINGFACE_TOKEN

# 4. 验证下载（可选）
python scripts/download_pyannote_models.py \
  --cache_dir ./models/huggingface/hub \
  --check
```

下载的模型（约 250MB）：
- `pyannote/speaker-diarization-3.1` - 主 pipeline 配置
- `pyannote/segmentation-3.0` - 分段模型
- `pyannote/wespeaker-voxceleb-resnet34-LM` - 说话人嵌入模型
- `pyannote/embedding` - 说话人聚类嵌入模型

#### 拷贝到离线环境

```bash
# 在联网机器上打包
tar -czvf pyannote-models.tar.gz models/huggingface/

# 拷贝到离线环境后解压
tar -xzvf pyannote-models.tar.gz
```

### Whisper 语音识别模型

Whisper 模型可以通过 Web 界面下载，也可以手动准备：

#### 方法一：手动下载（推荐离线部署）

从 HuggingFace 下载 GGML 格式模型：https://huggingface.co/ggerganov/whisper.cpp/tree/main

```bash
# 下载到 models/whisper 目录
cd models/whisper

# 下载所需模型（选择需要的大小）
wget https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin      # 74 MB
wget https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin      # 141 MB
wget https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin     # 465 MB
wget https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin    # 1.4 GB
wget https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin  # 2.9 GB
```

#### 方法二：通过 go-whisper CLI 下载

```bash
# 启动 whisper 容器后，使用 gowhisper CLI
docker exec aidg-whisper gowhisper download ggml-medium.bin
```

#### 拷贝到离线环境

```bash
# 在联网机器上打包
tar -czvf whisper-models.tar.gz models/whisper/

# 拷贝到离线环境后解压
tar -xzvf whisper-models.tar.gz
```

### 模型目录结构

部署前确保以下目录结构：

```
AIDG/
├── models/
│   ├── huggingface/
│   │   └── hub/
│   │       └── models--pyannote--*/     # PyAnnote 模型
│   └── whisper/
│       ├── ggml-tiny.bin                # Whisper 模型
│       ├── ggml-base.bin
│       ├── ggml-small.bin
│       └── ggml-large-v3.bin
├── data/
│   ├── projects/
│   ├── users/
│   └── meetings/
└── ...
```

---

## 环境变量

### 核心配置

| 变量名 | 说明 | 默认值 | 必需 |
|-------|------|--------|:----:|
| `ENV` | 运行环境 | development | |
| `PORT` | Web 服务端口 | 8000 | |
| `MCP_HTTP_PORT` | MCP 服务端口 | 8001 | |
| `LOG_LEVEL` | 日志级别 | info | |
| `LOG_FORMAT` | 日志格式 (json/console) | json | |

### 安全配置（生产环境必须修改）

| 变量名 | 说明 | 要求 |
|-------|------|------|
| `JWT_SECRET` | JWT 签名密钥 | 至少 32 字符 |
| `USER_JWT_SECRET` | 用户 JWT 密钥 | 至少 32 字符 |
| `IDP_ENCRYPTION_KEY` | IDP 加密密钥 | 32 字符 |
| `ADMIN_DEFAULT_PASSWORD` | 管理员初始密码 | 强密码 |
| `MCP_PASSWORD` | MCP 访问密码 | 建议复杂密码 |

### 服务配置

| 变量名 | 说明 | 可选值 |
|-------|------|--------|
| `DEPENDENCY_MODE` | 依赖执行模式 | disabled / fallback / remote |
| `DEPS_SERVICE_URL` | deps-service 地址 | http://aidg-deps-service:8080 |
| `WHISPER_MODE` | Whisper 模式 | disabled / go-whisper |
| `WHISPER_API_URL` | Whisper API 地址 | http://aidg-whisper:80 |
| `NLP_SERVICE_URL` | NLP 服务地址 | http://aidg-nlp:5001 |
| `ENABLE_SEMANTIC_SEARCH` | 启用语义搜索 | true / false |

### HuggingFace 配置（deps-service 需要）

| 变量名 | 说明 | 必需 |
|-------|------|:----:|
| `HUGGINGFACE_TOKEN` | HuggingFace API Token（首次下载模型时需要） | ⚠️ |
| `HF_HUB_CACHE` | Hub 缓存目录（PyAnnote 模型存放路径） | ✅ |

> **注意**：只需配置 `HF_HUB_CACHE` 环境变量。`HF_HOME`、`TRANSFORMERS_CACHE`、`TORCH_HOME` 是冗余配置，可以省略。

### 数据目录配置

| 变量名 | 默认值 |
|-------|--------|
| `PROJECTS_DIR` | /app/data/projects |
| `USERS_DIR` | /app/data/users |
| `MEETINGS_DIR` | /app/data/meetings |
| `AUDIT_LOGS_DIR` | /app/data/audit_logs |

---

## HTTPS 配置

### 方法一：使用覆盖配置

```bash
cd deployments/production

# 1. 生成自签名证书（测试用）
mkdir -p ../../certs
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout ../../certs/server.key \
  -out ../../certs/server.crt \
  -subj "/CN=localhost"

# 2. 使用 HTTPS 覆盖配置启动
docker compose -f docker-compose.standard.yml \
               -f docker-compose.https.yml up -d

# 3. 访问 HTTPS 服务
# https://localhost:443
```

### 方法二：使用反向代理（推荐）

使用 Nginx 或 Traefik 作为反向代理处理 TLS 终止：

```nginx
# nginx.conf 示例
server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /mcp {
        proxy_pass http://localhost:8001;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

---

## 故障排除

### 常见问题

**1. 服务启动失败**

```bash
# 查看日志
docker compose -f docker-compose.lite.yml logs aidg

# 检查健康状态
curl http://localhost:8000/health
curl http://localhost:8001/health
```

**2. deps-service 启动慢**

首次启动需要下载 PyAnnote 模型（约 1GB），请耐心等待。确保：
- `HUGGINGFACE_TOKEN` 已正确配置
- 网络可访问 huggingface.co

**3. 权限问题**

```bash
# 确保数据目录权限正确
sudo chown -R 1000:1000 data/
sudo chown -R 1000:1000 models/
```

**4. 端口冲突**

```bash
# 检查端口占用
lsof -i :8000
lsof -i :8001

# 修改端口映射（在 .env 中设置）
SERVER_PORT=9000
MCP_PORT=9001
```

### 健康检查

```bash
# 检查所有服务健康状态
curl http://localhost:8000/health  # Web API
curl http://localhost:8001/health  # MCP Server

# 检查 deps-service（仅 Standard/Full）
docker exec aidg-deps-service curl -f http://localhost:8080/api/v1/health
```

### 日志调试

```bash
# 实时查看日志
docker compose -f docker-compose.standard.yml logs -f

# 查看特定服务日志
docker compose -f docker-compose.standard.yml logs -f aidg

# 增加日志级别
# 在 .env 中设置 LOG_LEVEL=debug
```

---

## 目录结构

```
AIDG/
├── build/
│   ├── dockerfiles/           # 所有 Dockerfile
│   │   ├── Dockerfile         # 主服务镜像
│   │   ├── Dockerfile.deps    # 依赖服务镜像
│   │   ├── Dockerfile.nlp     # NLP 服务镜像
│   │   └── Dockerfile.lite    # 轻量镜像
│   ├── local/                 # 本地构建配置
│   │   ├── docker-compose.lite.yml
│   │   ├── docker-compose.standard.yml
│   │   ├── docker-compose.full.yml
│   │   └── .env.example
│   └── build-images.sh        # 统一构建脚本
├── deployments/
│   └── production/            # 生产部署配置（GHCR 镜像）
│       ├── docker-compose.lite.yml
│       ├── docker-compose.standard.yml
│       ├── docker-compose.full.yml
│       ├── docker-compose.https.yml
│       └── .env.example
├── data/                      # 数据目录（运行时）
├── models/                    # 模型文件（运行时）
└── certs/                     # TLS 证书（HTTPS 用）
```

---

## 相关链接

- [GitHub 仓库](https://github.com/houzhh15/AIDG)
- [MCP 协议文档](https://modelcontextprotocol.io/)
- [HuggingFace Token 获取](https://huggingface.co/settings/tokens)
