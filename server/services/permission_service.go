package services

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type UserPermissionRecord struct {
	ID          string          `json:"id,omitempty"`
	UserID      string          `json:"user_id"`
	Username    string          `json:"username"`
	Permissions map[string]bool `json:"permissions"`
	UpdatedAt   string          `json:"updated_at"`
}

type UserPermissionSaveRequest struct {
	UserID      string          `json:"user_id" binding:"required"`
	Permissions map[string]bool `json:"permissions" binding:"required"`
}

func getCurrentPermissionUser(c *gin.Context) (*User, bool) {
	user, err := currentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return nil, false
	}

	return user, true
}

func requirePermissionAdmin(c *gin.Context) (*User, bool) {
	user, ok := getCurrentPermissionUser(c)
	if !ok {
		return nil, false
	}
	if !isPermissionAdminUsername(user.Username) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return nil, false
	}
	return user, true
}

func isPermissionAdminUsername(username string) bool {
	return strings.ToLower(strings.TrimSpace(username)) == "minghong"
}

func DefaultDashboardPermissions(username string) map[string]bool {
	permissions := map[string]bool{
		"dashboard.com_api_commit.visible":   true,
		"dashboard.test_process.visible":     true,
		"dashboard.sandbox_accounts.visible": true,
		"dashboard.ui_auto.visible":          true,
		"dashboard.testcase_gen.visible":     true,
		"dashboard.skillify.visible":         true,
		"dashboard.performance.visible":      true,
		"dashboard.monkey_test.visible":      true,
		"monkey.run.view":                    true,
		"monkey.run.start":                   true,
		"monkey.run.stop":                    true,
		"dashboard.jungle.visible":           false,
		"dashboard.novel_reader.visible":     false,
		"dashboard.video_player.visible":     false,
		"dashboard.feishu_assistant.visible": true,
		"reports.test_reports.visible":       true,
		"reports.acceptance_reports.visible": true,
		"reports.testcase_gen.visible":       true,
	}
	permissions["settings.permissions.visible"] = isPermissionAdminUsername(username)
	return permissions
}

func ensurePermissionTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS user_permissions (
			user_id VARCHAR(128) PRIMARY KEY,
			username VARCHAR(128),
			permissions JSON NOT NULL,
			updated_at VARCHAR(64),
			raw_json JSON NOT NULL,
			migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`)
	return err
}

func listPermissionRecords() ([]UserPermissionRecord, error) {
	if err := ensurePermissionTable(); err != nil {
		return nil, err
	}
	return sqlListJSON[UserPermissionRecord]("user_permissions", "`migrated_at` ASC")
}

func getPermissionRecordByUserID(userID string) (*UserPermissionRecord, error) {
	records, err := listPermissionRecords()
	if err != nil {
		return nil, err
	}
	for i := range records {
		if records[i].UserID == userID {
			record := mergePermissionDefaults(records[i])
			return &record, nil
		}
	}
	return nil, nil
}

func mergePermissionDefaults(record UserPermissionRecord) UserPermissionRecord {
	if strings.TrimSpace(record.ID) == "" {
		record.ID = record.UserID
	}
	defaults := DefaultDashboardPermissions(record.Username)
	merged := make(map[string]bool, len(defaults))
	for key, value := range defaults {
		merged[key] = value
	}
	for key, value := range record.Permissions {
		merged[key] = value
	}
	if isPermissionAdminUsername(record.Username) {
		merged["settings.permissions.visible"] = true
	}
	record.Permissions = merged
	return record
}

func savePermissionRecord(record UserPermissionRecord) error {
	record = mergePermissionDefaults(record)
	record.UpdatedAt = time.Now().Format(time.RFC3339)
	if record.Permissions == nil {
		record.Permissions = DefaultDashboardPermissions(record.Username)
	}
	return sqlUpsertJSON("user_permissions", record)
}

func buildPermissionPayloadForUser(profile AccountProfile, records []UserPermissionRecord) UserPermissionRecord {
	defaults := DefaultDashboardPermissions(profile.Username)
	payload := UserPermissionRecord{
		ID:          profile.ID,
		UserID:      profile.ID,
		Username:    profile.Username,
		Permissions: defaults,
		UpdatedAt:   "",
	}
	for _, record := range records {
		if record.UserID == profile.ID {
			payload = mergePermissionDefaults(record)
			payload.Username = profile.Username
			return payload
		}
	}
	return payload
}

func GetPermissionListHandler(c *gin.Context) {
	if _, ok := requirePermissionAdmin(c); !ok {
		return
	}

	if err := ensurePermissionTable(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	accounts, err := ListAccountProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	records, err := listPermissionRecords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]UserPermissionRecord, 0, len(accounts))
	for _, account := range accounts {
		response = append(response, buildPermissionPayloadForUser(account, records))
	}
	c.JSON(http.StatusOK, response)
}

func GetCurrentUserPermissionsHandler(c *gin.Context) {
	user, ok := getCurrentPermissionUser(c)
	if !ok {
		return
	}

	record, err := getPermissionRecordByUserID(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if record == nil {
		c.JSON(http.StatusOK, mergePermissionDefaults(UserPermissionRecord{
			ID:          user.ID,
			UserID:      user.ID,
			Username:    user.Username,
			Permissions: DefaultDashboardPermissions(user.Username),
		}))
		return
	}

	c.JSON(http.StatusOK, record)
}

func SaveUserPermissionHandler(c *gin.Context) {
	if _, ok := requirePermissionAdmin(c); !ok {
		return
	}

	if err := ensurePermissionTable(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req UserPermissionSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user not found"})
		return
	}

	record := UserPermissionRecord{
		ID:          req.UserID,
		UserID:      req.UserID,
		Username:    user.Username,
		Permissions: req.Permissions,
	}
	if err := savePermissionRecord(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func ResetUserPermissionHandler(c *gin.Context) {
	if _, ok := requirePermissionAdmin(c); !ok {
		return
	}

	if err := ensurePermissionTable(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID := strings.TrimSpace(c.Param("user_id"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user id"})
		return
	}
	if err := sqlDeleteJSON("user_permissions", userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func init() {
	sqlJSONTables["user_permissions"] = sqlJSONTable{
		Table:      "user_permissions",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "user_id", Field: "user_id"},
			{Column: "username", Field: "username"},
			{Column: "permissions", Field: "permissions", JSON: true},
			{Column: "updated_at", Field: "updated_at"},
		},
	}
}

func (u UserPermissionRecord) MarshalJSON() ([]byte, error) {
	type alias UserPermissionRecord
	if u.Permissions == nil {
		u.Permissions = DefaultDashboardPermissions(u.Username)
	}
	return json.Marshal(alias(u))
}
