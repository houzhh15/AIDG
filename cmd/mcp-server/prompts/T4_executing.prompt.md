---
name: t4_executing
description: 用于生成代码实现提示词，驱动自动化编码的提示词模版。
version: 1.1
---

# AI 代码实现提示 (AI Code Implementation Prompt)

目的：读取执行步骤，生成精确的代码实现提示词，驱动自动化编码。token允许的情况下尽可能执行多个步骤。

执行以下流程：
1. get_user_current_task：无 → 输出 NO_TASK 终止。
2. 取证最小：
   - get_execution_plan (现有执行计划)
   - get_task_document slot_key=design (现有设计)
   - 可选：需求 → get_task_document slot_key=requirements（需要验证业务规则时）
   - 可选：架构 → get_project_document slot_key=architecture_design（架构边界/规范/接口时）
   - 可选：项目特性 → get_project_document slot_key=feature_list format=markdown
   - slot_key 白名单：task文档仅 requirements / design；project文档仅 feature_list / architecture_design；禁止臆造，缺失用 <缺失: ...> 标注。
   - 缺失用 <缺失: X> 标注。
3. 解析 "现有执行计划" 识别所有步骤及其状态, 生成本次执行Plan;
4. 组装 Effective Prompt（含：原始请求 + 摘要 + 计划）→ create_project_task_prompt；失败重试一次，再失败 PROMPT_RECORD_FAIL。
5. 执行Plan，包含以下几个步骤：
   1. 按照计划顺序，逐步执行每个未完成的步骤：
      - get_next_executable_step(project_id, task_id)：获取下一个可执行步骤 ID。
      - 读取步骤详情（描述、优先级等）。
      - 必要时，最小化读取相关文件（如设计文档、代码文件等）。
      - 生成精确的代码实现提示词，驱动自动化编码。
   2. 每完成一个步骤，执行 update_plan_step_status 更新状态，有效的状态值为：pending（待开始）、in-progress（进行中）、succeeded（成功完成）、failed（失败）、cancelled（已取消）。如果结果输出超过150字，请概括。
6. 汇总：输出 表格(plan 执行状态) 。

Go 规范简表：
- 错误透明返回，不吞；外部错误 wrap: fmt.Errorf("ctx: %w", err)
- 并发共享状态加锁或通道；禁止数据竞争
- JSON struct 字段都加 tag
- 单一职责：函数 ≤40 行，复杂拆分
- 新逻辑需最少 1 正常 + 1 异常测试
- 单文件建议 ≤500 行（>400 行需评估拆分），避免“上帝文件”

前端 (JS/TS/TSX) 简表：
- 组件函数 ≤200 行；UI/逻辑拆分（hooks + presentational）
- 状态最小化：局部优先，其次 context；避免层层 props drilling（必要时用自定义 hook）
- 副作用集中在 useEffect；避免在 render 中产生副作用
- 命名：组件 PascalCase；hook 以 use 开头；文件名与导出主组件一致
- 风格：统一使用 ES 模块 + const；禁止 any（除非 // TODO: narrow）
- API 请求封装在 `/src/api/`；UI 不直接拼接 URL
- CSS：优先模块化/原子化（避免全局泄漏）
- 测试：关键交互（事件/分支）≥1 条用例（如 vitest + rtl）

Python（如需）：脚本入口 + 纯函数，无隐藏副作用。

Effective Prompt 模板：
```
# System Role
Go/Python 工程助手，执行 <一句话目标>

# Meta
project_id:{project_id}
task_id:{task_id}
username:{username}
timestamp:{timestamp_iso}

# Context
<取证要点 或 <缺失: 无取证>>

# User Message
<原始指令>

# Plan
1. ...
2. ...
3. ...
4. ...

# Expected
<预期产物：文件/函数/测试等>
```

输出结构：
```
记录ID: <prompt_id>
| 步 | 描述 | 状态 | 备注 |
|----|------|------|------|
...

变更:
| 文件 | 动作 | 摘要 |

Gaps:
- <缺失: ...>
Next:
- 建议...
```

失败代码：NO_TASK / PLAN_EMPTY / PROMPT_RECORD_FAIL / EXEC_FAIL / TEST_FAIL

---
请开始执行当前任务的代码实现提示生成，要求：
- 严格参考设计文档，从中提取描述及代码片段，如果非必要不要进行调整。
- 如果有必要调整，请务必用mcp章节编辑工具同步更新设计文档。
- 请在任务时mcp添加简短任务总结。