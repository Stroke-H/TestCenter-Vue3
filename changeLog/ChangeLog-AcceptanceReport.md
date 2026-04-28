# ChangeLog - Acceptance Report

## 2026-04-28

### Added
- **Cloud Document Sync**: 验收报告详情新增“同步到云文档”按钮，显示条件与“发送到飞书”保持 Reporter 归属一致，并额外要求报告项目已在“项目代码”配置中绑定项目文档。
- **Protected Sync API**: 新增验收报告云文档同步接口，后端会二次校验报告归属和项目文档配置后，将报告内容写入对应飞书 docx 文档。
- **Auto Project Item Prefill**: 新建验收报告时，选择项目并填写版本号后会自动通过飞书项目 MCP 拉取对应版本的需求和缺陷链接，非关闭态缺陷填入“缺陷提交情况”，关闭态缺陷填入“缺陷修复情况”。

### Changed
- **Page Title Localization**: 验收报告页面标题、面包屑和新建入口文案统一调整为中文，和平台其他业务页保持一致。
- **Proxy-safe Data Loading**: 验收报告页改为统一走同源 `/api` 代理访问受保护接口，修复通过 `strokeh.local` 访问时因直连 `localhost:8080` 导致的列表加载失败问题。

## 2026-04-24

### Added
- **Retryable Data Loading**: 验收报告列表、项目配置、设备配置读取接入一次自动重试机制，数据库或网络短暂 timeout 时会弹出重试提示并自动重新请求。

### Changed
- **Cookie Session Auth**: 验收报告保存、列表读取和飞书推送支持新的 HttpOnly Cookie 登录态，同时保留历史 `Authorization` 兼容。

## 2026-03-24

### Added
- **Acceptance Dashboard**: 实现了验收报告概览看板，聚合显示报告总量、测试人数及覆盖项目数。
- **Interactive Report List**: 
    - 采用 `el-table` 构建了列表页，支持针对“测试分类”的多维筛选。
    - 实现了前端关键字实时搜索功能。
- **Dual-Mode Preview System**:
    - **View Mode**: 支持查看报告详情，包括测试范围、环境设备及结论。
    - **Create Mode**: 实现了全新的报告录入界面，支持联动选择“项目代码”并自动补全“设备白名单”。
- **Feishu Bot Integration**: 
    - 接入了 `send-feishu` 接口，支持将确认无误的验收报告一键推送至飞书沟通群。
    - 实现了身份保护逻辑：仅报告人本人可执行推送操作。
- **Configuration Synchronization**: 
    - 动态从后端 `config/projects` 与 `config/devices` 获取基础元数据。
    - 实现了项目代码与名称的联动自动补全。

### Changed
- **UI Consistency**: 采用现代化的卡片式布局与图标体系（Element Plus Icons），对标 Dashboard 设计语言。
- **Grid Layout**: 优化了统计卡片与主列表的响应式 `el-row` 布局，适配不同分辨率桌面端。
