# AIDG 精简与产品化重构规划（修订版）

# AIDG 精简与产品化重构规划（修订版）

> 分析范围：
>
> - AIDG：`/Users/tshinjeii/Documents/code/python/AIDG`
> - 参考实现：`/Users/tshinjeii/Documents/code/openclaw/mote`
>
> 本次仅进行了代码分析，**未修改、创建或删除任何文件**。

---

## 一、目标产品定义

AIDG 改造后的目标是一个**单代码实现、单镜像、兼容旧数据目录的 Lite 产品**：

- 保留项目、任务、文档、执行计划等基础 Lite 功能。
- 保留会议录制，但完全重写现有会议处理链路。
- 会议音频由浏览器采集，AIDG 负责可靠保存和任务调度。
- ASR 通过外部服务完成：
  - Ollama/Whisper-compatible ASR
  - oMLX OpenAI-compatible ASR
- 不再进行说话人分离。
- 保留：
  - 原始转写
  - 文本润色
  - 会议纪要生成和编辑
  - 会议纪要历史
  - 外部提交音频、文字或纪要
- 删除：
  - `aidg-nlp`
  - `aidg-deps`
  - 内置/伴生 `go-whisper`
  - PyAnnote
  - 旧会议 orchestrator
  - speaker 映射链路
  - `merge-segments`
  - 独立 `aidg-lite/`
- 最终只发布：

```text
ghcr.io/houzhh15/aidg:<version>
```

建议版本定位为：

```text
AIDG v2.0
```

内部属于破坏性重构，但对目录、卷、端口、主要 API 和存量数据提供兼容层。

---

# 二、从 mote Listen 借鉴什么

## 2.1 已确认的 Listen 实现

mote Listen 的核心代码集中在：

```text
internal/listener/
internal/dedicated/
api/v1/listener.go
shared/ui/src/pages/ListenPage.tsx
```

其实际架构是：

1. 浏览器使用 `MediaRecorder` 采集音频。
2. 约每 3 秒生成一个独立 WebM 块。
3. 将音频块上传到服务端。
4. 服务端通过 OpenAI-compatible 接口调用 ASR。
5. 通过 SSE 推送识别和分析结果。
6. 音频与历史记录落盘保存。
7. 停止会话时执行最终分析和汇总。

相关代码：

- Listen UI 和 ASR 配置：  
  `mote/shared/ui/src/pages/ListenPage.tsx:583-730`
- 会话状态与生命周期：  
  `mote/internal/listener/session.go:75-220`
- 音频块持久化和淘汰：  
  `mote/internal/listener/audio_buffer.go:45-119`
- 音频上传与同步 ASR：  
  `mote/api/v1/listener.go:115-182`
- SSE：  
  `mote/api/v1/listener.go:223-280`

## 2.2 Ollama 与 oMLX 的真实接口边界

mote 所谓 Ollama Whisper 实际调用：

```http
POST {endpoint}/v1/audio/transcriptions
Content-Type: multipart/form-data
```

表单字段：

```text
file
model
language
response_format=json
```

代码位置：

- `mote/internal/dedicated/ollama_asr.go:44-125`

oMLX 同样使用 OpenAI-compatible API：

```http
GET  /v1/models
POST /v1/audio/transcriptions
Authorization: Bearer <api-key>
```

代码位置：

- `mote/internal/dedicated/omlx_backend.go:33-127`

因此，新 AIDG 不需要实现两套完全不同的 HTTP 客户端。建议提供一个通用实现：

```text
OpenAICompatibleASRProvider
```

再通过 provider profile 区分 Ollama 和 oMLX 的：

- 默认地址
- 鉴权方式
- 模型发现
- 健康检查
- UI 展示名称

需要特别说明：

> 不是所有原生 Ollama 部署都自动提供 `/v1/audio/transcriptions`。产品文档应写为“Ollama/Whisper-compatible ASR endpoint”，并列出经过验证的服务版本。

## 2.3 建议借鉴的设计

可以借鉴：

- 浏览器短块录音
- ASR Provider/Transcriber 抽象
- OpenAI-compatible ASR 协议
- 会话状态模型
- transcript segment 模型
- SSE 实时推送
- 音频保留时间和容量上限
- 停止时 final flush
- 历史记录与重新生成机制

## 2.4 不建议照搬的部分

mote Listen 更适合轻量 Listen 场景，不能原样作为长时间会议处理系统：

- 活动会话主要保存在内存中。
- 上传音频后同步执行 ASR。
- ASR 失败没有持久化重试队列。
- 重启后不能完整恢复活动会话。
- 未配置 provider 时可能静默使用 Noop。
- 音频淘汰列表没有完整的重启重建机制。
- 单一事件 channel 可能不适合多个 SSE 消费者。
- ASR endpoint 需要更严格的 SSRF 防护。

因此建议：

> 复用 Listen 的协议、状态和 UI 思路，重新实现 AIDG 的持久化会话和可靠任务队列，不直接整体复制。

两个仓库均采用 Apache License 2.0：

- `AIDG/LICENSE:1-4`
- `mote/LICENSE:1-4`

如直接复制代码，需要保留适用的版权、许可证和 NOTICE 信息。

---

# 三、保留、重写、删除矩阵

| 领域 | 决策 | 说明 |
|---|---|---|
| 项目、任务、文档 | 保留 | Lite 基础能力 |
| 执行计划、标签、文档版本 | 保留 | 不依赖 NLP |
| 浏览器麦克风录制 | 重写 | 可复用设备枚举和权限处理 |
| 会议会话管理 | 完全重写 | 改为可恢复状态机 |
| 外部音频上传 | 保留并统一 | 与浏览器录音进入同一 ingest 层 |
| ASR | 完全重写 | 只调用外部 Ollama-compatible/oMLX |
| 实时文字记录 | 新实现 | 短块转写、SSE 推送 |
| 原始 transcript | 保留 | 作为不可覆盖的源数据 |
| 文本润色 | 保留并适配 | 从新 transcript 生成 |
| 会议纪要 | 保留并增强 | 生成、编辑、历史、外部提交 |
| 说话人分离 | 删除 | 不保留 PyAnnote 或 speaker pipeline |
| speaker 映射 | 删除 | 不再生成 speakers/mapped/global |
| `merge-segments` | 删除 | 新 transcript 不依赖 speaker merge |
| 旧会议 orchestrator | 删除 | 使用持久化 ASR job worker 替代 |
| `aidg-deps` | 删除 | 删除服务、客户端和镜像 |
| `aidg-nlp` | 删除 | 删除服务、客户端和镜像 |
| 内置 go-whisper | 删除 | ASR 改为外部 endpoint |
| `nlp_service/` | 删除 | 分支上可完全删除 |
| `aidg-lite/` | 删除 | 只保留根模块一份实现 |
| `LITE_MODE` 双入口 | 删除 | 产品不再有两套服务实现 |
| MCP | 暂保留 | 为兼容老部署的 `8001` |
| OCR/file_converter | 待决策 | 与本次 ASR/NLP 裁剪相对独立 |
| IdP/LDAP | 暂保留 | 作为可选用户认证能力 |

---

# 四、目标架构

```text
Browser / External Client
          │
          │ start / chunk / stop / import
          ▼
┌──────────────────────────────────────────────┐
│ ghcr.io/houzhh15/aidg                       │
│                                              │
│ HTTP API + Web UI :8000                      │
│ MCP Compatibility :8001                     │
│                                              │
│ ┌──────────────────────────────────────────┐ │
│ │ Project / Task / Document Lite Core      │ │
│ │ User / Auth / RBAC                       │ │
│ │ Meeting Session Service                  │ │
│ │ Chunk Ingest Service                     │ │
│ │ Persistent ASR Job Store                 │ │
│ │ ASR Worker                               │ │
│ │ Transcript Store                         │ │
│ │ Polish / Meeting Summary Service         │ │
│ │ SSE Event Hub                            │ │
│ └──────────────────────────────────────────┘ │
└───────────────────┬──────────────────────────┘
                    │ outbound HTTP
            ┌───────┴────────┐
            │ External ASR   │
            │ Ollama / oMLX  │
            └────────────────┘
```

持久化目录继续使用：

```text
/app/data/projects
/app/data/users
/app/data/meetings
/app/data/audit_logs
```

外部 ASR 不打包进 AIDG 镜像，也不作为新版 AIDG Compose 的强制子服务。

---

# 五、新会议录制系统设计

## 5.1 会话状态机

建议状态：

```text
created
  └─> recording
        └─> stopping
              └─> processing
                    ├─> completed
                    └─> completed_with_errors

recording / processing
  ├─> degraded
  ├─> failed
  └─> cancelled
```

状态含义：

- `recording`：正在接收音频。
- `stopping`：客户端已停止，但最后音频块可能仍在上传。
- `processing`：等待剩余 ASR 任务。
- `degraded`：ASR 不可用，但音频已安全保存。
- `completed_with_errors`：部分块识别失败，可人工重试。
- `completed`：所有预期块已处理完成。

停止录音不能直接把会议标记为完成，应执行：

1. 停止 MediaRecorder。
2. 上传最后一个块。
3. 服务端确认最终块。
4. 会话进入 `stopping`。
5. 等待所有 ASR job 到达终态。
6. 生成 transcript。
7. 可选执行润色和纪要。
8. 最终进入 completed 状态。

---

## 5.2 音频分块

AIDG 当前录音块约为 5 分钟：

- `frontend/src/components/AudioRecorder.tsx:60-65`

这不适合实时转写。

建议默认：

```yaml
meeting:
  chunk_duration: 5s
```

允许范围：

```text
3～30 秒
```

每块携带：

```json
{
  "session_id": "meeting-id",
  "index": 42,
  "client_chunk_id": "uuid",
  "started_at": "2026-08-07T09:30:00Z",
  "duration_ms": 5000,
  "format": "webm",
  "sha256": "...",
  "final": false
}
```

幂等键：

```text
(session_id, index)
```

同一索引重传时：

- SHA-256 相同：返回原 ACK。
- SHA-256 不同：返回 `409 chunk_conflict`。
- 不重复保存、不重复 ASR、不重复生成文字。

---

## 5.3 浏览器离线可靠性

现有前端有上传重试和 chunk index 延续：

- `frontend/src/hooks/useAudioUpload.ts:139-165`
- `frontend/src/hooks/useMediaRecorder.ts:70-155`

建议保留这些思路，但增加 IndexedDB outbox：

```text
captured
  -> pending
  -> uploading
  -> accepted
  -> transcribed
```

网络中断或页面刷新后：

1. 前端查询服务端已收到的 chunk。
2. 服务端返回最大连续索引和缺口列表。
3. 前端从 IndexedDB 重传缺失块。
4. 服务端返回幂等 ACK。
5. ACK 后才能删除本地音频块。

这对长会议和弱网络是必要能力。

---

## 5.4 后端数据布局

必须保持旧顶层路径：

```text
/app/data/meetings/<meeting-id>/
```

建议在每个旧会议目录内增加新结构：

```text
manifest.json
session.json

chunks/
  000000.webm
  000000.json
  000001.webm
  000001.json

jobs/
  000000.json
  000001.json

transcripts/
  segments.jsonl
  transcript.txt

documents/
  polished.md
  meeting-summary.md
  history/

events/
  events.jsonl
```

同时继续生成旧文件名兼容投影：

```text
merged_all.txt
polish_all.md
meeting-summary.md
```

旧文件只读保留，不应删除：

```text
chunk_*_segments.json
chunk_*_speakers.json
chunk_*_speakers_mapped.json
chunk_*_speakers_mapped_global.json
chunk_*_merged.txt
```

新系统不再生成 speaker 文件，但不能在升级时清理用户已有数据。

---

## 5.5 持久化 ASR 队列

不能采用上传请求内同步完成 ASR 的方案。

建议每个音频块创建一个持久化 job：

```json
{
  "id": "meeting-id:42",
  "chunk_index": 42,
  "state": "pending",
  "attempts": 0,
  "next_attempt_at": null,
  "provider": "ollama",
  "model": "whisper-large-v3",
  "last_error": ""
}
```

状态：

```text
pending
  -> processing
       ├─> completed
       ├─> retry_wait
       └─> dead
```

错误分类：

| 错误 | 行为 |
|---|---|
| 网络错误、超时 | 指数退避重试 |
| HTTP 408、429 | 重试 |
| HTTP 5xx | 重试 |
| HTTP 400 | 标记请求或格式错误 |
| HTTP 401、403 | 标记配置错误，不无限重试 |
| HTTP 404 | 检查 endpoint/model，不无限重试 |
| 非法 JSON | 标记外部响应错误 |
| 进程中断 | 启动后将遗留 processing 重置为 pending |

worker 必须支持：

- 有限并发，默认 1～2。
- context 取消。
- 优雅退出。
- 停止领取新任务。
- 等待当前请求结束或超时。
- 正确关闭 HTTP response body。
- 不产生 goroutine 泄漏。

---

## 5.6 ASR Provider

建议接口：

```go
type Provider interface {
    Transcribe(
        ctx context.Context,
        request TranscribeRequest,
    ) (TranscribeResult, error)

    Check(ctx context.Context) error
    ListModels(ctx context.Context) ([]Model, error)
}
```

底层通用实现：

```text
OpenAICompatibleASRProvider
```

provider profile：

```text
ollama
omlx
```

Ollama 配置示例：

```yaml
asr:
  enabled: true
  provider: ollama
  endpoint: http://host.docker.internal:11434
  model: whisper
  language: auto
  request_timeout: 120s
  concurrency: 1
```

oMLX 配置示例：

```yaml
asr:
  enabled: true
  provider: omlx
  endpoint: http://host.docker.internal:18000
  model: whisper-large-v3
  api_key_file: /run/secrets/omlx_api_key
  request_timeout: 120s
  concurrency: 1
```

注意：参考代码中 oMLX 默认使用 `localhost:8000`，而 AIDG 自身也使用 `8000`。同宿主部署时必须修改 oMLX 端口或使用独立 Docker 网络地址。

---

## 5.7 ASR 安全边界

ASR endpoint 属于外部网络边界，需防止 SSRF：

- 只允许 `http`、`https`。
- 禁止 URL 中携带用户名密码。
- endpoint 不接受任意 query。
- 默认禁止 HTTP redirect。
- 对私网访问采用明确配置，而不是默认任意访问。
- 限制 DNS 解析和目标 IP。
- 限制请求超时。
- 限制响应体大小。
- 验证响应 `Content-Type`。
- 验证 JSON 字段类型和文字长度。
- 不记录 API key。
- 不在错误日志输出完整外部响应。
- 正确关闭 response body。

由于容器通常需要访问宿主机或同一 Docker network，建议：

```yaml
asr:
  allow_private_networks: true
  allowed_hosts:
    - host.docker.internal
    - omlx
```

比允许管理员配置任意 URL 更安全。

---

## 5.8 Transcript 结构

由于删除说话人分离，不应继续输出虚假的 speaker 信息：

```json
{
  "id": "segment-id",
  "chunk_index": 42,
  "text": "会议识别文本",
  "language": "zh",
  "audio_start_ms": 210000,
  "audio_end_ms": 215000,
  "recognized_at": "...",
  "provider": "ollama",
  "model": "whisper",
  "revision": 1
}
```

前端展示：

```text
[00:03:30] 会议识别文本
```

并明确显示：

> 当前未启用说话人识别，文字记录不区分发言人。

后续可以支持人工标注发言人，但不应在本阶段构造不可靠的自动 speaker 信息。

---

## 5.9 原始文本、润色与纪要分层

必须保留三层数据：

```text
raw_transcript
      ↓
polished_transcript
      ↓
meeting_summary
```

原则：

- 原始 ASR 不被润色结果覆盖。
- 润色结果可以重新生成。
- 纪要可以从 raw 或 polished 生成。
- 用户手工编辑的版本不能被后台自动覆盖。
- 每次自动生成记录：
  - provider
  - model
  - prompt version
  - generated_at
  - source revision
- 外部系统可直接提交纪要，不强制要求存在录音或 ASR。

兼容输出：

```text
raw transcript       -> merged_all.txt
polished transcript  -> polish_all.md
meeting summary      -> meeting-summary.md
```

---

# 六、单根模块收敛

最终保留：

```text
/go.mod
/cmd/server
/cmd/mcp-server
/frontend
/pkg
```

删除整个：

```text
/aidg-lite/
```

包括：

```text
aidg-lite/go.mod
aidg-lite/main.go
aidg-lite/internal/
aidg-lite/pkg/
aidg-lite/frontend/
```

同时删除根服务中的双实现：

```text
LITE_MODE
runLiteMode
LiteMode 条件路由
Lite 固定 JWT secret
Lite 固定管理员密码
匿名固定用户和全 scopes
```

相关入口：

- `cmd/server/main.go:165-175`
- `aidg-lite/pkg/server/server.go:50-58`
- `aidg-lite/pkg/server/server.go:99-114`

最终不再区分 Full 与 Lite 后端，只保留一个正式产品实现。

ASR、MCP、IdP 等使用 capability 配置控制，但关闭某个 capability 不能改变认证要求。

---

# 七、老部署兼容方案

## 7.1 兼容原则

```text
镜像名不变
主端口不变
目录不变
卷挂载不变
旧 API 暂时保留
旧文件只读兼容
旧环境变量可读取并告警
新数据不覆盖旧 artifacts
```

## 7.2 兼容矩阵

| 项目 | 兼容级别 | 方案 |
|---|---|---|
| `ghcr.io/houzhh15/aidg` | 完全兼容 | 镜像名不变 |
| HTTP 端口 `8000` | 完全兼容 | 保持默认监听 |
| MCP `8001` | 完全兼容目标 | 保持旧 Compose 映射 |
| `/health` | 完全兼容 | 只检查核心服务 |
| `/api/v1/health` | 完全兼容 | 保留 |
| `/app/data/projects` | 完全兼容 | 不改路径和现有格式 |
| `/app/data/users` | 迁移兼容 | 旧 JSON 可读，渐进升级 |
| `/app/data/meetings` | 适配兼容 | 顶层路径不变，新结构写入子目录 |
| `/app/data/audit_logs` | 完全兼容目标 | 修正代码实际使用位置 |
| `/app/config` | 完全兼容 | 支持只读挂载和新 YAML |
| `/models` | 无害兼容 | 旧挂载存在也不影响启动 |
| `/data` | Legacy 兼容 | 暂时保留 alias |
| `server_tasks.json` | 适配兼容 | 读取旧 schema |
| `merged_all.txt` | 适配兼容 | 新 transcript 继续生成 |
| `polish_all.md` | 适配兼容 | 新润色继续生成 |
| speaker 文件 | 只读兼容 | 忽略但不删除 |
| 旧会议 API | 兼容一个大版本 | 通过 adapter 转发 |
| `WHISPER_API_URL` | 映射兼容 | 映射到新 ASR endpoint |
| `ENABLE_SPEAKER_DIARIZATION` | 废弃兼容 | 接受、忽略、启动告警 |
| `DEPS_SERVICE_URL` | 废弃兼容 | 接受、忽略、启动告警 |
| `DEPENDENCY_MODE` | 废弃兼容 | 接受、忽略、启动告警 |
| 旧 JWT | 条件兼容 | Secret 不变时短期可解析 |
| 旧 Full Compose | 部分兼容 | aidg 可运行，但附属容器仍会被启动 |
| 旧 Lite Compose | 基本兼容 | 单容器可直接替换 |

## 7.3 端口问题

当前存在 MCP 端口不一致：

- production Compose 使用 `8001`：  
  `deployments/production/docker-compose.standard.yml:64-75`
- Dockerfile 和 healthcheck 部分位置使用 `8081`：  
  `build/dockerfiles/Dockerfile:130-144`  
  `deployments/docker/healthcheck.sh:6-21`

建议：

- 主 HTTP 固定保持 `8000`。
- MCP 新默认统一为 `8001`。
- 继续兼容 `MCP_HTTP_PORT`。
- 老环境显式配置 `8081` 时继续监听 `8081`。
- 不能只依赖 Dockerfile 的 `EXPOSE` 判断实际端口。

## 7.4 旧 Compose 的兼容边界

新 aidg 镜像可以在旧 Compose 下启动，前提是：

- 不再等待 whisper/deps 健康。
- 忽略废弃环境变量。
- 旧模型目录挂载不会导致错误。
- 原端口与数据卷保持不变。

但只替换 aidg 镜像并不能自动删除旧附属容器。旧 Compose 仍会启动：

- go-whisper
- aidg-deps
- aidg-nlp

真正切换到单镜像部署必须使用新版 Compose，并执行：

```text
docker compose up -d --remove-orphans
```

镜像自身不能删除其他容器。

---

# 八、数据迁移与回滚

## 8.1 会议数据迁移

建议启动过程：

```text
detect
  -> validate
  -> write migration marker
  -> lazy adapt
  -> start
```

策略：

1. 识别旧 `server_tasks.json`。
2. 不批量覆盖旧任务文件。
3. 增加版本化 `migration-state.json`。
4. 首次访问旧会议时生成新 manifest。
5. 有 `merged_all.txt` 时导入为：
   ```json
   {"source":"legacy_merged_text"}
   ```
6. 没有 merged 文件但有 segments 时，可生成 transcript。
7. speaker 文件不参与新文字生成。
8. 原文件不删除、不重命名。
9. 新文件采用临时文件、`fsync`、rename 原子写入。
10. 新结构写入旧会议目录中的独立子目录。

## 8.2 回滚能力

可以保证：

- 原会议文件仍存在。
- 原 speaker 和 segment 文件不被删除。
- 原 `server_tasks.json` 可保留。
- 切换回旧镜像时，旧会议数据仍在。

不能完全保证：

- 旧镜像能够理解新会议 session/job。
- 新创建的会议能被旧 UI 完整显示。
- Argon2id 用户密码能被旧镜像验证。
- 新 Token 模型被旧镜像识别。

因此升级前必须备份：

```text
/app/data/projects
/app/data/users
/app/data/meetings
/app/data/meetings/server_tasks.json
/app/data/audit_logs
```

用户安全升级后如需回滚，不能只替换镜像，还要恢复用户数据快照。

---

# 九、配置与 Secret 管理

## 9.1 推荐优先级

```text
程序安全默认值
  < /app/config/aidg.yaml
  < CONFIG_FILE 指定文件
  < 旧环境变量兼容值
  < AIDG_* 新环境变量
  < *_FILE / Secret
```

## 9.2 配置示例

```yaml
version: 2

server:
  port: 8000
  mcp_port: 8001

data:
  projects_dir: /app/data/projects
  users_dir: /app/data/users
  meetings_dir: /app/data/meetings
  audit_logs_dir: /app/data/audit_logs

asr:
  enabled: true
  provider: ollama
  endpoint: http://host.docker.internal:11434
  model: whisper
  language: auto
  timeout: 120s
  concurrency: 1
  allow_private_networks: true
  allowed_hosts:
    - host.docker.internal

meeting:
  chunk_duration: 5s
  audio_retention: 168h
  max_audio_size_mb: 2048

security:
  jwt_secret_file: /run/secrets/aidg_jwt_secret
  cors_allowed_origins:
    - https://aidg.example.com
```

## 9.3 旧环境变量映射

| 旧变量 | 新配置 | 策略 |
|---|---|---|
| `PORT` | `server.port` | 保留 |
| `MCP_HTTP_PORT` | `server.mcp_port` | 保留 |
| `PROJECTS_DIR` | `data.projects_dir` | 保留 |
| `USERS_DIR` | `data.users_dir` | 保留 |
| `MEETINGS_DIR` | `data.meetings_dir` | 保留 |
| `AUDIT_LOGS_DIR` | `data.audit_logs_dir` | 保留 |
| `USER_JWT_SECRET` | `security.jwt_secret` | 保留 |
| `JWT_SECRET` | legacy alias | 明确用途后逐步废弃 |
| `WHISPER_API_URL` | `asr.endpoint` | 自动映射 |
| `WHISPER_MODE` | `asr.provider/enabled` | 映射并告警 |
| `ENABLE_SPEAKER_DIARIZATION` | 无 | 忽略并告警 |
| `DEPS_SERVICE_URL` | 无 | 忽略并告警 |
| `DEPENDENCY_MODE` | 无 | 忽略并告警 |
| `HUGGINGFACE_TOKEN` | 无 | 删除 |
| `HF_HUB_CACHE` | 无 | 删除 |

不建议立即删除全部旧环境变量。建议兼容一个 major release，并在启动日志输出一次废弃警告。

---

# 十、用户管理产品化

用户管理改造沿用上一版结论，但为保证旧部署回滚，建议分两步实施。

## 10.1 第一阶段：兼容性安全加固

先不立即切 SQLite，优先完成：

- JSON 原子写入。
- 权限改为 `0600`。
- 临时文件、`fsync`、rename。
- 损坏文件拒绝启动或进入恢复模式。
- 增加 `.bak`。
- 删除默认密码。
- JWT 增加过期。
- 严格限制 JWT 签名算法。
- 权限变化后 Token 失效。
- 用户管理审计。
- 登录限速。

当前问题证据：

- 无盐 SHA-256：  
  `cmd/server/internal/users/manager.go:74-79`  
  `cmd/server/internal/users/manager.go:121-125`
- 默认密码：  
  `cmd/server/internal/users/manager.go:168-182`
- JWT 无过期：  
  `cmd/server/internal/users/manager.go:92-97`  
  `cmd/server/internal/users/manager.go:279-297`
- 用户 JSON 直接 `0644` 覆盖：  
  `cmd/server/internal/users/manager.go:143-154`

## 10.2 第二阶段：密码和存储迁移

密码：

- 使用 Argon2id，或至少 bcrypt。
- 识别旧 64 位 SHA-256。
- 用户首次成功登录后升级哈希。
- 新密码使用 PHC 格式。
- 改密、禁用、删除、权限变更时增加 `token_version`。

Token：

- access token：15～60 分钟。
- refresh token：7～30 天。
- refresh token 只保存哈希。
- 加入：
  ```text
  exp
  nbf
  iss
  aud
  jti
  token_version
  ```

用户存储：

- 短期继续兼容 `users.json`。
- 中期迁移到：
  ```text
  /app/data/users/aidg.db
  ```
- 使用 SQLite、WAL 和 schema migration。
- 迁移必须显式、幂等、可生成报告。
- SQLite 稳定前保留旧 JSON 只读备份。

角色、用户角色、IdP、audit 当前部分混入项目目录：

- `cmd/server/main.go:275-320`

建议逐步收敛到：

```text
/app/data/users/
  users.json
  aidg.db
  roles/
  user_roles/
  identity_providers/

/app/data/audit_logs/
```

新位置不存在时继续读取旧位置，但不能静默删除旧文件。

---

# 十一、分阶段实施计划

## 阶段 0：建立兼容基线

分支：

```text
test/v2-compatibility-baseline
```

工作：

- 固化旧 API contract。
- 收集真实会议目录样本。
- 收集不同版本 `server_tasks.json`。
- 记录旧 Compose 的端口、卷、环境变量。
- 建立旧镜像 smoke test。
- 建立升级和回滚测试数据。
- 明确 MCP、OCR/file_converter、IdP 的最终范围。

验收：

- 至少三个真实老部署样本可完成启动测试。
- 有数据备份和恢复演练。
- 所有兼容项形成自动化检查。

---

## 阶段 1：单根模块与配置骨架

分支：

```text
refactor/v2-single-root-module
feat/v2-config-and-secrets
```

工作：

- 根模块成为唯一实现。
- 移除 `LITE_MODE` 双入口。
- 建立 YAML、环境变量和 Secret 加载。
- 保留旧环境变量映射。
- 统一端口和目录。
- 暂不删除旧会议代码。

验收：

- 旧 Lite Compose 可运行。
- 旧 Standard Compose 中 aidg 主容器可运行。
- 旧数据目录可读。
- Secret 不出现在日志。
- 新旧配置优先级测试通过。

---

## 阶段 2：新会议会话与持久化

分支：

```text
feat/v2-meeting-session-core
```

工作：

- 新会话状态机。
- manifest、chunk、job、transcript store。
- 幂等音频 ingest。
- 兼容读取 `server_tasks.json`。
- 旧 meeting-summary 接入新存储。
- SSE event hub。
- worker 启动恢复。
- 外部完整音频上传适配。

验收：

- 服务重启后会议和 pending job 可恢复。
- 重传 chunk 不重复处理。
- 旧会议纪要正常读取和编辑。
- 文件损坏不被静默覆盖。
- 无 goroutine、文件句柄和 HTTP body 泄漏。

回滚：

- 新数据只写旧会议目录的新子目录。
- 不覆盖旧 artifacts。

---

## 阶段 3：外部 ASR Provider

分支：

```text
feat/v2-asr-provider
```

工作：

- OpenAI-compatible ASR client。
- Ollama provider profile。
- oMLX provider profile。
- 模型列表、健康检查。
- 超时、重试和并发控制。
- SSRF 防护。
- Secret API key。
- degraded 模式。
- ASR 结果持久化。

验收：

- Ollama-compatible fixture 通过。
- oMLX fixture 通过。
- 429、5xx、超时可恢复。
- 400、401 不无限重试。
- ASR 停机不丢音频。
- AIDG 重启后继续处理。

回滚：

```yaml
asr:
  enabled: false
```

仍可录音、保存、上传文字和编辑纪要。

---

## 阶段 4：前端录音重写

分支：

```text
feat/v2-recording-ui
```

工作：

- 5 分钟块改为 3～10 秒。
- 每块独立可解码 WebM。
- IndexedDB outbox。
- 幂等 ACK。
- 缺块恢复。
- SSE 实时 transcript。
- ASR 状态与重试 UI。
- 增加 Ollama、oMLX 配置。
- 删除 speaker UI。

验收：

- 连续录音至少 2 小时。
- 页面刷新后可恢复上传。
- 断网 10 分钟后可继续。
- 最终块不丢失。
- 重传不生成重复文字。
- Chrome、Edge、Safari 兼容测试通过。

---

## 阶段 5：润色、纪要与旧 API 投影

分支：

```text
feat/v2-meeting-documents
compat/v2-legacy-meeting-api
```

工作：

- 保存 raw transcript。
- 生成 polished transcript。
- 生成 meeting summary。
- 继续生成 `merged_all.txt`、`polish_all.md`。
- 保留旧 meeting-summary API。
- 保存生成元数据。
- 用户编辑版本不被自动覆盖。
- 支持外部直接提交纪要。

验收：

- 旧 API 可读取新内容。
- 新旧 summary API 一致。
- 没有 ASR 时仍可手工或外部提交纪要。
- 重新生成不会覆盖用户版本。

---

## 阶段 6：用户安全和管理产品化

分支：

```text
feat/v2-user-security
feat/v2-user-admin-productization
```

工作：

- 原子 JSON 存储。
- Argon2id/bcrypt。
- 旧 SHA-256 登录升级。
- JWT 生命周期。
- refresh token。
- `token_version`。
- 登录限速。
- 管理操作审计。
- 用户列表字段隔离。
- 管理 UI 分页与搜索。
- 禁止删除最后一个管理员。
- 禁止自动合并同名本地与 IdP 用户。

验收：

- 旧用户可正常登录。
- 密码升级成功。
- 改密、禁用、权限回收后旧 Token 失效。
- 无固定默认密码。
- 用户文件权限为 `0600`。
- 普通用户看不到敏感管理字段。

---

## 阶段 7：彻底删除旧代码

分支：

```text
chore/v2-remove-legacy-meeting-pipeline
chore/v2-remove-nlp-deps-whisper
chore/v2-remove-aidg-lite-module
```

删除：

```text
aidg-lite/
nlp_service/
cmd/deps-service/
cmd/merge-segments/
旧会议 orchestrator
Whisper 内置调用
PyAnnote 脚本
speaker merge API
旧模型下载接口
deps/nlp/lite Dockerfile
相关模型和构建逻辑
```

同步清理：

- `go.mod`
- Python requirements
- Makefile
- 构建脚本
- Compose
- CI
- 前端
- 测试
- 配置
- 文档

验收：

- 仓库中无 aidg-nlp、aidg-deps、go-whisper、PyAnnote 引用。
- 不再生成 speaker 文件。
- 不再编译 `merge-segments`。
- 只有一个 Go 主模块和一个前端。
- 全量构建和测试通过。

---

## 阶段 8：单镜像部署与发布

分支：

```text
feat/v2-single-image-deployment
docs/v2-upgrade-and-rollback
```

工作：

- 只构建 `ghcr.io/houzhh15/aidg`。
- 新 Compose 只保留 aidg。
- 保持原端口和 volume。
- 外部 ASR 通过 endpoint 配置。
- Secret 使用文件挂载。
- 发布升级、备份和回滚说明。
- 使用 `--remove-orphans` 清理旧附属容器。

验收：

- AIDG 本身只需要一个容器。
- 镜像中没有 Whisper/PyAnnote/HuggingFace 模型。
- 旧数据卷可直接挂载。
- 非 root 运行。
- 旧 Compose 的 aidg 主容器替换测试通过。
- 新 Compose 单服务部署通过。

---

# 十二、Git 分支与合并顺序

建议集成分支：

```text
release/v2-lite-productization
```

推荐顺序：

```text
main
 └─ release/v2-lite-productization
     ├─ test/v2-compatibility-baseline
     ├─ refactor/v2-single-root-module
     ├─ feat/v2-config-and-secrets
     ├─ feat/v2-meeting-session-core
     ├─ feat/v2-asr-provider
     ├─ feat/v2-recording-ui
     ├─ feat/v2-meeting-documents
     ├─ compat/v2-legacy-meeting-api
     ├─ feat/v2-user-security
     ├─ feat/v2-user-admin-productization
     ├─ chore/v2-remove-legacy-meeting-pipeline
     ├─ chore/v2-remove-nlp-deps-whisper
     ├─ chore/v2-remove-aidg-lite-module
     ├─ feat/v2-single-image-deployment
     └─ docs/v2-upgrade-and-rollback
```

发布标签：

```text
v1-legacy-final
v2.0.0-rc.1
v2.0.0-rc.2
v2.0.0
```

不建议一开始就删除旧代码。正确顺序是：

```text
兼容测试
→ 新实现
→ 真实数据升级验证
→ 回滚演练
→ 独立分支彻底删除旧代码
```

---

# 十三、测试重点

## 后端

- 会话状态机
- chunk 幂等和哈希冲突
- worker 重启恢复
- ASR 重试分类
- transcript 排序和去重
- 旧 `server_tasks.json` 读取
- 旧会议目录适配
- 原子文件写入
- 配置优先级
- Secret 文件
- JWT 过期和撤销
- SHA-256 到 Argon2id 迁移

## 前端

- 麦克风权限拒绝
- 设备断开
- 暂停和恢复
- 网络中断
- 页面刷新
- IndexedDB 容量不足
- SSE 重连
- 最后一块上传
- 双标签页冲突
- 两小时连续录音

## ASR

- Ollama-compatible 正常响应
- oMLX 正常响应
- 超时、429、5xx
- 400、401、403
- 非法 JSON
- 超大响应
- 模型不存在
- AIDG 和 ASR 分别重启
- ASR 不可用时音频仍保留

## 安全

- 跨项目访问会议音频
- 路径穿越
- 恶意 meeting ID
- 超大 multipart
- 临时文件清理
- ASR endpoint SSRF
- redirect 到 metadata 服务
- API key 日志泄漏
- JWT algorithm confusion
- 登录暴力尝试
- transcript/summary Markdown XSS

## 部署兼容

至少覆盖原有：

```text
docker-compose.lite.yml
docker-compose.standard.yml
docker-compose.full.yml
docker-compose.yml
```

以及：

- 旧端口 `8000/8001`
- 显式 MCP `8081`
- 旧数据卷
- 只读 `/app/config`
- 旧 `/models` 挂载
- 非 root UID
- ASR 在宿主机
- ASR 在同一 Docker network

---

# 十四、实施前仍需确认的产品决策

1. **官方支持哪些 Ollama/Whisper-compatible 服务版本？**  
   参考 Mote Listen

2. **音频默认保留多久？**  
   需要决定：
   - ASR 成功后删除

3. **MCP 是否属于基础产品？**  
   为兼容旧部署，当前建议保留 `8001`。

4. **OCR/file_converter 是否保留？**  
   当前镜像仍包含 Python、Tesseract、Poppler 和 supervisor。若不保留，可以继续显著缩小镜像。决策：不保留

5. **旧会议 API 保留多久？**  
   旧的MCP和Web API都不保留

6. **用户存储是否本次立即切换 SQLite？**  
   为兼容回滚，建议先完成 JSON 原子化与密码迁移，再独立实施 SQLite。

---

## 最终推荐实施顺序

```text
兼容基线
→ 单根模块和配置体系
→ 新会议持久化
→ 外部 ASR
→ 前端录音重写
→ 润色和纪要
→ 用户安全
→ 删除旧会议/NLP/deps/whisper/aidg-lite
→ 单镜像部署
```

该方案同时满足三个核心目标：

1. **会议录制得以保留并实现可靠重写。**
2. **仓库和镜像真正精简为单一 AIDG 产品。**
3. **老部署可直接替换主镜像，目录、卷、端口和主要数据保持兼容。**
