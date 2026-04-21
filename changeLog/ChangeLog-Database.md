# Database Infrastructure ChangeLog

## 2026-04-21

- Added a unified Go database infrastructure layer for future MySQL integration.
- Added environment-based MySQL configuration with safe, password-free API output.
- Added Gin diagnostics endpoints:
  - `GET /api/database/config`
  - `GET /api/database/health`
- Added connection pool initialization, ping validation, basic server metadata query, and pool stats reporting.
- Added JSON file based default database configuration at `server/data/database_config.json`.
- Added MySQL schema for all `server/data/*.jsonl` files.
- Archived the one-time JSONL migration utility under `平台功能测试文件管理/mysql-jsonl-migration/` after migration completed.
