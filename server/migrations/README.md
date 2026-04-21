# Database Migrations

This directory stores ordered SQL migration files for MySQL-backed modules.

Current convention:

- Each `server/data/*.jsonl` file maps to one table.
- Table names match the file name without `.jsonl`.
- Stable scalar fields get regular columns.
- Nested or flexible payloads use MySQL `JSON` columns.
- Every table keeps a `raw_json` column so migration can preserve the original record.
