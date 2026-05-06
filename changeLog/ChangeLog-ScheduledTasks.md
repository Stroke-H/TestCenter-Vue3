# ChangeLog - Scheduled Tasks

## 2026-04-30

### Changed
- **Scheduled Drama Report Snapshot**: 定时任务触发的剧集播放接口测试也改为在执行完成后生成独立报告快照，并把执行记录保存到该快照地址，避免历史定时任务报告继续被后续运行覆盖。

## 2026-05-06

### Changed
- **Scheduled Task Audit Logging**: 定时任务执行完成后，现在会按 `ai_operation_logs` 的统一结构补写一条 AI 操作审计日志，包含任务名、项目、环境、执行人、执行结果、耗时与报告地址，便于在飞书助手的“AI 操作审计日志”区间内回看定时任务执行轨迹。

## 2026-04-24

### Added
- **AI Report Summary Notification**: 定时任务完成后会自动读取本次生成的剧集检测报告，调用 Opus4.7 生成中文总结，并通过飞书机器人发送给任务创建人。
- **Plain Text Message Formatting**: 飞书通知统一为纯文本格式，清理 Markdown 标记和表格符号，避免出现 `**`、反引号等特殊字符。
- **Report Freshness Guard**: 分析前校验报告文件更新时间，避免任务失败时误分析上一次残留报告。
- **Fallback Notification**: AI 分析失败时保留基础通知，不影响任务完成记录、执行报告入库和下次调度。

### Changed
- **Execution Report Delivery**: 定时任务完成消息从“请去平台查看”升级为包含结果、耗时、报告摘要和报告地址的可读通知。
- **Edit Interaction**: Scheduled tasks 列表移除单独的 Edit 按钮，点击整条任务记录即可打开编辑弹窗；运行中的任务点击时给出不可编辑提示。

## 2026-04-28

### Changed
- **Console Localization**: 飞书助手页面标题、面包屑、概览卡片、会话列表、定时任务表格和编辑弹窗统一调整为中文后台文案，降低模块间风格割裂感。
- **Proxy-safe Assistant Requests**: 飞书助手控制台改为统一通过同源 `/api` 访问日志、项目和定时任务接口，降低非 localhost 域名访问时的数据链路不一致风险。
