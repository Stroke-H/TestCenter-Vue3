# TestCenter (Vue 3 + Go Hub)

🚀 **TestCenter** 是一个集成了 **飞书自动化**、**AI 智能分析** 与 **多维自动化测试** 的轻量级一体化测试工具平台。它不仅是一个前端展示站，更是一个连接飞书生态与后端测试引擎的智能枢纽。

---

## 🌟 核心功能模块

### 1. 🤖 飞书助手与智能机器人 (Feishu Bot & Assistant)

- **验收报告自动化**: 支持通过飞书机器人指令 (`swi`, `swa`, `version`) 自动拉取需求与缺陷数据并生成标准化验收报告。
- **Wiki 文档同步**: 自动将生成的报告同步至飞书 Wiki 知识库，支持格式清理与归档。
- **MQL 精准过滤**: 内置平台级 MQL 筛选机制，自动识别 iOS/Android 需求分类，杜绝数据越权。
- **实时监控面板**: 提供 Feishu Assistant 仪表盘，支持实时会话监控、AI 操作审计日志 (Audit Log)。

### 2. 🎮 UI 自动化测试引擎 (Ninja - Playwright)

- **关键字驱动**: 支持零代码、关键字配置化的测试用例编写。
- **实时执行器**: 基于 WebSocket 的实时执行引擎，支持日志流实时回传与状态监控。
- **智能探测器**: 内置 Inspector 工具，支持元素一键扫描与功能“技能化 (Skillification)”。

### 3. 📊 性能审计与 AI 报告解析

- **API 性能压测**: 集成 [k6](https://k6.io/)，支持剧集播放链路等复杂业务流程的并发测试。
- **网页性能审计**: 集成 Lighthouse，自动化评估页面加载性能。
- **AI 智能建议**: 通过 AI 对压测结果进行深度解读，识别 P95 延迟瓶颈并给出优化建议。

### 4. 📝 AI 需求工程与用例生成

- **智能拆解**: 输入业务需求，AI 自动进行原子化拆解。
- **用例步进**: 自动化生成多维度的测试用例（正常路径、边界值、异常流）。
- **导出共享**: 支持一键导出到 Excel 等标准化文档。

---

## 🛠 技术架构

- **前端**: Vue 3 (Composition API), Pinia, Element Plus, Vite, TypeScript.
- **后端**: Go (Gin Framework), WebSocket (Real-time Streaming).
- **AI**: DeepSeek/OpenAI 兼容协议接入, 专用会话管理系统.
- **数据层**: JSONL 轻量化持久化方案, 预留 MySQL 连接池与健康检查框架，支持后续平滑迁移。

---

## 📂 目录结构

```text
TestCenter_Vue3/
├── server/               # Go 后端引擎中心
│   ├── feishu/           # 飞书机器人、MCP 桥接及 AI 脑干
│   ├── services/         # 核心业务逻辑 (Auth, Report, K6, Playwright)
│   └── data/             # 持久化数据与配置文件 (.jsonl, .json)
├── src/                  # Vue 3 前端核心
│   ├── api/              # 请求封装 (Axios 拦截器)
│   ├── views/            # 业务页面 (Dashboard, FeishuAssistant, Playwright...)
│   └── stores/           # 状态管理
├── k6-scripts/           # K6 性能测试脚本
├── report/               # Lighthouse 报告产出目录
└── start.sh              # 一键拉起前后台服务的启动脚本
```

---

## 🚀 快速启动

1. **环境准备**: 确保已安装 Node.js (v18+), Go (v1.21+), 以及 K6 环境。
2. **配置固定访问入口**  
   如果你希望其他设备稳定访问服务，应该固定一个入口地址，例如 `www.inspdance.com` 或 `testcenter.lan`，并让使用者始终打开同一个地址：
   ```bash
   cp .env.lan.example .env.lan.local
   ```
   然后把其中的 `TESTCENTER_LAN_HOST`、`TESTCENTER_FRONTEND_ORIGIN`、`TESTCENTER_BACKEND_ORIGIN` 统一改成这个固定入口。IP 地址只作为排查备用，不作为长期分享入口。
3. **启动服务**:
  ```bash
   ./start.sh
  ```
   该脚本会自动并行拉起 5173 (Vue) 和 8080 (Go) 两个核心服务，并优先使用 `.env.lan.local` 中配置的固定入口；未配置时默认使用 `www.inspdance.com`。

### MySQL 连接配置

后端已预留统一的 MySQL 接入框架，默认连接写在 `server/data/database_config.json`，环境变量可覆盖默认值。

启动后可通过 `GET /api/database/config` 查看脱敏配置，通过 `GET /api/database/health` 检查连接池和数据库连通性。
JSONL 到 MySQL 的一次性迁移已完成，历史迁移脚本已归档到 `平台功能测试文件管理/mysql-jsonl-migration/`。

### Windows 迁移

从 Mac 迁移到 Windows 时，请先拉取 `windows` 分支，再按 [Windows 迁移部署说明](docs/windows-migration.md) 还原私有配置包并启动服务。
Windows 端 Codex 接手时先读 [WINDOWS_CODEX_README.md](WINDOWS_CODEX_README.md)。

---

## 📜 变更记录

所有的重大功能演进均记录在 `changeLog/` 目录下，建议在提交 PR 前同步更新对应的模块日志。
