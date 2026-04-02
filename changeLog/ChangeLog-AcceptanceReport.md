# ChangeLog - Acceptance Report

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
