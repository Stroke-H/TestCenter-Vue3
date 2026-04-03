# ChangeLog: 飞书 Wiki 集成与自动验收报告联动 (2026-04-03)

## 1. 自动同步逻辑 (Automation)
- **[NEW] 报告自动同步**: 更新 `generate_acceptance_report` 工具，支持在生成报告后，自动将其同步至项目关联的飞书文档中。
- **[NEW] 智能格式化**: 
    - 自动根据版本号生成 H2 标题（如 `V2.58.0`）。
    - 自动剔除文档中的冗余信息（如项目名称、报告标题行），保持 Wiki 内容精简。
- **[OPTIMIZE] 并行处理**: 执行同步操作时采用 Goroutine 异步处理，确保助手回复速度不受网络波动影响。

## 2. 基础能力增强 (Foundation)
- **[NEW] 多块写入支持**: 在 `feishu_client.go` 中新增 `AddBlocksToDocx` 方法，支持单次 API 调用写入多个 Docx 块（H2 + 正文）。
- **[NEW] 项目模型扩展**: 在 `Project` 模型中新增 `WikiURL` 字段，支持为每个项目独立配置文档关联。

## 3. 系统设置优化 (UI/UX)
- **[NEW] 项目文档关联配置**: 
    - 在“项目代码”设置中增加“项目文档关联”列表显示及编辑功能。
    - 在新增/编辑项目弹窗中增加文档链接输入框（非必选项）。
- **[OPTIMIZE] 快速跳转**: 在项目列表中可直接点击链接图标跳转至对应的飞书 Wiki 或文档。

---
*Status: Verified & Deployed*

[Previous entries preserved below...]

# ChangeLog: 飞书 Wiki 集成与智能助手体验优化 (2026-03-25)
...
