package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	acceptanceTodoStatusPending = "pending"
	acceptanceTodoStatusDone    = "done"
	acceptanceTodoActionDone    = "acceptance_todo_done"
	acceptanceTodoReminderHour  = 10
)

type AcceptanceTodoReminder struct {
	ID                string     `json:"id"`
	ReportID          string     `json:"report_id"`
	ProjectCode       string     `json:"project_code"`
	ProjectName       string     `json:"project_name"`
	Reporter          string     `json:"reporter"`
	RecipientOpenID   string     `json:"-"`
	FeatureItems      []string   `json:"feature_items"`
	DueAt             time.Time  `json:"due_at"`
	Status            string     `json:"status"`
	LastNotifiedAt    *time.Time `json:"last_notified_at,omitempty"`
	NotificationCount int        `json:"notification_count"`
	DoneAt            *time.Time `json:"done_at,omitempty"`
	DoneByOpenID      string     `json:"-"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

var (
	acceptanceTodoLoopOnce sync.Once
	acceptanceTodoRunMu    sync.Mutex
	acceptanceTodoBulletRE = regexp.MustCompile(`^\s*(?:[-*•·]+|\d+[.)、]|[（(]?\d+[）)])\s*`)
)

func InitAcceptanceTodoReminderService() {
	acceptanceTodoLoopOnce.Do(func() {
		if err := ensureAcceptanceTodoReminderTable(); err != nil {
			log.Printf("[AcceptanceTodo] initialize table failed: %v", err)
		}
		if err := ensureManualTodoReminderTable(); err != nil {
			log.Printf("[ManualTodo] initialize table failed: %v", err)
		}
		if err := ensureTodoProjectReviewStateTable(); err != nil {
			log.Printf("[TodoReviewState] initialize table failed: %v", err)
		}

		go func() {
			runDueAcceptanceTodoReminders(time.Now())
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for now := range ticker.C {
				runDueAcceptanceTodoReminders(now)
			}
		}()
	})
}

func ensureAcceptanceTodoReminderTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS acceptance_todo_reminders (
			id VARCHAR(64) NOT NULL,
			report_id VARCHAR(128) NOT NULL,
			project_code VARCHAR(128) NOT NULL,
			project_name VARCHAR(255) NOT NULL,
			reporter VARCHAR(128) NOT NULL,
			recipient_open_id VARCHAR(128) NOT NULL DEFAULT '',
			feature_items JSON NOT NULL,
			due_at DATETIME NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			last_notified_at DATETIME NULL,
			notification_count INT NOT NULL DEFAULT 0,
			done_at DATETIME NULL,
			done_by_open_id VARCHAR(128) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_acceptance_todo_report (report_id),
			INDEX idx_acceptance_todo_due (status, due_at)
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
	`)
	return err
}

func acceptanceTodoReporterEnabled(reporter string) bool {
	return strings.EqualFold(strings.TrimSpace(reporter), "minghong")
}

func QueueAcceptanceTodoReminder(report AcceptanceReport) error {
	if !acceptanceTodoReporterEnabled(report.Reporter) {
		return nil
	}
	projectName := resolveAcceptanceTodoProjectName(report.ProjectCode, report.ProjectName)
	if !isTTminsProjectText(report.ProjectCode, projectName) {
		return nil
	}

	reportID := strings.TrimSpace(report.ID)
	if reportID == "" {
		return fmt.Errorf("TTmins acceptance report is missing id")
	}
	reporter := strings.TrimSpace(report.Reporter)
	if reporter == "" {
		return fmt.Errorf("TTmins acceptance report %s is missing its operator", reportID)
	}
	if err := ensureAcceptanceTodoReminderTable(); err != nil {
		return err
	}

	featureItems := extractAcceptanceTodoFeatures(report.UpdateRequirements)
	featureJSON, err := json.Marshal(featureItems)
	if err != nil {
		return err
	}

	now := time.Now()
	dueAt := nextAcceptanceTodoReminderTime(now)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO acceptance_todo_reminders (
			id, report_id, project_code, project_name, reporter, recipient_open_id,
			feature_items, due_at, status, notification_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, '', ?, ?, ?, 0, ?, ?)
		ON DUPLICATE KEY UPDATE
			project_code = VALUES(project_code),
			project_name = VALUES(project_name),
			reporter = VALUES(reporter),
			feature_items = VALUES(feature_items),
			updated_at = VALUES(updated_at)
	`, uuid.New().String(), reportID, strings.TrimSpace(report.ProjectCode), projectName,
		reporter, featureJSON,
		dueAt, acceptanceTodoStatusPending, now, now)
	if err != nil {
		return err
	}

	log.Printf("[AcceptanceTodo] queued TTmins follow-up: report=%s project=%s due=%s",
		reportID, strings.TrimSpace(report.ProjectCode), dueAt.Format("2006-01-02 15:04:05"))
	return nil
}

func resolveAcceptanceTodoProjectName(projectCode string, fallback string) string {
	if value := strings.TrimSpace(fallback); value != "" {
		return value
	}
	code := strings.TrimSpace(projectCode)
	if ConfigServiceInstance != nil {
		projects, err := ConfigServiceInstance.GetAllProjects()
		if err == nil {
			for _, project := range projects {
				if strings.EqualFold(strings.TrimSpace(project.ProjectCode), code) {
					if name := strings.TrimSpace(project.ProjectName); name != "" {
						return name
					}
				}
			}
		}
	}
	return code
}

func isTTminsProjectText(projectCode string, projectName string) bool {
	value := strings.ToLower(strings.TrimSpace(projectCode) + " " + strings.TrimSpace(projectName))
	return strings.Contains(value, "ttmins")
}

func extractAcceptanceTodoFeatures(value string) []string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	parts := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '\n' || r == '；' || r == ';'
	})

	seen := map[string]bool{}
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := acceptanceTodoBulletRE.ReplaceAllString(strings.TrimSpace(part), "")
		item = strings.TrimSpace(strings.TrimPrefix(item, "版本更新测试需求点："))
		item = strings.TrimSpace(strings.TrimPrefix(item, "版本更新测试需求点:"))
		if item == "" {
			continue
		}
		lower := strings.ToLower(item)
		if lower == "无" || lower == "暂无" || lower == "none" || lower == "n/a" || lower == "na" {
			continue
		}
		key := strings.ToLower(strings.Join(strings.Fields(item), ""))
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
	}

	if len(items) == 0 {
		return []string{"验收报告未填写版本更新测试需求点，请按本次验收范围复核"}
	}
	return items
}

func nextAcceptanceTodoReminderTime(now time.Time) time.Time {
	loc := acceptanceTodoLocation()
	localNow := now.In(loc)
	nextDay := localNow.AddDate(0, 0, 1)
	return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), acceptanceTodoReminderHour, 0, 0, 0, loc)
}

func acceptanceTodoLocation() *time.Location {
	if shanghai, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return shanghai
	}
	return time.Local
}

func runDueAcceptanceTodoReminders(now time.Time) {
	if !acceptanceTodoRunMu.TryLock() {
		return
	}
	defer acceptanceTodoRunMu.Unlock()

	if reminders, err := listPendingAcceptanceTodoReminders(now); err != nil {
		log.Printf("[AcceptanceTodo] load due reminders failed: %v", err)
	} else {
		for _, group := range groupAcceptanceTodoRemindersByReporter(reminders) {
			if !acceptanceTodoReporterEnabled(group[0].Reporter) {
				continue
			}
			// 未绑定飞书的用户只保留平台待办，不发起卡片请求，也不记录发送失败。
			// 待用户完成绑定后，后续调度会自动重新识别并发送仍处于 pending 的待办。
			if resolveExactFeishuOpenID(group[0].Reporter) == "" {
				continue
			}
			if err := sendAcceptanceTodoReminderGroup(group, now); err != nil {
				log.Printf("[AcceptanceTodo] reminder group for %s send failed: %v", group[0].Reporter, err)
			}
		}
	}
	if err := runDueManualTodoReminders(now); err != nil {
		log.Printf("[ManualTodo] load due reminders failed: %v", err)
	}
}

func groupAcceptanceTodoRemindersByReporter(reminders []AcceptanceTodoReminder) [][]AcceptanceTodoReminder {
	groups := make([][]AcceptanceTodoReminder, 0)
	indexes := make(map[string]int)
	for _, reminder := range reminders {
		key := strings.ToLower(strings.TrimSpace(reminder.Reporter))
		if index, ok := indexes[key]; ok {
			groups[index] = append(groups[index], reminder)
			continue
		}
		indexes[key] = len(groups)
		groups = append(groups, []AcceptanceTodoReminder{reminder})
	}
	return groups
}

func listPendingAcceptanceTodoReminders(now time.Time) ([]AcceptanceTodoReminder, error) {
	if err := ensureAcceptanceTodoReminderTable(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, report_id, project_code, project_name, reporter, recipient_open_id,
			feature_items, due_at, status, last_notified_at, notification_count,
			done_at, done_by_open_id, created_at, updated_at
		FROM acceptance_todo_reminders
		WHERE status = ? AND due_at <= ?
		ORDER BY due_at ASC
	`, acceptanceTodoStatusPending, now.In(acceptanceTodoLocation()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reminders := make([]AcceptanceTodoReminder, 0)
	for rows.Next() {
		reminder, err := scanAcceptanceTodoReminder(rows)
		if err != nil {
			return nil, err
		}
		if shouldSendAcceptanceTodoReminder(reminder, now) {
			reminders = append(reminders, reminder)
		}
	}
	return reminders, rows.Err()
}

type acceptanceTodoScanner interface {
	Scan(dest ...any) error
}

func scanAcceptanceTodoReminder(scanner acceptanceTodoScanner) (AcceptanceTodoReminder, error) {
	var reminder AcceptanceTodoReminder
	var featureJSON []byte
	var lastNotifiedAt sql.NullTime
	var doneAt sql.NullTime
	err := scanner.Scan(
		&reminder.ID,
		&reminder.ReportID,
		&reminder.ProjectCode,
		&reminder.ProjectName,
		&reminder.Reporter,
		&reminder.RecipientOpenID,
		&featureJSON,
		&reminder.DueAt,
		&reminder.Status,
		&lastNotifiedAt,
		&reminder.NotificationCount,
		&doneAt,
		&reminder.DoneByOpenID,
		&reminder.CreatedAt,
		&reminder.UpdatedAt,
	)
	if err != nil {
		return reminder, err
	}
	if err := json.Unmarshal(featureJSON, &reminder.FeatureItems); err != nil {
		return reminder, err
	}
	if lastNotifiedAt.Valid {
		value := lastNotifiedAt.Time
		reminder.LastNotifiedAt = &value
	}
	if doneAt.Valid {
		value := doneAt.Time
		reminder.DoneAt = &value
	}
	return reminder, nil
}

func shouldSendAcceptanceTodoReminder(reminder AcceptanceTodoReminder, now time.Time) bool {
	if reminder.Status != acceptanceTodoStatusPending || reminder.DueAt.After(now) {
		return false
	}
	loc := acceptanceTodoLocation()
	current := now.In(loc)
	// Overdue reminders also wait until the daily reminder hour, not midnight.
	if current.Hour() < acceptanceTodoReminderHour {
		return false
	}
	if reminder.LastNotifiedAt == nil {
		return true
	}
	last := reminder.LastNotifiedAt.In(loc)
	return last.Year() != current.Year() || last.YearDay() != current.YearDay()
}

func sendAcceptanceTodoReminder(reminder AcceptanceTodoReminder, now time.Time) error {
	return sendAcceptanceTodoReminderGroup([]AcceptanceTodoReminder{reminder}, now)
}

func sendAcceptanceTodoReminderGroup(reminders []AcceptanceTodoReminder, now time.Time) error {
	if len(reminders) == 0 {
		return fmt.Errorf("acceptance reminder group is empty")
	}
	reporter := strings.TrimSpace(reminders[0].Reporter)
	if !acceptanceTodoReporterEnabled(reporter) {
		return fmt.Errorf("acceptance todo reminders are only enabled for minghong")
	}
	for _, reminder := range reminders {
		if !strings.EqualFold(strings.TrimSpace(reminder.Reporter), reporter) {
			return fmt.Errorf("acceptance reminder group contains multiple operators")
		}
	}

	// Always resolve from the current platform binding. The stored open_id is an
	// audit snapshot of the last delivery, not a routing fallback.
	openID := resolveExactFeishuOpenID(reporter)
	if openID == "" {
		return fmt.Errorf("reporter %s has no bound Feishu account", reporter)
	}

	if err := sendFeishuOpenIDInteractiveCard(openID, BuildAcceptanceTodoReminderGroupCard(reminders)); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, reminder := range reminders {
		if reminder.Status == acceptanceTodoStatusDone {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE acceptance_todo_reminders
			SET recipient_open_id = ?, last_notified_at = ?,
				notification_count = notification_count + 1, updated_at = ?
			WHERE id = ? AND status = ?
		`, openID, now, now, reminder.ID, acceptanceTodoStatusPending); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("[AcceptanceTodo] reminder group sent: reports=%d recipient=%s", len(reminders), reporter)
	return nil
}

// SendAcceptanceTodoReminderNowForReport queues a TTmins report and immediately
// sends its reminder to the Feishu account bound to the report operator. This is
// intended for controlled verification or manual recovery; normal reminders are
// still delivered by the scheduler starting on the following day.
func SendAcceptanceTodoReminderNowForReport(reportID string) error {
	reportID = strings.TrimSpace(reportID)
	if reportID == "" {
		return fmt.Errorf("report_id is required")
	}

	report, err := GetAcceptanceReportByID(reportID)
	if err != nil {
		return err
	}
	projectName := resolveAcceptanceTodoProjectName(report.ProjectCode, report.ProjectName)
	if !isTTminsProjectText(report.ProjectCode, projectName) {
		return fmt.Errorf("acceptance report %s is not a TTmins project", reportID)
	}

	reporter := strings.TrimSpace(report.Reporter)
	if reporter == "" {
		return fmt.Errorf("acceptance report %s has no operator", reportID)
	}
	if resolveExactFeishuOpenID(reporter) == "" {
		return fmt.Errorf("report operator %s has no exactly matched bound Feishu account", reporter)
	}

	if err := QueueAcceptanceTodoReminder(*report); err != nil {
		return err
	}
	if err := ensureAcceptanceTodoReminderTable(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	reminder, err := getAcceptanceTodoReminderByReportID(ctx, db, reportID)
	if err != nil {
		return err
	}
	if reminder.Status == acceptanceTodoStatusDone {
		return fmt.Errorf("acceptance reminder for report %s is already completed", reportID)
	}
	return sendAcceptanceTodoReminder(reminder, time.Now())
}

// SendAcceptanceTodoRemindersNowForReporterDate backfills all TTmins reports
// submitted by one platform operator on a date and sends them as one card.
func SendAcceptanceTodoRemindersNowForReporterDate(reporter string, reportDate string) (int, error) {
	reporter = strings.TrimSpace(reporter)
	reportDate = strings.TrimSpace(reportDate)
	if reporter == "" || reportDate == "" {
		return 0, fmt.Errorf("reporter and report_date are required")
	}
	if _, err := time.ParseInLocation("2006-01-02", reportDate, acceptanceTodoLocation()); err != nil {
		return 0, fmt.Errorf("invalid report_date %q: %w", reportDate, err)
	}
	if resolveExactFeishuOpenID(reporter) == "" {
		return 0, fmt.Errorf("report operator %s has no exactly matched bound Feishu account", reporter)
	}

	reports, err := GetAcceptanceReports()
	if err != nil {
		return 0, err
	}
	reportIDs := make([]string, 0)
	for _, report := range reports {
		if !strings.EqualFold(strings.TrimSpace(report.Reporter), reporter) || !acceptanceReportCreatedOn(report, reportDate) {
			continue
		}
		projectName := resolveAcceptanceTodoProjectName(report.ProjectCode, report.ProjectName)
		if !isTTminsProjectText(report.ProjectCode, projectName) {
			continue
		}
		if err := QueueAcceptanceTodoReminder(report); err != nil {
			return 0, err
		}
		reportIDs = append(reportIDs, strings.TrimSpace(report.ID))
	}
	if len(reportIDs) == 0 {
		return 0, fmt.Errorf("no TTmins acceptance reports found for %s on %s", reporter, reportDate)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return 0, err
	}
	reminders := make([]AcceptanceTodoReminder, 0, len(reportIDs))
	for _, reportID := range reportIDs {
		reminder, err := getAcceptanceTodoReminderByReportID(ctx, db, reportID)
		if err != nil {
			return 0, err
		}
		reminders = append(reminders, reminder)
	}
	if err := sendAcceptanceTodoReminderGroup(reminders, time.Now()); err != nil {
		return 0, err
	}
	return len(reminders), nil
}

func acceptanceReportCreatedOn(report AcceptanceReport, reportDate string) bool {
	createdAt := strings.TrimSpace(report.CreatedAt)
	return len(createdAt) >= len(reportDate) && createdAt[:len(reportDate)] == reportDate
}

func resolveExactFeishuOpenID(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ""
	}
	user, err := findUserByUsername(username)
	if err == nil && user != nil {
		return strings.TrimSpace(user.FeishuOpenID)
	}
	return ""
}

func CompleteAcceptanceTodoReminder(reminderID string, operatorOpenID string) (AcceptanceTodoReminder, error) {
	reminderID = strings.TrimSpace(reminderID)
	operatorOpenID = strings.TrimSpace(operatorOpenID)
	if reminderID == "" {
		return AcceptanceTodoReminder{}, fmt.Errorf("reminder_id is required")
	}
	if operatorOpenID == "" {
		return AcceptanceTodoReminder{}, fmt.Errorf("无法识别待办操作人")
	}
	if err := ensureAcceptanceTodoReminderTable(); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if err := ensureTodoProjectReviewStateTable(); err != nil {
		return AcceptanceTodoReminder{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	reminder, err := getAcceptanceTodoReminder(ctx, db, reminderID)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if reminder.RecipientOpenID != "" && !strings.EqualFold(reminder.RecipientOpenID, operatorOpenID) {
		return AcceptanceTodoReminder{}, fmt.Errorf("仅提醒接收人可以完成该待办")
	}
	if reminder.Status == acceptanceTodoStatusDone {
		return reminder, nil
	}
	if reminder.Status != acceptanceTodoStatusPending {
		return AcceptanceTodoReminder{}, fmt.Errorf("该待办已处理")
	}

	now := time.Now()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE acceptance_todo_reminders
		SET status = ?, done_at = ?, done_by_open_id = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, acceptanceTodoStatusDone, now, operatorOpenID, now, reminderID, acceptanceTodoStatusPending)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return AcceptanceTodoReminder{}, fmt.Errorf("该待办已处理")
	}
	if err := clearTodoProjectNotApproved(ctx, tx, reminder.Reporter, reminder.ProjectCode); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if err := tx.Commit(); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	return getAcceptanceTodoReminder(ctx, db, reminderID)
}

func ListPendingAcceptanceTodoRemindersForReporter(reporter string) ([]AcceptanceTodoReminder, error) {
	reporter = strings.TrimSpace(reporter)
	if reporter == "" {
		return nil, fmt.Errorf("reporter is required")
	}
	if err := ensureAcceptanceTodoReminderTable(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, report_id, project_code, project_name, reporter, recipient_open_id,
			feature_items, due_at, status, last_notified_at, notification_count,
			done_at, done_by_open_id, created_at, updated_at
		FROM acceptance_todo_reminders
		WHERE LOWER(reporter) = LOWER(?) AND status = ?
		ORDER BY due_at ASC, created_at ASC
	`, reporter, acceptanceTodoStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reminders := make([]AcceptanceTodoReminder, 0)
	for rows.Next() {
		reminder, err := scanAcceptanceTodoReminder(rows)
		if err != nil {
			return nil, err
		}
		reminders = append(reminders, reminder)
	}
	return reminders, rows.Err()
}

func CompleteAcceptanceTodoReminderForReporter(reminderID string, reporter string, operatorOpenID string) (AcceptanceTodoReminder, error) {
	reminderID = strings.TrimSpace(reminderID)
	reporter = strings.TrimSpace(reporter)
	if reminderID == "" || reporter == "" {
		return AcceptanceTodoReminder{}, fmt.Errorf("reminder_id and reporter are required")
	}
	if err := ensureAcceptanceTodoReminderTable(); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if err := ensureTodoProjectReviewStateTable(); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	reminder, err := getAcceptanceTodoReminder(ctx, db, reminderID)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(reminder.Reporter), reporter) {
		return AcceptanceTodoReminder{}, fmt.Errorf("只能完成自己的待办")
	}
	if reminder.Status == acceptanceTodoStatusDone {
		return reminder, nil
	}
	if reminder.Status != acceptanceTodoStatusPending {
		return AcceptanceTodoReminder{}, fmt.Errorf("该待办已处理")
	}
	if strings.TrimSpace(operatorOpenID) == "" {
		operatorOpenID = "platform:" + reporter
	}
	now := time.Now()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE acceptance_todo_reminders
		SET status = ?, done_at = ?, done_by_open_id = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, acceptanceTodoStatusDone, now, operatorOpenID, now, reminderID, acceptanceTodoStatusPending)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return AcceptanceTodoReminder{}, fmt.Errorf("该待办已处理")
	}
	if err := clearTodoProjectNotApproved(ctx, tx, reporter, reminder.ProjectCode); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if err := tx.Commit(); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	return getAcceptanceTodoReminder(ctx, db, reminderID)
}

func MarkAcceptanceTodoNotApprovedForReporter(reminderID string, reporter string, operatorOpenID string) (AcceptanceTodoReminder, error) {
	reminderID = strings.TrimSpace(reminderID)
	reporter = strings.TrimSpace(reporter)
	if reminderID == "" || reporter == "" {
		return AcceptanceTodoReminder{}, fmt.Errorf("reminder_id and reporter are required")
	}
	if err := ensureAcceptanceTodoReminderTable(); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if err := ensureTodoProjectReviewStateTable(); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	reminder, err := getAcceptanceTodoReminder(ctx, db, reminderID)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(reminder.Reporter), reporter) {
		return AcceptanceTodoReminder{}, fmt.Errorf("只能处理自己的待办")
	}
	if reminder.Status == acceptanceTodoStatusNotApproved {
		return reminder, nil
	}
	if reminder.Status != acceptanceTodoStatusPending {
		return AcceptanceTodoReminder{}, fmt.Errorf("该待办已处理")
	}
	if strings.TrimSpace(operatorOpenID) == "" {
		operatorOpenID = "platform:" + reporter
	}
	now := time.Now().In(acceptanceTodoLocation())
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE acceptance_todo_reminders
		SET status = ?, done_at = ?, done_by_open_id = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, acceptanceTodoStatusNotApproved, now, operatorOpenID, now, reminderID, acceptanceTodoStatusPending)
	if err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return AcceptanceTodoReminder{}, fmt.Errorf("该待办已处理")
	}
	if err := markTodoProjectNotApproved(ctx, tx, reporter, reminder.ProjectCode, myTodoTypeAcceptance, reminder.ID, now); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	if err := tx.Commit(); err != nil {
		return AcceptanceTodoReminder{}, err
	}
	return getAcceptanceTodoReminder(ctx, db, reminderID)
}

func ListMyAcceptanceTodoRemindersHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
		return
	}
	reminders, err := ListPendingAcceptanceTodoRemindersForReporter(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": reminders, "count": len(reminders)})
}

func CompleteMyAcceptanceTodoReminderHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
		return
	}
	reminder, err := CompleteAcceptanceTodoReminderForReporter(c.Param("id"), user.Username, user.FeishuOpenID)
	if err != nil {
		if strings.Contains(err.Error(), "自己的待办") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "待办已完成，后续不再提醒", "item": reminder})
}

func getAcceptanceTodoReminder(ctx context.Context, db *sql.DB, reminderID string) (AcceptanceTodoReminder, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, report_id, project_code, project_name, reporter, recipient_open_id,
			feature_items, due_at, status, last_notified_at, notification_count,
			done_at, done_by_open_id, created_at, updated_at
		FROM acceptance_todo_reminders
		WHERE id = ?
		LIMIT 1
	`, reminderID)
	return scanAcceptanceTodoReminder(row)
}

func getAcceptanceTodoReminderByReportID(ctx context.Context, db *sql.DB, reportID string) (AcceptanceTodoReminder, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, report_id, project_code, project_name, reporter, recipient_open_id,
			feature_items, due_at, status, last_notified_at, notification_count,
			done_at, done_by_open_id, created_at, updated_at
		FROM acceptance_todo_reminders
		WHERE report_id = ?
		LIMIT 1
	`, reportID)
	return scanAcceptanceTodoReminder(row)
}

// GetAcceptanceTodoRemindersForCard reloads a card group after a Done action.
// Only reminders delivered to the current Feishu operator are returned.
func GetAcceptanceTodoRemindersForCard(reminderIDs []string, operatorOpenID string) ([]AcceptanceTodoReminder, error) {
	operatorOpenID = strings.TrimSpace(operatorOpenID)
	if operatorOpenID == "" {
		return nil, fmt.Errorf("无法识别待办操作人")
	}
	if len(reminderIDs) == 0 || len(reminderIDs) > 50 {
		return nil, fmt.Errorf("invalid reminder group")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	reminders := make([]AcceptanceTodoReminder, 0, len(reminderIDs))
	seen := make(map[string]bool)
	for _, reminderID := range reminderIDs {
		reminderID = strings.TrimSpace(reminderID)
		if reminderID == "" || seen[reminderID] {
			continue
		}
		seen[reminderID] = true
		reminder, err := getAcceptanceTodoReminder(ctx, db, reminderID)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(strings.TrimSpace(reminder.RecipientOpenID), operatorOpenID) {
			return nil, fmt.Errorf("待办分组包含不属于当前操作人的项目")
		}
		reminders = append(reminders, reminder)
	}
	if len(reminders) == 0 {
		return nil, fmt.Errorf("待办分组为空")
	}
	return reminders, nil
}

func BuildAcceptanceTodoReminderCard(reminder AcceptanceTodoReminder, completed bool) map[string]any {
	if completed {
		reminder.Status = acceptanceTodoStatusDone
	}
	return BuildAcceptanceTodoReminderGroupCard([]AcceptanceTodoReminder{reminder})
}

func BuildAcceptanceTodoReminderGroupCard(reminders []AcceptanceTodoReminder) map[string]any {
	pendingCount := 0
	for _, reminder := range reminders {
		if reminder.Status != acceptanceTodoStatusDone {
			pendingCount++
		}
	}

	headerTemplate := "indigo"
	headerTitle := fmt.Sprintf("TTmins 过审跟进提醒 · %d 个项目", len(reminders))
	intro := fmt.Sprintf("今天您需要跟进以下 **%d 个项目** 的过审进程：", len(reminders))
	footerText := fmt.Sprintf("剩余 %d 项未完成 · 未完成项每天提醒一次", pendingCount)
	if pendingCount == 0 {
		headerTemplate = "green"
		headerTitle = "TTmins 过审跟进 · 全部完成"
		intro = fmt.Sprintf("✅ 以下 **%d 个项目** 的过审跟进均已完成。", len(reminders))
		footerText = "全部待办已完成，后续不再提醒"
	}

	elements := []map[string]any{
		{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": intro,
			},
		},
	}
	for index, reminder := range reminders {
		projectLabel := strings.TrimSpace(reminder.ProjectName)
		if projectLabel == "" {
			projectLabel = strings.TrimSpace(reminder.ProjectCode)
		}
		codeSuffix := ""
		if reminder.ProjectCode != "" && !strings.EqualFold(reminder.ProjectCode, projectLabel) {
			codeSuffix = "（" + reminder.ProjectCode + "）"
		}
		featureLines := make([]string, 0, len(reminder.FeatureItems))
		for _, item := range reminder.FeatureItems {
			featureLines = append(featureLines, "• "+item)
		}
		statusPrefix := ""
		if reminder.Status == acceptanceTodoStatusDone {
			statusPrefix = "✅ "
		}
		elements = append(elements,
			map[string]any{"tag": "hr"},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": fmt.Sprintf("%s**%d. %s%s**\n**待验证功能**\n%s", statusPrefix, index+1, projectLabel, codeSuffix, strings.Join(featureLines, "\n")),
				},
			},
		)
	}
	if pendingCount > 0 {
		elements = append(elements, map[string]any{
			"tag": "action",
			"actions": []map[string]any{
				{
					"tag": "button",
					"text": map[string]any{
						"tag":     "plain_text",
						"content": "查看我的待办",
					},
					"type": "primary",
					"url":  PlatformFrontendURL("/my-todos"),
				},
			},
		})
	}
	elements = append(elements, map[string]any{
		"tag": "note",
		"elements": []map[string]any{
			{"tag": "plain_text", "content": footerText},
		},
	})

	return map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
			"update_multi":     false,
		},
		"header": map[string]any{
			"template": headerTemplate,
			"title": map[string]any{
				"tag":     "plain_text",
				"content": headerTitle,
			},
		},
		"elements": elements,
	}
}
