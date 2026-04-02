# ChangeLog: 飞书 Wiki 集成与智能助手体验优化 (2026-03-25)

## 1. 飞书 Wiki & 文档交互能力 (Core Feature)
- **[NEW] Feishu API Client**: 封装了 `feishu_client.go`，支持 `tenant_access_token` 自动维护及 Docx 块级增量操作。
- **[NEW] 文档读写工具**:
    - `write_to_feishu_wiki`: 支持解析 Wiki Node Token 并将内容增量插入文档最上方。
    - `read_feishu_wiki`: 支持读取文档正文内容，供 AI 决策使用。
- **[FIX] 消息拦截器重构**: 移除了 `event_service.go` 中对 Wiki 链接的硬编码拦截，将控制权完全交还 AI 意图引擎。
- **[OPTIMIZE] 意图识别增强**: 重构 `session_manager.go` 中的 System Prompt，明确区分“写意图”与“读意图”的工具调用触发点。

## 2. 智能助手界面交互 (Web UI)
- **[OPTIMIZE] 输入框自适应**: 给助手输入框增加了 `autosize` 支持，可随输入内容行数自动增高。
- **[OPTIMIZE] 气泡换行显示**: 优化了聊天气泡的 CSS，支持长链接、长日志的自动换行，避免 UI 撑破。
- **[FIX] 身份识别对齐**: 修复了 Web 端助手无法识别已登录用户身份的 Bug，改为直接通过平台 ID 进行免绑定校验。

## 3. 相关模块联动 (Module Synergy)
- **验收报告联动**: 飞书端生成的报告逻辑已迁移并整合至专有的 [AcceptanceReport](file:///Users/apple/TestCenter_Vue3/changeLog/ChangeLog-AcceptanceReport.md) 记录中，确保存储结构的隔离与一致性。

---
*Status: Verified & Ready for Deployment*
