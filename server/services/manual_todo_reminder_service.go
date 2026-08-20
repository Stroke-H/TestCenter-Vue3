package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	myTodoTypeAcceptance = "acceptance"
	myTodoTypeManual     = "manual"
)

type ManualTodoReminder struct {
	ID                string     `json:"id"`
	OwnerUsername     string     `json:"owner_username"`
	ProjectCode       string     `json:"project_code"`
	ProjectName       string     `json:"project_name"`
	EventContent      string     `json:"event_content"`
	DueAt             time.Time  `json:"due_at"`
	Status            string     `json:"status"`
	RecipientOpenID   string     `json:"-"`
	LastNotifiedAt    *time.Time `json:"last_notified_at,omitempty"`
	NotificationCount int        `json:"notification_count"`
	DoneAt            *time.Time `json:"done_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// MyTodoItem is the common response shape used by the personal todo page.
// Acceptance follow-ups and user-created reminders stay separately persisted.
type MyTodoItem struct {
	ID                string     `json:"id"`
	TodoType          string     `json:"todo_type"`
	ReportID          string     `json:"report_id,omitempty"`
	ProjectCode       string     `json:"project_code"`
	ProjectName       string     `json:"project_name"`
	Reporter          string     `json:"reporter"`
	FeatureItems      []string   `json:"feature_items"`
	DueAt             time.Time  `json:"due_at"`
	Status            string     `json:"status"`
	LastNotifiedAt    *time.Time `json:"last_notified_at,omitempty"`
	NotificationCount int        `json:"notification_count"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type CreateManualTodoRequest struct {
	ProjectCode  string `json:"project_code"`
	EventContent string `json:"event_content"`
	DueAt        string `json:"due_at"`
}

func ensureManualTodoReminderTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS manual_todo_reminders (
			id VARCHAR(64) NOT NULL,
			owner_username VARCHAR(128) NOT NULL,
			project_code VARCHAR(128) NOT NULL,
			project_name VARCHAR(255) NOT NULL,
			event_content TEXT NOT NULL,
			due_at DATETIME NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			recipient_open_id VARCHAR(128) NOT NULL DEFAULT '',
			last_notified_at DATETIME NULL,
			notification_count INT NOT NULL DEFAULT 0,
			done_at DATETIME NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (id),
			INDEX idx_manual_todo_owner (owner_username, status),
			INDEX idx_manual_todo_due (status, due_at, notification_count)
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
	`)
	return err
}

func parseManualTodoDueAt(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("请选择提醒时间")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.In(acceptanceTodoLocation()), nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if parsed, err := time.ParseInLocation(layout, value, acceptanceTodoLocation()); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("提醒时间格式不正确")
}

func resolveManualTodoProject(projectCode string) (string, string, error) {
	projectCode = strings.TrimSpace(projectCode)
	if projectCode == "" {
		return "", "", fmt.Errorf("请选择项目")
	}
	if ConfigServiceInstance == nil {
		return "", "", fmt.Errorf("项目配置服务尚未初始化")
	}
	projects, err := ConfigServiceInstance.GetAllProjects()
	if err != nil {
		return "", "", err
	}
	for _, project := range projects {
		if strings.EqualFold(strings.TrimSpace(project.ProjectCode), projectCode) ||
			strings.EqualFold(strings.TrimSpace(project.ID), projectCode) {
			code := strings.TrimSpace(project.ProjectCode)
			name := strings.TrimSpace(project.ProjectName)
			if code == "" {
				code = strings.TrimSpace(project.ID)
			}
			if name == "" {
				name = code
			}
			return code, name, nil
		}
	}
	return "", "", fmt.Errorf("所选项目不存在或已被删除")
}

func CreateManualTodoReminder(ownerUsername string, request CreateManualTodoRequest, now time.Time) (ManualTodoReminder, error) {
	ownerUsername = strings.TrimSpace(ownerUsername)
	if ownerUsername == "" {
		return ManualTodoReminder{}, fmt.Errorf("无法识别待办创建人")
	}
	eventContent := strings.TrimSpace(request.EventContent)
	if eventContent == "" {
		return ManualTodoReminder{}, fmt.Errorf("请填写待办事件")
	}
	if len([]rune(eventContent)) > 2000 {
		return ManualTodoReminder{}, fmt.Errorf("待办事件不能超过 2000 个字符")
	}
	dueAt, err := parseManualTodoDueAt(request.DueAt)
	if err != nil {
		return ManualTodoReminder{}, err
	}
	if !dueAt.After(now.In(acceptanceTodoLocation())) {
		return ManualTodoReminder{}, fmt.Errorf("提醒时间必须晚于当前时间")
	}
	projectCode, projectName, err := resolveManualTodoProject(request.ProjectCode)
	if err != nil {
		return ManualTodoReminder{}, err
	}
	if err := ensureManualTodoReminderTable(); err != nil {
		return ManualTodoReminder{}, err
	}

	createdAt := now.In(acceptanceTodoLocation())
	reminder := ManualTodoReminder{
		ID: uuid.New().String(), OwnerUsername: ownerUsername,
		ProjectCode: projectCode, ProjectName: projectName,
		EventContent: eventContent, DueAt: dueAt,
		Status: acceptanceTodoStatusPending, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return ManualTodoReminder{}, err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO manual_todo_reminders (
			id, owner_username, project_code, project_name, event_content, due_at,
			status, recipient_open_id, notification_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, '', 0, ?, ?)
	`, reminder.ID, reminder.OwnerUsername, reminder.ProjectCode, reminder.ProjectName,
		reminder.EventContent, reminder.DueAt, reminder.Status, reminder.CreatedAt, reminder.UpdatedAt)
	if err != nil {
		return ManualTodoReminder{}, err
	}
	return reminder, nil
}

func runDueManualTodoReminders(now time.Time) error {
	reminders, err := listDueManualTodoReminders(now)
	if err != nil {
		return err
	}
	for _, reminder := range reminders {
		if err := sendManualTodoReminder(reminder, now); err != nil {
			// Keep notification_count at zero so the next minute can retry.
			log.Printf("[ManualTodo] reminder for %s send failed: %v", reminder.OwnerUsername, err)
		}
	}
	return nil
}

func listDueManualTodoReminders(now time.Time) ([]ManualTodoReminder, error) {
	if err := ensureManualTodoReminderTable(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, owner_username, project_code, project_name, event_content, due_at,
			status, recipient_open_id, last_notified_at, notification_count,
			done_at, created_at, updated_at
		FROM manual_todo_reminders
		WHERE status = ? AND due_at <= ? AND notification_count = 0
		ORDER BY due_at ASC
	`, acceptanceTodoStatusPending, now.In(acceptanceTodoLocation()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ManualTodoReminder, 0)
	for rows.Next() {
		item, err := scanManualTodoReminder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type manualTodoScanner interface {
	Scan(dest ...any) error
}

func scanManualTodoReminder(scanner manualTodoScanner) (ManualTodoReminder, error) {
	var reminder ManualTodoReminder
	var lastNotifiedAt sql.NullTime
	var doneAt sql.NullTime
	err := scanner.Scan(
		&reminder.ID, &reminder.OwnerUsername, &reminder.ProjectCode, &reminder.ProjectName,
		&reminder.EventContent, &reminder.DueAt, &reminder.Status, &reminder.RecipientOpenID,
		&lastNotifiedAt, &reminder.NotificationCount, &doneAt, &reminder.CreatedAt, &reminder.UpdatedAt,
	)
	if err != nil {
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

func sendManualTodoReminder(reminder ManualTodoReminder, now time.Time) error {
	openID := resolveExactFeishuOpenID(reminder.OwnerUsername)
	if openID == "" {
		return fmt.Errorf("用户 %s 未绑定飞书账号", reminder.OwnerUsername)
	}
	if err := sendFeishuOpenIDInteractiveCard(openID, BuildManualTodoReminderCard(reminder)); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		UPDATE manual_todo_reminders
		SET recipient_open_id = ?, last_notified_at = ?, notification_count = 1, updated_at = ?
		WHERE id = ? AND status = ? AND notification_count = 0
	`, openID, now, now, reminder.ID, acceptanceTodoStatusPending)
	return err
}

func BuildManualTodoReminderCard(reminder ManualTodoReminder) map[string]any {
	project := strings.TrimSpace(reminder.ProjectName)
	if project == "" {
		project = strings.TrimSpace(reminder.ProjectCode)
	}
	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true, "update_multi": false},
		"header": map[string]any{
			"template": "indigo",
			"title":    map[string]any{"tag": "plain_text", "content": "个人待办提醒"},
		},
		"elements": []map[string]any{
			{
				"tag": "div",
				"text": map[string]any{
					"tag": "lark_md",
					"content": fmt.Sprintf("**项目：** %s（%s）\n**待办事件：**\n%s\n**计划时间：** %s",
						project, reminder.ProjectCode, reminder.EventContent,
						reminder.DueAt.In(acceptanceTodoLocation()).Format("2006-01-02 15:04")),
				},
			},
			{
				"tag": "action",
				"actions": []map[string]any{{
					"tag": "button", "type": "primary",
					"text": map[string]any{"tag": "plain_text", "content": "查看我的待办"},
					"url":  PlatformFrontendURL("/my-todos"),
				}},
			},
			{
				"tag":      "note",
				"elements": []map[string]any{{"tag": "plain_text", "content": "该提醒仅发送一次，待办需在平台内手动标记完成"}},
			},
		},
	}
}

func ListPendingManualTodoRemindersForOwner(ownerUsername string) ([]ManualTodoReminder, error) {
	ownerUsername = strings.TrimSpace(ownerUsername)
	if ownerUsername == "" {
		return nil, fmt.Errorf("owner_username is required")
	}
	if err := ensureManualTodoReminderTable(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, owner_username, project_code, project_name, event_content, due_at,
			status, recipient_open_id, last_notified_at, notification_count,
			done_at, created_at, updated_at
		FROM manual_todo_reminders
		WHERE LOWER(owner_username) = LOWER(?) AND status = ?
		ORDER BY due_at ASC, created_at ASC
	`, ownerUsername, acceptanceTodoStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ManualTodoReminder, 0)
	for rows.Next() {
		item, err := scanManualTodoReminder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func CompleteManualTodoReminderForOwner(reminderID string, ownerUsername string) (ManualTodoReminder, error) {
	reminderID = strings.TrimSpace(reminderID)
	ownerUsername = strings.TrimSpace(ownerUsername)
	if reminderID == "" || ownerUsername == "" {
		return ManualTodoReminder{}, fmt.Errorf("待办编号和操作人不能为空")
	}
	if err := ensureManualTodoReminderTable(); err != nil {
		return ManualTodoReminder{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return ManualTodoReminder{}, err
	}
	reminder, err := getManualTodoReminder(ctx, db, reminderID)
	if err != nil {
		return ManualTodoReminder{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(reminder.OwnerUsername), ownerUsername) {
		return ManualTodoReminder{}, fmt.Errorf("只能完成自己的待办")
	}
	if reminder.Status == acceptanceTodoStatusDone {
		return reminder, nil
	}
	now := time.Now().In(acceptanceTodoLocation())
	if _, err := db.ExecContext(ctx, `
		UPDATE manual_todo_reminders SET status = ?, done_at = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, acceptanceTodoStatusDone, now, now, reminderID, acceptanceTodoStatusPending); err != nil {
		return ManualTodoReminder{}, err
	}
	return getManualTodoReminder(ctx, db, reminderID)
}

func getManualTodoReminder(ctx context.Context, db *sql.DB, reminderID string) (ManualTodoReminder, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, owner_username, project_code, project_name, event_content, due_at,
			status, recipient_open_id, last_notified_at, notification_count,
			done_at, created_at, updated_at
		FROM manual_todo_reminders WHERE id = ? LIMIT 1
	`, reminderID)
	return scanManualTodoReminder(row)
}

func acceptanceTodoToMyTodo(reminder AcceptanceTodoReminder) MyTodoItem {
	return MyTodoItem{
		ID: reminder.ID, TodoType: myTodoTypeAcceptance, ReportID: reminder.ReportID,
		ProjectCode: reminder.ProjectCode, ProjectName: reminder.ProjectName,
		Reporter: reminder.Reporter, FeatureItems: reminder.FeatureItems,
		DueAt: reminder.DueAt, Status: reminder.Status, LastNotifiedAt: reminder.LastNotifiedAt,
		NotificationCount: reminder.NotificationCount, CreatedAt: reminder.CreatedAt, UpdatedAt: reminder.UpdatedAt,
	}
}

func manualTodoToMyTodo(reminder ManualTodoReminder) MyTodoItem {
	return MyTodoItem{
		ID: reminder.ID, TodoType: myTodoTypeManual,
		ProjectCode: reminder.ProjectCode, ProjectName: reminder.ProjectName,
		Reporter: reminder.OwnerUsername, FeatureItems: []string{reminder.EventContent},
		DueAt: reminder.DueAt, Status: reminder.Status, LastNotifiedAt: reminder.LastNotifiedAt,
		NotificationCount: reminder.NotificationCount, CreatedAt: reminder.CreatedAt, UpdatedAt: reminder.UpdatedAt,
	}
}

func ListMyTodoRemindersHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
		return
	}
	acceptanceItems, err := ListPendingAcceptanceTodoRemindersForReporter(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	manualItems, err := ListPendingManualTodoRemindersForOwner(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]MyTodoItem, 0, len(acceptanceItems)+len(manualItems))
	for _, reminder := range acceptanceItems {
		items = append(items, acceptanceTodoToMyTodo(reminder))
	}
	for _, reminder := range manualItems {
		items = append(items, manualTodoToMyTodo(reminder))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].DueAt.Before(items[j].DueAt)
	})
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func CreateMyManualTodoReminderHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
		return
	}
	var request CreateManualTodoRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "待办参数不完整"})
		return
	}
	reminder, err := CreateManualTodoReminder(user.Username, request, time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "待办创建成功", "item": manualTodoToMyTodo(reminder)})
}

func CompleteMyTodoReminderHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
		return
	}
	var request struct {
		TodoType string `json:"todo_type"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "待办类型参数不正确"})
			return
		}
	}
	todoType := strings.ToLower(strings.TrimSpace(request.TodoType))
	if todoType == "" {
		todoType = myTodoTypeAcceptance
	}
	if todoType == myTodoTypeManual {
		reminder, err := CompleteManualTodoReminderForOwner(c.Param("id"), user.Username)
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "自己的待办") {
				status = http.StatusForbidden
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "待办已完成，后续不再提醒", "item": manualTodoToMyTodo(reminder)})
		return
	}
	if todoType != myTodoTypeAcceptance {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未知的待办类型"})
		return
	}
	reminder, err := CompleteAcceptanceTodoReminderForReporter(c.Param("id"), user.Username, user.FeishuOpenID)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "自己的待办") {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "待办已完成，后续不再提醒", "item": acceptanceTodoToMyTodo(reminder)})
}
