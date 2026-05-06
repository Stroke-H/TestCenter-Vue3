# ChangeLog - K6 Integration

## 2026-04-30

### Updated
- **Unique Drama Report Snapshot**: 修复剧集播放接口测试历史报告互相覆盖的问题。现在每次剧集播放测试完成后，都会基于当次运行生成一份独立的 HTML 报告快照，并将执行记录中的 `reportUrl` 指向这份唯一文件，不再所有历史记录共用同一个 `drama_check_report.html`。
- **Single-Issue Summary Promotion**: 优化剧集播放接口测试报告的异常摘要文案。对于仅命中 1 条异常的剧集，报告摘要现在会直接显示具体异常内容，例如缺失章节或单一计数异常，而不再统一只显示“发现 1 项异常”。

## 2026-03-16

### Updated
- **Dynamic Iteration Logic**: 移除了 `k6_config.json` 中硬编码的 `itersPerVu: 3`。
- **Full Coverage Enhancement**: 激活了 `drama_check_flow.js` 中的自动计算逻辑。现在测试会根据 `drama_info.json` 中的实际剧集总数动态分配每个 VU 的迭代次数（例如 7570 条数据下，30 个 VU 每个执行 253 次），确保 100% 数据覆盖率，同时保持恒定的 30 次并发。

## 2026-03-13

### Added
- **K6 Environment Stub**: 在项目根目录创建了专门用于存放压测类脚本的 `k6-scripts/` 目录。
- **Type Definitions**: 通过 `npm install -D @types/k6` 添加了 K6 的 TypeScript/JavaScript 代码提示与类型推断支持。
- **Template Script**: 添加了 `episode.js` 压测范例脚本，包含了基于 Virtual Users (VU) 的并发连接模拟与基础业务链路断言 (200 OK & JSON Body Validation)。
- **HTML Reporter Integration**: 在 `episode.js` 中引入了 `k6-reporter` 插件，用于在测试结束时输出一份可视化的 `summary.html` 分析报告。

### Go Backend Integration 
- **Server Initialization**: 初始化了新的 Go 模块 (`testcenter-server`)，包含 Gin 路由与 Gorilla WebSocket 依赖。
- **WebSocket Streaming**: 新建 `RunK6TestHandler` 接口，它能利用 `os/exec` 后台执行 `k6 run` 命令，并将进程的标准输出与错误流实时发送到前端的 WebSocket 连接。
- **Static Reports Host**: 通过 `r.StaticFS` 给生成的 K6 压测 HTML 报告提供静态网页浏览能力。

### Vue Frontend Updates
- **Real-time Logging**: 将之前的定时器模拟日志功能切换成了原生的 `WebSocket` 事件侦听，并将收到的真实 K6 控制台字符串即时追加到滚动面板中。
- **Report Visualization**: 在前端引入了对于压测报告完结后的动态视图切换。测试完成后隐藏控制台，内嵌展示调度端生成的报告 iframe (`http://localhost:8080/reports/summary.html`)。
