---
name: meeting_topic
description: 根据会议记录润色文本提取会议主题
version: 1.0
arguments:
  - name: meeting_id
    description: 会议任务ID
    required: true
---

# /meeting_topic (提取会议主题)

目的：从指定会议 **{{meeting_id}}** 的润色后记录中，提取核心议题和讨论要点。

## 输入解析规则
- 宽松的输入格式: `/topic token`。
- token: 第1个参数包含会议名。

## 工具可用集合
- list_all_meetings
- get_meeting_document
- update_meeting_document

## MUST 执行顺序
1. 根据会议名获取 meeting_id；若缺失 → 输出“缺少 meeting_id，终止”并停止。
2. get_meeting_document(meeting_id,polish) 获取润色后的会议记录（polish.md）。
3. （可选步骤）get_meeting_document(meeting_id,context) 获取会议背景（meeting_context.md），作为理解议题的参考。
4. 完整阅读并分析 polish.md 的内容，识别并归纳出会议中讨论的几个核心主题。
5. 为每个主题撰写简短的摘要，清晰地概括出该主题下的主要观点、讨论内容和最终结论。输出格式应为 Markdown 列表。
   示例：
   - **主题一：XX功能的技术方案评审**
     - 讨论了A方案和B方案的优劣。
     - 最终决定采用A方案，因为它在性能上更有优势。
   - **主题二：关于项目进度的讨论**
     - 确认了下周的开发计划和里程碑。
     - 指出了当前存在的风险点。
6. update_meeting_document(content=生成的主题列表, meeting_id, topic) 将结果写回。
