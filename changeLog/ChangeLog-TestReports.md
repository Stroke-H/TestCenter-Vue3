# ChangeLog - Test Reports

## 2026-06-02

### Added
- **Monkey Continue After Crash**: 新增可选“Crash 后继续执行”模式，默认仍为遇到真实 Crash 立即停止并留证；报告会标记真实 App Crash、结束原因和过滤掉的系统噪声数量。
- **Monkey SSE Live Stream**: Monkey 真机测试改为通过 SSE 实时推送运行摘要、截图节点和风险证据，支持心跳、断线重连与最近事件续传；HTTP 保留为启动、停止、历史资源读取和低频状态校准通道。
- **Monkey Foreground Guard**: Monkey 真机测试新增后台前台应用守护，每 2 秒检查当前 resumed Activity；连续确认偏离测试包名后会关闭普通三方前台应用并重新拉起目标 App，同时实时记录恢复次数和最近偏离包名。

### Fixed
- **Monkey Runtime Checkpoint Status**: 修复 Monkey 每次截图后提前将运行状态结算为完成，导致前端在首张截图后停止轮询的问题。
- **Monkey Evidence Noise Filter**: Monkey 风险节点仅收录目标 App 相关日志，系统背景噪声继续保留在完整日志中但不再触发异常加密采样。
- **Monkey Activity Detection**: Activity 识别兼容新版 Android `dumpsys activity activities` 输出，减少截图节点显示 `UnknownActivity`。
- **Monkey Stop Status Consistency**: 手动停止或超时退出会直接结算为 `stopped`，避免 SSE 实时通道先收到错误的失败终态。
- **Monkey Stop Finalization**: 修复前端点击 Stop 后设备端 Monkey 仍可能继续运行的问题；停止操作会显式终止设备端 Monkey 进程，等待最终截图、摘要和报告落盘后再返回结果。
- **Monkey Atomic Report Viewer**: 真实 Monkey 报告统一进入 Three.js 原子图查看器，不再把节点归档 JSON 直接显示为报告正文；主动停止生成的阶段性报告和历史同类记录状态改为 `Stopped`，真实 Crash 仍保留 `Failed`。
- **Monkey Atomic Viewer Framing**: 原子图相机根据节点球体半径和可用视口自动居中取景，右侧节点详情面板支持自适应宽度、内部滚动和窄屏上下布局。
- **Monkey Screenshot Preview Recovery**: Monkey 截图资源纳入统一后端地址归一化，并通过当前登录态鉴权读取，修复不同访问入口下执行结果和历史报告无法加载采样图片的问题。
- **Monkey Atomic Result Retry**: 执行完成页仅在截图节点成功读取后结束状态校准，节点归档稍有延迟时会自动重试，避免完成态原子图偶发空白。
- **Monkey Readable Evidence Summary**: 异常节点摘要改为面向用户的中文解释，原始日志收纳到摘要末尾的 `Detail` 悬浮入口中，兼顾可读性和排障信息完整度。

## 2026-05-22

### Changed
- **Unified Report Storage**: 所有报告文件统一收束到根目录 `report/`，接口/K6/剧集播放类报告写入 `report/api_report/`，Web 性能报告写入 `report/web_test_report/`，旧 `k6-scripts/reports` 与 `server/storage/test-runs` 不再作为报告存储位置。
- **Embedded Failed Checks Analytics**: 剧集播放 Failed Checks 趋势分析从弹窗入口调整为报告大厅内嵌模块，固定显示在顶部操作区与报告信息表格之间。
- **K6 Report Filter Alignment**: 报告大厅顶部筛选去除“全部”，剧集播放 Failed Checks 趋势仅在 K6 压测页签展示，并将历史“业务自动化”报告归类到 K6 压测列表。
- **K6 First Report View**: 进入测试报告页默认展示 K6 压测页签，报告信息表格改为随页面向下延伸，由整页滚动承载更多记录。
- **Analytics Selection Label**: 将趋势模块中的“当前选中”调整为“桑基图选中报告”，明确它代表当前展开失败来源流向的报告点。
- **Environment Badge**: 报告信息编号旁新增环境角标，K6 压测按执行环境显示橙色 `Test` 或绿色 `Prod`，缺少历史环境信息和非 K6 报告默认显示 `Prod`。
- **Element Plus Radio Compatibility**: 报告类型切换组件改用 `value` 绑定，消除 Element Plus 3.0 radio API 弃用警告。
- **Sankey Label Padding**: 优化失败来源流向桑基图右侧自适应留白与标签宽度，避免右侧节点文案被卡片边框裁切且减少无效空白。
- **Monkey Report Filter**: 报告大厅新增 Monkey 测试筛选栏，真实 Monkey 执行完成后的报告记录会集中展示在该分类下。
- **Monkey Atomic Demo Report**: Monkey 测试列表新增 600 节点验收报告原子图 Demo，点击后直接打开 Three.js 空间模型并支持节点截图预览。
- **Monkey Demo Dialog Fit**: 优化 Monkey 原子图 Demo 弹窗高度自适应与右侧信息面板滚动，避免告警、严重等统计卡片在小视口下被截断。

## 2026-05-21

### Added
- **Failed Checks Trend View**: 报告大厅标题旁新增 Failed Checks 趋势入口，支持从剧集播放接口测试报告读取 Failed Checks、失败类型构成，并在点击某次报告后用桑基图展开失败来源流向。
- **ECharts Report Analytics**: Failed Checks 趋势分析切换为 ECharts 渲染折线图、堆叠柱状图和桑基图，提升图表质感、悬浮提示和点击交互体验。
- **Immutable Test Run Archive**: 剧集播放接口测试新增基于 `runId` 的归档目录，按次保存 metadata、实时日志、HTML artifact 与结构化 metrics，报告访问不再依赖历史域名或临时文件名。
- **Test Run Analytics API**: 新增 `/api/test-runs/analytics/drama-failed-checks`，趋势图直接读取归档指标，不再从可覆盖的本地 HTML 文件夹推测数据。
- **Test Run Artifact API**: 新增 `/api/test-runs/:runId/artifacts/report` 与 `/api/test-runs/:runId/logs`，为后续报告详情和日志回放提供稳定入口。

### Fixed
- **Analytics Dialog Render Lifecycle**: 修复 Failed Checks 趋势弹窗重复打开时，折线图/堆叠柱状图与桑基图因 loading 时序和旧 ECharts 实例残留导致首次只显示部分图表、再次打开数据丢失的问题。
- **Failure Breakdown Semantics**: Failed Checks 趋势图继续使用 k6 检查失败数，桑基图改用异常分类合计作为源头，避免将不同统计口径强行混用导致流向总量对不上。
- **Analytics Data Source Guard**: Failed Checks 趋势视图改为优先读取后端执行记录中的唯一剧集报告快照，执行记录接口失败时不再静默切换到本地文件夹；本地兜底也会排除会被覆盖的 `drama_check_report.html`。
- **Legacy Report URL Recovery**: 后端返回和保存执行记录时会把历史 `strokeh.local`、`localhost`、旧 IP 报告链接归一到当前后端入口，恢复旧报告在域名切换后的可访问性。
- **Drama Snapshot Guard**: 剧集播放接口测试完成后如果没有收到唯一 HTML 快照文件名，不再把会被覆盖的 `drama_check_report.html` 写入历史报告，避免后续趋势数据源被污染。
- **Scheduled Drama Archive Fix**: 修复定时剧集播放接口测试使用固定任务 ID 生成 HTML，导致每天覆盖同一份 `drama_check_report_ST-*.html` 的问题；后续定时任务按运行时间生成独立归档，并且报告大厅优先打开归档 artifact。
- **Truthful Report Source Rule**: 清理剧集报告趋势与报告大厅的本地文件扫描兜底，主动执行和定时执行都必须拿到唯一归档 artifact 才记录报告；没有归档就按无报告显示。
- **Archived Report URL Normalization**: 前端将 `/api/test-runs/...` 归档报告统一识别为后端资源，避免报告 iframe 因域名、端口或访问入口变化继续使用旧地址。

### Changed
- **Analytics Glass Dialog**: Failed Checks 趋势弹窗改为液态玻璃质感外框，并收敛弹窗内容高度，避免底部操作区超出边界。

## 2026-04-24

### Added
- **Transient Failure Retry**: 测试报告列表、性能报告文件列表和执行报告读取接入一次自动重试机制，遇到 timeout、网络错误或 500/502/503/504 时会提示“网络波动，正在重新尝试...”。

### Changed
- **Request Reliability**: 报告读取类请求统一走可重试读取逻辑，降低 MySQL 偶发 timeout 导致页面空白的概率。

## 2026-03-30

### Added
- **AI Intelligence Summary**: 深度集成了 AI 报告分析面板。当测试报告具备 `analysisResult` 时，会在详情弹窗下方自动渲染由 AI 生成的性能指引与优化建议。
- **Lighthouse File Verification**: 增强了 Web 性能分析报告的显示逻辑。系统现在会实时校验后端 `report` 目录下是否存在对应的 HTML 文件，仅显示真实有效的报告记录。

## 2026-03-24


### Added
- **AI-Powered Report Assistant**: 实现了基于 AI 简写（如 `swa, 2.58.0`）自动填充复杂报告模版的功能。
- **Acceptance Data Migration**: 成功从旧版 MySQL 数据库迁移了 50 份历史验收报告及 20 台测试机数据。
- **Linked Data Models**: 建立了报告、项目代码与测试负责人之间的底层数据关联映射。

## 2026-03-23

### Added
- **Acceptance Report Center**: 新增“验收报告”核心模块，采用原生 Vue 3 高效重构，取代原始 Mock 页面。
- **Multi-Dimension Metrics**: 实现了报告总数、今日新增及项目覆盖率等多维度的可视化看板。
- **Persistence Foundation**: 接入了 JSONL 后端存储，支持报告的实时保存与历史追溯。

## 2026-03-13

### Added
- **Route Injection**: 注册了 `/reports` Vue 路由节点并在 `MainLayout` 左侧栏成功暴露出导航入口。
- **TestReports List View**: 新建了 `src/views/TestReports/index.vue`。
  - **Dynamic Filtering**: 在顶栏实现了基于 Element-Plus `el-radio-group` 的扁平化状态切换组件，可按“接口类型/压测类”实现快速筛选。
  - **Fuzzy Search**: 实现了针对测试报告 ID 与名称的实时前端模糊检索功能 (`computed` 衍算)。
  - **Modern Table Data**: 利用 `el-table` 包装基础卡片展示，使用定制化的 SVG / Icon 构建了美观的文件列表列。
  - **Viewer Modal**: 新增了全屏级弹窗与 `<iframe class="report-iframe" />` 的插槽整合，当测试类型为“压测”且具备 `reportUrl` 时，可一键读取 Go 后台存储好的 HTML 色彩大图报表。

### Changed
- **Pinia Persistence**: 移除了原来写死在组件里的硬编码 `mockReports`，接入了全局的 `useReportStore` (`src/stores/modules/reports.ts`)，结合 `localStorage` 配合 WebSocket 生命周期实现了用户真实运行历史的自动化保存与多页签同步展示。
