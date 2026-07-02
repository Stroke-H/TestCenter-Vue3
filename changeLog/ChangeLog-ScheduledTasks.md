# ChangeLog - Scheduled Tasks

## 2026-06-30

### Fixed
- **Running Task Recovery Reschedule**: 周期定时任务在后端重启后若发现旧 `running` 状态但执行进程已不存在，会记录中断结果并自动顺延到下一次未来执行时间，不再永久停留在 `paused`。
- **Early Schedule State Commit**: 剧集播放定时任务执行结束后会优先回写任务状态和下次执行时间，再执行报告归档、AI 总结和飞书通知，避免收尾链路异常导致后续周期不再触发。
- **Paused Task Status Click**: 定时任务状态标签点击行为补齐，`PAUSED` 等非运行状态点击状态标签会打开编辑弹窗，避免标签拦截行点击后没有反馈。

### Added
- **External Subtitle Scheduled Test**: 新增“剧集外挂字幕测试”定时任务类型，独立于剧集播放接口测试执行。任务会筛选 `embedded_subtitle=0` 的外挂剧，按章节在线请求 VTT 字幕并全量检查语种匹配、时间轴超过 10 分钟、结束早于开始和时间轴倒退等异常。
- **Subtitle Failure Artifacts**: 字幕测试只保存异常字幕的精准 cue 或文件级异常，不缓存正常字幕正文；运行产物包含 `subtitle_summary.json`、`subtitle_failures.jsonl` 和 `subtitle_report.html`，并会归档到执行报告详情页。
- **Subtitle Task Feishu Notice**: 字幕测试完成后会发送独立的飞书通知卡片，展示外挂剧数量、字幕检查进度、异常文件、语种异常、时间轴异常和拉取失败统计。
- **Subtitle Prepare Progress**: 字幕测试准备阶段改为有限并发拉取章节列表，并持续刷新 `subtitle_summary.json` 与后台进度日志，避免全量剧库准备阶段长时间无可见反馈。
- **Subtitle Missing URL Detection**: 字幕测试会把 `embedded_subtitle=0` 但章节数据中未解析到 VTT 地址的剧集作为“缺失字幕”异常上报，避免只检查到有 URL 的外挂剧。
- **Subtitle Report Redesign**: 剧集外挂字幕测试报告改为更接近剧集播放接口测试报告的版式，包含顶部状态区、关键指标卡、分类统计和按剧集分组的异常明细。
- **Subtitle Online Scope Fix**: 剧集外挂字幕测试改为以 `all_online_ids` 作为权威在线剧集范围，再使用 `drama/list` 补全元信息，避免正式服 `drama/list?online=1` 返回更宽集合导致误扫非目标剧集。
- **Subtitle Collection Milestones**: 字幕测试准备阶段新增按外挂剧数量计算的 10% 里程碑日志，并在日志中附带总在线剧集数作为上下文，避免把非外挂剧误表述为已收集。
- **Scheduled Command Streaming Logs**: 定时任务执行外部脚本时改为实时流式输出 stdout/stderr，避免字幕全量检查这类长任务在后台日志中长时间无反馈。
- **Subtitle Check Milestones**: 字幕文件检查阶段新增每 10% 的里程碑日志，展示已检查项数和异常数。
- **Subtitle Full-run Optimization**: 字幕检查默认关闭 AI 复核，新增 URL 级去重与跨运行缓存，重复字幕 URL 会复用上次检查结果，显著减少正式服全量检查的网络请求和 AI token 消耗。
- **Subtitle Check Heartbeat**: 字幕文件检查阶段新增每 60 秒心跳日志，长时间未跨过 10% 里程碑时也会持续输出当前检查数、百分比、异常数和缓存命中数。
- **Subtitle Report Classification**: 剧集外挂字幕报告新增“字幕数量异常、缺失字幕、语种异常、时间轴异常、拉取失败”分类展示，避免不同异常混在同一个列表里。
- **Subtitle Count Validation**: 字幕准备阶段会读取剧集 `chapters` 并在内容检查前比对正片 VTT 字幕文件数量；数量不一致会独立上报但不阻断后续字幕内容检查。
- **Subtitle Language AI Review**: 语种疑似不符时会通过配置的 AI 提供方复核，只有 AI 确认不符才按语种异常上报，并在报告里展示不符合的字幕片段和原因。
- **Subtitle Report Entry**: 剧集外挂字幕测试报告归入测试报告页“接口验证”分类；历史 `字幕测试` 类型记录也会兼容显示到该分类下。
- **Manual Subtitle Check Entry**: 仪表盘 API 工具和智能助手快捷入口配置新增“剧集外挂字幕测试”入口，可在执行测试页手动启动字幕检查。
- **Manual Subtitle Check Logs**: 新增字幕检查手动运行链路，准备数据、字幕检查和报告生成阶段会把后端脚本日志实时输出到执行页日志区，并在完成后归档到“接口验证”报告列表。
- **Subtitle Check Concurrency**: 字幕准备和字幕文件拉取默认并发从 6 提升到 16，仍支持通过 `SUBTITLE_CHAPTER_CONCURRENCY` 与 `SUBTITLE_FETCH_CONCURRENCY` 配置覆盖，上限 24。
- **Subtitle Prepare Log Detail**: 字幕准备阶段进度日志新增本段新增正片字幕、累计正片字幕、有字幕剧、未解析字幕剧和数量异常剧统计，避免累计值不变时误以为重复打印。
- **Subtitle Prepare Log Clarity**: 字幕准备阶段日志改为展示本段处理外挂剧数、本段新增字幕、本段新增命中剧、累计命中率和连续未新增提示；手动执行页会按 50 部进度同步推进准备阶段进度条，并合并相邻重复日志。
- **Subtitle Source Fallback**: 字幕准备阶段在 `chapter/list` 未解析到完整正片 VTT 时，会按剧集语种请求 `all_subtitle` 兜底，修复部分外挂剧实际有字幕却被误报为“缺失字幕”的问题。
- **Subtitle Report Navigation**: 字幕报告异常明细按异常类型支持展开/收起，并在右下角新增返回顶部按钮，减少大量异常时的纵向滚动成本。
- **Subtitle AI Review Disabled By Default**: 剧集外挂字幕测试默认关闭 AI 语种复核，避免全量检查缓存未命中时产生过高 token 消耗；仅在显式设置 `SUBTITLE_AI_ENABLED=1` 时启用。
- **Subtitle Japanese Traditional Chinese Tolerance**: 日语剧集字幕语种检查新增繁体中文容错，字幕样本包含繁体中文时不再按语种异常上报。

## 2026-06-11

### Added
- **Drama Metadata Reuse**: 剧集播放定时任务会复用 K6 前置数据阶段保存的剧集标题元信息，在飞书 AI 报告和异常剧集明细中稳定展示 `int_id`、英文标题与中文标题，减少任务结束后再次查询管理接口导致的网络失败。
- **Scheduled Notice Diagnostics**: 定时任务通知链路补充 AI 总结生成、飞书卡片发送、纯文本兜底发送的关键日志，便于定位 once 任务报告生成成功但通知未到达的问题。

## 2026-06-10

### Changed
- **Once Task Cleanup**: `Once` 定时任务执行完成后会自动从定时任务列表中移除，只保留执行报告、飞书通知和审计记录。
- **Running State Recovery**: 定时任务会识别后端进程已不存在但状态仍为 `running` 的异常记录，并自动恢复为可编辑/可删除的暂停状态。
- **Running Task Actions**: 定时任务表去除独立操作栏，只有 `RUNNING` 状态可点击；点击后可选择暂停任务或仅停止本次执行，停止本次后后续周期仍按下一次触发时间继续。

### Fixed
- **Past Schedule Guard**: 新建或编辑定时任务时，前端会限制选择过去时间，后端也会拒绝早于当前时间的 `next_run`，避免误设过去时间后任务被立即触发。

## 2026-06-05

### Added
- **Scheduled Task Retry**: 剧集播放定时任务新增任务级重试机制；当整条执行链路因短暂网络波动、准备数据失败或 K6 命令异常失败时，会按 5 分钟、10 分钟、15 分钟最多重试 3 次，避免偶发波动直接导致当天任务失败。

### Changed
- **Final-result Reporting**: 定时任务报告大厅、飞书通知和审计日志只在最终尝试结束后写入结果；若重试后成功，会记录“第 N 次尝试成功”，若全部失败，会保留每次尝试的失败原因。现有 720p 失败 case 二次复验逻辑保持不变。

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
