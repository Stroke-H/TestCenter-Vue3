# ChangeLog - Dashboard

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
