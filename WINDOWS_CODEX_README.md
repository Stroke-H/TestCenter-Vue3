# Windows Codex 接手说明

这份文件给 Windows 电脑上的 Codex 看，用于快速理解 TestCenter 从 Mac 迁移后的落地流程。面向人类用户的完整说明见 `docs/windows-migration.md`。

## 当前目标

用户要在 Windows 上从 GitHub 拉取 `windows` 分支代码，然后用本地私有迁移包还原真实配置和运行数据，做到快速启动 TestCenter，不因为路径、配置文件位置或环境变量缺失踩低级坑。

## 分支规则

- 当前迁移分支：`windows`
- 不要新建临时分支，除非用户明确要求。
- 不要把私有迁移包、真实配置、运行报告提交到 GitHub。
- `windows_private_migration_*.tar.gz` 已在 `.gitignore` 中忽略。

## Windows 端第一步

```powershell
git clone -b windows https://github.com/Stroke-H/TestCenter-Vue3.git
cd TestCenter-Vue3
```

如果仓库已经存在：

```powershell
git fetch origin
git checkout windows
git pull
```

## 私有迁移包

用户会手动把 Mac 上生成的私有包拷到 Windows 项目根目录，例如：

```text
windows_private_migration_20260702.tar.gz
```

这个包包含真实配置和运行数据，不能提交。

包内关键路径：

```text
.env.lan.local
drama_info.json
server/data/
backup/legacy/
report/
```

还原命令：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\restore_windows_private_bundle.ps1 -Bundle .\windows_private_migration_20260702.tar.gz
```

还原脚本会解压到项目根目录，并校验这些路径：

```text
.env.lan.local
drama_info.json
server\data\database_config.json
server\data\feishu_config.json
server\data\ai_config.json
backup\legacy
report
```

## 依赖安装

```powershell
npm install
cd server
go mod download
cd ..
```

必须具备：

```powershell
git --version
node -v
npm -v
go version
```

k6 只有执行接口压测时才必需：

```powershell
k6 version
```

## 启动方式

优先使用 Windows 启动脚本：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start_windows.ps1
```

脚本会：

- 加载 `.env.lan.local`
- 设置 `TESTCENTER_BACKEND_HOST=0.0.0.0`
- 设置 `TESTCENTER_BACKEND_PORT=8080`
- 设置 `TESTCENTER_DB_CONFIG_FILE=<项目根目录>\server\data\database_config.json`
- 启动 Go 后端
- 启动 Vite 前端

启动后访问：

```text
http://localhost:5173/dashboard
```

## 如果要手动启动

后端：

```powershell
$env:TESTCENTER_BACKEND_HOST="0.0.0.0"
$env:TESTCENTER_BACKEND_PORT="8080"
$env:TESTCENTER_DB_CONFIG_FILE="$PWD\server\data\database_config.json"
cd server
go run main.go
```

前端另开一个 PowerShell：

```powershell
npm run dev
```

## 重点检查项

如果启动后配置缺失，先检查：

```powershell
Test-Path .\.env.lan.local
Test-Path .\drama_info.json
Test-Path .\server\data\database_config.json
Test-Path .\server\data\feishu_config.json
Test-Path .\server\data\ai_config.json
Test-Path .\report
```

如果数据库连接失败，重点看：

```text
server\data\database_config.json
```

Windows 上 MySQL 地址、端口、账号、密码可能和 Mac 不一致。

## 验证命令

代码改动后优先跑：

```powershell
npm run build
cd server
go test ./...
cd ..
```

如果只改迁移文档或 PowerShell 脚本，至少检查：

```powershell
git diff --check
```

## 安全边界

严禁提交：

```text
.env.lan.local
drama_info.json
server/data/*.json
server/data/*.jsonl
server/data/processes/
backup/legacy/*.jsonl
report/
windows_private_migration_*.tar.gz
```

提交前必须看：

```powershell
git status --short
git diff --cached --name-status
```

如果看到真实配置、私有包、报告目录进入暂存区，必须取消暂存。

## 相关文件

- `docs/windows-migration.md`：给用户看的完整 Windows 迁移说明。
- `scripts/restore_windows_private_bundle.ps1`：还原私有迁移包。
- `scripts/start_windows.ps1`：Windows 一键启动。
- `.env.lan.example`：公开示例配置，不包含真实敏感信息。

