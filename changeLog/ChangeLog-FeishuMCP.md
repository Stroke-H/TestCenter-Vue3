# ChangeLog: 飞书项目 MCP 集成 (In Progress)

## 进度追踪 (2026-04-10)

- **[NEW] 基础架构准备**: 
  - 在系统的核心配置文件 (`config.go`) 中添加了对 `MCPConfig` 的支持，使得系统可以通过环境变量读取 MCP 的 URL 和安全 Token。
  - 在系统启动流程 (`bootstrap.go`) 之中挂载了 `InitMCPClient()`，保证服务启动时 MCP Client 会并行拉取可用工具。
- **[NEW] MCP 适配客户端 (`mcp_bridge.go`)**: 
  - 完整封装了 `JSON-RPC 2.0` 标准的组装逻辑，支持发送 `tools/list` 与 `tools/call`。
  - 完成了自动化的工具桥接逻辑：读取 MCP Server 提供的工具列表后，自动将其 `InputSchema` (JSON) 反序列化为本系统的 `jsonschema.Definition`，注册至 AI 的工具箱 (`tool_registry.go`) 中。
- **[OPTIMIZE] AI 大脑提示词增强**:
  - 更新了全局 `System Prompt`，明确告知机器人现在已经打通了“飞书项目”，可以进行工作项的查询（如搜 Bug、看看板）和变更处理。

---
*当前处于第一阶段完成，正在等待服务器端测试与 Token 的配置部署。*
