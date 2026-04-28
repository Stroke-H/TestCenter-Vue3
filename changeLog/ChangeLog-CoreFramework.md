# ChangeLog - CoreFramework

## 2026-04-28

### Changed
- **Unified Frontend API Routing**: 前端多处写死 `http://localhost:8080/api` 的调用改为统一走同源 `/api` 代理，修复通过 `strokeh.local:5173` 等非 localhost 域名访问时的数据加载失败问题。
- **Auth-ready Page Fetching**: 验收报告与飞书助手在页面初始化时会优先等待当前登录态完成恢复，再请求受保护数据，减少登录态初始化竞态导致的空白或误报。

## 2026-04-24

### Added
- **SQL Session Login**: 登录成功后新增 SQL-backed `user_sessions` 会话记录，并通过 HttpOnly Cookie 保存浏览器登录态，默认有效期一个月。
- **Four-state Auth Model**: 前端登录态从简单的 token 存在判断升级为 `checking`、`authenticated`、`anonymous`、`unreachable` 四态，区分登录失效与后端临时不可达。
- **Logout Session Revocation**: 新增 `/api/auth/logout`，退出登录时会撤销当前服务端 session 并清理 Cookie。

### Changed
- **Auth Persistence**: 主登录链路不再依赖前端可读的 `localStorage.token` 作为长期凭证，降低后端重启或网络抖动导致误登出的概率。
- **Compatibility Layer**: 后端鉴权保留旧 `Authorization` 用户 ID 的兼容兜底，避免一次性影响历史接口和 WebSocket 场景。

## 2026-03-24

### Added
- **Premium Account System**: 实现了完整的用户注册、登录及登出流程。
- **Glassmorphism Auth UI**: 采用玻璃拟态设计风格构建了登录与注册页面，增强了视觉的高级感。
- **Pinia Auth Persistence**: 集成了权限状态管理，支持 Token 持久化存储与自动注入请求头。
- **JSONL Persistence Layer**: 实现了基于 JSON Lines 的轻量级、高性能数据持久化层，支持并发安全的读写操作。
- **Legacy Password Support**: 实现了对 Werkzeug (PBKDF2-SHA256/Scrypt) 加密格式的原生校验支持。
- **Unified Identifier Login**: 支持通过用户名或邮箱进行统一登录身份识别。

## 2026-03-23

### Added
- **Feishu Bot Bridge (Go)**: 实现了独立的飞书机器人网关模块，基于官方 SDK 提供稳定消息收集。
- **WebSocket Connection**: 引入了 WebSocket 长连接模式，支持在无公网 IP 环境下直接与飞书服务进行联调。
- **Standardized Message Model**: 建立了 AI 全链路通用的飞书消息标准化数据模型。

## 2026-03-12

### Added
- **Repository Setup**: 使用 `create-vite` 搭建了 Vue 3 + TypeScript 骨架工程。
- **Dependencies Integration**: 引入核心路由库 Vue Router 4，状态管理库 Pinia，HTTP 客户端 Axios 以及 UI 框架 Element Plus。
- **Standard Project Structure**: 规划了 `/api`/`/components`/`/layouts`/`/router`/`/stores`/`/types`/`/utils`/`/views`/`/assets` 等企业级标准的文件与业务层级。
- **Axios Encapsulation**: 抽象了基础请求头、拦截器与全局 Error Handler 的 `request.ts`。

### Changed
- **Main Layout**: 初始化一个包含侧滑菜单、面包屑与 `<router-view>` 淡入淡出切页动态的 `MainLayout.vue` 主布局框架。
- **Global Theme Override**: 覆写 `element-plus` 的 Primary (Blue)/Success/Danger 变量，确立了系统的浅色设计语言基础。

## 2026-03-13

### Added
- **Unified Startup Script**: 新增 `./start.sh`，支持一键自检环境（Node、Go、K6）缺失，并并发拉起 Vue 前端与 Go 后端子进程，优化了本地微服务启动体验。
