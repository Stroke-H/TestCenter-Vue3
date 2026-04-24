# ChangeLog - Scheduled Tasks

## 2026-04-24

### Added
- **AI Report Summary Notification**: 定时任务完成后会自动读取本次生成的剧集检测报告，调用 Opus4.7 生成中文总结，并通过飞书机器人发送给任务创建人。
- **Plain Text Message Formatting**: 飞书通知统一为纯文本格式，清理 Markdown 标记和表格符号，避免出现 `**`、反引号等特殊字符。
- **Report Freshness Guard**: 分析前校验报告文件更新时间，避免任务失败时误分析上一次残留报告。
- **Fallback Notification**: AI 分析失败时保留基础通知，不影响任务完成记录、执行报告入库和下次调度。

### Changed
- **Execution Report Delivery**: 定时任务完成消息从“请去平台查看”升级为包含结果、耗时、报告摘要和报告地址的可读通知。
