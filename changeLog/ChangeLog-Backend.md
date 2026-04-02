# ChangeLog - Backend (TestCenter Server)

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
