# ChangeLog - Backend (TestCenter Server)

## 2026-04-13

### Changed
- **TestCase Generation Pipeline**:
    - 调整需求拆解与智能增强策略，默认聚焦 App 核心功能与必要边界，减少泛化的兼容性/性能类需求点扩写。
    - 更新测试用例生成 Prompt，取消对每个功能点强制四维全覆盖的要求，改为按风险选择测试维度并限制单点产出数量。
    - 新增测试用例 `Category` 分类归并逻辑，并在导出 Excel 时同步输出分类列。
    - 增加测试用例标题/步骤/预期结果归一化去重，降低冗余用例入库比例。
- **TestCase Record APIs**:
    - `GenerationRecord` 扩展 `project_code` 与 `module` 字段。
    - `main.go` 新增 `PUT /api/testcase-gen/records/:id` 路由，支持更新历史用例记录。
- **Feishu Bot Result Delivery**:
    - 对 `generate_acceptance_report` 与 `generate_smart_test_cases_from_doc` 增加工具结果直通逻辑，避免工具返回文本被 AI 二次压缩。
    - 修复飞书测试用例生成结果中的预览/下载链接丢失问题。

## 2026-03-31

### Added
- **AI Analyzer Service**: 
    - 集成 GPT-4o 端点，自动化分析 Lighthouse 生成的 JSON 报告。
    - **Summarization Engine**: 实现了性能分扣分项提取、关键路径优化建议及长文档摘要生成。
- **Performance Execution Pipeline**: 
    - **Lighthouse Runner**: 封装了 `npx lighthouse` 进程调用，支持生成交互式 HTML 报告。
    - **K6 Stress Runner**: 封装了 K6 执行引擎，输出结构化压测指标至前端。
- **Acceptance Logic Bridge**: 
    - 实现了报告保存、列表获取与飞书推送的闭环 API 路由。

## 2026-03-24

### Added
- **Feishu Integrated Client**: 
    - **Token Handler**: 实现了 `tenant_access_token` 的定时自动续期逻辑。
    - **Docx Block Operator**: 完善了飞书文档块级增量插入与正文内容提取。
- **JSONL Persistence Layer**: 
    - 基于 Go 实现的 `Line-based JSON` 持久化引擎。
    - 提供了并发安全的 `RwLock` 保证，适配高频读写的验收报告存储需求。
- **Legacy Auth Support**: 
    - 集成了 PBKDF 校验算法，完美支持历史阶段导入的加密用户密码。

## 2026-03-23

### Added
- **WebSocket Bridge**: 
    - 实现了基于 Gin + Gorilla WebSocket 的全双工通信总线。
    - **JungleChess Logic**: 建立了房间号匹配与状态广播机制。
- **API Proxy Manager**: 
    - 实现了匿名跨域请求转发，支持全量 Header 透传与 Cookie 模拟。

### Changed
- **Error Handling**: 统一了 `controller` 层的 API 返回包格式 (`{code, data, msg}`)，增强了前端错误处理的鲁棒性。
- **Startup self-check**: `main.go` 启动时新增了环境变量及第三方二进制包（k6/lighthouse）的存在性校验。
