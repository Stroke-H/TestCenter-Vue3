# ChangeLog - Dashboard

## 2026-05-27

### Added
- **Acceptance Project Tree Entry**: 仪表盘“测试流程工具”分区新增“验收项目树”入口，用于快速进入验收报告项目树视图。

## 2026-05-26

### Added
- **Monkey Hologram Demo Entry**: 仪表盘“性能测试”分区新增“Monkey测试”入口，进入执行页后可配置包名、设备、事件数、throttle、截图间隔和 seed，模拟 `adb shell monkey`、`logcat` 与 `screencap` 管线，并在执行完成后生成可点击预览截图的原子结构全息图谱。

### Changed
- **Monkey 3D Space Model**: Monkey 图谱从 CSS 伪 3D 升级为 Three.js WebGL 空间模型，支持拖拽旋转、节点点击拾取、风险节点发光、中心 App 标识、球面节点分布、标签缩放与截图预览联动。

## 2026-04-28

### Changed
- **TestCase Card Routing**: 仪表盘“测试用例生成”卡片改为直接跳转到“智能测试用例生成”页面，不再默认进入历史记录页。

## 2026-04-24

### Added
- **Video Player Entry**: 在仪表盘“其他拓展”分区新增视频播放器入口，支持进入视频播放页面与摸鱼浮窗模式。
- **Permission-aware Cards**: 仪表盘卡片接入权限配置，API 工具、流程验证、测试账号管理、UI 自动化、用例生成、节点 Skill 化、性能测试、斗兽棋、小说阅读器、视频播放器等入口会按用户权限显示或隐藏。

### Changed
- **Sidebar Cleanup**: 视频播放器不再出现在左侧侧边栏，只保留仪表盘入口。
- **Report Center Menu**: 报告中心菜单与用例报告入口按权限配置统一控制。

## 2026-03-30

### Added
- **Web前端压测入口**: 在性能测试分区首位新增了“Web前端压测”工具卡片，支持快速发起 Lighthouse 性能审计。
- **Card Optimization**: 优化了性能测试分区的卡片顺序，将高频使用的 Web 压测工具调整至靠左第一个位置。

## 2026-03-24

### Added
- **Global AI Assistant v2**: 实现了常驻右下角的悬浮 AI 助手，支持全局唤起。
- **Interactive Report Editor**: 新增了全局弹窗式验收报告编辑器，支持在 AI 生成后进行实时编辑、确认与持久化存储。
- **UI Avoidance Logic**: 在编辑器打开时，悬浮助手会自动淡出缩放以避免视觉遮挡及交互冲突。

## 2026-03-23

### Added
- **Feishu Assistant Dashboard**: 将原“测试用例管理”重构为“飞书助手监控大盘”。
- **Session Cycles Tracker**: 实现了会话周期追踪表，聚合显示聊天频次、最后活跃时间及会话状态。
- **Interactive Chat Drawer**: 新增了会话详情抽屉，支持查看完整上下文历史及 AI 实时回复联调。
- **Bot Subsystem Overview**: 在大盘顶部增加了 Bot 状态看板（总用户数、活跃会话等）。

## 2026-03-19

### Added
- **Account Deletion Parameter**: 在“删除账号”工具页面新增了“填入参数”输入框，支持用户在执行操作前输入必要的参数。
- **Auto Project Matching**: 实现了参数自动解析功能，当输入包含 `app` 标识的参数串时，系统会自动匹配并切换“项目”字段（支持 NovelNova 和 ShortsWave）。
- **Anonymous Login Proxy**: 实现了账号删除工具的匿名登录功能。通过 Go 后端代理绕过 CORS，并支持全量 Header 透传，登录结果直接以格式化 JSON 形式在前端弹窗展示。
- **Interactive Deletion Flow**: 完善了注销账号的交互流程。登录后会提取并高亮显示 `user_id` 和 `user_name` 进行二次确认；确认后会自动使用最新的 `session_token` 完成注销请求，并将结果实时输出至日志区。
- **Multi-Env Support**: 支持了 ShortsWave 和 NovelNova 的多环境切换（测试服/正式服），系统会根据“测试服务器”下拉框自动映射对应的 IP 或域名。

## 2026-03-13

### Changed
- **UI Redesign**: 按照参照图要求，将深色主题仪表盘彻底重构为浅色工具启动台（Light-themed Tool Launcher），引入分类网格布局、互动卡片阴影动画和 Element Plus 主题定制。
- **Card Categories**: 将全量卡片重新组织为：`Recently Used`、`API Tools`、`UI Automation` 以及 `Performance`。
- **Responsive Layout**: 取消了 Dashboard 中最大宽度 1200px 的限制，现实现自适应宽度填满内容区。

### Added
- **Dynamic Recently Used Module**: 实现了在 `Recently Used` 下只动态显示最新执行过的最多四个功能卡片（初始不使用时隐藏该区块）。
- **LocalStorage State Caching**: 为 Recently Used 增加了浏览器本地缓存，功能使用顺序在刷新后能保持一致并提供 "Clear History" 清理功能。
- **Navigation Linking**: 所有的卡片动作按钮（Launch、Open 等）已经成功挂载全局路由 (`router.push`)，跳转至 `/com_api_commit` 页面，并传递 `name` 与 `desc`。
- **New API Tool Card**: 在 API Tools 分区首位插入了新的 `剧集播放接口测试` 工具卡片，用于作为后续 K6 性能测试页面的入口。
