# ChangeLog - Test Process

## 2026-05-28

### Changed
- **Project Tree Rename**: “验收项目树”统一更名为“项目树”。
- **Project Tree Live Refresh**: 项目树会在验收报告保存、跨页签报告变更、窗口重新聚焦或页面重新可见时自动刷新，确保新验收报告能及时出现在版本节点中。
- **Project Tree Timeline Layout**: 项目树版本节点改为左右交叉时间线展示，桌面端一左一右分布，移动端自动收敛为单列。
- **Project Tree Header Cleanup**: 项目树页面移除标题说明和项目/版本节点/验收记录统计卡片，仅保留返回、筛选、搜索和刷新操作。
- **Project Tree Version Sorting**: 项目树版本节点改为优先按版本号倒序排列，时间字段仅取测试时间范围的结束时间作为提审/报告时间兜底排序。
- **Project Tree Record Expansion**: 项目树记录行只显示测试结束时间，需求点内容默认折叠为两行，并支持通过上下角标展开查看完整内容。
- **Project Tree Smart Expansion**: 项目树需求点折叠按钮改为仅在内容超过两行时显示，并优化上下角标在按钮内的居中显示。
- **Project Tree Memo Bookmark**: 项目树新增按项目保存的“项目配置记录”备忘录，可记录项目回溯信息、环境注意事项和书签链接。
- **Project Memo Spacing**: 项目配置记录便利贴使用稳定的等距纵向排列，避免创建多条记录后因卡片倾斜造成视觉间距不一致。
- **Project Memo Empty State Cleanup**: 项目配置记录为空时仅保留标题栏和顶部新增按钮，不再展示重复的空状态提示与颜色按钮。

## 2026-05-27

### Added
- **Acceptance Project Tree**: 新增“验收项目树”页面，可按项目代码聚合验收报告，并以版本号生成节点展示测试时间与测试需求点。

### Changed
- **Project Tree Scope**: 验收项目树新增项目单选筛选，默认展示最近有报告的项目，页面内不再一次性铺开所有项目记录。
- **Project Tree Record Display**: 验收项目树节点记录改为纯信息展示，取消“查看”按钮和跳转行为。

## 2026-04-24

### Changed
- **Sandbox Account Type Filter**: 测试账号管理台移除“全部账号”tab，默认进入“沙盒账号”分类。
- **Project Filter Scope**: 项目筛选移除“全部”选项，只展示当前账号类型下实际存在账号的项目，并在切换账号类型或数据刷新后自动选中首个可用项目。

## 2026-03-31

### Added
- **MindMap Canvas**: 
    - 实现了基于 CSS Grid 与纯逻辑递归渲染的业务流程脑图界面。
    - **Interactivity**: 支持 0.5x 到 2.0x 的平滑手势/滚轮缩放。
    - **Panning**: 实现了基于鼠标拖拽的画布平移功能，确保大型业务流程的可视化操作。
- **Node Management**: 
    - 实现了节点的增删改查。
    - **Drag & Drop**: 支持节点层级的实时拖拽调整，并具备父子循环检测（Prevent Cyclic Movement）。
- **Status Control**: 
    - 实现了节点的状态机转换（完成/未开始）。
    - 提供了快速双击切换与展开菜单操作。
- **Layout Switcher**: 
    - 实现了在“水平（横向）”与“垂直（纵向）”生长模式间的无感切换。
- **Persistence Layer**: 
    - 对接后端 `api/processes` 接口实现流程的实时保存与按 ID 加载。

### Changed
- **Visual Design**: 
    - 采用简约高端的卡片样式，配合呼吸感边框（Hovering Glow）增强了操作反馈。
    - 背景层集成了点阵（Grid Dots）纹理，增强了工具感与空间维度。
- **Navigation Integration**: 
    - 注册了独立路由并集成了仓库（Repository）管理列表页。
