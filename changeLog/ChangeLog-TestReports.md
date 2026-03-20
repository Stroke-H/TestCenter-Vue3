# ChangeLog - Test Reports

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
