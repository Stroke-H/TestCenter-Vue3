# ChangeLog - CoreFramework

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
