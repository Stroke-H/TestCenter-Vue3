package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	DefectStatusNew      = "new"
	DefectStatusActive   = "active"
	DefectStatusResolved = "resolved"
	DefectStatusClosed   = "closed"

	defectPermissionVisible = "defects.visible"
	defectPermissionCreate  = "defects.create"
	defectPermissionEdit    = "defects.edit"
	defectPermissionProcess = "defects.process"
	defectPermissionVerify  = "defects.verify"
	defectPermissionManage  = "defects.manage"

	defectAttachmentMaxBytes = 20 << 20
)

type DefectStep struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

type DefectEnvironment struct {
	Platform    string `json:"platform"`
	Environment string `json:"environment"`
	AppVersion  string `json:"app_version"`
	OSVersion   string `json:"os_version"`
	DeviceModel string `json:"device_model"`
	Network     string `json:"network"`
}

type Defect struct {
	AllowedActions  map[string]bool   `json:"allowed_actions,omitempty"`
	ID              string            `json:"id"`
	DefectNo        string            `json:"defect_no"`
	Title           string            `json:"title"`
	ProjectCode     string            `json:"project_code"`
	ProjectName     string            `json:"project_name"`
	FoundVersion    string            `json:"found_version"`
	ResolvedVersion string            `json:"resolved_version"`
	DefectType      string            `json:"defect_type"`
	Severity        int               `json:"severity"`
	Priority        int               `json:"priority"`
	Status          string            `json:"status"`
	Resolution      string            `json:"resolution"`
	ReporterID      string            `json:"reporter_id"`
	ReporterName    string            `json:"reporter_name"`
	AssigneeID      string            `json:"assignee_id"`
	AssigneeName    string            `json:"assignee_name"`
	VerifierID      string            `json:"verifier_id"`
	VerifierName    string            `json:"verifier_name"`
	DueDate         string            `json:"due_date"`
	Precondition    string            `json:"precondition"`
	Steps           []DefectStep      `json:"steps"`
	ActualResult    string            `json:"actual_result"`
	ExpectedResult  string            `json:"expected_result"`
	Description     string            `json:"description"`
	Environment     DefectEnvironment `json:"environment"`
	Tags            []string          `json:"tags"`
	CustomFields    map[string]any    `json:"custom_fields"`
	ReopenCount     int               `json:"reopen_count"`
	RowVersion      int               `json:"row_version"`
	Archived        bool              `json:"archived"`
	ResolvedAt      string            `json:"resolved_at"`
	ClosedAt        string            `json:"closed_at"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
}

type DefectComment struct {
	ID         string `json:"id"`
	DefectID   string `json:"defect_id"`
	AuthorID   string `json:"author_id"`
	AuthorName string `json:"author_name"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
}

type DefectHistory struct {
	ID           string          `json:"id"`
	DefectID     string          `json:"defect_id"`
	Action       string          `json:"action"`
	OperatorID   string          `json:"operator_id"`
	OperatorName string          `json:"operator_name"`
	Before       json.RawMessage `json:"before,omitempty"`
	After        json.RawMessage `json:"after,omitempty"`
	Comment      string          `json:"comment"`
	CreatedAt    string          `json:"created_at"`
}

type DefectAttachment struct {
	ID           string `json:"id"`
	DefectID     string `json:"defect_id"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	Size         int64  `json:"size"`
	UploaderID   string `json:"uploader_id"`
	UploaderName string `json:"uploader_name"`
	CreatedAt    string `json:"created_at"`
	DownloadURL  string `json:"download_url"`
	storageName  string
}

type DefectDetail struct {
	Defect      Defect             `json:"defect"`
	Comments    []DefectComment    `json:"comments"`
	History     []DefectHistory    `json:"history"`
	Attachments []DefectAttachment `json:"attachments"`
}

type DefectListResponse struct {
	Items    []Defect `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

type DefectSaveRequest struct {
	Title          string            `json:"title"`
	ProjectCode    string            `json:"project_code"`
	FoundVersion   string            `json:"found_version"`
	DefectType     string            `json:"defect_type"`
	Severity       int               `json:"severity"`
	Priority       int               `json:"priority"`
	AssigneeID     string            `json:"assignee_id"`
	VerifierID     string            `json:"verifier_id"`
	DueDate        string            `json:"due_date"`
	Precondition   string            `json:"precondition"`
	Steps          []DefectStep      `json:"steps"`
	ActualResult   string            `json:"actual_result"`
	ExpectedResult string            `json:"expected_result"`
	Description    string            `json:"description"`
	Environment    DefectEnvironment `json:"environment"`
	Tags           []string          `json:"tags"`
	CustomFields   map[string]any    `json:"custom_fields"`
	RowVersion     int               `json:"row_version"`
}

type DefectTransitionRequest struct {
	Action          string `json:"action"`
	AssigneeID      string `json:"assignee_id"`
	Resolution      string `json:"resolution"`
	ResolvedVersion string `json:"resolved_version"`
	Comment         string `json:"comment"`
}

type DefectCommentRequest struct {
	Content string `json:"content"`
}

var (
	defectSchemaMu    sync.Mutex
	defectSchemaReady bool
)

func InitDefectManagementService() error {
	return ensureDefectSchema(context.Background())
}

func ensureDefectSchema(parent context.Context) error {
	defectSchemaMu.Lock()
	defer defectSchemaMu.Unlock()
	if defectSchemaReady {
		return nil
	}

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS defects (
			id VARCHAR(36) PRIMARY KEY,
			defect_no VARCHAR(32) NOT NULL UNIQUE,
			title VARCHAR(255) NOT NULL,
			project_code VARCHAR(128) NOT NULL,
			project_name VARCHAR(255) NOT NULL,
			found_version VARCHAR(128) NOT NULL,
			resolved_version VARCHAR(128) NOT NULL,
			defect_type VARCHAR(64) NOT NULL,
			severity TINYINT NOT NULL,
			priority TINYINT NOT NULL,
			status VARCHAR(32) NOT NULL,
			resolution VARCHAR(32) NOT NULL,
			reporter_id VARCHAR(128) NOT NULL,
			reporter_name VARCHAR(128) NOT NULL,
			assignee_id VARCHAR(128) NOT NULL,
			assignee_name VARCHAR(128) NOT NULL,
			verifier_id VARCHAR(128) NOT NULL,
			verifier_name VARCHAR(128) NOT NULL,
			due_date VARCHAR(16) NOT NULL,
			precondition LONGTEXT NOT NULL,
			steps_json JSON NOT NULL,
			actual_result LONGTEXT NOT NULL,
			expected_result LONGTEXT NOT NULL,
			description LONGTEXT NOT NULL,
			environment_json JSON NOT NULL,
			tags_json JSON NOT NULL,
			reopen_count INT NOT NULL DEFAULT 0,
			row_version INT NOT NULL DEFAULT 1,
			archived TINYINT(1) NOT NULL DEFAULT 0,
			resolved_at VARCHAR(64) NOT NULL,
			closed_at VARCHAR(64) NOT NULL,
			created_at VARCHAR(64) NOT NULL,
			updated_at VARCHAR(64) NOT NULL,
			INDEX idx_defects_project (project_code),
			INDEX idx_defects_status (status),
			INDEX idx_defects_assignee (assignee_id),
			INDEX idx_defects_updated (updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS defect_sequences (
			date_key CHAR(8) PRIMARY KEY,
			current_value INT NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS defect_comments (
			id VARCHAR(36) PRIMARY KEY,
			defect_id VARCHAR(36) NOT NULL,
			author_id VARCHAR(128) NOT NULL,
			author_name VARCHAR(128) NOT NULL,
			content LONGTEXT NOT NULL,
			created_at VARCHAR(64) NOT NULL,
			INDEX idx_defect_comments_defect (defect_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS defect_histories (
			id VARCHAR(36) PRIMARY KEY,
			defect_id VARCHAR(36) NOT NULL,
			action VARCHAR(32) NOT NULL,
			operator_id VARCHAR(128) NOT NULL,
			operator_name VARCHAR(128) NOT NULL,
			before_json JSON NULL,
			after_json JSON NULL,
			comment LONGTEXT NOT NULL,
			created_at VARCHAR(64) NOT NULL,
			INDEX idx_defect_histories_defect (defect_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS defect_attachments (
			id VARCHAR(36) PRIMARY KEY,
			defect_id VARCHAR(36) NOT NULL,
			original_name VARCHAR(255) NOT NULL,
			storage_name VARCHAR(255) NOT NULL,
			mime_type VARCHAR(128) NOT NULL,
			size BIGINT NOT NULL,
			uploader_id VARCHAR(128) NOT NULL,
			uploader_name VARCHAR(128) NOT NULL,
			created_at VARCHAR(64) NOT NULL,
			INDEX idx_defect_attachments_defect (defect_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS defect_field_definitions (
			id VARCHAR(36) PRIMARY KEY,
			field_key VARCHAR(64) NOT NULL,
			name VARCHAR(128) NOT NULL,
			field_type VARCHAR(32) NOT NULL,
			project_code VARCHAR(128) NOT NULL,
			required TINYINT(1) NOT NULL DEFAULT 0,
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			list_visible TINYINT(1) NOT NULL DEFAULT 0,
			filterable TINYINT(1) NOT NULL DEFAULT 0,
			options_json JSON NOT NULL,
			sort_order INT NOT NULL DEFAULT 0,
			created_by VARCHAR(128) NOT NULL,
			created_at VARCHAR(64) NOT NULL,
			updated_at VARCHAR(64) NOT NULL,
			UNIQUE KEY uk_defect_field_scope (project_code, field_key),
			INDEX idx_defect_fields_scope (project_code, enabled, sort_order)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS defect_custom_values (
			defect_id VARCHAR(36) NOT NULL,
			field_id VARCHAR(36) NOT NULL,
			value_json JSON NOT NULL,
			updated_at VARCHAR(64) NOT NULL,
			PRIMARY KEY (defect_id, field_id),
			INDEX idx_defect_custom_field (field_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS defect_project_versions (
			project_code VARCHAR(128) PRIMARY KEY,
			versions_json JSON NOT NULL,
			updated_by VARCHAR(128) NOT NULL,
			updated_at VARCHAR(64) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS defect_project_settings (
			project_code VARCHAR(128) PRIMARY KEY,
			permission_mode VARCHAR(16) NOT NULL DEFAULT 'open',
			updated_by VARCHAR(128) NOT NULL,
			updated_at VARCHAR(64) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS defect_role_assignments (
			project_code VARCHAR(128) NOT NULL,
			user_id VARCHAR(128) NOT NULL,
			role_key VARCHAR(32) NOT NULL,
			updated_at VARCHAR(64) NOT NULL,
			PRIMARY KEY (project_code, user_id),
			INDEX idx_defect_roles_user (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	defectSchemaReady = true
	return nil
}

func userDisplayName(user *User) string {
	if strings.TrimSpace(user.Nickname) != "" {
		return strings.TrimSpace(user.Nickname)
	}
	return strings.TrimSpace(user.Username)
}

func defectPermissionAllowed(user *User, key string) (bool, error) {
	if user == nil {
		return false, nil
	}
	if isPermissionAdminUsername(user.Username) {
		return true, nil
	}
	record, err := getPermissionRecordByUserID(user.ID)
	if err != nil {
		return false, err
	}
	permissions := DefaultDashboardPermissions(user.Username)
	if record != nil {
		permissions = record.Permissions
	}
	return permissions[key] != false, nil
}

func RequireDefectPermission(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := CurrentUserFromRequest(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录状态已失效，请重新登录"})
			c.Abort()
			return
		}
		allowed, err := defectPermissionAllowed(user, key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "没有缺陷管理操作权限"})
			c.Abort()
			return
		}
		c.Set("defect_user", user)
		c.Next()
	}
}

func defectUser(c *gin.Context) (*User, bool) {
	if value, ok := c.Get("defect_user"); ok {
		if user, valid := value.(*User); valid {
			return user, true
		}
	}
	user, err := CurrentUserFromRequest(c)
	return user, err == nil
}

func defectCanManage(user *User) bool {
	allowed, _ := defectPermissionAllowed(user, defectPermissionManage)
	return allowed
}

func defectCanEditRecord(ctx context.Context, user *User, defect Defect) bool {
	if !defectProjectAllows(ctx, user, defect.ProjectCode, "edit") {
		return false
	}
	role := defectProjectRole(ctx, user, defect.ProjectCode)
	allowed, _ := defectPermissionAllowed(user, defectPermissionEdit)
	if !allowed {
		return false
	}
	if defectCanManage(user) && (role == "open" || role == "admin" || role == "lead") {
		return true
	}
	return user.ID == defect.ReporterID || user.ID == defect.AssigneeID || role == "lead"
}

func normalizeDefectSaveRequest(req *DefectSaveRequest) error {
	req.Title = strings.TrimSpace(req.Title)
	req.ProjectCode = strings.TrimSpace(req.ProjectCode)
	req.FoundVersion = strings.TrimSpace(req.FoundVersion)
	req.DefectType = strings.TrimSpace(req.DefectType)
	req.AssigneeID = strings.TrimSpace(req.AssigneeID)
	req.VerifierID = strings.TrimSpace(req.VerifierID)
	req.DueDate = strings.TrimSpace(req.DueDate)
	req.Precondition = strings.TrimSpace(req.Precondition)
	req.ActualResult = strings.TrimSpace(req.ActualResult)
	req.ExpectedResult = strings.TrimSpace(req.ExpectedResult)
	req.Description = strings.TrimSpace(req.Description)
	if req.Title == "" || req.ProjectCode == "" {
		return errors.New("缺陷标题和所属项目不能为空")
	}
	if len([]rune(req.Title)) > 255 {
		return errors.New("缺陷标题不能超过 255 个字符")
	}
	if req.Severity < 1 || req.Severity > 4 {
		req.Severity = 3
	}
	if req.Priority < 1 || req.Priority > 4 {
		req.Priority = 3
	}
	if req.DefectType == "" {
		req.DefectType = "function"
	}
	if req.DueDate != "" {
		if _, err := time.Parse("2006-01-02", req.DueDate); err != nil {
			return errors.New("期望解决日期格式不正确")
		}
	}
	steps := make([]DefectStep, 0, len(req.Steps))
	for _, step := range req.Steps {
		step.Content = strings.TrimSpace(step.Content)
		if step.Content == "" {
			continue
		}
		if step.ID == "" {
			step.ID = uuid.NewString()
		}
		steps = append(steps, step)
	}
	req.Steps = steps
	tagSeen := map[string]bool{}
	tags := make([]string, 0, len(req.Tags))
	for _, tag := range req.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || tagSeen[strings.ToLower(tag)] {
			continue
		}
		tagSeen[strings.ToLower(tag)] = true
		tags = append(tags, tag)
	}
	req.Tags = tags
	return nil
}

func defectProject(projectCode string) (string, error) {
	if ConfigServiceInstance == nil {
		return "", errors.New("项目配置服务尚未初始化")
	}
	project, err := ConfigServiceInstance.GetProjectBySubCode(projectCode)
	if err != nil || project == nil {
		return "", errors.New("所属项目不存在")
	}
	return project.ProjectName, nil
}

func defectAccount(userID string) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", nil
	}
	user, err := GetUserByID(userID)
	if err != nil {
		return "", errors.New("选择的平台账号不存在")
	}
	return userDisplayName(user), nil
}

func nextDefectNo(ctx context.Context, tx *sql.Tx) (string, error) {
	dateKey := time.Now().Format("20060102")
	var sequence int
	err := tx.QueryRowContext(ctx, "SELECT current_value FROM defect_sequences WHERE date_key = ? FOR UPDATE", dateKey).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) {
		sequence = 1
		if _, err := tx.ExecContext(ctx, "INSERT INTO defect_sequences (date_key, current_value) VALUES (?, ?)", dateKey, sequence); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		sequence++
		if _, err := tx.ExecContext(ctx, "UPDATE defect_sequences SET current_value = ? WHERE date_key = ?", sequence, dateKey); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("BUG-%s-%04d", dateKey, sequence), nil
}

func insertDefectHistory(ctx context.Context, tx *sql.Tx, defectID, action string, user *User, before, after any, comment string) error {
	var beforeJSON, afterJSON any
	if before != nil {
		encoded, err := json.Marshal(before)
		if err != nil {
			return err
		}
		beforeJSON = string(encoded)
	}
	if after != nil {
		encoded, err := json.Marshal(after)
		if err != nil {
			return err
		}
		afterJSON = string(encoded)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO defect_histories
		(id, defect_id, action, operator_id, operator_name, before_json, after_json, comment, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), defectID, action, user.ID, userDisplayName(user), beforeJSON, afterJSON, strings.TrimSpace(comment), time.Now().Format(time.RFC3339Nano))
	return err
}

func createDefect(ctx context.Context, user *User, req DefectSaveRequest) (*Defect, error) {
	if err := normalizeDefectSaveRequest(&req); err != nil {
		return nil, err
	}
	projectName, err := defectProject(req.ProjectCode)
	if err != nil {
		return nil, err
	}
	if err := ensureDefectSchema(ctx); err != nil {
		return nil, err
	}
	if !defectProjectAllows(ctx, user, req.ProjectCode, "create") {
		return nil, errors.New("没有在该项目提交缺陷的权限")
	}
	if err := validateDefectVersion(ctx, req.ProjectCode, req.FoundVersion, ""); err != nil {
		return nil, err
	}
	assigneeName, err := defectAccount(req.AssigneeID)
	if err != nil {
		return nil, err
	}
	verifierName, err := defectAccount(req.VerifierID)
	if err != nil {
		return nil, err
	}
	if err := validateDefectCustomFields(ctx, req.ProjectCode, req.CustomFields); err != nil {
		return nil, err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	defectNo, err := nextDefectNo(ctx, tx)
	if err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339Nano)
	stepsJSON, _ := json.Marshal(req.Steps)
	environmentJSON, _ := json.Marshal(req.Environment)
	tagsJSON, _ := json.Marshal(req.Tags)
	defect := Defect{
		ID: uuid.NewString(), DefectNo: defectNo, Title: req.Title,
		ProjectCode: req.ProjectCode, ProjectName: projectName, FoundVersion: req.FoundVersion,
		DefectType: req.DefectType, Severity: req.Severity, Priority: req.Priority,
		Status: DefectStatusNew, ReporterID: user.ID, ReporterName: userDisplayName(user),
		AssigneeID: req.AssigneeID, AssigneeName: assigneeName, VerifierID: req.VerifierID, VerifierName: verifierName,
		DueDate: req.DueDate, Precondition: req.Precondition, Steps: req.Steps,
		ActualResult: req.ActualResult, ExpectedResult: req.ExpectedResult, Description: req.Description,
		Environment: req.Environment, Tags: req.Tags, CustomFields: req.CustomFields, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO defects (
			id, defect_no, title, project_code, project_name, found_version, resolved_version,
			defect_type, severity, priority, status, resolution, reporter_id, reporter_name,
			assignee_id, assignee_name, verifier_id, verifier_name, due_date, precondition,
			steps_json, actual_result, expected_result, description, environment_json, tags_json,
			reopen_count, row_version, archived, resolved_at, closed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, 0, '', '', ?, ?)
	`, defect.ID, defect.DefectNo, defect.Title, defect.ProjectCode, defect.ProjectName, defect.FoundVersion,
		defect.DefectType, defect.Severity, defect.Priority, defect.Status, defect.ReporterID, defect.ReporterName,
		defect.AssigneeID, defect.AssigneeName, defect.VerifierID, defect.VerifierName, defect.DueDate, defect.Precondition,
		string(stepsJSON), defect.ActualResult, defect.ExpectedResult, defect.Description, string(environmentJSON), string(tagsJSON), now, now)
	if err != nil {
		return nil, err
	}
	if err := insertDefectHistory(ctx, tx, defect.ID, "create", user, nil, defect, "提交缺陷"); err != nil {
		return nil, err
	}
	if err := saveDefectCustomFields(ctx, tx, defect.ID, defect.ProjectCode, req.CustomFields); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &defect, nil
}

const defectSelectColumns = `
	id, defect_no, title, project_code, project_name, found_version, resolved_version,
	defect_type, severity, priority, status, resolution, reporter_id, reporter_name,
	assignee_id, assignee_name, verifier_id, verifier_name, due_date, precondition,
	steps_json, actual_result, expected_result, description, environment_json, tags_json,
	reopen_count, row_version, archived, resolved_at, closed_at, created_at, updated_at`

type defectScanner interface {
	Scan(dest ...any) error
}

func scanDefect(scanner defectScanner) (Defect, error) {
	var defect Defect
	var stepsJSON, environmentJSON, tagsJSON string
	var archived int
	err := scanner.Scan(
		&defect.ID, &defect.DefectNo, &defect.Title, &defect.ProjectCode, &defect.ProjectName,
		&defect.FoundVersion, &defect.ResolvedVersion, &defect.DefectType, &defect.Severity,
		&defect.Priority, &defect.Status, &defect.Resolution, &defect.ReporterID, &defect.ReporterName,
		&defect.AssigneeID, &defect.AssigneeName, &defect.VerifierID, &defect.VerifierName,
		&defect.DueDate, &defect.Precondition, &stepsJSON, &defect.ActualResult, &defect.ExpectedResult,
		&defect.Description, &environmentJSON, &tagsJSON, &defect.ReopenCount, &defect.RowVersion,
		&archived, &defect.ResolvedAt, &defect.ClosedAt, &defect.CreatedAt, &defect.UpdatedAt,
	)
	if err != nil {
		return defect, err
	}
	defect.Archived = archived != 0
	defect.Steps = []DefectStep{}
	defect.Tags = []string{}
	_ = json.Unmarshal([]byte(stepsJSON), &defect.Steps)
	_ = json.Unmarshal([]byte(environmentJSON), &defect.Environment)
	_ = json.Unmarshal([]byte(tagsJSON), &defect.Tags)
	defect.CustomFields = map[string]any{}
	return defect, nil
}

func getDefect(ctx context.Context, id string) (*Defect, error) {
	if err := ensureDefectSchema(ctx); err != nil {
		return nil, err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	defect, err := scanDefect(db.QueryRowContext(ctx, "SELECT "+defectSelectColumns+" FROM defects WHERE id = ? AND archived = 0", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &defect, nil
}

func listDefects(ctx context.Context, user *User, values map[string]string) (DefectListResponse, error) {
	response := DefectListResponse{Items: []Defect{}}
	if err := ensureDefectSchema(ctx); err != nil {
		return response, err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return response, err
	}
	where := []string{"archived = 0"}
	args := []any{}
	if clause, scopeArgs := defectProjectScopeSQL(user); clause != "" {
		where = append(where, clause)
		args = append(args, scopeArgs...)
	}
	addEqual := func(column, key string) {
		if value := strings.TrimSpace(values[key]); value != "" {
			where = append(where, column+" = ?")
			args = append(args, value)
		}
	}
	addEqual("project_code", "project_code")
	addEqual("found_version", "found_version")
	addEqual("status", "status")
	addEqual("assignee_id", "assignee_id")
	addEqual("reporter_id", "reporter_id")
	if value := strings.TrimSpace(values["severity"]); value != "" {
		where = append(where, "severity = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(values["priority"]); value != "" {
		where = append(where, "priority = ?")
		args = append(args, value)
	}
	if search := strings.TrimSpace(values["search"]); search != "" {
		where = append(where, "(defect_no LIKE ? OR title LIKE ? OR description LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}
	for key, value := range values {
		if !strings.HasPrefix(key, "cf_") || strings.TrimSpace(value) == "" {
			continue
		}
		fieldKey := strings.TrimPrefix(key, "cf_")
		where = append(where, `EXISTS (SELECT 1 FROM defect_custom_values dcv
			JOIN defect_field_definitions dfd ON dfd.id=dcv.field_id
			WHERE dcv.defect_id=defects.id AND dfd.field_key=? AND JSON_UNQUOTE(dcv.value_json) LIKE ?)`)
		args = append(args, fieldKey, "%"+strings.TrimSpace(value)+"%")
	}
	whereSQL := strings.Join(where, " AND ")
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM defects WHERE "+whereSQL, args...).Scan(&response.Total); err != nil {
		return response, err
	}
	response.Page, _ = strconv.Atoi(values["page"])
	response.PageSize, _ = strconv.Atoi(values["page_size"])
	if response.Page < 1 {
		response.Page = 1
	}
	if response.PageSize < 1 {
		response.PageSize = 20
	}
	if response.PageSize > 100 {
		response.PageSize = 100
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, response.PageSize, (response.Page-1)*response.PageSize)
	rows, err := db.QueryContext(ctx, "SELECT "+defectSelectColumns+" FROM defects WHERE "+whereSQL+" ORDER BY updated_at DESC, defect_no DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return response, err
	}
	defer rows.Close()
	for rows.Next() {
		defect, err := scanDefect(rows)
		if err != nil {
			return response, err
		}
		response.Items = append(response.Items, defect)
	}
	if err := rows.Err(); err != nil {
		return response, err
	}
	if err := hydrateDefectCustomFields(ctx, response.Items); err != nil {
		return response, err
	}
	return response, nil
}

func updateDefect(ctx context.Context, user *User, id string, req DefectSaveRequest) (*Defect, error) {
	if err := normalizeDefectSaveRequest(&req); err != nil {
		return nil, err
	}
	current, err := getDefect(ctx, id)
	if err != nil || current == nil {
		return current, err
	}
	if !defectCanEditRecord(ctx, user, *current) {
		return nil, errors.New("没有编辑该缺陷的权限")
	}
	previousVersion := ""
	if current.ProjectCode == req.ProjectCode {
		previousVersion = current.FoundVersion
	}
	if err := validateDefectVersion(ctx, req.ProjectCode, req.FoundVersion, previousVersion); err != nil {
		return nil, err
	}
	if req.ProjectCode != current.ProjectCode {
		allowed, err := defectPermissionAllowed(user, defectPermissionCreate)
		if err != nil || !allowed || !defectProjectAllows(ctx, user, req.ProjectCode, "create") {
			return nil, errors.New("没有向目标项目转移缺陷的权限")
		}
	}
	projectName, err := defectProject(req.ProjectCode)
	if err != nil {
		return nil, err
	}
	assigneeName, err := defectAccount(req.AssigneeID)
	if err != nil {
		return nil, err
	}
	verifierName, err := defectAccount(req.VerifierID)
	if err != nil {
		return nil, err
	}
	if err := validateDefectCustomFields(ctx, req.ProjectCode, req.CustomFields); err != nil {
		return nil, err
	}
	if req.RowVersion > 0 && req.RowVersion != current.RowVersion {
		return nil, errors.New("缺陷已被其他人更新，请刷新后重试")
	}
	updated := *current
	updated.Title, updated.ProjectCode, updated.ProjectName = req.Title, req.ProjectCode, projectName
	updated.FoundVersion, updated.DefectType = req.FoundVersion, req.DefectType
	updated.Severity, updated.Priority = req.Severity, req.Priority
	updated.AssigneeID, updated.AssigneeName = req.AssigneeID, assigneeName
	updated.VerifierID, updated.VerifierName = req.VerifierID, verifierName
	updated.DueDate, updated.Precondition = req.DueDate, req.Precondition
	updated.Steps, updated.ActualResult, updated.ExpectedResult = req.Steps, req.ActualResult, req.ExpectedResult
	updated.Description, updated.Environment, updated.Tags, updated.CustomFields = req.Description, req.Environment, req.Tags, req.CustomFields
	updated.RowVersion++
	updated.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	stepsJSON, _ := json.Marshal(updated.Steps)
	environmentJSON, _ := json.Marshal(updated.Environment)
	tagsJSON, _ := json.Marshal(updated.Tags)

	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE defects SET title=?, project_code=?, project_name=?, found_version=?, defect_type=?,
		severity=?, priority=?, assignee_id=?, assignee_name=?, verifier_id=?, verifier_name=?, due_date=?,
		precondition=?, steps_json=?, actual_result=?, expected_result=?, description=?, environment_json=?,
		tags_json=?, row_version=?, updated_at=? WHERE id=? AND row_version=? AND archived=0
	`, updated.Title, updated.ProjectCode, updated.ProjectName, updated.FoundVersion, updated.DefectType,
		updated.Severity, updated.Priority, updated.AssigneeID, updated.AssigneeName, updated.VerifierID,
		updated.VerifierName, updated.DueDate, updated.Precondition, string(stepsJSON), updated.ActualResult,
		updated.ExpectedResult, updated.Description, string(environmentJSON), string(tagsJSON), updated.RowVersion,
		updated.UpdatedAt, id, current.RowVersion)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, errors.New("缺陷已被其他人更新，请刷新后重试")
	}
	if err := insertDefectHistory(ctx, tx, id, "update", user, current, updated, "编辑缺陷"); err != nil {
		return nil, err
	}
	if err := saveDefectCustomFields(ctx, tx, id, updated.ProjectCode, req.CustomFields); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &updated, nil
}

func validateDefectTransition(status, action string) (string, error) {
	switch action {
	case "confirm":
		if status == DefectStatusNew {
			return DefectStatusActive, nil
		}
	case "resolve":
		if status == DefectStatusNew || status == DefectStatusActive {
			return DefectStatusResolved, nil
		}
	case "close":
		if status == DefectStatusResolved {
			return DefectStatusClosed, nil
		}
	case "reopen":
		if status == DefectStatusResolved || status == DefectStatusClosed {
			return DefectStatusActive, nil
		}
	}
	return "", fmt.Errorf("当前状态不允许执行 %s 操作", action)
}

func transitionDefect(ctx context.Context, user *User, id string, req DefectTransitionRequest) (*Defect, error) {
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	req.Comment = strings.TrimSpace(req.Comment)
	current, err := getDefect(ctx, id)
	if err != nil || current == nil {
		return current, err
	}
	permission := defectPermissionProcess
	if req.Action == "close" || req.Action == "reopen" {
		permission = defectPermissionVerify
	}
	allowed, err := defectPermissionAllowed(user, permission)
	if err != nil || !allowed {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("没有执行该生命周期操作的权限")
	}
	projectAction := "process"
	if req.Action == "close" {
		projectAction = "verify"
	} else if req.Action == "reopen" {
		projectAction = "reopen"
	}
	if !defectProjectAllows(ctx, user, current.ProjectCode, projectAction) {
		return nil, errors.New("没有在该项目执行此操作的权限")
	}
	nextStatus, err := validateDefectTransition(current.Status, req.Action)
	if err != nil {
		return nil, err
	}
	if req.Action == "resolve" && strings.TrimSpace(req.Resolution) == "" {
		return nil, errors.New("解决缺陷时必须选择解决方案")
	}
	if req.Action == "reopen" && req.Comment == "" {
		return nil, errors.New("重新激活时必须填写原因")
	}
	updated := *current
	updated.Status = nextStatus
	updated.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	updated.RowVersion++
	if req.AssigneeID != "" {
		updated.AssigneeID = strings.TrimSpace(req.AssigneeID)
		updated.AssigneeName, err = defectAccount(updated.AssigneeID)
		if err != nil {
			return nil, err
		}
	}
	switch req.Action {
	case "resolve":
		if err := validateDefectVersion(ctx, current.ProjectCode, strings.TrimSpace(req.ResolvedVersion), ""); err != nil {
			return nil, err
		}
		updated.Resolution = strings.TrimSpace(req.Resolution)
		updated.ResolvedVersion = strings.TrimSpace(req.ResolvedVersion)
		updated.ResolvedAt = updated.UpdatedAt
		updated.ClosedAt = ""
	case "close":
		updated.VerifierID = user.ID
		updated.VerifierName = userDisplayName(user)
		updated.ClosedAt = updated.UpdatedAt
	case "reopen":
		updated.Resolution = ""
		updated.ResolvedVersion = ""
		updated.ResolvedAt = ""
		updated.ClosedAt = ""
		updated.ReopenCount++
	}

	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE defects SET status=?, resolution=?, resolved_version=?, assignee_id=?, assignee_name=?,
		verifier_id=?, verifier_name=?, resolved_at=?, closed_at=?, reopen_count=?, row_version=?, updated_at=?
		WHERE id=? AND row_version=? AND archived=0
	`, updated.Status, updated.Resolution, updated.ResolvedVersion, updated.AssigneeID, updated.AssigneeName,
		updated.VerifierID, updated.VerifierName, updated.ResolvedAt, updated.ClosedAt, updated.ReopenCount,
		updated.RowVersion, updated.UpdatedAt, id, current.RowVersion)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, errors.New("缺陷状态已被其他人更新，请刷新后重试")
	}
	if err := insertDefectHistory(ctx, tx, id, req.Action, user, current, updated, req.Comment); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &updated, nil
}

func listDefectComments(ctx context.Context, defectID string) ([]DefectComment, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id, defect_id, author_id, author_name, content, created_at
		FROM defect_comments WHERE defect_id=? ORDER BY created_at ASC`, defectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DefectComment{}
	for rows.Next() {
		var item DefectComment
		if err := rows.Scan(&item.ID, &item.DefectID, &item.AuthorID, &item.AuthorName, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func listDefectHistory(ctx context.Context, defectID string) ([]DefectHistory, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id, defect_id, action, operator_id, operator_name,
		COALESCE(before_json, JSON_OBJECT()), COALESCE(after_json, JSON_OBJECT()), comment, created_at
		FROM defect_histories WHERE defect_id=? ORDER BY created_at ASC`, defectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DefectHistory{}
	for rows.Next() {
		var item DefectHistory
		var beforeJSON, afterJSON []byte
		if err := rows.Scan(&item.ID, &item.DefectID, &item.Action, &item.OperatorID, &item.OperatorName,
			&beforeJSON, &afterJSON, &item.Comment, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Before, item.After = json.RawMessage(beforeJSON), json.RawMessage(afterJSON)
		items = append(items, item)
	}
	return items, rows.Err()
}

func listDefectAttachments(ctx context.Context, defectID string) ([]DefectAttachment, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id, defect_id, original_name, storage_name, mime_type, size,
		uploader_id, uploader_name, created_at FROM defect_attachments WHERE defect_id=? ORDER BY created_at ASC`, defectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DefectAttachment{}
	for rows.Next() {
		var item DefectAttachment
		if err := rows.Scan(&item.ID, &item.DefectID, &item.OriginalName, &item.storageName, &item.MimeType,
			&item.Size, &item.UploaderID, &item.UploaderName, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.DownloadURL = "/api/defects/" + defectID + "/attachments/" + item.ID
		items = append(items, item)
	}
	return items, rows.Err()
}

func defectDetail(ctx context.Context, id string) (*DefectDetail, error) {
	defect, err := getDefect(ctx, id)
	if err != nil || defect == nil {
		return nil, err
	}
	comments, err := listDefectComments(ctx, id)
	if err != nil {
		return nil, err
	}
	history, err := listDefectHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	attachments, err := listDefectAttachments(ctx, id)
	if err != nil {
		return nil, err
	}
	defect.CustomFields, err = loadDefectCustomFields(ctx, id)
	if err != nil {
		return nil, err
	}
	return &DefectDetail{Defect: *defect, Comments: comments, History: history, Attachments: attachments}, nil
}

func addDefectComment(ctx context.Context, user *User, defectID, content string) (*DefectComment, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("评论内容不能为空")
	}
	if defect, err := getDefect(ctx, defectID); err != nil || defect == nil {
		return nil, err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	comment := &DefectComment{
		ID: uuid.NewString(), DefectID: defectID, AuthorID: user.ID,
		AuthorName: userDisplayName(user), Content: content, CreatedAt: time.Now().Format(time.RFC3339Nano),
	}
	_, err = db.ExecContext(ctx, `INSERT INTO defect_comments
		(id, defect_id, author_id, author_name, content, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		comment.ID, comment.DefectID, comment.AuthorID, comment.AuthorName, comment.Content, comment.CreatedAt)
	if err != nil {
		return nil, err
	}
	_, _ = db.ExecContext(ctx, "UPDATE defects SET updated_at=?, row_version=row_version+1 WHERE id=?", comment.CreatedAt, defectID)
	return comment, nil
}

func allowedDefectAttachment(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	allowed := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
		".txt": true, ".log": true, ".json": true, ".zip": true, ".pdf": true,
		".mp4": true, ".mov": true,
	}
	return allowed[ext]
}

func saveDefectAttachment(ctx context.Context, user *User, defectID string, header *multipart.FileHeader) (*DefectAttachment, error) {
	if header == nil || header.Size <= 0 {
		return nil, errors.New("附件为空")
	}
	if header.Size > defectAttachmentMaxBytes {
		return nil, errors.New("单个附件不能超过 20MB")
	}
	if !allowedDefectAttachment(header.Filename) {
		return nil, errors.New("不支持该附件类型")
	}
	defect, err := getDefect(ctx, defectID)
	if err != nil || defect == nil {
		return nil, err
	}
	source, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer source.Close()
	attachmentID := uuid.NewString()
	storageName := attachmentID + strings.ToLower(filepath.Ext(header.Filename))
	directory := filepath.Join("data", "defect_attachments", defectID)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, err
	}
	targetPath := filepath.Join(directory, storageName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(target, io.LimitReader(source, defectAttachmentMaxBytes+1)); err != nil {
		target.Close()
		_ = os.Remove(targetPath)
		return nil, err
	}
	if err := target.Close(); err != nil {
		_ = os.Remove(targetPath)
		return nil, err
	}
	attachment := &DefectAttachment{
		ID: attachmentID, DefectID: defectID, OriginalName: filepath.Base(header.Filename),
		MimeType: header.Header.Get("Content-Type"), Size: header.Size, UploaderID: user.ID,
		UploaderName: userDisplayName(user), CreatedAt: time.Now().Format(time.RFC3339Nano),
		DownloadURL: "/api/defects/" + defectID + "/attachments/" + attachmentID, storageName: storageName,
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		_ = os.Remove(targetPath)
		return nil, err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO defect_attachments
		(id, defect_id, original_name, storage_name, mime_type, size, uploader_id, uploader_name, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, attachment.ID, attachment.DefectID, attachment.OriginalName,
		attachment.storageName, attachment.MimeType, attachment.Size, attachment.UploaderID, attachment.UploaderName, attachment.CreatedAt)
	if err != nil {
		_ = os.Remove(targetPath)
		return nil, err
	}
	return attachment, nil
}

func getDefectAttachment(ctx context.Context, defectID, attachmentID string) (*DefectAttachment, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	var item DefectAttachment
	err = db.QueryRowContext(ctx, `SELECT id, defect_id, original_name, storage_name, mime_type, size,
		uploader_id, uploader_name, created_at FROM defect_attachments WHERE defect_id=? AND id=?`, defectID, attachmentID).Scan(
		&item.ID, &item.DefectID, &item.OriginalName, &item.storageName, &item.MimeType, &item.Size,
		&item.UploaderID, &item.UploaderName, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &item, err
}

func GetDefectMetaHandler(c *gin.Context) {
	projects, err := ConfigServiceInstance.GetAllProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	accounts, err := ListAccountProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	user, _ := defectUser(c)
	fields, err := listDefectFieldDefinitions(c.Request.Context(), "", true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	permissions := map[string]bool{}
	for _, key := range []string{defectPermissionVisible, defectPermissionCreate, defectPermissionEdit, defectPermissionProcess, defectPermissionVerify, defectPermissionManage} {
		permissions[key], _ = defectPermissionAllowed(user, key)
	}
	type safeAccount struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar,omitempty"`
	}
	safeAccounts := make([]safeAccount, 0, len(accounts))
	for _, account := range accounts {
		safeAccounts = append(safeAccounts, safeAccount{ID: account.ID, Username: account.Username, Nickname: account.Nickname, Avatar: account.Avatar})
	}
	projectActions := map[string]map[string]bool{}
	for _, project := range projects {
		projectActions[project.ProjectCode] = map[string]bool{
			"create": permissions[defectPermissionCreate] && defectProjectAllows(c.Request.Context(), user, project.ProjectCode, "create"),
		}
	}
	versions, err := listDefectVersions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for project := range versions {
		if !defectProjectAllows(c.Request.Context(), user, project, "view") {
			delete(versions, project)
		}
	}
	versionOptions := map[string][]string{}
	for project, config := range versions {
		versionOptions[project] = config.Versions
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects, "accounts": safeAccounts, "permissions": permissions, "fields": fields, "project_actions": projectActions, "project_versions": versionOptions, "project_version_stages": versions})
}

func ListDefectsHandler(c *gin.Context) {
	values := map[string]string{}
	for _, key := range []string{"search", "project_code", "found_version", "status", "severity", "priority", "assignee_id", "reporter_id", "page", "page_size"} {
		values[key] = c.Query(key)
	}
	for key, entries := range c.Request.URL.Query() {
		if strings.HasPrefix(key, "cf_") && len(entries) > 0 {
			values[key] = entries[0]
		}
	}
	user, _ := defectUser(c)
	response, err := listDefects(c.Request.Context(), user, values)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range response.Items {
		response.Items[i].AllowedActions = defectAllowedActions(c.Request.Context(), user, response.Items[i])
	}
	c.JSON(http.StatusOK, response)
}

func CreateDefectHandler(c *gin.Context) {
	user, _ := defectUser(c)
	var req DefectSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺陷数据格式不正确"})
		return
	}
	defect, err := createDefect(c.Request.Context(), user, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, defect)
}

func GetDefectHandler(c *gin.Context) {
	detail, err := defectDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if detail == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "缺陷不存在"})
		return
	}
	user, _ := defectUser(c)
	if !defectProjectAllows(c.Request.Context(), user, detail.Defect.ProjectCode, "view") {
		c.JSON(http.StatusForbidden, gin.H{"error": "没有查看该项目缺陷的权限"})
		return
	}
	detail.Defect.AllowedActions = defectAllowedActions(c.Request.Context(), user, detail.Defect)
	c.JSON(http.StatusOK, detail)
}

func UpdateDefectHandler(c *gin.Context) {
	user, _ := defectUser(c)
	var req DefectSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺陷数据格式不正确"})
		return
	}
	defect, err := updateDefect(c.Request.Context(), user, c.Param("id"), req)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "权限") {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	if defect == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "缺陷不存在"})
		return
	}
	c.JSON(http.StatusOK, defect)
}

func TransitionDefectHandler(c *gin.Context) {
	user, _ := defectUser(c)
	var req DefectTransitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "生命周期操作参数不正确"})
		return
	}
	defect, err := transitionDefect(c.Request.Context(), user, c.Param("id"), req)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "权限") {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	if defect == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "缺陷不存在"})
		return
	}
	c.JSON(http.StatusOK, defect)
}

func AddDefectCommentHandler(c *gin.Context) {
	user, _ := defectUser(c)
	defect, err := getDefect(c.Request.Context(), c.Param("id"))
	if err != nil || defect == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "缺陷不存在"})
		return
	}
	if !defectProjectAllows(c.Request.Context(), user, defect.ProjectCode, "comment") {
		c.JSON(http.StatusForbidden, gin.H{"error": "没有在该项目评论的权限"})
		return
	}
	var req DefectCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论数据格式不正确"})
		return
	}
	comment, err := addDefectComment(c.Request.Context(), user, c.Param("id"), req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, comment)
}

func UploadDefectAttachmentHandler(c *gin.Context) {
	user, _ := defectUser(c)
	defect, err := getDefect(c.Request.Context(), c.Param("id"))
	if err != nil || defect == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "缺陷不存在"})
		return
	}
	if !defectCanEditRecord(c.Request.Context(), user, *defect) {
		c.JSON(http.StatusForbidden, gin.H{"error": "没有向该缺陷上传附件的权限"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, defectAttachmentMaxBytes+1<<20)
	header, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择需要上传的附件"})
		return
	}
	attachment, err := saveDefectAttachment(c.Request.Context(), user, c.Param("id"), header)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, attachment)
}

func DownloadDefectAttachmentHandler(c *gin.Context) {
	user, _ := defectUser(c)
	if defect, err := getDefect(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else if defect == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "缺陷不存在"})
		return
	} else if !defectProjectAllows(c.Request.Context(), user, defect.ProjectCode, "view") {
		c.JSON(http.StatusForbidden, gin.H{"error": "没有查看该项目附件的权限"})
		return
	}
	attachment, err := getDefectAttachment(c.Request.Context(), c.Param("id"), c.Param("attachment_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if attachment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "附件不存在"})
		return
	}
	path := filepath.Join("data", "defect_attachments", attachment.DefectID, attachment.storageName)
	c.FileAttachment(path, attachment.OriginalName)
}
