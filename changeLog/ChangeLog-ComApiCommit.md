# ChangeLog - ComApiCommit

## 2026-03-13

### Added
- **Base View Construction**: 新增基础功能承接页面 `/com_api_commit`，使用左右两栏布局展示。
- **Action Control State Machine**: 添加底部状态栏，支持模拟启动计时、日志刷新呈现与终止执行 (`Stop`/`Execute`)。
- **Dynamic Data Binding**: 通过获取上游路由通过 Query 携带的关键入参（Name 和 Description）绑定到模板上实现定制化的内容渲染。
- **Log Simulation Engine**: 开发了一套计时递增轮询机制，利用数组结构驱动右侧类控制台界面的 Mock 假日志追加流。
- **Bugfix (Close Button)**: 修复了右上角原本缺乏连线的“关闭” (X) 图标，现在点击该按钮可中断执行并正确 `router.push('/')` 返回仪表盘首页。
