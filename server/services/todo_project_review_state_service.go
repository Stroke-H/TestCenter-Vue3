package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const acceptanceTodoStatusNotApproved = "not_approved"

type todoStateExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func ensureTodoProjectReviewStateTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS todo_project_review_states (
			owner_username VARCHAR(128) NOT NULL,
			project_code VARCHAR(128) NOT NULL,
			previous_version_not_approved TINYINT(1) NOT NULL DEFAULT 0,
			source_todo_type VARCHAR(20) NOT NULL DEFAULT '',
			source_todo_id VARCHAR(64) NOT NULL DEFAULT '',
			marked_at DATETIME NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (owner_username, project_code),
			INDEX idx_todo_review_state_active (owner_username, previous_version_not_approved)
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
	`)
	return err
}

func normalizeTodoProjectStateKey(projectCode string) string {
	return strings.ToLower(strings.TrimSpace(projectCode))
}

func markTodoProjectNotApproved(
	ctx context.Context,
	executor todoStateExecutor,
	ownerUsername string,
	projectCode string,
	todoType string,
	todoID string,
	now time.Time,
) error {
	ownerUsername = strings.TrimSpace(ownerUsername)
	projectCode = strings.TrimSpace(projectCode)
	if ownerUsername == "" || projectCode == "" {
		return fmt.Errorf("待办操作人和项目编号不能为空")
	}
	_, err := executor.ExecContext(ctx, `
		INSERT INTO todo_project_review_states (
			owner_username, project_code, previous_version_not_approved,
			source_todo_type, source_todo_id, marked_at, updated_at
		) VALUES (?, ?, 1, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			previous_version_not_approved = 1,
			source_todo_type = VALUES(source_todo_type),
			source_todo_id = VALUES(source_todo_id),
			marked_at = VALUES(marked_at),
			updated_at = VALUES(updated_at)
	`, ownerUsername, projectCode, todoType, todoID, now, now)
	return err
}

func clearTodoProjectNotApproved(
	ctx context.Context,
	executor todoStateExecutor,
	ownerUsername string,
	projectCode string,
) error {
	_, err := executor.ExecContext(ctx, `
		DELETE FROM todo_project_review_states
		WHERE LOWER(owner_username) = LOWER(?) AND LOWER(project_code) = LOWER(?)
	`, strings.TrimSpace(ownerUsername), strings.TrimSpace(projectCode))
	return err
}

func listTodoProjectNotApprovedStates(ownerUsername string) (map[string]bool, error) {
	ownerUsername = strings.TrimSpace(ownerUsername)
	if ownerUsername == "" {
		return nil, fmt.Errorf("owner_username is required")
	}
	if err := ensureTodoProjectReviewStateTable(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT project_code
		FROM todo_project_review_states
		WHERE LOWER(owner_username) = LOWER(?) AND previous_version_not_approved = 1
	`, ownerUsername)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := make(map[string]bool)
	for rows.Next() {
		var projectCode string
		if err := rows.Scan(&projectCode); err != nil {
			return nil, err
		}
		states[normalizeTodoProjectStateKey(projectCode)] = true
	}
	return states, rows.Err()
}

func attachTodoProjectReviewStates(items []MyTodoItem, states map[string]bool) {
	for index := range items {
		items[index].PreviousVersionNotApproved = states[normalizeTodoProjectStateKey(items[index].ProjectCode)]
	}
}
