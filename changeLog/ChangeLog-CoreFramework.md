# ChangeLog - CoreFramework

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
