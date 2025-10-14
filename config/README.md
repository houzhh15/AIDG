# AIDG 配置文件指南

本目录包含 AIDG 系统的配置文件示例,支持开发环境和生产环境的不同部署需求。

## 📁 文件列表

| 文件 | 用途 | 依赖模式 | 适用场景 |
|------|------|----------|----------|
| `config.dev.yaml` | 开发环境配置 | `local` | 本地开发、单元测试 |
| `config.prod.yaml` | 生产环境配置 | `fallback` | 生产部署、高可用场景 |

---

## 🔧 核心配置项说明

### 依赖执行配置 (dependency)

**依赖执行配置**是本次设计的核心,用于控制外部命令行工具 (FFmpeg/PyAnnote) 的执行方式。

#### 执行模式 (mode)

| 模式 | 说明 | 优点 | 缺点 | 适用场景 |
|------|------|------|------|----------|
| **local** | 直接调用本地安装的工具 (`exec.Command`) | • 简单直接\u003cbr\u003e• 无网络开销\u003cbr\u003e• 适合开发 | • 依赖主容器包含工具\u003cbr\u003e• 镜像体积大 (1.87GB) | 开发环境、宿主机部署 |
| **remote** | 通过 HTTP 调用独立依赖服务 | • 解耦部署\u003cbr\u003e• 主镜像轻量 (96.4MB)\u003cbr\u003e• 资源隔离 | • 网络依赖\u003cbr\u003e• 单点故障风险 | 生产环境 (有依赖服务) |
| **fallback** | 优先远程,失败自动降级到本地 | • 高可用性\u003cbr\u003e• 灵活部署\u003cbr\u003e• 零停机更新 | • 需要同时配置远程和本地 | **推荐生产环境** |

**示例配置**:

```yaml
dependency:
  mode: fallback  # 推荐:优先远程,失败降级本地
  service_url: "http://deps-service:8080"
  shared_volume_path: "/data"
  local_binary_paths:
    ffmpeg: "/usr/local/bin/ffmpeg"
    python: "/usr/bin/python3"
  timeout: 600s
  allowed_commands:
    - ffmpeg
    - pyannote
```

#### 配置字段详解

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `mode` | string | 是 | `local` | 执行模式: `local`/`remote`/`fallback` |
| `service_url` | string | 否 | - | 依赖服务 HTTP 端点 (remote/fallback 模式必填) |
| `shared_volume_path` | string | 是 | `/data` | 共享卷基础路径,主服务和依赖服务必须挂载相同卷 |
| `local_binary_paths` | map | 否 | - | 本地工具路径,用于 local 模式或 fallback 降级 |
| `timeout` | duration | 否 | `300s` | 默认命令执行超时时间 (建议 5-10 分钟) |
| `allowed_commands` | array | 否 | `[]` | 命令白名单,安全控制 (空表示允许所有) |

---

## 🚀 快速开始

### 开发环境部署

**前提条件**:
- 本地已安装 FFmpeg: `brew install ffmpeg` (macOS) 或 `apt-get install ffmpeg` (Linux)
- 本地已安装 Python 3.8+: `python3 --version`
- 安装 PyAnnote 依赖: `pip3 install pyannote.audio`

**启动步骤**:

```bash
# 1. 使用开发配置启动
./bin/server -config config/config.dev.yaml

# 或使用 Docker Compose
docker-compose -f docker-compose.yml up -d

# 2. 验证服务
curl http://localhost:8081/health

# 3. 测试音频处理
curl -X POST http://localhost:8081/api/v1/meetings \
  -H "Content-Type: application/json" \
  -d '{"name": "测试会议", "description": "开发环境测试"}'
```

### 生产环境部署

**方式 A: Docker Compose 一键部署 (推荐)**

```bash
# 1. 创建 docker-compose.prod.yml (示例见下文)

# 2. 启动完整堆栈 (主服务 + 依赖服务)
docker-compose -f docker-compose.prod.yml up -d

# 3. 验证服务
curl http://localhost:8081/health         # 主服务
curl http://localhost:8080/api/v1/health  # 依赖服务

# 4. 观察日志
docker-compose -f docker-compose.prod.yml logs -f
```

**方式 B: 用户自建依赖服务镜像**

```bash
# 1. 构建依赖服务镜像 (包含 FFmpeg + PyAnnote)
./scripts/build-deps-service.sh --with-ffmpeg --with-pyannote

# 2. 启动依赖服务
docker run -d --name deps-service \
  -v $(pwd)/data:/data \
  -p 8080:8080 \
  aidg-deps:latest

# 3. 启动主服务 (使用轻量镜像)
docker run -d --name aidg-main \
  -v $(pwd)/data:/data \
  -v $(pwd)/config:/app/config \
  -e CONFIG_FILE=/app/config/config.prod.yaml \
  -p 8081:8081 \
  ghcr.io/your-org/aidg:latest
```

**方式 C: 仅本地模式 (无依赖服务)**

```bash
# 1. 修改配置: dependency.mode: local

# 2. 使用完整镜像 (包含 FFmpeg + PyAnnote)
docker run -d --name aidg-full \
  -v $(pwd)/data:/data \
  -v $(pwd)/config:/app/config \
  -e CONFIG_FILE=/app/config/config.prod.yaml \
  -p 8081:8081 \
  ghcr.io/your-org/aidg-full:latest
```

---

## 📦 Docker Compose 配置示例

### 完整堆栈配置 (主服务 + 依赖服务)

创建 `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  # 主服务 (AIDG Core, 96.4MB 轻量镜像)
  aidg-main:
    image: ghcr.io/your-org/aidg:latest
    container_name: aidg-main
    restart: unless-stopped
    
    ports:
      - "8081:8081"      # API Server
      - "9090:9090"      # Prometheus Metrics
    
    volumes:
      # 共享数据卷 (与依赖服务共享)
      - shared-data:/data
      # 模型缓存 (Whisper)
      - ./models:/models:ro
      # 配置文件
      - ./config:/app/config:ro
    
    environment:
      # 配置文件路径
      - CONFIG_FILE=/app/config/config.prod.yaml
      
      # 依赖执行配置 (可选,覆盖配置文件)
      - DEPENDENCY_MODE=fallback
      - DEPENDENCY_SERVICE_URL=http://deps-service:8080
      - DEPENDENCY_SHARED_VOLUME=/data
      - DEPENDENCY_TIMEOUT=600s
      
      # HuggingFace Token (从宿主机环境变量读取)
      - HUGGINGFACE_TOKEN=${HUGGINGFACE_TOKEN}
      
      # 日志级别
      - LOG_LEVEL=info
      - LOG_FORMAT=json
    
    depends_on:
      deps-service:
        condition: service_healthy
    
    networks:
      - aidg-network
    
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8081/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    
    # 资源限制 (生产环境建议配置)
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '1.0'
          memory: 1G

  # 依赖服务 (FFmpeg + PyAnnote, 1.8GB)
  deps-service:
    image: ghcr.io/your-org/aidg-deps:latest
    container_name: deps-service
    restart: unless-stopped
    
    ports:
      - "8080:8080"      # Command Executor API
    
    volumes:
      # 共享数据卷 (与主服务共享)
      - shared-data:/data
    
    environment:
      # 命令白名单配置
      - ALLOWED_COMMANDS=ffmpeg,pyannote
      
      # 安全配置 (可选)
      # - AUTH_TOKEN=${DEPS_SERVICE_TOKEN}
      
      # 日志级别
      - LOG_LEVEL=info
    
    networks:
      - aidg-network
    
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s
    
    # 资源限制 (依赖服务通常需要更多资源)
    deploy:
      resources:
        limits:
          cpus: '4.0'
          memory: 4G
        reservations:
          cpus: '2.0'
          memory: 2G

# 网络配置
networks:
  aidg-network:
    driver: bridge

# 卷配置
volumes:
  # 共享数据卷 (主服务和依赖服务共同访问)
  shared-data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: ./data  # 宿主机路径
```

### 轻量部署配置 (仅主服务,降级到本地)

创建 `docker-compose.lite.yml`:

```yaml
version: '3.8'

services:
  aidg-main:
    image: ghcr.io/your-org/aidg-full:latest  # 使用完整镜像
    container_name: aidg-main
    restart: unless-stopped
    
    ports:
      - "8081:8081"
    
    volumes:
      - ./data:/data
      - ./models:/models:ro
      - ./config:/app/config:ro
    
    environment:
      - CONFIG_FILE=/app/config/config.prod.yaml
      - DEPENDENCY_MODE=local  # 本地模式
      - LOG_LEVEL=info
    
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8081/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

---

## 🧪 配置验证

### 验证配置文件语法

```bash
# 使用 yamllint (推荐安装)
yamllint config/config.dev.yaml
yamllint config/config.prod.yaml

# 或使用 Python
python3 -c "import yaml; yaml.safe_load(open('config/config.dev.yaml'))"
```

### 验证依赖可用性

```bash
# 开发环境: 检查本地工具
ffmpeg -version
python3 -c "import pyannote.audio; print(pyannote.audio.__version__)"

# 生产环境: 检查依赖服务
curl http://localhost:8080/api/v1/health

# 测试命令执行
curl -X POST http://localhost:8080/api/v1/execute \
  -H "Content-Type: application/json" \
  -d '{
    "command": "ffmpeg",
    "args": ["-version"],
    "timeout": "5s"
  }'
```

### 测试降级功能

```bash
# 1. 启动完整堆栈
docker-compose -f docker-compose.prod.yml up -d

# 2. 验证远程模式正常
docker-compose logs aidg-main | grep "remote"

# 3. 停止依赖服务
docker-compose stop deps-service

# 4. 触发音频处理 (应自动降级到本地)
curl -X POST http://localhost:8081/api/v1/meetings/test-meeting/start-recording

# 5. 观察降级日志
docker-compose logs -f aidg-main | grep "fallback"
# 预期输出: "远程执行失败,降级到本地"

# 6. 验证功能可用
curl http://localhost:8081/api/v1/meetings/test-meeting
```

---

## 🔍 故障排查

### 问题 1: 503 依赖不可用

**症状**: 日志显示 `dependency_unavailable` 或 `service not available`

**排查步骤**:

```bash
# 1. 检查依赖服务是否运行
docker ps | grep deps-service

# 2. 验证网络连通性
docker exec aidg-main curl -v http://deps-service:8080/api/v1/health

# 3. 检查共享卷挂载
docker exec aidg-main ls -la /data
docker exec deps-service ls -la /data

# 4. 查看配置
docker exec aidg-main cat /app/config/config.prod.yaml | grep dependency -A 10

# 5. 查看日志
docker-compose logs deps-service | tail -50
```

**常见原因**:
- 依赖服务未启动: `docker-compose up -d deps-service`
- 网络配置错误: 检查 `service_url` 是否正确
- 共享卷路径不一致: 确保两个服务挂载相同卷

### 问题 2: 文件找不到

**症状**: 日志显示 `no such file or directory` 或 `file not found`

**排查步骤**:

```bash
# 1. 验证共享卷路径配置
docker exec aidg-main env | grep DEPENDENCY_SHARED_VOLUME
# 预期: /data

# 2. 检查文件权限
docker exec aidg-main ls -la /data/meetings/

# 3. 确认两个服务挂载相同卷
docker inspect aidg-main | grep -A 5 Mounts
docker inspect deps-service | grep -A 5 Mounts

# 4. 测试文件创建
docker exec aidg-main touch /data/test.txt
docker exec deps-service ls -la /data/test.txt
```

**常见原因**:
- 共享卷路径配置错误: 检查 `shared_volume_path`
- 权限不足: 确保容器用户有读写权限 (UID/GID 一致)
- 卷未正确挂载: 检查 `docker-compose.yml` 卷配置

### 问题 3: 命令执行超时

**症状**: 日志显示 `timeout` 或 `context deadline exceeded`

**排查步骤**:

```bash
# 1. 检查超时配置
docker exec aidg-main env | grep DEPENDENCY_TIMEOUT

# 2. 观察资源使用
docker stats aidg-main deps-service

# 3. 查看 Prometheus 指标
curl http://localhost:9090/metrics | grep dependency_command_duration
```

**解决方法**:
- 增加超时配置: `timeout: 900s` (15 分钟)
- 提高资源配额: 增加 CPU/内存限制
- 优化音频处理: 减小 chunk_size_seconds

### 问题 4: 降级不生效

**症状**: 远程服务故障后系统仍报错,未降级到本地

**排查步骤**:

```bash
# 1. 确认模式为 fallback
docker exec aidg-main env | grep DEPENDENCY_MODE
# 预期: fallback

# 2. 验证本地工具可用
docker exec aidg-main ffmpeg -version
docker exec aidg-main python3 --version

# 3. 检查降级逻辑日志
docker-compose logs aidg-main | grep -E "fallback|降级"

# 4. 测试本地执行
docker exec aidg-main /usr/local/bin/ffmpeg -version
```

**常见原因**:
- 模式未设置为 `fallback`: 修改配置为 `mode: fallback`
- 本地工具不可用: 使用完整镜像或安装工具
- 降级逻辑未触发: 检查错误是否为网络错误

---

## 🔒 安全最佳实践

### 命令白名单

**推荐配置**:

```yaml
dependency:
  allowed_commands:
    - ffmpeg
    - pyannote
  # 禁止: rm, curl, wget, bash, sh 等危险命令
```

### 敏感配置管理

**不推荐** (硬编码):

```yaml
diarization:
  huggingface_token: "hf_xxxxxxxxxxxxxxxxxxxxx"  # ❌ 不要硬编码
```

**推荐** (环境变量):

```yaml
diarization:
  huggingface_token: "${HUGGINGFACE_TOKEN}"  # ✅ 从环境变量读取
```

**最佳实践** (密钥管理服务):

- AWS Secrets Manager
- HashiCorp Vault
- Kubernetes Secrets

### 容器安全

```yaml
services:
  aidg-main:
    # 使用非 root 用户
    user: "1000:1000"
    
    # 限制容器能力
    cap_drop:
      - ALL
    cap_add:
      - CHOWN
      - SETUID
      - SETGID
    
    # 只读根文件系统
    read_only: true
    
    # 临时文件系统
    tmpfs:
      - /tmp
```

---

## 📊 监控与告警

### Prometheus 指标

**关键指标**:

```promql
# 命令执行成功率
rate(dependency_command_executions_total{status="success"}[5m])
/ rate(dependency_command_executions_total[5m])

# P95 执行延迟
histogram_quantile(0.95, 
  rate(dependency_command_duration_seconds_bucket[5m]))

# 降级事件频率
rate(dependency_degradation_events_total[5m])
```

### Grafana 仪表板

**推荐面板**:

1. **命令执行成功率** (按命令、模式分类)
2. **执行延迟分布** (P50/P95/P99)
3. **降级事件时间线**
4. **依赖服务健康状态**

### 告警规则

```yaml
groups:
  - name: dependency_alerts
    rules:
      # 依赖服务不可用
      - alert: DependencyServiceDown
        expr: up{job="deps-service"} == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "依赖服务不可用"
      
      # 命令执行成功率低
      - alert: HighCommandFailureRate
        expr: |
          rate(dependency_command_executions_total{status="failed"}[5m])
          / rate(dependency_command_executions_total[5m]) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "命令执行失败率 > 10%"
      
      # 频繁降级
      - alert: FrequentDegradation
        expr: rate(dependency_degradation_events_total[5m]) > 0.05
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "降级事件频繁,检查依赖服务稳定性"
```

---

## 📚 参考文档

- [设计文档: 通用命令行工具远程调用服务](../docs/DEPENDENCY_EXECUTION_DESIGN.md)
- [部署指南](../docs/deployment.md)
- [故障排查手册](../docs/troubleshooting.md)
- [Docker Compose 指南](../DOCKER_COMPOSE_GUIDE.md)

---

## 📝 更新日志

| 版本 | 日期 | 变更说明 |
|------|------|----------|
| v0.1 | 2025-10-12 | 初始版本,支持 local/remote/fallback 三种模式 |

---

**维护者**: AIDG 开发团队  
**最后更新**: 2025-10-12
