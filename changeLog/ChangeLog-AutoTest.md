# ChangeLog-AutoTest

## 模块定位
UI 自动化模块（AutoTestCenter）是当前项目中的可视化 Playwright 自动化工作台，覆盖：
- 用例仓库
- 三栏式用例编辑器
- 关键字驱动步骤编排
- WebSocket 执行控制台
- Inspector 可视化录制

核心文件：
- `src/views/UIAutoTest/index.vue`
- `src/views/UIAutoTest/Editor.vue`
- `src/views/UIAutoTest/Runner.vue`
- `src/stores/modules/playwright.ts`
- `server/services/playwright_service.go`
- `server/services/playwright_keywords.go`
- `server/services/playwright_ws.go`
- `server/services/inspector_ws.go`

## 当前进度
- 阶段 1 基础骨架：已完成
- 阶段 2 用例编辑器：已完成
- 阶段 3 执行引擎：已完成
- 阶段 4 执行报告：未完成
- 阶段 5 Inspector：已完成基础能力并持续细化体验

## 当前已实现能力
- 支持测试套件与测试用例的 JSONL 持久化 CRUD。
- 支持关键字驱动的步骤编排，当前内置关键字包括 `Launch`、`Goto`、`Click`、`Fill`、`Press`、`TextContent`、`WaitForSelector`、`Screenshot`、`Sleep`、`AssertURL`、`AssertText`、`Close`。
- 支持用例编辑器内的步骤拖拽排序、属性编辑、变量引用。
- 支持 Runner 页面通过 WebSocket 执行用例，并实时展示步骤状态、日志、截图。
- 支持 Inspector 通过 Playwright 打开真实浏览器并进行可视化录制。

## Inspector 当前能力
- 开启录制时自动补齐 `Launch` 与 `Goto` 基础步骤。
- 点击页面元素时自动追加 `Click` 步骤。
- 在输入框中输入内容时自动追加 `Fill` 步骤。
- 对输入框按 `Enter` / `Tab` / `Escape` 时自动追加 `Press` 步骤。
- 优先录制稳定 selector，减少绝对 DOM 路径带来的脆弱性。
- 点击导航类元素时不再阻断原页面行为，录制过程中页面可以正常跳转。

## 本轮修复与增强

### 2026-04-02
- 修复 Inspector 点击录制后不新增 step 卡片的问题。之前如果存在选中步骤，只会更新已有 `selector`，用户会误以为没有录制成功。
- 修复 Inspector 对页面原始点击行为的干扰，移除默认 `preventDefault/stopPropagation`，避免百度贴吧、Google 搜索等点击后页面不跳转。
- 新增输入录制，输入框内容变化会生成 `Fill` 步骤。
- 新增键盘录制与 `Press` 关键字，支持录制 `Enter` 等关键按键。
- 优化 selector 生成逻辑，优先使用稳定属性定位，降低动态页面点击执行超时的概率。
- 修复 Acceptance Report 页面统计卡图标响应式 warning，减少控制台噪音。

### 2026-04-28
- 将 UI 自动化编辑器和 Playwright Store 的接口入口统一切换为同源 `/api/playwright`，修复在 `strokeh.local` 等代理域名访问下直连 `localhost:8080` 造成的数据读取失败风险。

## 当前已知限制
- Inspector 目前仍以点击、输入、基础按键为主，尚未覆盖下拉选择、复选框、单选框、拖拽、悬停等更复杂交互。
- 对于极度动态的站点，虽然 selector 已优化，但仍可能需要人工二次修正。
- 执行报告模块尚未独立沉淀为 AutoTestCenter 的历史报告中心，目前主要依赖 Runner 实时态展示。

## 下一步建议
- 增加 `SelectOption`、checkbox、radio、hover 的录制支持。
- 在录制点击前自动评估是否插入 `WaitForSelector`，提升脚本稳定性。
- 建立 AutoTest 专属执行报告存储与查询页面，补齐 Phase 4。
- 为 Inspector 增加“选择器拾取模式”和“连续录制模式”区分，减少误操作。
