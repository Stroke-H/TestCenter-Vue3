# ChangeLog - Test Reports

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
