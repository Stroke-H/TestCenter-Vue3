# TestCenter Windows 到 Mac mini 一次性迁移指导

> 本文是 `2026-08-03` 生成的一次性迁移说明。目标机器是 Apple Silicon Mac mini。迁移包包含真实账号、密码、Token、数据库、报告和运行数据，只能私下传输和保存，严禁上传 GitHub 或公共网盘。

## 1. Codex 接手指令

在 Mac mini 上让 Codex 先完整阅读本文，再按顺序执行。不要先拉取 GitHub、不要覆盖包内源码、不要跳过数据库行数校验。每完成一节都确认命令成功后再继续。

迁移目标：

- 保留 Windows 当前源码及未提交修改。
- 保留项目私有配置、飞书/AI 配置、账号、报告和运行数据。
- 将 Windows MySQL `mysql` 库中的项目业务表迁入 Mac 本机独立的 `testcenter` 库。
- 在 Mac mini 上先启动后端，再启动前端，并允许同一局域网设备访问。

## 2. 迁移包内容

解压后根目录就是项目目录，关键内容如下：

```text
TestCenter-MacMini-Migration-20260803/
├── MAC_MINI_MIGRATION_README.md
├── migration/
│   ├── database/
│   │   ├── testcenter_business_20260803.sql
│   │   └── table_row_counts.csv
│   ├── config/
│   │   └── database_config.mac.json
│   └── secrets/
│       └── mac_mysql_app_password.txt
├── scripts/
│   ├── import_database_macos.sh
│   └── start_macos.sh
├── server/data/
├── report/
├── backup/
├── src/
└── ...其余项目源码
```

以下内容特意没有打包：

- `.git/`：Git 历史可从远端重新获取，且当前包已经保留未提交源码。
- `node_modules/`：必须在 Apple Silicon 上重新安装原生依赖。
- `dist/`：应在 Mac 上重新构建。
- Windows 日志、进程文件和旧迁移压缩包。

## 3. 数据库快照范围

快照来自 Windows 当前 MySQL，导出时间为 `2026-08-03 15:08` 左右。原项目把业务表放在 MySQL 的 `mysql` 系统库中；本次只导出了项目业务表，没有导出 `user`、`db`、`global_grants` 等 MySQL 系统权限表。

Mac 上必须导入独立的 `testcenter` 数据库，禁止把 SQL 导回 `mysql` 系统库。

本次 SQL 包含 24 张表、共 1234 行：

| 表 | 行数 |
|---|---:|
| acceptance_report | 51 |
| acceptance_reports | 180 |
| ai_chat_histories | 111 |
| ai_operation_logs | 121 |
| execution_reports | 136 |
| feishu_messages | 157 |
| novel_projects | 2 |
| project_code_and_name | 17 |
| project_config_records | 18 |
| projects | 37 |
| pw_cases | 1 |
| pw_keywords | 2 |
| pw_suites | 2 |
| sandbox_accounts | 12 |
| scheduled_tasks | 1 |
| test_case_steps | 224 |
| test_cases | 69 |
| test_phone | 20 |
| test_phones | 21 |
| testcase_history | 3 |
| testers | 3 |
| user_permissions | 4 |
| user_sessions | 35 |
| users | 7 |

精确清单以 `migration/database/table_row_counts.csv` 为准。导入脚本会逐表比对，任何一张表数量不同都会失败退出。

该 SQL 已在 Windows MySQL 中导入临时验证库并完成逐表核对，验证结果为 24 张表、1234 行全部一致，验证后临时库已删除。SQL 文件 SHA-256：

```text
0EA6EA4BC6E6B5FE795A8425E15768922DFB07AC0D2A23A18DB6EDDD9EBB81CC
```

## 4. 在 Mac mini 上解压

把 ZIP 放到 Mac mini，例如 `~/Downloads`，打开终端执行：

```bash
mkdir -p ~/Projects
cd ~/Projects
unzip ~/Downloads/TestCenter-MacMini-Migration-20260803.zip
cd TestCenter-MacMini-Migration-20260803
xattr -dr com.apple.quarantine . 2>/dev/null || true
chmod +x scripts/import_database_macos.sh scripts/start_macos.sh
shasum -a 256 -c migration/SHA256SUMS.txt
```

校验过程可能需要一段时间。必须看到全部文件均为 `OK` 后再继续；任何 `FAILED` 都说明复制或解压不完整，应重新传输 ZIP。

如果解压后的目录名略有不同，以实际目录为准，但必须在同时包含 `package.json`、`server/`、`migration/` 的根目录执行后续命令。

## 5. 安装 Apple Silicon 依赖

先确认架构：

```bash
uname -m
```

结果必须是 `arm64`。安装 Homebrew（若已有可跳过）：

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
eval "$(/opt/homebrew/bin/brew shellenv)"
```

安装项目依赖：

```bash
brew install node go mysql ffmpeg k6
brew services start mysql
```

版本检查：

```bash
node -v
npm -v
go version
mysql --version
ffmpeg -version | head -n 1
k6 version
```

项目当前要求 Go `1.25.5`，Node 建议使用 `22.x`。若 Homebrew 提供的 Go 版本低于 `1.25.5`，让 Codex安装官方 Apple Silicon Go 包后再继续。

## 6. 一键导入数据库

确保 MySQL 服务已启动：

```bash
brew services list | grep mysql
```

执行本次迁移专用脚本：

```bash
MIGRATION_CONFIRM=YES bash scripts/import_database_macos.sh
```

脚本会询问 MySQL `root` 密码。Homebrew 新安装的 MySQL 如果 root 没有密码，直接回车。

脚本执行内容：

1. 删除 Mac 本地同名 `testcenter` 数据库并重新创建。
2. 创建本项目专用的 `testcenter` 数据库用户。
3. 导入 `testcenter_business_20260803.sql`。
4. 把 Mac 专用连接配置安装到 `server/data/database_config.json`。
5. 按 CSV 逐表核对行数。

只有最后出现 `Database migration completed successfully.` 才算成功。不要手工把 SQL 导入 `mysql` 系统库。

## 7. 检查局域网配置

查看 Mac 当前有线网络 IP：

```bash
route get default | awk '/interface:/{print $2}' | xargs ipconfig getifaddr
```

编辑项目根目录的 `.env.lan.local`：

```bash
nano .env.lan.local
```

至少检查：

```text
TESTCENTER_LAN_HOST=<Mac 的局域网 IP 或已经指向该 IP 的域名>
TESTCENTER_FRONTEND_ORIGIN=http://<Mac 的局域网 IP 或域名>:5173
TESTCENTER_BACKEND_ORIGIN=http://<Mac 的局域网 IP 或域名>:8080
```

文件中的剧集测试账号和地址已经随迁移包保留，不要发到公开位置。

如果继续使用 `www.inspdance.com`，需要把局域网 DNS 或客户端 hosts 中的记录改为 Mac mini 的新局域网 IP。Cloudflare 的公网代理不能代理 `172.16.x.x` 这类私网地址；同一局域网访问应使用局域网 DNS/hosts 或直接使用 Mac IP。

## 8. 安装项目依赖并构建

迁移包没有携带 Windows 的依赖目录，必须在 Mac 上重新安装：

```bash
npm ci
cd server
go mod download
go test ./...
cd ..
npm run build
```

如需使用 Playwright 浏览器自动化，再执行：

```bash
cd server
go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5700.1 install
cd ..
```

## 9. 启动项目

在项目根目录执行：

```bash
bash scripts/start_macos.sh
```

脚本启动顺序固定为：

1. 检查依赖和数据库配置。
2. 启动 Go 后端并等待 `8080` 就绪。
3. 后端正常后再启动 Vite 前端。
4. 输出本机和局域网访问地址。

按 `Ctrl+C` 会同时停止前后端。日志位于：

```text
logs/backend-macos.log
logs/frontend-macos.log
```

## 10. 功能验收清单

本机先检查：

```bash
curl -I http://127.0.0.1:5173/dashboard
curl -i http://127.0.0.1:8080/api/auth/me
```

后端 `/api/auth/me` 未登录时返回 `401` 属于正常。随后在浏览器逐项确认：

- 登录与用户权限正常。
- 仪表盘、项目和测试设备数据存在。
- 验收报告列表及历史报告可打开。
- AI 操作审计日志、飞书消息和定时任务数据存在。
- 测试用例、Playwright 用例和沙盒账号数据存在。
- 剧集播放接口测试能执行。
- `report/` 中历史报告可访问。
- 局域网另一台电脑可通过 Mac IP 的 `5173` 端口访问。

数据库复核：

```bash
mysql -h 127.0.0.1 -u testcenter -p testcenter
```

密码位于 `migration/secrets/mac_mysql_app_password.txt`。进入 MySQL 后可执行：

```sql
SHOW TABLES;
SELECT COUNT(*) FROM acceptance_reports;
SELECT COUNT(*) FROM ai_operation_logs;
SELECT COUNT(*) FROM projects;
```

应分别得到 `180`、`121`、`37`。完整数量以 CSV 为准。

## 11. 常见问题

### MySQL root 无法连接

先确认服务：

```bash
brew services restart mysql
mysql -u root
```

若 `mysql -u root` 可以连接但脚本不行，执行：

```bash
MYSQL_ROOT_HOST=localhost MIGRATION_CONFIRM=YES bash scripts/import_database_macos.sh
```

### Go 版本不满足要求

```bash
go version
which go
```

Apple Silicon Homebrew 正常路径通常是 `/opt/homebrew/bin/go`。不要使用从 Intel Mac 复制来的 Go 二进制。

### 前端能开但接口报错

查看：

```bash
tail -n 200 logs/backend-macos.log
cat server/data/database_config.json
```

确认数据库为 `testcenter`、主机为 `127.0.0.1`，并确认 `8080` 端口已监听。

### 其他电脑无法访问

```bash
lsof -nP -iTCP:5173 -sTCP:LISTEN
lsof -nP -iTCP:8080 -sTCP:LISTEN
```

两项都应监听，不应只绑定 `127.0.0.1`。在 macOS“系统设置 > 网络 > 防火墙”中允许 Node 和 Go 接收入站连接，并确认两台设备处于同一局域网、没有访客网络隔离或 VPN 接管。

## 12. 迁移完成后的安全收尾

验收完成后：

1. 把原始 ZIP 移到加密磁盘或安全离线位置。
2. 保留 `migration/database` 作为本次迁移快照备份。
3. 将 `migration/secrets/mac_mysql_app_password.txt` 权限设为仅当前用户可读：

```bash
chmod 600 migration/secrets/mac_mysql_app_password.txt
chmod 600 server/data/database_config.json
```

4. 不要执行 `git add migration server/data .env.lan.local drama_info.json report`。
5. Mac 全部验收通过前，不要清理 Windows 原项目和数据库。
