# 统一文档架构设计

## 📐 架构概览

### 核心原则
1. **单一真实来源**：`compiled.md` 是文档的唯一权威来源
2. **完整历史追踪**：`chunks.ndjson` 记录所有修改历史
3. **派生视图**：`sections.json` + `sections/` 是从 `compiled.md` 派生的章节视图
4. **统一版本管理**：使用 `meta.json` 中的版本号进行并发控制

## 📁 文件结构

```
docs/requirements/
├── compiled.md        ← 最终文档（唯一真实来源）
├── meta.json          ← 文档元数据（版本号、ETag、更新时间）
├── chunks.ndjson      ← 完整的变更历史（包括全文编辑和章节编辑）
├── sections.json      ← 章节元数据（从 compiled.md 派生）
└── sections/          ← 章节文件（从 compiled.md 派生）
    ├── section_001.md
    ├── section_002.md
    └── ...
```

## 🔄 数据流

### 1. 全文编辑（Full Document Edit）

```
用户编辑
    ↓
saveTaskDocument API
    ↓
DocService.Append(op="replace_full")
    ↓
├─→ 写入 compiled.md
├─→ 记录到 chunks.ndjson
├─→ 更新 meta.json (version++)
└─→ SyncFromCompiled()
        ↓
    重新生成 sections.json 和 sections/
```

### 2. 章节编辑（Section Edit）

```
用户编辑章节
    ↓
updateTaskSection API
    ↓
SectionService.UpdateSection()
    ↓
├─→ 修改 sections/section_xxx.md
├─→ 更新 sections.json
└─→ SyncToCompiled()
        ↓
    重新拼接 compiled.md
        ↓
DocService.Append(op="replace_full", source="update_section")
    ↓
├─→ 记录到 chunks.ndjson
├─→ 更新 meta.json (version++)
└─→ SyncFromCompiled()
        ↓
    重新验证 sections.json 和 sections/
```

### 3. 章节全文编辑（Section Full Edit）

```
用户编辑父章节及其所有子章节
    ↓
updateTaskSectionFull API
    ↓
SectionService.UpdateSectionFull()
    ↓
├─→ 读取 compiled.md
├─→ ReplaceSectionRange() 替换父章节范围
└─→ DocService.Append(op="replace_full", source="update_section_full")
        ↓
    ├─→ 写入 compiled.md
    ├─→ 记录到 chunks.ndjson
    ├─→ 更新 meta.json (version++)
    └─→ SyncFromCompiled()
            ↓
        重新生成 sections.json 和 sections/
```

## 🔒 并发控制

### 统一版本号
- 所有操作都使用 `meta.json.Version` 进行并发检查
- 前端传递 `expected_version` 参数
- 后端验证版本号，不匹配则返回 409 Conflict

### 锁机制
- `DocService` 使用互斥锁保护 `compiled.md` 的写入
- `SectionService` 通过调用 `DocService` 自动获得锁保护
- 保证了跨模式的并发安全

```go
// DocService 中的锁
func (s *DocService) Append(...) {
    l := s.GetLock(projectID, taskID, docType)
    l.Lock()
    defer l.Unlock()
    // ... 修改 compiled.md
}

// SectionService 通过 DocService 获得锁保护
func (s *sectionServiceImpl) UpdateSection(...) {
    // ... 修改 sections/
    // ... 拼接 compiled.md
    s.docService.Append(...)  // ← 自动获得锁
}
```

## 📊 chunks.ndjson 格式

每一行是一个 JSON 对象，记录一次文档修改：

```json
{
  "sequence": 1,
  "timestamp": "2025-10-14T12:00:00Z",
  "op": "replace_full",
  "content": "# 文档标题\n\n内容...",
  "user": "section_edit",
  "source": "update_section",
  "hash": "abc123...",
  "active": true
}
```

### 操作类型 (op)
- `add_full`: 首次添加完整内容
- `replace_full`: 替换整个文档
- `append`: 追加内容（暂未使用）

### 来源标识 (source)
- `put`: 全文编辑 API
- `update_section`: 单章节编辑
- `update_section_full`: 章节全文编辑
- `insert_section`: 插入新章节
- `delete_section`: 删除章节

## 🎯 优势

### 1. 完整历史追踪
- 所有修改都记录在 `chunks.ndjson` 中
- 可以回溯任意版本
- 支持审计和合规需求

### 2. 灵活的编辑方式
- 支持全文编辑（适合大规模重写）
- 支持章节编辑（适合局部修改）
- 两种方式无缝切换

### 3. 数据一致性
- `compiled.md` 是唯一真实来源
- 章节文件总是从 `compiled.md` 派生
- 避免了数据不一致问题

### 4. 并发安全
- 统一的版本号管理
- 跨模式的锁机制
- 防止并发修改导致的数据丢失

### 5. 易于理解
- 数据流清晰
- 单向依赖关系
- 易于调试和维护

## 🔧 实现细节

### SyncFromCompiled()
从 `compiled.md` 重新生成章节视图：

```
1. 读取 compiled.md
2. ParseSections() 解析标题结构
3. 清空 sections/ 目录
4. 为每个章节创建独立文件
5. 更新 sections.json
```

### SyncToCompiled()
从章节文件重新拼接 `compiled.md`：

```
1. 读取 sections.json
2. 按顺序读取每个章节文件
3. 拼接标题 + 内容
4. 写入 compiled.md
```

### ReplaceSectionRange()
替换 `compiled.md` 中父章节及其所有子章节：

```
1. 读取 compiled.md
2. 定位父章节的起始位置
3. 检测新内容是否包含同级或更高级标题
4. 如果有：替换到文档末尾
5. 如果没有：只替换到下一个同级章节
6. 返回新的 compiled.md 内容
```

## 📈 性能考虑

### 重复同步优化
- `DocService.Append` 调用 `SyncFromCompiled()` 后
- `SectionService` 不再需要显式调用
- 内容相同时重新解析开销很小

### 大文档优化
- 章节编辑只修改单个文件
- `SyncToCompiled()` 只拼接，不解析
- 大文档性能更好

### 历史记录管理
- `chunks.ndjson` 可以定期归档
- 保留最近 N 个版本的快速访问
- 旧版本移动到归档存储

## 🧪 测试场景

### 场景 1：全文编辑后查看章节
```
1. 用户通过"全文"编辑添加新章节
2. 保存
3. 刷新页面
✅ 章节树正确显示新章节
✅ chunks.ndjson 记录了这次修改
```

### 场景 2：章节编辑后查看全文
```
1. 用户通过章节树编辑单个章节
2. 保存
3. 切换到"全文"视图
✅ 全文显示更新后的内容
✅ chunks.ndjson 记录了这次修改
```

### 场景 3：并发编辑检测
```
1. 用户 A 读取文档（版本 5）
2. 用户 B 读取文档（版本 5）
3. 用户 A 保存（版本 → 6）
4. 用户 B 保存（expected_version=5）
✅ 返回 409 Conflict
✅ 用户 B 需要刷新后重试
```

### 场景 4：历史回溯
```
1. 查看 chunks.ndjson
2. 找到特定 sequence 的版本
3. 读取该版本的 content
✅ 可以查看任意历史版本
```

## 🚀 未来扩展

### 1. 版本对比
- 利用 chunks.ndjson 的历史记录
- 实现版本间的 diff 功能
- 显示"谁在何时修改了什么"

### 2. 协作编辑
- 基于统一版本号的 OT/CRDT
- 实时同步多用户编辑
- 冲突自动合并

### 3. 备份和恢复
- 定期导出 chunks.ndjson
- 一键恢复到任意历史版本
- 灾难恢复方案

### 4. 权限控制
- 章节级别的编辑权限
- 审批流程（修改需要审核）
- 锁定特定章节

## 📝 总结

这个统一架构通过以下设计实现了两种编辑模式的完美融合：

1. **单一真实来源**：`compiled.md` 是权威
2. **完整历史**：`chunks.ndjson` 记录一切
3. **灵活视图**：`sections/` 提供结构化编辑
4. **统一版本**：`meta.json` 管理并发
5. **自动同步**：确保数据一致性

无论用户选择哪种编辑方式，系统都能保证数据的完整性、一致性和可追溯性。
