# ChangeLog - ComApiCommit

## 2026-05-27

### Added
- **Real Android Monkey Runs**: Monkey 测试从前端 Demo 升级为真实 Android 设备执行链路，支持 ADB 自动探测、设备刷新、真机 `adb shell monkey` 执行、logcat/monkey 日志采集、截图归档和真实截图 3D 节点渲染。
- **Monkey Target Package Selector**: Monkey 测试目标改为可搜索下拉栏，会读取当前连接 Android 设备上的三方 App 包名，避免手动输入包名出错。
- **Monkey Package Lookup Performance**: 三方 App 包名查询新增 5 分钟设备级缓存、8 秒超时和前端去重调用，降低重复 ADB 查询导致的等待时间。
- **Monkey Runtime Logs**: Monkey 执行期间前端会轮询展示 monkey/logcat 最新日志，不再只显示后端 GIN 请求记录。
- **Precision And Dense Sampling**: Monkey 测试新增高精度/低精度模式，高精度每 10 秒截图一次，低精度每 30 秒截图一次；异常加密采样默认开启，warning 按 5 秒/张持续 60 秒，critical 按 2 秒/张持续 120 秒。
- **Monkey Seed Compatibility**: Monkey 随机种子默认改为数字；非数字 seed 会在后端稳定转换为数字，避免 Android 原生 monkey 因 `Seed is not a number` 直接退出。
- **Monkey Screenshot Retention**: Monkey 截图归档新增保留策略，7 天后压缩截图，30 天后清理 normal 节点图片，仅保留异常节点图片和全部节点元数据。

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
