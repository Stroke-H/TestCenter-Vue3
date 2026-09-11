package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DefectFieldDefinition struct {
	ID          string   `json:"id"`
	FieldKey    string   `json:"field_key"`
	Name        string   `json:"name"`
	FieldType   string   `json:"field_type"`
	ProjectCode string   `json:"project_code"`
	Required    bool     `json:"required"`
	Enabled     bool     `json:"enabled"`
	ListVisible bool     `json:"list_visible"`
	Filterable  bool     `json:"filterable"`
	Options     []string `json:"options"`
	SortOrder   int      `json:"sort_order"`
	CreatedBy   string   `json:"created_by"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type DefectProjectMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	RoleKey  string `json:"role_key"`
}

type DefectProjectPermission struct {
	ProjectCode    string                `json:"project_code"`
	ProjectName    string                `json:"project_name"`
	PermissionMode string                `json:"permission_mode"`
	Members        []DefectProjectMember `json:"members"`
	UpdatedAt      string                `json:"updated_at"`
}

type DefectProjectPermissionRequest struct {
	PermissionMode string                `json:"permission_mode"`
	Members        []DefectProjectMember `json:"members"`
}

type DefectDistributionItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type DefectTrendItem struct {
	Date    string `json:"date"`
	Created int    `json:"created"`
	Closed  int    `json:"closed"`
}

type DefectStats struct {
	Total              int                      `json:"total"`
	New                int                      `json:"new"`
	Active             int                      `json:"active"`
	Resolved           int                      `json:"resolved"`
	Closed             int                      `json:"closed"`
	Overdue            int                      `json:"overdue"`
	Reopened           int                      `json:"reopened"`
	ClosureRate        float64                  `json:"closure_rate"`
	AverageResolveHour float64                  `json:"average_resolve_hours"`
	Status             []DefectDistributionItem `json:"status_distribution"`
	Severity           []DefectDistributionItem `json:"severity_distribution"`
	Projects           []DefectDistributionItem `json:"project_distribution"`
	Assignees          []DefectDistributionItem `json:"assignee_distribution"`
	Trend              []DefectTrendItem        `json:"trend"`
}

type DefectBatchRequest struct {
	IDs             []string `json:"ids"`
	Action          string   `json:"action"`
	AssigneeID      string   `json:"assignee_id"`
	Severity        int      `json:"severity"`
	Priority        int      `json:"priority"`
	Resolution      string   `json:"resolution"`
	ResolvedVersion string   `json:"resolved_version"`
	Comment         string   `json:"comment"`
}

type DefectBatchResult struct {
	ID       string `json:"id"`
	DefectNo string `json:"defect_no"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func defectProjectScopeSQL(user *User) (string, []any) {
	if user == nil {
		return "1=0", nil
	}
	if isPermissionAdminUsername(user.Username) {
		return "", nil
	}
	return `(NOT EXISTS (
		SELECT 1 FROM defect_project_settings dps
		WHERE dps.project_code = defects.project_code AND dps.permission_mode = 'restricted'
	) OR EXISTS (
		SELECT 1 FROM defect_role_assignments dra
		WHERE dra.project_code = defects.project_code AND dra.user_id = ?
	))`, []any{user.ID}
}

func defectProjectRole(ctx context.Context, user *User, projectCode string) string {
	if user == nil {
		return ""
	}
	if isPermissionAdminUsername(user.Username) {
		return "admin"
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return ""
	}
	var mode string
	err = db.QueryRowContext(ctx, "SELECT permission_mode FROM defect_project_settings WHERE project_code=?", projectCode).Scan(&mode)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ""
	}
	var role string
	err = db.QueryRowContext(ctx, "SELECT role_key FROM defect_role_assignments WHERE project_code=? AND user_id=?", projectCode, user.ID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) && mode != "restricted" {
		return "open"
	}
	if err != nil {
		return ""
	}
	return role
}

func defectProjectAllows(ctx context.Context, user *User, projectCode, action string) bool {
	role := defectProjectRole(ctx, user, projectCode)
	return defectRoleAllows(role, action)
}

func defectRoleAllows(role, action string) bool {
	if role == "admin" || role == "open" || role == "lead" {
		return true
	}
	switch action {
	case "view":
		return role == "tester" || role == "developer" || role == "viewer"
	case "comment":
		return role == "tester" || role == "developer"
	case "create", "verify", "reopen":
		return role == "tester"
	case "edit":
		return role == "tester" || role == "developer"
	case "process":
		return role == "developer"
	default:
		return false
	}
}

// Return effective permissions for this record, including ownership restrictions.
func defectAllowedActions(ctx context.Context, user *User, defect Defect) map[string]bool {
	role := defectProjectRole(ctx, user, defect.ProjectCode)
	result := map[string]bool{}
	for action, key := range map[string]string{
		"process": defectPermissionProcess, "verify": defectPermissionVerify,
		"reopen": defectPermissionVerify, "archive": defectPermissionProcess,
		"comment": defectPermissionVisible,
	} {
		allowed, err := defectPermissionAllowed(user, key)
		result[action] = err == nil && allowed && defectRoleAllows(role, action)
	}
	result["edit"] = defectCanEditRecord(ctx, user, defect)
	result["archive"] = result["archive"] && (role == "admin" || role == "lead" || defectCanManage(user))
	return result
}

func listDefectFieldDefinitions(ctx context.Context, projectCode string, includeAll bool) ([]DefectFieldDefinition, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	query := `SELECT id, field_key, name, field_type, project_code, required, enabled,
		list_visible, filterable, options_json, sort_order, created_by, created_at, updated_at
		FROM defect_field_definitions`
	args := []any{}
	if !includeAll {
		query += " WHERE enabled=1 AND (project_code='' OR project_code=?)"
		args = append(args, projectCode)
	}
	query += " ORDER BY project_code ASC, sort_order ASC, created_at ASC"
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DefectFieldDefinition{}
	for rows.Next() {
		var item DefectFieldDefinition
		var required, enabled, listVisible, filterable int
		var optionsJSON string
		if err := rows.Scan(&item.ID, &item.FieldKey, &item.Name, &item.FieldType, &item.ProjectCode,
			&required, &enabled, &listVisible, &filterable, &optionsJSON, &item.SortOrder,
			&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Required, item.Enabled = required != 0, enabled != 0
		item.ListVisible, item.Filterable = listVisible != 0, filterable != 0
		item.Options = []string{}
		_ = json.Unmarshal([]byte(optionsJSON), &item.Options)
		items = append(items, item)
	}
	return items, rows.Err()
}

func isEmptyCustomValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case []string:
		return len(typed) == 0
	}
	return false
}

func validateDefectCustomFields(ctx context.Context, projectCode string, values map[string]any) error {
	definitions, err := listDefectFieldDefinitions(ctx, projectCode, false)
	if err != nil {
		return err
	}
	known := map[string]DefectFieldDefinition{}
	for _, definition := range definitions {
		known[definition.FieldKey] = definition
		value := values[definition.FieldKey]
		if definition.Required && isEmptyCustomValue(value) {
			return fmt.Errorf("自定义字段“%s”不能为空", definition.Name)
		}
		if isEmptyCustomValue(value) {
			continue
		}
		if (definition.FieldType == "select" || definition.FieldType == "multi_select") && len(definition.Options) > 0 {
			allowed := map[string]bool{}
			for _, option := range definition.Options {
				allowed[option] = true
			}
			switch typed := value.(type) {
			case string:
				if !allowed[typed] {
					return fmt.Errorf("自定义字段“%s”的选项无效", definition.Name)
				}
			case []any:
				for _, entry := range typed {
					if !allowed[fmt.Sprint(entry)] {
						return fmt.Errorf("自定义字段“%s”的选项无效", definition.Name)
					}
				}
			}
		}
	}
	for key := range values {
		if _, ok := known[key]; !ok {
			return fmt.Errorf("自定义字段 %s 已失效，请刷新页面", key)
		}
	}
	return nil
}

func saveDefectCustomFields(ctx context.Context, tx *sql.Tx, defectID, projectCode string, values map[string]any) error {
	definitions, err := listDefectFieldDefinitions(ctx, projectCode, false)
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339Nano)
	for _, definition := range definitions {
		value, exists := values[definition.FieldKey]
		if !exists || isEmptyCustomValue(value) {
			if _, err := tx.ExecContext(ctx, "DELETE FROM defect_custom_values WHERE defect_id=? AND field_id=?", defectID, definition.ID); err != nil {
				return err
			}
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO defect_custom_values (defect_id, field_id, value_json, updated_at)
			VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE value_json=VALUES(value_json), updated_at=VALUES(updated_at)`,
			defectID, definition.ID, string(encoded), now); err != nil {
			return err
		}
	}
	return nil
}

func loadDefectCustomFields(ctx context.Context, defectID string) (map[string]any, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT d.field_key, v.value_json
		FROM defect_custom_values v JOIN defect_field_definitions d ON d.id=v.field_id
		WHERE v.defect_id=? ORDER BY d.sort_order`, defectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := map[string]any{}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			values[key] = value
		}
	}
	return values, rows.Err()
}

func hydrateDefectCustomFields(ctx context.Context, defects []Defect) error {
	if len(defects) == 0 {
		return nil
	}
	placeholders := make([]string, len(defects))
	args := make([]any, len(defects))
	indexes := map[string]int{}
	for index := range defects {
		placeholders[index] = "?"
		args[index] = defects[index].ID
		indexes[defects[index].ID] = index
		defects[index].CustomFields = map[string]any{}
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT v.defect_id, d.field_key, v.value_json
		FROM defect_custom_values v JOIN defect_field_definitions d ON d.id=v.field_id
		WHERE v.defect_id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var defectID, key, raw string
		if err := rows.Scan(&defectID, &key, &raw); err != nil {
			return err
		}
		index, ok := indexes[defectID]
		if !ok {
			continue
		}
		var value any
		if json.Unmarshal([]byte(raw), &value) == nil {
			defects[index].CustomFields[key] = value
		}
	}
	return rows.Err()
}

var defectFieldKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

func saveDefectFieldDefinition(ctx context.Context, user *User, fieldID string, input DefectFieldDefinition) (*DefectFieldDefinition, error) {
	input.FieldKey = strings.ToLower(strings.TrimSpace(input.FieldKey))
	input.Name = strings.TrimSpace(input.Name)
	input.FieldType = strings.TrimSpace(input.FieldType)
	input.ProjectCode = strings.TrimSpace(input.ProjectCode)
	if input.Name == "" || !defectFieldKeyPattern.MatchString(input.FieldKey) {
		return nil, errors.New("字段名称不能为空，字段标识必须为小写字母开头的字母、数字或下划线")
	}
	allowedTypes := map[string]bool{"text": true, "textarea": true, "number": true, "select": true, "multi_select": true, "date": true, "user": true, "boolean": true, "url": true}
	if !allowedTypes[input.FieldType] {
		return nil, errors.New("不支持的字段类型")
	}
	if (input.FieldType == "select" || input.FieldType == "multi_select") && len(input.Options) == 0 {
		return nil, errors.New("选择类型至少需要一个选项")
	}
	if input.ProjectCode != "" {
		if _, err := defectProject(input.ProjectCode); err != nil {
			return nil, err
		}
	}
	if fieldID == "" {
		fieldID = uuid.NewString()
		input.CreatedAt = time.Now().Format(time.RFC3339Nano)
		input.CreatedBy = user.ID
	} else {
		db, _, err := DatabaseManager.DB(ctx)
		if err != nil {
			return nil, err
		}
		if err := db.QueryRowContext(ctx, "SELECT created_by, created_at FROM defect_field_definitions WHERE id=?", fieldID).Scan(&input.CreatedBy, &input.CreatedAt); err != nil {
			return nil, errors.New("字段配置不存在")
		}
	}
	input.ID = fieldID
	input.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	optionsJSON, _ := json.Marshal(input.Options)
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO defect_field_definitions
		(id, field_key, name, field_type, project_code, required, enabled, list_visible, filterable,
		options_json, sort_order, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE field_key=VALUES(field_key), name=VALUES(name), field_type=VALUES(field_type),
		project_code=VALUES(project_code), required=VALUES(required), enabled=VALUES(enabled),
		list_visible=VALUES(list_visible), filterable=VALUES(filterable), options_json=VALUES(options_json),
		sort_order=VALUES(sort_order), updated_at=VALUES(updated_at)`,
		input.ID, input.FieldKey, input.Name, input.FieldType, input.ProjectCode, boolInt(input.Required), boolInt(input.Enabled),
		boolInt(input.ListVisible), boolInt(input.Filterable), string(optionsJSON), input.SortOrder, input.CreatedBy, input.CreatedAt, input.UpdatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, errors.New("同一作用范围内已存在相同字段标识")
		}
		return nil, err
	}
	return &input, nil
}

func listDefectProjectPermissions(ctx context.Context) ([]DefectProjectPermission, error) {
	projects, err := ConfigServiceInstance.GetAllProjects()
	if err != nil {
		return nil, err
	}
	accounts, err := ListAccountProfiles()
	if err != nil {
		return nil, err
	}
	accountMap := map[string]AccountProfile{}
	for _, account := range accounts {
		accountMap[account.ID] = account
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	settings := map[string]DefectProjectPermission{}
	rows, err := db.QueryContext(ctx, "SELECT project_code, permission_mode, updated_at FROM defect_project_settings")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item DefectProjectPermission
		if err := rows.Scan(&item.ProjectCode, &item.PermissionMode, &item.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		settings[item.ProjectCode] = item
	}
	rows.Close()
	memberMap := map[string][]DefectProjectMember{}
	memberRows, err := db.QueryContext(ctx, "SELECT project_code, user_id, role_key FROM defect_role_assignments ORDER BY role_key, user_id")
	if err != nil {
		return nil, err
	}
	defer memberRows.Close()
	for memberRows.Next() {
		var projectCode, userID, roleKey string
		if err := memberRows.Scan(&projectCode, &userID, &roleKey); err != nil {
			return nil, err
		}
		account := accountMap[userID]
		memberMap[projectCode] = append(memberMap[projectCode], DefectProjectMember{UserID: userID, Username: account.Username, Nickname: account.Nickname, RoleKey: roleKey})
	}
	result := make([]DefectProjectPermission, 0, len(projects))
	for _, project := range projects {
		item := settings[project.ProjectCode]
		item.ProjectCode, item.ProjectName = project.ProjectCode, project.ProjectName
		if item.PermissionMode == "" {
			item.PermissionMode = "open"
		}
		item.Members = memberMap[project.ProjectCode]
		if item.Members == nil {
			item.Members = []DefectProjectMember{}
		}
		result = append(result, item)
	}
	return result, nil
}

func saveDefectProjectPermission(ctx context.Context, user *User, projectCode string, req DefectProjectPermissionRequest) error {
	projectCode = strings.TrimSpace(projectCode)
	if _, err := defectProject(projectCode); err != nil {
		return err
	}
	if req.PermissionMode != "open" && req.PermissionMode != "restricted" {
		return errors.New("权限模式不正确")
	}
	allowedRoles := map[string]bool{"lead": true, "tester": true, "developer": true, "viewer": true}
	seen := map[string]bool{}
	for _, member := range req.Members {
		if seen[member.UserID] || !allowedRoles[member.RoleKey] {
			return errors.New("项目成员或角色配置不正确")
		}
		seen[member.UserID] = true
		if _, err := GetUserByID(member.UserID); err != nil {
			return errors.New("项目权限中包含不存在的账号")
		}
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO defect_project_settings (project_code, permission_mode, updated_by, updated_at)
		VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE permission_mode=VALUES(permission_mode), updated_by=VALUES(updated_by), updated_at=VALUES(updated_at)`,
		projectCode, req.PermissionMode, user.ID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM defect_role_assignments WHERE project_code=?", projectCode); err != nil {
		return err
	}
	for _, member := range req.Members {
		if _, err := tx.ExecContext(ctx, `INSERT INTO defect_role_assignments (project_code, user_id, role_key, updated_at)
			VALUES (?, ?, ?, ?)`, projectCode, member.UserID, member.RoleKey, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func defectStats(ctx context.Context, user *User, projectCode, dateFrom, dateTo string) (DefectStats, error) {
	stats := DefectStats{Status: []DefectDistributionItem{}, Severity: []DefectDistributionItem{}, Projects: []DefectDistributionItem{}, Assignees: []DefectDistributionItem{}, Trend: []DefectTrendItem{}}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return stats, err
	}
	where := []string{"archived=0"}
	args := []any{}
	if clause, scopeArgs := defectProjectScopeSQL(user); clause != "" {
		where = append(where, clause)
		args = append(args, scopeArgs...)
	}
	if projectCode != "" {
		where = append(where, "project_code=?")
		args = append(args, projectCode)
	}
	if dateFrom != "" {
		where = append(where, "created_at>=?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		where = append(where, "created_at<?")
		if parsed, err := time.Parse("2006-01-02", dateTo); err == nil {
			dateTo = parsed.AddDate(0, 0, 1).Format("2006-01-02")
		}
		args = append(args, dateTo)
	}
	rows, err := db.QueryContext(ctx, `SELECT status, severity, project_code, project_name, assignee_name,
		reopen_count, due_date, created_at, resolved_at, closed_at FROM defects WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return stats, err
	}
	defer rows.Close()
	statusCounts, severityCounts := map[string]int{}, map[string]int{}
	projectCounts, projectLabels, assigneeCounts := map[string]int{}, map[string]string{}, map[string]int{}
	trendMap := map[string]*DefectTrendItem{}
	now := time.Now()
	var resolutionHours float64
	var resolvedSamples int
	for rows.Next() {
		var status, project, projectName, assignee, dueDate, createdAt, resolvedAt, closedAt string
		var severity, reopenCount int
		if err := rows.Scan(&status, &severity, &project, &projectName, &assignee, &reopenCount, &dueDate, &createdAt, &resolvedAt, &closedAt); err != nil {
			return stats, err
		}
		stats.Total++
		statusCounts[status]++
		severityCounts[strconv.Itoa(severity)]++
		projectCounts[project]++
		projectLabels[project] = projectName
		if assignee == "" {
			assignee = "未指派"
		}
		assigneeCounts[assignee]++
		if reopenCount > 0 {
			stats.Reopened++
		}
		if dueDate != "" && status != DefectStatusClosed && dueDate < now.Format("2006-01-02") {
			stats.Overdue++
		}
		createdTime, createdErr := time.Parse(time.RFC3339Nano, createdAt)
		if createdErr == nil {
			day := createdTime.Format("2006-01-02")
			if trendMap[day] == nil {
				trendMap[day] = &DefectTrendItem{Date: day}
			}
			trendMap[day].Created++
			if resolvedAt != "" {
				if resolvedTime, err := time.Parse(time.RFC3339Nano, resolvedAt); err == nil && resolvedTime.After(createdTime) {
					resolutionHours += resolvedTime.Sub(createdTime).Hours()
					resolvedSamples++
				}
			}
		}
		if closedAt != "" {
			if closedTime, err := time.Parse(time.RFC3339Nano, closedAt); err == nil {
				day := closedTime.Format("2006-01-02")
				if trendMap[day] == nil {
					trendMap[day] = &DefectTrendItem{Date: day}
				}
				trendMap[day].Closed++
			}
		}
	}
	stats.New, stats.Active = statusCounts[DefectStatusNew], statusCounts[DefectStatusActive]
	stats.Resolved, stats.Closed = statusCounts[DefectStatusResolved], statusCounts[DefectStatusClosed]
	if stats.Total > 0 {
		stats.ClosureRate = float64(stats.Closed) * 100 / float64(stats.Total)
	}
	if resolvedSamples > 0 {
		stats.AverageResolveHour = resolutionHours / float64(resolvedSamples)
	}
	statusLabels := map[string]string{"new": "待确认", "active": "处理中", "resolved": "待验证", "closed": "已关闭"}
	for _, key := range []string{"new", "active", "resolved", "closed"} {
		stats.Status = append(stats.Status, DefectDistributionItem{Key: key, Label: statusLabels[key], Count: statusCounts[key]})
	}
	severityLabels := map[string]string{"1": "致命", "2": "严重", "3": "一般", "4": "轻微"}
	for _, key := range []string{"1", "2", "3", "4"} {
		stats.Severity = append(stats.Severity, DefectDistributionItem{Key: key, Label: severityLabels[key], Count: severityCounts[key]})
	}
	for key, count := range projectCounts {
		stats.Projects = append(stats.Projects, DefectDistributionItem{Key: key, Label: projectLabels[key], Count: count})
	}
	for key, count := range assigneeCounts {
		stats.Assignees = append(stats.Assignees, DefectDistributionItem{Key: key, Label: key, Count: count})
	}
	sort.Slice(stats.Projects, func(i, j int) bool { return stats.Projects[i].Count > stats.Projects[j].Count })
	sort.Slice(stats.Assignees, func(i, j int) bool { return stats.Assignees[i].Count > stats.Assignees[j].Count })
	if len(stats.Projects) > 10 {
		stats.Projects = stats.Projects[:10]
	}
	if len(stats.Assignees) > 10 {
		stats.Assignees = stats.Assignees[:10]
	}
	start := now.AddDate(0, 0, -13)
	for index := 0; index < 14; index++ {
		day := start.AddDate(0, 0, index).Format("2006-01-02")
		if trendMap[day] == nil {
			stats.Trend = append(stats.Trend, DefectTrendItem{Date: day})
		} else {
			stats.Trend = append(stats.Trend, *trendMap[day])
		}
	}
	return stats, rows.Err()
}

func batchUpdateSimple(ctx context.Context, user *User, defect *Defect, req DefectBatchRequest) error {
	if defect == nil {
		return errors.New("缺陷不存在")
	}
	if !defectProjectAllows(ctx, user, defect.ProjectCode, "process") {
		return errors.New("没有该项目的处理权限")
	}
	updated := *defect
	actionLabel := req.Action
	switch req.Action {
	case "assign":
		name, err := defectAccount(req.AssigneeID)
		if err != nil {
			return err
		}
		updated.AssigneeID, updated.AssigneeName = req.AssigneeID, name
	case "severity":
		if req.Severity < 1 || req.Severity > 4 {
			return errors.New("严重程度不正确")
		}
		updated.Severity = req.Severity
	case "priority":
		if req.Priority < 1 || req.Priority > 4 {
			return errors.New("优先级不正确")
		}
		updated.Priority = req.Priority
	case "archive":
		if !defectAllowedActions(ctx, user, *defect)["archive"] {
			return errors.New("只有缺陷管理员或项目负责人可以归档")
		}
		updated.Archived = true
	default:
		return errors.New("不支持的批量操作")
	}
	updated.RowVersion++
	updated.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE defects SET assignee_id=?, assignee_name=?, severity=?, priority=?, archived=?, row_version=?, updated_at=?
		WHERE id=? AND row_version=?`, updated.AssigneeID, updated.AssigneeName, updated.Severity, updated.Priority,
		boolInt(updated.Archived), updated.RowVersion, updated.UpdatedAt, defect.ID, defect.RowVersion)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return errors.New("缺陷已被其他人更新")
	}
	if err := insertDefectHistory(ctx, tx, defect.ID, "batch_"+actionLabel, user, defect, updated, req.Comment); err != nil {
		return err
	}
	return tx.Commit()
}

func GetDefectStatsHandler(c *gin.Context) {
	user, _ := defectUser(c)
	stats, err := defectStats(c.Request.Context(), user, strings.TrimSpace(c.Query("project_code")), strings.TrimSpace(c.Query("date_from")), strings.TrimSpace(c.Query("date_to")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func BatchDefectsHandler(c *gin.Context) {
	user, _ := defectUser(c)
	var req DefectBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 || len(req.IDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择 1 至 100 条缺陷"})
		return
	}
	permission := defectPermissionProcess
	if strings.EqualFold(strings.TrimSpace(req.Action), "close") || strings.EqualFold(strings.TrimSpace(req.Action), "reopen") {
		permission = defectPermissionVerify
	}
	allowed, err := defectPermissionAllowed(user, permission)
	if err != nil || !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "没有批量处理权限"})
		return
	}
	results := make([]DefectBatchResult, 0, len(req.IDs))
	for _, id := range req.IDs {
		defect, fetchErr := getDefect(c.Request.Context(), id)
		result := DefectBatchResult{ID: id}
		if defect != nil {
			result.DefectNo = defect.DefectNo
		}
		if fetchErr != nil || defect == nil {
			result.Error = "缺陷不存在"
			results = append(results, result)
			continue
		}
		var actionErr error
		if req.Action == "confirm" || req.Action == "resolve" || req.Action == "close" || req.Action == "reopen" {
			_, actionErr = transitionDefect(c.Request.Context(), user, id, DefectTransitionRequest{
				Action: req.Action, AssigneeID: req.AssigneeID, Resolution: req.Resolution,
				ResolvedVersion: req.ResolvedVersion, Comment: req.Comment,
			})
		} else {
			actionErr = batchUpdateSimple(c.Request.Context(), user, defect, req)
		}
		result.Success = actionErr == nil
		if actionErr != nil {
			result.Error = actionErr.Error()
		}
		results = append(results, result)
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func ListDefectFieldsHandler(c *gin.Context) {
	if _, ok := requirePermissionAdmin(c); !ok {
		return
	}
	fields, err := listDefectFieldDefinitions(c.Request.Context(), "", true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, fields)
}

func CreateDefectFieldHandler(c *gin.Context) {
	if _, ok := requirePermissionAdmin(c); !ok {
		return
	}
	user, _ := defectUser(c)
	var input DefectFieldDefinition
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "字段配置格式不正确"})
		return
	}
	field, err := saveDefectFieldDefinition(c.Request.Context(), user, "", input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, field)
}

func UpdateDefectFieldHandler(c *gin.Context) {
	if _, ok := requirePermissionAdmin(c); !ok {
		return
	}
	user, _ := defectUser(c)
	var input DefectFieldDefinition
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "字段配置格式不正确"})
		return
	}
	field, err := saveDefectFieldDefinition(c.Request.Context(), user, c.Param("field_id"), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, field)
}

func ListDefectProjectPermissionsHandler(c *gin.Context) {
	if _, ok := requirePermissionAdmin(c); !ok {
		return
	}
	items, err := listDefectProjectPermissions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func SaveDefectProjectPermissionHandler(c *gin.Context) {
	if _, ok := requirePermissionAdmin(c); !ok {
		return
	}
	user, _ := defectUser(c)
	var req DefectProjectPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目权限格式不正确"})
		return
	}
	if err := saveDefectProjectPermission(c.Request.Context(), user, c.Param("project_code"), req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
