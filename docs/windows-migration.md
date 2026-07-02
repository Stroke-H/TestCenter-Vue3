# Windows 迁移部署说明

本文用于把当前 Mac 上的 TestCenter 迁移到 Windows 电脑。代码走 GitHub 的 `windows` 分支，真实配置和运行数据走本地私有迁移包，不上传 GitHub。

## 1. Windows 前置环境

请先安装：

- Git
- Node.js 18 或更高版本
- Go 1.21 或更高版本
- k6（需要执行剧集播放接口测试时安装）

安装后在 PowerShell 验证：

```powershell
git --version
node -v
npm -v
go version
k6 version
```

## 2. 拉取代码

```powershell
git clone -b windows https://github.com/Stroke-H/TestCenter-Vue3.git
cd TestCenter-Vue3
```

如果已经 clone 过：

```powershell
git fetch origin
git checkout windows
git pull
```

## 3. 安装依赖

```powershell
npm install
cd server
go mod download
cd ..
```

## 4. 还原私有迁移包

把 Mac 上生成的私有包复制到 Windows 项目根目录，例如：

```text
TestCenter-Vue3\windows_private_migration_20260702.tar.gz
```

然后在项目根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\restore_windows_private_bundle.ps1 -Bundle .\windows_private_migration_20260702.tar.gz
```

脚本会把以下内容解压到固定路径：

```text
.env.lan.local
drama_info.json
server\data\
backup\legacy\
report\
```

并校验关键文件是否存在，避免解压到错误目录。

## 5. 检查 Windows 本机配置

迁移完成后重点检查：

```powershell
Test-Path .\.env.lan.local
Test-Path .\server\data\database_config.json
Test-Path .\server\data\feishu_config.json
Test-Path .\server\data\ai_config.json
Test-Path .\drama_info.json
```

如果 Windows 电脑的局域网主机名、数据库地址、MySQL 账号不同，需要编辑：

```text
.env.lan.local
server\data\database_config.json
```

## 6. Windows 一键启动

在项目根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start_windows.ps1
```

脚本会：

- 加载 `.env.lan.local`
- 固定 `TESTCENTER_BACKEND_HOST=0.0.0.0`
- 固定 `TESTCENTER_BACKEND_PORT=8080`
- 固定 `TESTCENTER_DB_CONFIG_FILE=<项目根目录>\server\data\database_config.json`
- 启动 Go 后端
- 启动 Vite 前端

启动后访问：

```text
http://localhost:5173/dashboard
```

局域网其他设备访问时，使用 `.env.lan.local` 中配置的 `TESTCENTER_FRONTEND_ORIGIN`。

## 7. 常见问题

### PowerShell 提示禁止运行脚本

使用本说明里的命令：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start_windows.ps1
```

### 端口被占用

检查：

```powershell
netstat -ano | findstr :5173
netstat -ano | findstr :8080
```

结束占用进程：

```powershell
taskkill /PID <PID> /F
```

### 数据库连接失败

检查：

```text
server\data\database_config.json
```

Windows 上 MySQL 地址、端口、账号、密码可能和 Mac 不同，必要时改成本机或可访问的数据库地址。

### 剧集接口测试配置缺失

检查：

```text
.env.lan.local
drama_info.json
server\data\ai_config.json
server\data\feishu_config.json
```

这些文件来自私有迁移包，不在 GitHub 仓库里。

## 8. 安全提醒

私有迁移包包含真实配置、密码、token、历史报告和运行数据：

- 不要提交到 GitHub
- 不要发到公开群
- 不要放进共享网盘公共目录
- Windows 迁移完成后可以把压缩包移到安全位置备份
