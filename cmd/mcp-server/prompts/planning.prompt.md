````prompt
# 设计文档 → 可执行开发分解提示 (Design Decomposition Minimal)

目的：把一份设计文档转化为 AI/工程师可直接执行的“代码实现蓝图”，粒度精确到函数/文件/测试用例/迁移步骤/依赖顺序，但不要拆分太多步骤，把相邻的强依赖步骤放在一起。

适用场景：已有《需求/设计文档》，需要形成结构化实现任务集，后续可驱动自动化编码、进度跟踪。

流程：
1. get_user_current_task：无 → 输出 NO_TASK 终止。
2. 取证最小：
  - get_execution_plan (现有执行计划)
  - get_task_document slot_key=design (现有设计)
  - 现有代码（根目录下）
  - 可选：需求 → get_task_document slot_key=requirements（需要验证业务规则时）
  - 可选：架构 → get_project_document slot_key=architecture_design（架构边界/规范/接口时）
  - 可选：项目特性 → get_project_document slot_key=feature_list format=markdown
  - slot_key 白名单：task=requirements|design|test；project=feature_list|architecture_design；禁止自造。
   - 缺失用 <缺失: X> 标注。
3. 组装 Effective Prompt（含：原始请求 + 摘要 + 计划）→ create_project_task_prompt；失败重试一次，再失败 PROMPT_RECORD_FAIL。
4. 后执行，生成markdown格式的“任务分解清单”,包含完整的执行步骤,格式要求见下。
5. update_execution_plan(project_id, task_id, content=任务分解清单)。

### 附：章节级编辑标准流程 (Section-Level Editing Workflow)
若某执行步骤需要对现有文档做“局部”编辑（而非整体重写），请遵循：
1. 获取章节树：`get_task_doc_sections`
2. 可选获取单章：`get_task_doc_section`
3. 产出最小修改片段（仅改必需内容）
4. 应用：`update_task_doc_section` / `insert_task_doc_section` / `delete_task_doc_section`
5. 仅在确认整体重构且具备 FULL_OVERRIDE_CONFIRM 语义时再考虑 `update_task_document`。

---
**# markdown 格式要求 (必须严格遵守)**

1.  **YAML Frontmatter:** 文件开头必须包含一个 YAML Frontmatter，包含以下字段：
    - `plan_id`: (自动生成一个 UUID)
    - `task_id`: "{task_id}"
    - `status`: "Pending Approval"
    - `created_at`: (当前 ISO 8601 时间)
    - `updated_at`: (当前 ISO 8601 时间)
    - `dependencies`: 一个定义步骤依赖关系的对象数组。例如 `[{ source: 'step-03', target: 'step-01' }]` 表示 step-03 依赖于 step-01。

2.  **Markdown Body:**
    - 必须是一个 Checkbox 列表 (`- [ ]`)。
    - 每个步骤必须以 `step-XX:` 开头。
    - 可以在步骤描述后用 `key:value` 形式添加 `priority` 属性。

**# 输出示例**

---
plan_id: "a1b2c3d4-e5f6-7890-1234-567890abcdef"
task_id: "task_1759127546"
status: "Pending Approval"
created_at: "2025-09-29T18:10:00Z"
updated_at: "2025-09-29T18:10:00Z"
dependencies:
  - { source: 'step-03', target: 'step-01' }
  - { source: 'step-03', target: 'step-02' }
---
- [ ] step-01: 创建 `execution_plan_service.go` 文件。 priority:high
- [ ] step-02: 定义 `ExecutionPlanService` 结构体。 priority:high
- [ ] step-03: 实现 `UpdatePlan` 方法。 priority:medium



---
版本：planning.min.v1
````
