# AIDG CLI 技能文档 (skill.md)

AIDG CLI (`aidg`) 是 AI 辅助开发治理平台的命令行工具，与 MCP Server 31 个已注册工具 1:1 对应。通过直接调用后端 HTTP API，实现与 MCP 工具等价的命令行操作。

## 全局配置

| 配置项 | 命令行标志 | 环境变量 | 默认值 |
|--------|-----------|---------|--------|
| 服务器地址 | `--server-url` | `AIDG_SERVER_URL` | `http://localhost:8000` |
| 认证令牌 | `--token` | `AIDG_TOKEN` | (无) |
| 项目ID | `-p, --project-id` | `AIDG_PROJECT_ID` | (自动回退到当前任务) |
| 任务ID | `-t, --task-id` | - | (自动回退到当前任务) |
| 输出格式 | `-o, --output` | - | `text` (可选: `json`) |

配置文件路径: `~/.aidg/config.yaml`

优先级: 命令行标志 > 环境变量 > 配置文件

---

## 命令目录

### user - 用户任务绑定管理

#### `aidg user get-current-task`
获取当前用户绑定的项目和任务。返回当前的 project_id、task_id 及任务基本信息。

**示例:**
```bash
aidg user get-current-task
```

#### `aidg user set-current-task`
设置当前用户绑定的项目和任务。后续命令中若未指定 project_id/task_id，将自动使用此绑定。

| 参数 | 说明 |
|------|------|
| `-p, --project-id` | 项目ID |
| `-t, --task-id` | 任务ID |

**示例:**
```bash
aidg user set-current-task -p AI-Dev-Gov -t task_123
```

---

### task - 任务管理

#### `aidg task list`
列出项目的所有任务。

| 参数 | 说明 |
|------|------|
| `-p, --project-id` | 项目ID（可选，自动回退） |

**示例:**
```bash
aidg task list -p AI-Dev-Gov
```

#### `aidg task create`
创建新任务。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--name` | 是 | 任务名称 |
| `--description` | 否 | 任务描述 |
| `--assignee` | 否 | 负责人 |
| `--status` | 否 | 状态: todo/in-progress/review/completed |
| `--feature-id` | 否 | 特性ID |
| `--feature-name` | 否 | 特性名称 |
| `--module` | 否 | 模块 |

**示例:**
```bash
aidg task create --name "实现登录功能" --status todo --assignee zhangsan
```

#### `aidg task get`
获取任务详情。包含任务的基本信息和文档完成状态。

**示例:**
```bash
aidg task get -p AI-Dev-Gov -t task_123
```

#### `aidg task update`
更新任务信息。只需传入要修改的字段。

| 参数 | 说明 |
|------|------|
| `--name` | 任务名称 |
| `--description` | 任务描述 |
| `--assignee` | 负责人 |
| `--status` | 状态 |
| `--feature-id` | 特性ID |
| `--feature-name` | 特性名称 |
| `--module` | 模块 |

**示例:**
```bash
aidg task update --status in-progress
```

#### `aidg task delete`
删除任务。

**示例:**
```bash
aidg task delete -t task_123
```

#### `aidg task next-incomplete`
获取项目中下一个有未完成文档的任务。可指定要检查的文档类型，不指定则检查全部五项。返回推荐优先完成的文档类型。

| 参数 | 说明 |
|------|------|
| `--doc-type` | 文档类型筛选: requirements/design/plan/execution/test |

**示例:**
```bash
aidg task next-incomplete --doc-type requirements
```

#### `aidg task prompts`
获取任务的提示词历史记录。包含所有已记录的最终提示词。

**示例:**
```bash
aidg task prompts
```

#### `aidg task create-prompt`
记录提示词到任务历史。用于 AI 开发过程治理的可追溯性。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--username` | 是 | 用户名 |
| `--content` | 是 | 提示词内容 |

**示例:**
```bash
aidg task create-prompt --username zhangsan --content "请实现用户登录API"
```

---

### task-doc (td) - 任务文档读写

#### `aidg task-doc get`
获取任务的文档内容（需求/设计/测试）。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--slot-key` | 是 | 文档类型: requirements/design/test |
| `--include-recommendations` | 否 | 包含推荐内容 |

**示例:**
```bash
aidg task-doc get --slot-key requirements
```

#### `aidg task-doc update`
更新任务文档（全文覆盖）。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--slot-key` | 是 | 文档类型 |
| `--content` | 是 | 文档内容 |

**示例:**
```bash
aidg task-doc update --slot-key design --content "# 设计文档\n## 概述..."
```

#### `aidg task-doc append`
追加内容到任务文档。支持乐观锁。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--slot-key` | 是 | 文档类型 |
| `--content` | 是 | 追加内容 |
| `--expected-version` | 否 | 期望版本号（乐观锁） |

**示例:**
```bash
aidg task-doc append --slot-key requirements --content "## 新增需求..."
```

---

### task-section (ts) - 任务章节级编辑

#### `aidg task-section list`
获取文档的章节树结构。返回所有章节的ID、标题、层级和父子关系。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--doc-type` | 是 | 文档类型: requirements/design/test |

**示例:**
```bash
aidg task-section list --doc-type requirements
```

#### `aidg task-section get`
获取单个章节的内容。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--doc-type` | 是 | 文档类型 |
| `--section-id` | 是 | 章节ID |
| `--include-children` | 否 | 包含子章节内容 |

**示例:**
```bash
aidg task-section get --doc-type design --section-id section_003
```

#### `aidg task-section update`
更新单个章节的内容（不含标题）。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--doc-type` | 是 | 文档类型 |
| `--section-id` | 是 | 章节ID |
| `--content` | 是 | 正文内容（不含标题） |
| `--expected-version` | 否 | 期望版本号 |

**示例:**
```bash
aidg task-section update --doc-type requirements --section-id section_003 --content "更新后的内容"
```

#### `aidg task-section insert`
插入新章节。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--doc-type` | 是 | 文档类型 |
| `--title` | 是 | 章节标题 |
| `--content` | 是 | 章节内容 |
| `--after-section-id` | 否 | 插入到此章节之后 |

**示例:**
```bash
aidg task-section insert --doc-type design --title "## 3.8 新增模块" --content "模块描述..."
```

#### `aidg task-section delete`
删除章节。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--doc-type` | 是 | 文档类型 |
| `--section-id` | 是 | 章节ID |
| `--cascade` | 否 | 级联删除子章节 |

**示例:**
```bash
aidg task-section delete --doc-type design --section-id section_005 --cascade
```

#### `aidg task-section sync`
同步章节与编译文档。当章节和编译文档内容不一致时使用。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--doc-type` | 是 | 文档类型 |
| `--direction` | 是 | 同步方向: from_compiled/to_compiled |

**示例:**
```bash
aidg task-section sync --doc-type requirements --direction from_compiled
```

---

### project-doc (pd) - 项目文档管理

#### `aidg project-doc get`
获取项目文档（特性列表或架构设计）。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--slot-key` | 是 | 文档类型: feature_list/architecture_design |
| `--format` | 否 | 输出格式: json/markdown（默认 markdown） |

**示例:**
```bash
aidg project-doc get --slot-key feature_list --format json
```

#### `aidg project-doc update`
更新项目文档。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--slot-key` | 是 | 文档类型 |
| `--content` | 是 | 文档内容 |
| `--format` | 否 | 文档格式 |

**示例:**
```bash
aidg project-doc update --slot-key architecture_design --content "# 架构设计..."
```

---

### meeting (mtg) - 会议管理

#### `aidg meeting list`
列出所有会议。

**示例:**
```bash
aidg meeting list
```

#### `aidg meeting doc-get`
获取会议文档。根据 slot-key 不同，调用不同的 API 路径。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--meeting-id` | 是 | 会议ID |
| `--slot-key` | 是 | 文档类型: meeting_info/polish/context/summary/topic/merged_all |

**示例:**
```bash
aidg meeting doc-get --meeting-id 0919_AI讨论 --slot-key summary
```

#### `aidg meeting doc-update`
更新会议文档。仅支持 summary/topic/polish 三种类型。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--meeting-id` | 是 | 会议ID |
| `--slot-key` | 是 | 文档类型: summary/topic/polish |
| `--content` | 是 | 文档内容 |

**示例:**
```bash
aidg meeting doc-update --meeting-id 0919_AI讨论 --slot-key summary --content "会议总结..."
```

#### `aidg meeting sections`
获取会议文档的章节树结构。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--meeting-id` | 是 | 会议ID |
| `--slot-key` | 是 | 文档类型: polish/summary/topic |

**示例:**
```bash
aidg meeting sections --meeting-id 0919_AI讨论 --slot-key summary
```

---

### meeting-section (ms) - 会议章节编辑

#### `aidg meeting-section update`
更新会议文档章节内容。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--meeting-id` | 是 | 会议ID |
| `--slot-key` | 是 | 文档类型: polish/summary/topic |
| `--section-id` | 是 | 章节ID |
| `--content` | 是 | 章节内容 |
| `--expected-version` | 否 | 期望版本号 |

**示例:**
```bash
aidg meeting-section update --meeting-id 0919_AI讨论 --slot-key summary --section-id section_001 --content "更新内容"
```

---

### plan - 执行计划管理

#### `aidg plan get`
获取当前任务的执行计划。返回 Markdown 格式的计划内容，包含 YAML Frontmatter 和步骤列表。

**示例:**
```bash
aidg plan get
```

#### `aidg plan update`
更新/提交执行计划。内容必须遵循严格的 Markdown + YAML Frontmatter 格式。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--content` | 是 | 执行计划内容（Markdown格式） |

**示例:**
```bash
aidg plan update --content "$(cat execution_plan.md)"
```

#### `aidg plan next-step`
获取下一个可执行步骤。基于依赖关系、优先级和步骤序号智能决策。

**示例:**
```bash
aidg plan next-step
```

#### `aidg plan step-status`
更新步骤执行状态。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--step-id` | 是 | 步骤ID (如 step-01) |
| `--status` | 是 | 状态: pending/in-progress/succeeded/failed/cancelled |
| `--output` | 否 | 执行输出/日志 |

**示例:**
```bash
aidg plan step-status --step-id step-01 --status succeeded --output "文件创建完成"
```

---

### summary - 任务总结管理

#### `aidg summary list`
列出任务的所有总结。

| 参数 | 说明 |
|------|------|
| `--start-week` | 起始周 (如 2026-W01) |
| `--end-week` | 结束周 |

**示例:**
```bash
aidg summary list
```

#### `aidg summary add`
新增任务总结。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--time` | 是 | 时间 |
| `--content` | 是 | 总结内容 |

**示例:**
```bash
aidg summary add --time "2026-02-17" --content "完成了CLI工具开发"
```

#### `aidg summary update`
更新任务总结。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--summary-id` | 是 | 总结ID |
| `--time` | 否 | 时间 |
| `--content` | 否 | 总结内容 |

**示例:**
```bash
aidg summary update --summary-id sum_001 --content "更新后的总结"
```

#### `aidg summary delete`
删除任务总结。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--summary-id` | 是 | 总结ID |

**示例:**
```bash
aidg summary delete --summary-id sum_001
```

#### `aidg summary query-by-week`
按周范围查询任务总结。可跨任务查询。

| 参数 | 必选 | 说明 |
|------|------|------|
| `--start-week` | 是 | 起始周 (如 2026-W01) |
| `--end-week` | 否 | 结束周 |

**示例:**
```bash
aidg summary query-by-week --start-week 2026-W01 --end-week 2026-W04
```

---

## 快速开始

```bash
# 1. 设置环境变量
export AIDG_SERVER_URL=http://localhost:8000
export AIDG_TOKEN=your-token

# 2. 查看当前绑定的任务
aidg user get-current-task

# 3. 列出项目任务
aidg task list

# 4. 获取需求文档
aidg task-doc get --slot-key requirements

# 5. 查看执行计划
aidg plan get

# 6. 获取下一个可执行步骤
aidg plan next-step
```
