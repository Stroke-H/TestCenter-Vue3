package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"testcenter-server/database"
)

type tableMigration struct {
	File       string
	Table      string
	PrimaryKey string
	Columns    []columnMapping
}

type columnMapping struct {
	Column string
	Field  string
	JSON   bool
}

var migrations = []tableMigration{
	{
		File:       "acceptance_reports.jsonl",
		Table:      "acceptance_reports",
		PrimaryKey: "id",
		Columns: []columnMapping{
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
	{
		File:       "ai_chat_histories.jsonl",
		Table:      "ai_chat_histories",
		PrimaryKey: "session_id",
		Columns: []columnMapping{
			{Column: "session_id", Field: "session_id"},
			{Column: "user_id", Field: "user_id"},
			{Column: "start_time", Field: "start_time"},
			{Column: "end_time", Field: "end_time"},
			{Column: "history", Field: "history", JSON: true},
		},
	},
	{
		File:       "ai_operation_logs.jsonl",
		Table:      "ai_operation_logs",
		PrimaryKey: "id",
		Columns: []columnMapping{
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
	{
		File:       "execution_reports.jsonl",
		Table:      "execution_reports",
		PrimaryKey: "id",
		Columns: []columnMapping{
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
	{
		File:       "feishu_messages.jsonl",
		Table:      "feishu_messages",
		PrimaryKey: "msg_id",
		Columns: []columnMapping{
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
	{
		File:       "projects.jsonl",
		Table:      "projects",
		PrimaryKey: "id",
		Columns: []columnMapping{
			{Column: "id", Field: "id"},
			{Column: "project_code", Field: "project_code"},
			{Column: "project_name", Field: "project_name"},
			{Column: "short_code", Field: "short_code"},
			{Column: "wiki_url", Field: "wiki_url"},
			{Column: "workspace", Field: "workspace"},
			{Column: "created_at", Field: "created_at"},
		},
	},
	{
		File:       "pw_cases.jsonl",
		Table:      "pw_cases",
		PrimaryKey: "id",
		Columns: []columnMapping{
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
	{
		File:       "pw_keywords.jsonl",
		Table:      "pw_keywords",
		PrimaryKey: "id",
		Columns: []columnMapping{
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
	{
		File:       "pw_suites.jsonl",
		Table:      "pw_suites",
		PrimaryKey: "id",
		Columns: []columnMapping{
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
	{
		File:       "sandbox_accounts.jsonl",
		Table:      "sandbox_accounts",
		PrimaryKey: "id",
		Columns: []columnMapping{
			{Column: "id", Field: "id"},
			{Column: "account_type", Field: "account_type"},
			{Column: "account", Field: "account"},
			{Column: "password", Field: "password"},
			{Column: "project_code", Field: "project_code"},
			{Column: "created_at", Field: "created_at"},
		},
	},
	{
		File:       "scheduled_tasks.jsonl",
		Table:      "scheduled_tasks",
		PrimaryKey: "id",
		Columns: []columnMapping{
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
	{
		File:       "test_phones.jsonl",
		Table:      "test_phones",
		PrimaryKey: "id",
		Columns: []columnMapping{
			{Column: "id", Field: "id"},
			{Column: "device_name", Field: "device_name"},
			{Column: "os", Field: "os"},
			{Column: "model", Field: "model"},
			{Column: "allowed_app", Field: "allowed_app"},
			{Column: "created_at", Field: "created_at"},
		},
	},
	{
		File:       "testcase_history.jsonl",
		Table:      "testcase_history",
		PrimaryKey: "id",
		Columns: []columnMapping{
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
	{
		File:       "users.jsonl",
		Table:      "testers",
		PrimaryKey: "id",
		Columns: []columnMapping{
			{Column: "id", Field: "id"},
			{Column: "username", Field: "username"},
			{Column: "nickname", Field: "nickname"},
			{Column: "email", Field: "email"},
			{Column: "password_hash", Field: "password_hash"},
			{Column: "created_at", Field: "created_at"},
			{Column: "feishu_open_id", Field: "feishu_open_id"},
		},
	},
}

func main() {
	dryRun := flag.Bool("dry-run", false, "only validate jsonl files and print row counts")
	flag.Parse()

	dataDir := getenv("TESTCENTER_JSONL_DATA_DIR", "data")
	if *dryRun {
		total, err := dryRunFiles(dataDir)
		if err != nil {
			exitf("dry run failed: %v", err)
		}
		fmt.Printf("Dry run completed. %d rows found.\n", total)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := database.NewManager()
	defer manager.Close()

	db, _, err := manager.DB(ctx)
	if err != nil {
		exitf("connect database failed: %v", err)
	}

	if err := applySchema(ctx, db); err != nil {
		exitf("apply schema failed: %v", err)
	}

	total := 0
	for _, migration := range migrations {
		count, err := migrateFile(ctx, db, dataDir, migration)
		if err != nil {
			exitf("migrate %s failed: %v", migration.File, err)
		}
		total += count
		fmt.Printf("[OK] %-28s -> %-22s %d rows\n", migration.File, migration.Table, count)
	}

	fmt.Printf("Migration completed. %d rows processed.\n", total)
}

func dryRunFiles(dataDir string) (int, error) {
	total := 0
	for _, migration := range migrations {
		count, err := countValidJSONLLines(filepath.Join(dataDir, migration.File))
		if err != nil {
			return total, fmt.Errorf("%s: %w", migration.File, err)
		}
		total += count
		fmt.Printf("[DRY] %-28s -> %-22s %d rows\n", migration.File, migration.Table, count)
	}
	return total, nil
}

func countValidJSONLLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer file.Close()

	count := 0
	decoder := json.NewDecoder(file)
	for {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			return count, fmt.Errorf("invalid json record %d: %w", count+1, err)
		}
		count++
	}
	return count, nil
}

func applySchema(ctx context.Context, db *sql.DB) error {
	path := filepath.Join("migrations", "001_create_jsonl_tables.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	for _, statement := range splitSQLStatements(string(data)) {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("statement failed: %w\n%s", err, statement)
		}
	}
	return nil
}

func migrateFile(ctx context.Context, db *sql.DB, dataDir string, migration tableMigration) (int, error) {
	path := filepath.Join(dataDir, migration.File)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer file.Close()

	stmt := buildUpsertSQL(migration)
	count := 0
	decoder := json.NewDecoder(file)
	for {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			return count, fmt.Errorf("invalid json record %d: %w", count+1, err)
		}

		raw, err := json.Marshal(record)
		if err != nil {
			return count, fmt.Errorf("marshal raw json record %d: %w", count+1, err)
		}

		args, err := buildArgs(migration, record, string(raw))
		if err != nil {
			return count, err
		}
		if _, err := db.ExecContext(ctx, stmt, args...); err != nil {
			return count, fmt.Errorf("insert record %d: %w", count+1, err)
		}
		count++
	}
	return count, nil
}

func buildUpsertSQL(migration tableMigration) string {
	columns := make([]string, 0, len(migration.Columns)+1)
	placeholders := make([]string, 0, len(migration.Columns)+1)
	updates := make([]string, 0, len(migration.Columns)+1)

	for _, column := range migration.Columns {
		columns = append(columns, quoteIdent(column.Column))
		placeholders = append(placeholders, "?")
		if column.Column != migration.PrimaryKey {
			updates = append(updates, fmt.Sprintf("%s = VALUES(%s)", quoteIdent(column.Column), quoteIdent(column.Column)))
		}
	}
	columns = append(columns, quoteIdent("raw_json"))
	placeholders = append(placeholders, "?")
	updates = append(updates, "`raw_json` = VALUES(`raw_json`)")
	updates = append(updates, "`migrated_at` = CURRENT_TIMESTAMP")

	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		quoteIdent(migration.Table),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
		strings.Join(updates, ", "),
	)
}

func buildArgs(migration tableMigration, record map[string]any, raw string) ([]any, error) {
	args := make([]any, 0, len(migration.Columns)+1)
	for _, column := range migration.Columns {
		value := record[column.Field]
		if column.JSON {
			if value == nil {
				args = append(args, nil)
				continue
			}
			data, err := json.Marshal(value)
			if err != nil {
				return nil, fmt.Errorf("marshal json column %s: %w", column.Column, err)
			}
			args = append(args, string(data))
			continue
		}
		args = append(args, normalizeScalar(value))
	}
	args = append(args, raw)
	return args, nil
}

func normalizeScalar(value any) any {
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
	case bool:
		return typed
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(data)
	}
}

func splitSQLStatements(sqlText string) []string {
	var statements []string
	var current strings.Builder
	for _, line := range strings.Split(sqlText, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
		if strings.HasSuffix(trimmed, ";") {
			statement := strings.TrimSpace(current.String())
			statement = strings.TrimSuffix(statement, ";")
			if statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
		}
	}
	if statement := strings.TrimSpace(current.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
