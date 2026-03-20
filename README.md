# TestCenter (Vue 3 前端)

这是一个基于 Vue 3 + Vite + TypeScript 搭建的轻量级测试工具平台前端项目。
项目采用统一的浅色工作台风格开发，内置了路由控制、状态持久化缓存以及 Mock 测试执行机制。

## 项目依赖
- **框架**: Vue 3 (Composition API, `<script setup>`)
- **构建工具**: Vite
- **语言**: TypeScript
- **路由**: Vue Router 4
- **状态管理**: Pinia
- **UI 组件库**: Element Plus
- **HTTP 客户端**: Axios
- **压测引擎**: [k6](https://k6.io/) (环境依赖, 支持 JS 脚本)

## 目录结构说明

```text
TestCenter_Vue3/
├── api/                  # HTTP 请求封装层
│   └── request.ts        # Axios 实例、拦截器及基础配置
├── k6-scripts/           # K6 API 压测脚本目录
│   └── episode.js        # 剧集播放链路压测范例脚本
├── assets/               # 静态资源
│   └── styles/           # 全局样式
│       └── index.css     # 浅色主题配置，CSS 变量定义
├── changeLog/            # 变更日志模块目录 (要求在每次重大修改后主动更新)
│   ├── ChangeLog-ComApiCommit.md
│   ├── ChangeLog-CoreFramework.md
│   └── ChangeLog-Dashboard.md
├── components/           # 全局公共组件
├── layouts/              # 页面级布局组件
│   └── MainLayout.vue    # 应用骨架 (侧边栏菜单 + 顶栏导航)
├── router/               # 路由表配置
│   └── index.ts          # createWebHistory 路由管理
├── stores/               # 全局状态管理 (Pinia)
│   ├── app.ts            # 全局 UI 状态 (如侧滑菜单折叠状态)
│   └── index.ts          # Pinia 入口
├── types/                # TS 类型定义声明
│   └── index.ts          # 基础请求返回类型定义
├── utils/                # 纯函数辅助工具库
│   └── index.ts          # common 工具 (如 formatDate 等)
├── views/                # 主要视图路由组件
│   ├── ComApiCommit/     # 通用 API 测试执行落地页
│   │   └── index.vue     # Mock 执行页面，左右分栏，吸底操作区
│   └── Dashboard/        # 平台启动台首页
│       └── index.vue     # 工具启动面板 (包含最近使用缓存、API Tools 等分类)
├── App.vue               # 根实例挂载点
├── main.ts               # 项目启动入口文件，全局注册插件
├── index.html            # Vite 注入模板
├── package.json          # npm 依赖描述文件
├── tsconfig.json         # TS 主配置文件
├── tsconfig.app.json     # 业务代码 TS 配置
├── tsconfig.node.json    # 构建脚本 TS 配置
└── vite.config.ts        # Vite 打包及 dev server 配置 (Path Alias: @)
```

## 开发约定
- **架构**: MVVM 架构，组件化开发。
- **命名**: 变量使用 `camelCase`，组件名及大模块使用 `PascalCase`。
- **文档**: 所有重要变更记录在 `changeLog` 对应模块的 `.md` 中。
- **组件抽取**: 超过 300 行的业务组件须考虑拆分为复用的 sub-component。
- **样式**: 本地样式必须带上 `scoped`。全局覆盖（如覆写 element-plus）必须集中在 `assets/styles/index.css` 或通过 `:deep()` 控制级联范围。

## 运行与启动
我们提供了一个自动检测 Node、Go 与 K6 环境依赖，并自动拉起前端 Vue 和后端引擎的命令程序。

要在本地同时启动前后台双子星应用，运行：
```bash
./start.sh
```

*(或者你也可以分别手动执行 `npm run dev` 以及在 `server` 目录下执行 `go run main.go`*)*