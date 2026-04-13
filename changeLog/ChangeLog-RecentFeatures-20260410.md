# ChangeLog: Recent Features Sync (2026-04)

## 2026-04-13 补充同步

### 测试用例生成收敛优化 (TestCase Generator)
- **[OPTIMIZE] 生成策略收敛**: 调整 `testcase_gen_service.go` 的需求拆解与用例生成 Prompt，默认聚焦 App 本身功能测试，不再机械扩展兼容性/无障碍/纯性能类低相关测试点。
- **[OPTIMIZE] 去除强制四维覆盖**: 取消“每个功能点必须 POSITIVE/NEGATIVE/EXCEPTION/CONCURRENCY 全覆盖”的硬约束，改为按风险选择维度，优先产出主流程与高价值功能用例。
- **[NEW] 用例分类字段**: 为测试用例新增 `category` 字段，支持 `常规功能测试 / 边界极限测试 / 异常容错测试 / 稳定性并发测试` 四类展示与导出。
- **[OPTIMIZE] 本地去重与收敛**: 后端新增标题/步骤/预期结果归一化去重逻辑，降低同义改写型重复用例落盘概率。

### 测试用例历史与管理增强
- **[NEW] 历史记录元信息**: `GenerationRecord` 增加 `project_code` 与 `module` 字段，用于后续分类归档与筛选。
- **[NEW] 历史记录更新接口**: 后端新增 `PUT /api/testcase-gen/records/:id`，支持前端在查看模式下更新已保存的测试用例记录。
- **[OPTIMIZE] 历史页体验**: `TestCaseGen/List.vue` 升级为统计卡片 + 项目/模块筛选 + 项目汇总侧栏的管理页布局。

### 飞书机器人结果返回修复
- **[FIX] 工具结果直通**: 对 `generate_acceptance_report` 与 `generate_smart_test_cases_from_doc` 增加结果直通逻辑，避免工具返回的完整文本、URL 或 Markdown 链接被 AI 二次压缩后丢失。
- **[FIX] 测试用例生成链接保真**: 修复“在线预览 / 下载 Excel”在飞书回复中退化成普通文本的问题，确保最终消息保留原始可点击链接。

## 1. 飞书自动测试用例生成 (TestCase Generator)
- **[NEW] 后端服务**: 新增 `testcase_gen_service.go` 和 `testcase_gen.go` 数据模型，支持基于 AI 的测试需求分解与用例多维生成。
- **[NEW] 前端界面**: 添加 `TestCaseGen` 视图模块，配合飞书机器人实现多步生成进度实时反馈，并输出汇总测试用例表格。

## 2. 自动化测试执行引擎与控制台 (UI Automation Console)
- **[NEW] 实时执行控制台**: 新增 `Runner.vue`，实现基于 WebSocket 的用例实时执行、日志流式输出和截图在线预览。
- **[NEW] 后端 Playwright 引擎集成**: 更新 `playwright_service.go` 和 `playwright_models.go`，建立前后端关键字与自动化执行器的实时双向通讯。
- **[OPTIMIZE] 用例编辑器优化**: 完善 `KeywordPanel.vue` 和 `Editor.vue`，支持复杂关键字集和用例可视化编排。修复 `Dashboard/index.vue` 模板语法错误。

## 3. Node Skillify 节点自动化能力 (Skillify Tool)
- **[NEW] 核心管理功能**: 新增 `SkillifyTool` 前端模块与后端的 `skillify_controller.go`, `skillify_service.go`。
- **[NEW] 流程整合**: 实现了从 `UI AutoTest` 到 `Skillify` 的无缝流转，包括“扫描 -> 技能化 -> 存储 -> 关联”闭环工作流。
- **[FIX] API 与运行时修复**: 解决了脚本标签缺失、API 404 等严重错误，保障执行链条稳定性。

## 4. 丛林棋功能增强 (Jungle Chess)
- **[NEW] AI 对战模式**: 引入 `useRobotAI.ts` 及其配套逻辑，实现多难度级别的智能机器人对战与状态联动。
- **[FIX] 连线与 UI 修复**: 
  - 修复撮合阶段因竞态条件导致幽灵弹窗的问题（优化 WebSocket 清理机制）。
  - 修复翻开的棋子在选中和移动时样式渲染异常（CSS Transforms 分离优化）。
  - 更新对局 UI，提供“思考中”等视觉反馈机制。

## 5. Web 压力测试分析 (K6 Web Stress Test)
- **[NEW] AI 智能分析**: 为 Lighthouse JSON 报告生成 AI 摘要。优化展示流程，将分析结果持久化至执行记录并在 Report Hall 中提供专门分析面板。

---
*Status: Synchronized & Ready for deployment.*
