# Database Migrations

This directory stores ordered SQL migration files for MySQL-backed modules.

Current convention:

- Runtime data is stored in MySQL tables.
- Many tables were originally migrated from `server/data/*.jsonl`, so some table names still mirror the former file names.
- Stable scalar fields get regular columns.
- Nested or flexible payloads use MySQL `JSON` columns.
- Every table keeps a `raw_json` column so the service can round-trip the original record shape while using SQL as the source of truth.
