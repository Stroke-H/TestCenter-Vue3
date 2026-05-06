package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type sqlJSONColumn struct {
	Column string
	Field  string
	JSON   bool
}

type sqlJSONTable struct {
	Table      string
	PrimaryKey string
	Columns    []sqlJSONColumn
}

var sqlJSONTables = map[string]sqlJSONTable{
	"projects": {
		Table:      "projects",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "project_code", Field: "project_code"},
			{Column: "project_name", Field: "project_name"},
			{Column: "short_code", Field: "short_code"},
			{Column: "wiki_url", Field: "wiki_url"},
			{Column: "workspace", Field: "workspace"},
			{Column: "created_at", Field: "created_at"},
		},
	},
	"test_phones": {
		Table:      "test_phones",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "device_name", Field: "device_name"},
			{Column: "os", Field: "os"},
			{Column: "model", Field: "model"},
			{Column: "allowed_app", Field: "allowed_app"},
			{Column: "created_at", Field: "created_at"},
		},
	},
	"sandbox_accounts": {
		Table:      "sandbox_accounts",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "account_type", Field: "account_type"},
			{Column: "account", Field: "account"},
			{Column: "password", Field: "password"},
			{Column: "project_code", Field: "project_code"},
			{Column: "created_at", Field: "created_at"},
		},
	},
	"testers": {
		Table:      "testers",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "username", Field: "username"},
			{Column: "nickname", Field: "nickname"},
			{Column: "email", Field: "email"},
			{Column: "avatar", Field: "avatar"},
			{Column: "password_hash", Field: "password_hash"},
			{Column: "created_at", Field: "created_at"},
			{Column: "feishu_open_id", Field: "feishu_open_id"},
		},
	},
	"scheduled_tasks": {
		Table:      "scheduled_tasks",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "name", Field: "name"},
			{Column: "function", Field: "function"},
			{Column: "schedule_type", Field: "schedule_type"},
			{Column: "creator", Field: "creator"},
			{Column: "next_run", Field: "next_run"},
			{Column: "next_run_at", Field: "next_run_at"},
			{Column: "test_project", Field: "test_project"},
			{Column: "test_project_code", Field: "test_project_code"},
			{Column: "test_env", Field: "test_env"},
			{Column: "status", Field: "status"},
			{Column: "description", Field: "description"},
			{Column: "last_run_at", Field: "last_run_at"},
			{Column: "last_result", Field: "last_result"},
			{Column: "created_at", Field: "created_at"},
			{Column: "updated_at", Field: "updated_at"},
		},
	},
	"pw_suites": {
		Table:      "pw_suites",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "name", Field: "name"},
			{Column: "description", Field: "description"},
			{Column: "variables", Field: "variables", JSON: true},
			{Column: "setup", Field: "setup", JSON: true},
			{Column: "teardown", Field: "teardown", JSON: true},
			{Column: "case_ids", Field: "case_ids", JSON: true},
			{Column: "skill_suites", Field: "skill_suites", JSON: true},
			{Column: "created_at", Field: "created_at"},
			{Column: "updated_at", Field: "updated_at"},
		},
	},
	"pw_cases": {
		Table:      "pw_cases",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "suite_id", Field: "suite_id"},
			{Column: "name", Field: "name"},
			{Column: "description", Field: "description"},
			{Column: "tags", Field: "tags", JSON: true},
			{Column: "variables", Field: "variables", JSON: true},
			{Column: "steps", Field: "steps", JSON: true},
			{Column: "created_at", Field: "created_at"},
			{Column: "updated_at", Field: "updated_at"},
		},
	},
	"pw_keywords": {
		Table:      "pw_keywords",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "name", Field: "name"},
			{Column: "description", Field: "description"},
			{Column: "args", Field: "args", JSON: true},
			{Column: "steps", Field: "steps", JSON: true},
			{Column: "suite_name", Field: "suite_name"},
			{Column: "source_url", Field: "source_url"},
			{Column: "created_at", Field: "created_at"},
		},
	},
	"testcase_history": {
		Table:      "testcase_history",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "title", Field: "title"},
			{Column: "project_code", Field: "project_code"},
			{Column: "module", Field: "module"},
			{Column: "requirement_text", Field: "requirement_text"},
			{Column: "created_at", Field: "created_at"},
			{Column: "points", Field: "points", JSON: true},
			{Column: "cases", Field: "cases", JSON: true},
		},
	},
	"acceptance_reports": {
		Table:      "acceptance_reports",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "project_name", Field: "project_name"},
			{Column: "project_code", Field: "project_code"},
			{Column: "version", Field: "version"},
			{Column: "test_owner", Field: "test_owner"},
			{Column: "reporter", Field: "reporter"},
			{Column: "test_time", Field: "test_time"},
			{Column: "test_env", Field: "test_env"},
			{Column: "test_devices", Field: "test_devices"},
			{Column: "test_conclusion", Field: "test_conclusion"},
			{Column: "bug_fix_status", Field: "bug_fix_status"},
			{Column: "bug_submission_status", Field: "bug_submission_status"},
			{Column: "update_requirements", Field: "update_requirements"},
			{Column: "created_at", Field: "created_at"},
			{Column: "updated_at", Field: "updated_at"},
			{Column: "status", Field: "status"},
		},
	},
	"execution_reports": {
		Table:      "execution_reports",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "name", Field: "name"},
			{Column: "type", Field: "type"},
			{Column: "status", Field: "status"},
			{Column: "duration", Field: "duration"},
			{Column: "createdAt", Field: "createdAt"},
			{Column: "author", Field: "author"},
			{Column: "reportUrl", Field: "reportUrl"},
			{Column: "analysisResult", Field: "analysisResult"},
		},
	},
	"ai_operation_logs": {
		Table:      "ai_operation_logs",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "tool_name", Field: "tool_name"},
			{Column: "project", Field: "project"},
			{Column: "env", Field: "env"},
			{Column: "user_id", Field: "user_id"},
			{Column: "user_name", Field: "user_name"},
			{Column: "status", Field: "status"},
			{Column: "detail", Field: "detail"},
			{Column: "timestamp", Field: "timestamp"},
		},
	},
	"ai_chat_histories": {
		Table:      "ai_chat_histories",
		PrimaryKey: "session_id",
		Columns: []sqlJSONColumn{
			{Column: "session_id", Field: "session_id"},
			{Column: "user_id", Field: "user_id"},
			{Column: "start_time", Field: "start_time"},
			{Column: "end_time", Field: "end_time"},
			{Column: "history", Field: "history", JSON: true},
		},
	},
	"feishu_messages": {
		Table:      "feishu_messages",
		PrimaryKey: "msg_id",
		Columns: []sqlJSONColumn{
			{Column: "msg_id", Field: "msg_id"},
			{Column: "event_id", Field: "event_id"},
			{Column: "chat_id", Field: "chat_id"},
			{Column: "chat_type", Field: "chat_type"},
			{Column: "sender_id", Field: "sender_id"},
			{Column: "sender_name", Field: "sender_name"},
			{Column: "msg_type", Field: "msg_type"},
			{Column: "content", Field: "content"},
			{Column: "raw_content", Field: "raw_content"},
			{Column: "mentions", Field: "mentions", JSON: true},
			{Column: "timestamp", Field: "timestamp"},
			{Column: "received_at", Field: "received_at"},
		},
	},
}

func sqlListJSON[T any](tableName string, orderBy string) ([]T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("SELECT `raw_json` FROM %s", quoteSQLIdent(tableName))
	if strings.TrimSpace(orderBy) != "" {
		query += " ORDER BY " + orderBy
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]T, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item T
		if err := json.Unmarshal(raw, &item); err == nil {
			items = append(items, item)
		}
	}
	return items, rows.Err()
}

func SQLListJSONForFeishu[T any](tableName string, orderBy string) ([]T, error) {
	return sqlListJSON[T](tableName, orderBy)
}

func sqlUpsertJSON(tableName string, item any) error {
	table, ok := sqlJSONTables[tableName]
	if !ok {
		return fmt.Errorf("sql json table not registered: %s", tableName)
	}

	record, raw, err := structToRecord(item)
	if err != nil {
		return err
	}
	stmt := buildSQLJSONUpsert(table)
	args, err := buildSQLJSONArgs(table, record, raw)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, stmt, args...)
	return err
}

func SQLUpsertJSONForFeishu(tableName string, item any) error {
	return sqlUpsertJSON(tableName, item)
}

func sqlReplaceAllJSON(tableName string, items any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+quoteSQLIdent(tableName)); err != nil {
		_ = tx.Rollback()
		return err
	}
	data, err := json.Marshal(items)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	var records []map[string]any
	if err := json.Unmarshal(data, &records); err != nil {
		_ = tx.Rollback()
		return err
	}
	table := sqlJSONTables[tableName]
	stmt := buildSQLJSONUpsert(table)
	for _, record := range records {
		raw, err := json.Marshal(record)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		args, err := buildSQLJSONArgs(table, record, string(raw))
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, stmt, args...); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func sqlDeleteJSON(tableName string, id any) error {
	table := sqlJSONTables[tableName]
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE %s = ?", quoteSQLIdent(tableName), quoteSQLIdent(table.PrimaryKey)), id)
	return err
}

func sqlClearJSON(tableName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "DELETE FROM "+quoteSQLIdent(tableName))
	return err
}

func structToRecord(item any) (map[string]any, string, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return nil, "", err
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, "", err
	}
	return record, string(data), nil
}

func buildSQLJSONUpsert(table sqlJSONTable) string {
	columns := make([]string, 0, len(table.Columns)+1)
	placeholders := make([]string, 0, len(table.Columns)+1)
	updates := make([]string, 0, len(table.Columns)+1)
	for _, column := range table.Columns {
		columns = append(columns, quoteSQLIdent(column.Column))
		placeholders = append(placeholders, "?")
		if column.Column != table.PrimaryKey {
			updates = append(updates, fmt.Sprintf("%s = VALUES(%s)", quoteSQLIdent(column.Column), quoteSQLIdent(column.Column)))
		}
	}
	columns = append(columns, "`raw_json`")
	placeholders = append(placeholders, "?")
	updates = append(updates, "`raw_json` = VALUES(`raw_json`)")
	updates = append(updates, "`migrated_at` = CURRENT_TIMESTAMP")
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s", quoteSQLIdent(table.Table), strings.Join(columns, ", "), strings.Join(placeholders, ", "), strings.Join(updates, ", "))
}

func buildSQLJSONArgs(table sqlJSONTable, record map[string]any, raw string) ([]any, error) {
	args := make([]any, 0, len(table.Columns)+1)
	for _, column := range table.Columns {
		value := record[column.Field]
		if column.JSON {
			if value == nil {
				args = append(args, nil)
				continue
			}
			data, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			args = append(args, string(data))
			continue
		}
		args = append(args, normalizeSQLScalar(value))
	}
	args = append(args, raw)
	return args, nil
}

func normalizeSQLScalar(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed)
		}
		return typed
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(data)
	}
}

func quoteSQLIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func sqlErrNoRows(err error) bool {
	return err == sql.ErrNoRows
}
