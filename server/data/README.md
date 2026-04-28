# Data Directory Notes

`server/data` keeps local-only runtime files. Real config files must not be committed:

- `ai_config.json`
- `database_config.json`
- `feishu_config.json`
- `processes/`

Use the checked-in example files as templates:

- `ai_config.example.json`
- `feishu_config.example.json`

Current business data should be read from and written to MySQL tables through the SQL-backed store in `server/services/sql_json_store.go`.

Legacy `.jsonl` snapshots are not part of the normal runtime write path for migrated modules. They have been archived under `backup/legacy/` for recovery use.
