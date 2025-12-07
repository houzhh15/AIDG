---
name: meeting_polish
description: 根据会议记录润色文本
version: 1.0
arguments:
  - name: meeting_id
    description: 会议任务ID
    required: true
---


# /meeting_polish (生成会议记录润色文本)

目的：为指定会议 **{{meeting_id}}** 生成会议详细记录（润色后的文本）。

## 工具可用集合
- list_all_meetings
- get_meeting_document
- update_meeting_document
- get_task_doc_sections
- update_task_section

## 润色要求
通过mcp工具获得原始文本 merged_all , 这是语音转换文字的对话记录，每5分钟一个chunk，其中有太多语音理解错误。请完整读取这个文件，结合上下文推测其中真实表达的意思，使用更流畅的语言，重新表达对话过程。尝试还原出当时对话的场景，提升整体的易读性。调用mcp工具获取 会议背景记录，可作为推测依据。以每5分钟为一个章节，完整重写会议详细过程，生成polish.md并提交mcp。内容样例见下方，请保留说话人标识（SPK00，SPK01等）和时间顺序，但润色内容请重新表达。


## 文本样例
（SPK00）“这里的邮件 显示画面下方，在主菜单下面再加一层：像是 ‘受信(收件箱) / 検索 / 迷惑メール / ……’ 等几个邮件相关的文件夹，一次性并排显示，点击就能快速展开。” 
（SPK01）“要不要从界面再确认下？那一块位置——受信也需要，白名单黑名单都要能点进，垃圾邮件判定状态能一目了然。还需要‘送信(已发送)’ 要不要单列？这些分类最好全部列出来给用户看到，然后用图标或标签区分更清晰。”
（SPK00）“嗯嗯，没错。这样用户就能更方便地操作邮件了。那我们就按照这个思路来设计界面吧！”

## MUST 执行顺序
1.  根据会议名获取 meeting_id；若缺失 → 输出“缺少 meeting_id，终止”并停止。
2.  get_meeting_document(meeting_id,merged_all) 获取原始会议记录（merged_all.md）。
3.  （可选步骤）get_meeting_document(meeting_id,context) 获取会议背景（meeting_context.md）。
4.  结合 merged_all.md 和 meeting_context.md 的信息，推测真实表达的意思，使用更流畅的语言，重新表达对话过程，提升易读性。
5.  以每5分钟为一个章节，生成润色后的会议记录。先使用 updata_meeting_document 提交所有的章节包括子章节。update_meeting_document(content=所有章节标题, meeting_id, polish) 
6.  调用 get_meeting_doc_sections 获得章节信息
7.  使用 update_meeting_section更新章节内容。但不要包含任何标题。

