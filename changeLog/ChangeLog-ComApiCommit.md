# ChangeLog - ComApiCommit

## 2026-03-30

### Added
- **Web前端压测专项适配**: 
    - 针对 Web 压测场景，实现了“测试链接”输入框的可编辑模式，支持用户自定义目标 URL。
    - 精简了 UI 界面，隐藏了不相关的“测试服务器”和“项目”选择器，使流程更聚焦。
    - 实现了前端 URL 基础特征校验逻辑。
    - 深度集成了 Lighthouse 引擎（通过 WebSocket 实时推送日志），并在执行完成后调用 **AI 分析引擎** 产出性能指引。
- **Interactive Report Integration**: 在控制台任务完成后，自动通过 Iframe 加载后端的 HTML 报告，并支持 AI 智能总结的展示。

## 2026-03-19

### Added
- **账号注销辅助工具 (Delete Account Tool)**:
    - **Anonymous Login Proxy**: 实现了账号删除工具的匿名登录代理，支持全量 Header 透传。
    - **Contextual Matching**: 实现了对传入参数中 `app` 标识的正则表达式解析，支持自动切换项目环境（ShortsWave / NovelNova）。
    - **Security Confirmation**: 设计并实现了高端的二次确认弹窗，高亮显示解析出的 `user_id` 和 `user_name`。
    - **Real-time Log Execution**: 登录后自动获取 `session_token` 并流式展示注销请求过程。

## 2026-03-13

### Added
- **Base View Construction**: 新增基础功能承接页面 `/com_api_commit`，使用左右两栏布局展示。
- **Action Control State Machine**: 添加底部状态栏，支持模拟启动计时、日志刷新呈现与终止执行 (`Stop`/`Execute`)。
- **Dynamic Data Binding**: 通过获取上游路由通过 Query 携带的关键入参（Name 和 Description）绑定到模板上实现定制化的内容渲染。
- **Log Simulation Engine**: 开发了一套计时递增轮询机制，利用数组结构驱动右侧类控制台界面的 Mock 假日志追加流。
- **Bugfix (Close Button)**: 修复了右上角原本缺乏连线的“关闭” (X) 图标，现在点击该按钮可中断执行并正确 `router.push('/')` 返回仪表盘首页。
