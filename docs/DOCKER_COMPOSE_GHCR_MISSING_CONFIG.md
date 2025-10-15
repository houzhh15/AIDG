# docker-compose.ghcr.yml 缺失配置分析

## 概览

`docker-compose.ghcr.yml` 作为使用 GHCR 镜像的基础配置，缺少了以下关键配置，这些配置对于外部依赖服务（Whisper、deps-service）是**必需的**。

---

## ❌ 缺失的关键配置

### 1. **Whisper 服务（转录服务）**

#### 当前状态
`docker-compose.ghcr.yml`: **完全缺失**

#### 应有配置
```yaml
services:
  whisper:
    image: ghcr.io/mutablelogic/go-whisper:latest
    platform: linux/amd64  # 或 linux/arm64
    container_name: aidg-whisper
    restart: unless-stopped
    ports:
      - "8082:80"
    volumes:
      - ./models/whisper:/data
      - ./data/meetings:/output
    networks:
      - aidg-network
```

#### 影响
- ❌ **无法进行音频转录**
- ❌ ASR Worker 失败（没有 Whisper API）
- ❌ 会议记录功能不可用

---

### 2. **Deps-Service（依赖服务）**

#### 当前状态
`docker-compose.ghcr.yml`: **完全缺失**

#### 应有配置
```yaml
services:
  deps-service:
    image: aidg-deps-service:latest  # 或构建
    container_name: aidg-deps-service
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - HUGGINGFACE_TOKEN=${HUGGINGFACE_TOKEN}
      - HF_HOME=/models/huggingface
    volumes:
      - ./data:/data
      - ./models:/models:ro
      - ./config:/app/config:ro
    networks:
      - aidg-network
```

#### 影响
- ❌ **无法进行 Speaker Diarization**（说话人分离）
- ❌ SD Worker 失败
- ❌ EMB Worker 失败
- ❌ 无法生成 SPK 文件和 embeddings

---

### 3. **依赖模式配置（Environment Variables）**

#### 当前状态
`docker-compose.ghcr.yml`: **完全缺失**

#### 应有配置
```yaml
environment:
  # === 依赖执行模式配置 ===
  - DEPENDENCY_MODE=fallback           # remote, fallback, local
  - ENABLE_AUDIO_CONVERSION=true
  - ENABLE_SPEAKER_DIARIZATION=true
  - ENABLE_DEGRADATION=true
  - DEPS_SERVICE_URL=http://aidg-deps-service:8080
  
  # === Whisper 配置 ===
  - WHISPER_MODE=go-whisper
  - WHISPER_API_URL=http://whisper:80
  
  # === 健康检查配置 ===
  - HEALTH_CHECK_INTERVAL=5m
  - HEALTH_CHECK_FAIL_THRESHOLD=3
  
  # === 离线模式 ===
  - ENABLE_OFFLINE=false               # GHCR 镜像通常在线使用
  - HF_HOME=/models/huggingface
```

#### 影响
- ❌ 无法正确路由到外部服务
- ❌ 不知道如何降级处理
- ❌ 健康检查不生效

---

### 4. **Volume 挂载**

#### 当前状态
`docker-compose.ghcr.yml`: **部分缺失**

#### 缺失的挂载
```yaml
volumes:
  # 已有（✅）
  - ./data/projects:/app/data/projects
  - ./data/users:/app/data/users
  - ./data/meetings:/app/data/meetings
  - ./data/audit_logs:/app/data/audit_logs
  
  # 缺失（❌）
  - ./data:/data                        # deps-service 需要
  - ./models:/models:ro                 # Whisper 和 PyAnnote 模型
  - ./bin/whisper:/app/bin/whisper:ro   # 可选：本地 Whisper 可执行文件
```

#### 影响
- ❌ deps-service 无法访问 `/data` 路径
- ❌ 模型文件无法共享
- ❌ 路径转换失败（`/app/data/` ↔ `/data/`）

---

### 5. **服务依赖（depends_on）**

#### 当前状态
`docker-compose.ghcr.yml`: **缺失**

#### 应有配置
```yaml
services:
  aidg:
    depends_on:
      whisper:
        condition: service_started
      deps-service:
        condition: service_started  # 可选：如果需要强依赖
```

#### 影响
- ⚠️ 服务启动顺序不确定
- ⚠️ aidg 可能先于依赖服务启动，导致初始连接失败

---

### 6. **安全配置（Security Hardening）**

#### 当前状态
`docker-compose.ghcr.yml`: **完全缺失**

#### 应有配置
```yaml
services:
  aidg:
    user: "1000:1000"
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    tmpfs:
      - /tmp:noexec,nosuid,nodev,size=100m
      - /app/tmp:noexec,nosuid,nodev,size=200m
```

#### 影响
- ⚠️ 容器以 root 运行（安全风险）
- ⚠️ 缺少权限限制
- ⚠️ 不符合生产环境安全标准

---

## 📋 完整的修复版本

### 修复后的 `docker-compose.ghcr.yml`

```yaml
# Docker Compose configuration for AIDG using published GHCR image
# This uses the pre-built image from GitHub Container Registry
# 
# Prerequisites:
#   1. Pull GHCR image:
#      docker pull ghcr.io/houzhh15-hub/aidg:v0.1.1
#   2. Build deps-service:
#      ./scripts/build-deps-service.sh
#   3. Set environment variables:
#      export HUGGINGFACE_TOKEN=hf_xxx
#
# Usage:
#   docker-compose -f docker-compose.ghcr.yml up -d
#   docker-compose -f docker-compose.ghcr.yml down
#   docker-compose -f docker-compose.ghcr.yml logs -f

services:
  # === Whisper 转录服务 ===
  whisper:
    image: ghcr.io/mutablelogic/go-whisper:latest
    platform: linux/amd64  # 根据你的平台选择 amd64 或 arm64
    container_name: aidg-whisper
    restart: unless-stopped
    ports:
      - "8082:80"
    volumes:
      - ./models/whisper:/data
      - ./data/meetings:/output
    networks:
      - aidg-network

  # === Deps-Service（依赖服务）===
  deps-service:
    image: aidg-deps-service:latest
    container_name: aidg-deps-service
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - HUGGINGFACE_TOKEN=${HUGGINGFACE_TOKEN:-}
      - HF_HOME=/models/huggingface
      - LOG_LEVEL=debug
    volumes:
      - ./data:/data
      - ./models:/models:ro
      - ./config:/app/config:ro
    networks:
      - aidg-network
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 5s
      retries: 3

  # === Unified AIDG Service ===
  aidg:
    image: ghcr.io/houzhh15-hub/aidg:${IMAGE_TAG:-v0.1.1}
    container_name: aidg-unified
    depends_on:
      whisper:
        condition: service_started
      deps-service:
        condition: service_started
    ports:
      - "8000:8000"  # Web Server (Human Interface)
      - "8081:8081"  # MCP Server (AI Interface)
    # Security hardening
    user: "1000:1000"
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    tmpfs:
      - /tmp:noexec,nosuid,nodev,size=100m
      - /app/tmp:noexec,nosuid,nodev,size=200m
    environment:
      # === 依赖执行模式配置 ===
      - DEPENDENCY_MODE=remote           # 使用远程 deps-service
      - ENABLE_AUDIO_CONVERSION=true
      - ENABLE_SPEAKER_DIARIZATION=true
      - ENABLE_DEGRADATION=true
      - DEPS_SERVICE_URL=http://aidg-deps-service:8080
      
      # === Whisper 配置 ===
      - WHISPER_MODE=go-whisper
      - WHISPER_API_URL=http://whisper:80
      
      # === 健康检查配置 ===
      - HEALTH_CHECK_INTERVAL=5m
      - HEALTH_CHECK_FAIL_THRESHOLD=3
      
      # === 基础配置 ===
      - ENV=development
      - PORT=8000
      - MCP_HTTP_PORT=8081
      - LOG_LEVEL=debug
      - LOG_FORMAT=console
      
      # === 安全配置 ===
      - JWT_SECRET=dev-secret-change-me-in-production-at-least-32-chars
      - USER_JWT_SECRET=dev-user-jwt-secret-at-least-32-chars
      - ADMIN_DEFAULT_PASSWORD=admin123
      - MCP_PASSWORD=dev-mcp-password
      
      # === 数据目录 ===
      - PROJECTS_DIR=/app/data/projects
      - USERS_DIR=/app/data/users
      - MEETINGS_DIR=/app/data/meetings
      - AUDIT_LOGS_DIR=/app/data/audit_logs
      
      # === MCP 配置 ===
      - MCP_SERVER_URL=http://localhost:8000
      
      # === CORS 配置 ===
      - CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:8000
      
      # === HuggingFace 配置 ===
      - HF_HOME=/models/huggingface
      - ENABLE_OFFLINE=false
      
      # === 向后兼容（不使用，但保留避免报错）===
      - FFMPEG_PATH=/usr/bin/ffmpeg
      - PYTHON_PATH=/opt/pyannote/bin/python3
      - DIARIZATION_SCRIPT=/external/scripts/pyannote_diarize.py
    volumes:
      # 数据持久化
      - ./data/projects:/app/data/projects
      - ./data/users:/app/data/users
      - ./data/meetings:/app/data/meetings
      - ./data/audit_logs:/app/data/audit_logs
      # deps-service 需要的路径
      - ./data:/data
      # 模型文件（只读）
      - ./models:/models:ro
      # 可选：本地 Whisper 可执行文件
      - ./bin/whisper:/app/bin/whisper:ro
    networks:
      - aidg-network
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "sh", "-c", "wget --no-verbose --tries=1 --spider http://localhost:8000/health && wget --no-verbose --tries=1 --spider http://localhost:8081/health"]
      interval: 30s
      timeout: 5s
      retries: 3

networks:
  aidg-network:
    driver: bridge
```

---

## 🔑 关键差异对比表

| 配置项 | docker-compose.ghcr.yml (原) | docker-compose.yml | 缺失影响 |
|--------|------------------------------|-----------------------|----------|
| **whisper 服务** | ❌ 无 | ✅ 有 | 🔴 无法转录 |
| **deps-service** | ❌ 无 | ✅ 有 | 🔴 无法 SD/EMB |
| **DEPENDENCY_MODE** | ❌ 无 | ✅ fallback | 🔴 不知道用谁 |
| **DEPS_SERVICE_URL** | ❌ 无 | ✅ 有 | 🔴 无法连接 |
| **WHISPER_API_URL** | ❌ 无 | ✅ 有 | 🔴 无法连接 |
| **depends_on** | ❌ 无 | ✅ 有 | 🟡 启动顺序 |
| **安全配置** | ❌ 无 | ✅ 有 | 🟡 安全风险 |
| **./data 挂载** | ❌ 无 | ✅ 有 | 🔴 路径错误 |
| **./models 挂载** | ❌ 无 | ✅ 有 | 🔴 模型缺失 |
| **健康检查配置** | ❌ 无 | ✅ 有 | 🟡 降级失败 |

**图例**:
- 🔴 严重：功能完全不可用
- 🟡 警告：功能降级或安全问题

---

## 📝 部署前检查清单

### 1. 环境变量
```bash
# 设置 HuggingFace Token
export HUGGINGFACE_TOKEN=hf_REPLACE_WITH_YOUR_TOKEN_HERE

# 验证
echo $HUGGINGFACE_TOKEN
```

### 2. 构建 deps-service
```bash
./scripts/build-deps-service.sh
```

### 3. 创建必要目录
```bash
mkdir -p data/{projects,users,meetings,audit_logs}
mkdir -p models/{whisper,huggingface}
mkdir -p bin/whisper
mkdir -p config
```

### 4. 拉取 GHCR 镜像
```bash
docker pull ghcr.io/houzhh15-hub/aidg:v0.1.1
docker pull ghcr.io/mutablelogic/go-whisper:latest
```

### 5. 启动服务
```bash
docker-compose -f docker-compose.ghcr.yml up -d
```

### 6. 验证服务
```bash
# 检查所有服务状态
docker-compose -f docker-compose.ghcr.yml ps

# 检查健康状态
curl http://localhost:8000/health
curl http://localhost:8081/health
curl http://localhost:8080/api/v1/health  # deps-service
curl http://localhost:8082/                # whisper

# 查看日志
docker-compose -f docker-compose.ghcr.yml logs -f
```

---

## 🎯 推荐配置策略

### 场景 1: 开发环境（推荐使用 docker-compose.yml）
- 使用本地构建的镜像
- 包含所有依赖
- 快速迭代

### 场景 2: 测试环境（使用修复后的 docker-compose.ghcr.yml）
- 使用 GHCR 镜像
- 包含 whisper + deps-service
- 接近生产环境

### 场景 3: 生产环境（使用 docker-compose.ghcr.yml + 优化）
- 固定版本标签（如 v0.1.1）
- 独立的 deps-service 集群
- 完整的安全加固
- 监控和告警

---

## 相关文档

- [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md) - Docker 部署指南
- [DEPS_SERVICE_GUIDE.md](DEPS_SERVICE_GUIDE.md) - Deps-Service 配置
- [BUILD_SCRIPTS_REFACTOR.md](BUILD_SCRIPTS_REFACTOR.md) - 构建脚本说明

---

**最后更新**: 2025-10-14
