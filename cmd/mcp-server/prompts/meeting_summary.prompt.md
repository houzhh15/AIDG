---
name: meeting_summary
description: 生成会议总结
arguments:
  - name: meeting_id
    description: 会议任务ID
---

# /meeting_summary (生成会议总结)

目的：为指定会议 **{{meeting_id}}** 生成一份结构化的会议总结。

## 工具可用集合
- list_all_meetings
- get_meeting_document
- update_meeting_document

## 总结要求
请用金字塔原理总结内容；明确出会议最终观点、论据、事实；最后给出行动上的指南、建议。最后补充：
- （1）文中的讨论提到了哪些知识点，分别属于什么领域，一般是用于什么
- （2）特别有 insight（观点与众不同，但又正确） 的内容单独强调。
- （3）What important truth do very few people agree withthis content?
- （4）可以提炼出来哪些有用的规律，这些规律可以用于指导哪些实践？
- （5）最后，请你结合你自己的知识，提出批判性见解，识别并挑战每个陈述中的每个假设。

## MUST 执行顺序
1.  根据会议名获取 meeting_id；若缺失 → 输出“缺少 meeting_id，终止”并停止。
2.  get_meeting_document(meeting_id,polish) 获取润色后的会议记录（polish_all.md）。
3.  （可选步骤）get_meeting_document(meeting_id,context) 获取会议背景（meeting_context.md）。
4.  综合 polish_all.md 和 meeting_context.md 的信息，全面分析会议内容。
5.  生成一份结构化的会议总结，参考总结要求。
6.  update_meeting_document(content=生成的会议总结, meeting_id, summary) 将结果写回。