# Database Infrastructure ChangeLog

## 2026-04-24

- Added SQL-backed `user_sessions` runtime table for durable login sessions across backend restarts.
- Standardized historical numeric tester IDs to UUID values and synchronized known user references in permission, AI chat history, and AI operation log tables.
- Clarified that migrated business modules use MySQL tables with `raw_json` as the runtime source of truth.
- Archived legacy JSONL snapshots under `backup/legacy/` and documented `server/data/` as active runtime config storage only.
- Cleaned up misleading JSONL naming in SQL-backed service code and migration documentation.

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
