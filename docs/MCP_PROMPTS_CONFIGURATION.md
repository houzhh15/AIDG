# MCP Server Prompts 配置说明

## 概述

MCP Server 支持提示词模板功能，允许 AI 工具通过 MCP 协议获取预定义的提示词模板。

## Prompts 目录位置

### 在 Docker 容器中

默认位置：`/app/prompts`

容器启动时，prompts 目录会被复制到 `/app/prompts`，包含以下模板文件：

```
/app/prompts/
├── design.prompt.md          # 设计文档提示词
├── executing.prompt.md       # 执行计划提示词
├── planning.prompt.md        # 规划阶段提示词
├── requirements.prompt.md    # 需求分析提示词
└── task_summary.prompt.md    # 任务总结提示词
```

### 自定义 Prompts 目录

可以通过环境变量 `MCP_PROMPTS_DIR` 指定自定义路径：

```yaml
# docker-compose.yml 示例
services:
  aidg:
    environment:
      - MCP_PROMPTS_DIR=/custom/prompts/path
    volumes:
      - ./my-prompts:/custom/prompts/path
```

## 环境变量配置

### MCP_PROMPTS_DIR

指定 prompts 模板目录的路径。

- **类型**: 字符串
- **默认值**: `./prompts`
- **支持**: 相对路径和绝对路径

**示例**:
```bash
# 使用绝对路径
MCP_PROMPTS_DIR=/app/prompts

# 使用相对路径（相对于工作目录）
MCP_PROMPTS_DIR=./custom-prompts

# 挂载外部目录
MCP_PROMPTS_DIR=/mnt/shared-prompts
```

### MCP_PROMPTS_CACHE_TTL

设置 prompts 缓存的过期时间（分钟）。

- **类型**: 整数
- **默认值**: `5` (5分钟)
- **特殊值**: `0` 表示禁用缓存

**示例**:
```bash
# 10分钟缓存
MCP_PROMPTS_CACHE_TTL=10

# 禁用缓存（每次都重新加载）
MCP_PROMPTS_CACHE_TTL=0

# 1小时缓存
MCP_PROMPTS_CACHE_TTL=60
```

## Docker Compose 配置示例

### 使用默认 prompts（推荐）

```yaml
services:
  aidg:
    image: ghcr.io/houzhh15-hub/aidg:latest
    # 无需额外配置，使用内置的 /app/prompts
```

### 挂载自定义 prompts

```yaml
services:
  aidg:
    image: ghcr.io/houzhh15-hub/aidg:latest
    environment:
      - MCP_PROMPTS_DIR=/app/custom-prompts
    volumes:
      - ./my-custom-prompts:/app/custom-prompts:ro
```

### 禁用 prompts 缓存（开发环境）

```yaml
services:
  aidg:
    image: ghcr.io/houzhh15-hub/aidg:latest
    environment:
      - MCP_PROMPTS_CACHE_TTL=0  # 实时加载，方便开发调试
    volumes:
      - ./cmd/mcp-server/prompts:/app/prompts:ro  # 挂载源码目录
```

## Prompts 文件格式

每个 prompt 文件是一个 Markdown 文件，包含以下结构：

```markdown
# 模板标题

## 描述
简要描述这个提示词的用途

## 参数
- `param1`: 参数1的描述 (必填)
- `param2`: 参数2的描述 (可选)

## 模板内容

这里是实际的提示词内容...
可以使用 {{param1}} 和 {{param2}} 占位符
```

## MCP 协议接口

### 列出所有 prompts

**请求**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "prompts/list"
}
```

**响应**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "prompts": [
      {
        "name": "design",
        "description": "设计文档生成提示词",
        "arguments": [...]
      },
      ...
    ]
  }
}
```

### 获取特定 prompt

**请求**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "prompts/get",
  "params": {
    "name": "design",
    "arguments": {
      "project_name": "My Project",
      "requirements": "..."
    }
  }
}
```

**响应**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "description": "设计文档生成提示词",
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "..."
        }
      }
    ]
  }
}
```

## 验证 Prompts 加载

### 查看日志

启动容器后，检查日志中的 prompts 相关信息：

```bash
# 查看 MCP server 日志
docker logs aidg-unified 2>&1 | grep PROMPTS

# 期望看到的日志：
# ✅ [PROMPTS] 模版目录: /app/prompts
# ✅ [PROMPTS] 已加载 5 个模版
```

如果看到以下日志，说明目录不存在：

```
❌ [PROMPTS] 模版目录不存在: /app/prompts
⚠️  [PROMPTS] 模版目录不可用，将返回空模版列表
```

### 测试 MCP 接口

```bash
# 测试 prompts/list 接口
curl -X POST http://localhost:8081/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "prompts/list"
  }'
```

## 故障排查

### 问题：日志显示 "模版目录不存在"

**可能原因**:
1. Dockerfile 未复制 prompts 目录
2. `MCP_PROMPTS_DIR` 环境变量指向错误路径
3. Volume 挂载路径不正确

**解决方案**:
```bash
# 1. 检查容器内部路径
docker exec -it aidg-unified ls -la /app/prompts

# 2. 验证环境变量
docker exec -it aidg-unified env | grep MCP_PROMPTS_DIR

# 3. 重新构建镜像（如果使用本地构建）
docker-compose build --no-cache
```

### 问题：Prompts 更新后不生效

**可能原因**: 缓存未过期

**解决方案**:
1. 等待缓存过期（默认5分钟）
2. 重启容器
3. 设置 `MCP_PROMPTS_CACHE_TTL=0` 禁用缓存

### 问题：自定义 prompts 无法加载

**检查清单**:
- [ ] 文件格式正确（Markdown）
- [ ] 文件名以 `.prompt.md` 或 `.md` 结尾
- [ ] 文件大小 < 1MB
- [ ] Volume 挂载路径正确
- [ ] 文件权限可读

## 最佳实践

1. **生产环境**: 使用内置的 prompts（已包含在镜像中）
2. **开发环境**: 挂载本地目录并禁用缓存
3. **自定义 prompts**: 创建独立的 volume 或使用配置管理系统
4. **版本控制**: 将自定义 prompts 纳入版本控制
5. **文档化**: 为每个自定义 prompt 编写清晰的说明

## 相关文件

- Dockerfile: 定义 prompts 目录的复制
- cmd/mcp-server/prompts.go: Prompts 加载逻辑
- cmd/mcp-server/prompts/*.prompt.md: 内置提示词模板

## 更新日志

- **v0.1.0-alpha** (2025-10-09): 初始版本，支持5个内置提示词模板
