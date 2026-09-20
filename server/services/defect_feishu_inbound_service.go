package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"testcenter-server/models"
)

const (
	defectFeishuInboundInitialDays = 7
	defectFeishuInboundMaxPages    = 30
)

var (
	defectFeishuInboundMu = sync.Mutex{}
	defectProjectCodeRE   = regexp.MustCompile(`(?i)(?:所属项目[^A-Z0-9]*|\[)(A\d{3,6})(?:\s|·|-|\])`)
	defectNumberRE        = regexp.MustCompile(`(?i)BUG-\d{8}-\d{4}`)
	defectTitlePrefixRE   = regexp.MustCompile(`^\s*\[(?:BUG-\d{8}-\d{4}|A\d{3,6}-\d{8}-\d{4})\]\s*`)
)

type DefectFeishuRefreshRequest struct {
	ProjectCode string `json:"project_code"`
}

type DefectFeishuRefreshIssue struct {
	Workspace  string `json:"workspace"`
	WorkItemID string `json:"work_item_id"`
	Title      string `json:"title"`
	Reason     string `json:"reason"`
}

type DefectFeishuRefreshResult struct {
	Read       int                        `json:"read"`
	Created    int                        `json:"created"`
	Updated    int                        `json:"updated"`
	Skipped    int                        `json:"skipped"`
	Conflicts  int                        `json:"conflicts"`
	Unresolved int                        `json:"unresolved"`
	Failed     int                        `json:"failed"`
	DurationMS int64                      `json:"duration_ms"`
	Issues     []DefectFeishuRefreshIssue `json:"issues"`
}

type defectFeishuRemoteItem struct {
	ProjectKey      string
	Workspace       string
	WorkItemID      string
	Name            string
	Priority        string
	Status          string
	Description     string
	Business        string
	Platform        string
	OwnerEmail      string
	OwnerName       string
	OperatorEmails  []string
	OperatorNames   []string
	CreatedDate     string
	RemoteUpdatedAt string
}

type defectFeishuInboundMapping struct {
	DefectID          string
	SyncedRowVersion  int
	RemotePayloadHash string
	RemoteUpdatedAt   string
	MappingUpdatedAt  string
}

func RefreshDefectsFromFeishuHandler(c *gin.Context) {
	user, ok := defectUser(c)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录状态已失效，请重新登录"})
		return
	}
	var req DefectFeishuRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, context.Canceled) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "刷新参数格式不正确"})
		return
	}
	req.ProjectCode = strings.TrimSpace(req.ProjectCode)
	if req.ProjectCode != "" && !defectProjectAllows(c.Request.Context(), user, req.ProjectCode, "create") {
		c.JSON(http.StatusForbidden, gin.H{"error": "没有在该项目同步缺陷的权限"})
		return
	}
	ctx, err := withDefectFeishuOAuthForUser(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := refreshDefectsFromFeishu(ctx, user, req.ProjectCode)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func refreshDefectsFromFeishu(ctx context.Context, user *User, selectedProject string) (DefectFeishuRefreshResult, error) {
	started := time.Now()
	result := DefectFeishuRefreshResult{Issues: []DefectFeishuRefreshIssue{}}
	defectFeishuInboundMu.Lock()
	defer defectFeishuInboundMu.Unlock()
	if err := ensureDefectSchema(ctx); err != nil {
		return result, err
	}
	projects, err := ConfigServiceInstance.GetAllProjects()
	if err != nil {
		return result, err
	}
	spaces := []defectFeishuSpaceConfig{defectFeishuOverseasConfig, defectFeishuTomatoConfig}
	if selectedProject != "" {
		space, ok := defectFeishuSpaceForProject(selectedProject)
		if !ok {
			return result, fmt.Errorf("项目 %s 未关联支持同步的飞书空间", selectedProject)
		}
		spaces = []defectFeishuSpaceConfig{space}
	}
	for _, space := range spaces {
		stateKey := space.ProjectKey
		if selectedProject != "" {
			stateKey += "|" + strings.ToUpper(selectedProject)
		}
		since := defectFeishuInboundSince(ctx, stateKey)
		items, err := listDefectFeishuRemoteItems(ctx, space, since)
		if err != nil {
			return result, fmt.Errorf("读取%s失败：%w", space.Workspace, err)
		}
		for _, item := range items {
			result.Read++
			outcome, issue := applyDefectFeishuRemoteItem(ctx, user, item, projects, selectedProject)
			switch outcome {
			case "created":
				result.Created++
			case "updated":
				result.Updated++
			case "conflict":
				result.Conflicts++
			case "unresolved":
				result.Unresolved++
			case "failed":
				result.Failed++
			default:
				result.Skipped++
			}
			if issue != nil && len(result.Issues) < 50 {
				result.Issues = append(result.Issues, *issue)
			}
		}
		if err := saveDefectFeishuInboundState(ctx, stateKey, user.ID, time.Now()); err != nil {
			return result, err
		}
	}
	result.DurationMS = time.Since(started).Milliseconds()
	return result, nil
}

func defectFeishuInboundSince(ctx context.Context, projectKey string) time.Time {
	db, _, err := DatabaseManager.DB(ctx)
	if err == nil {
		var value string
		if db.QueryRowContext(ctx, "SELECT last_refresh_at FROM defect_feishu_inbound_states WHERE project_key=?", projectKey).Scan(&value) == nil {
			if parsed, parseErr := time.Parse(time.RFC3339Nano, value); parseErr == nil {
				return parsed.Add(-2 * time.Minute)
			}
		}
	}
	return time.Now().AddDate(0, 0, -defectFeishuInboundInitialDays)
}

func saveDefectFeishuInboundState(ctx context.Context, projectKey, userID string, refreshedAt time.Time) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx, `INSERT INTO defect_feishu_inbound_states
		(project_key,last_refresh_at,updated_by,updated_at) VALUES (?,?,?,?)
		ON DUPLICATE KEY UPDATE last_refresh_at=VALUES(last_refresh_at),updated_by=VALUES(updated_by),updated_at=VALUES(updated_at)`,
		projectKey, refreshedAt.Format(time.RFC3339Nano), userID, now)
	return err
}

func listDefectFeishuRemoteItems(ctx context.Context, space defectFeishuSpaceConfig, since time.Time) ([]defectFeishuRemoteItem, error) {
	mql := fmt.Sprintf("SELECT `work_item_id`, `name`, `priority`, `work_item_status`, `description`, `business`, `field_f7ef16`, `owner`, `current_status_operator`, `updated_at`, `start_time` FROM `%s`.`%s` WHERE `updated_at` >= '%s' ORDER BY `updated_at` ASC",
		space.ProjectKey, space.WorkItemType, since.Format("2006-01-02"))
	items := []defectFeishuRemoteItem{}
	sessionID, groupID, total := "", "", 0
	for page := 1; page <= defectFeishuInboundMaxPages; page++ {
		arguments := map[string]any{"project_key": space.ProjectKey}
		if page == 1 {
			arguments["mql"] = mql
		} else {
			arguments["session_id"] = sessionID
			arguments["group_pagination_list"] = []map[string]any{{"group_id": groupID, "page_num": page}}
		}
		text, err := callDefectFeishuMCP(ctx, "search_by_mql", arguments)
		if err != nil {
			return nil, err
		}
		pageItems, nextSession, nextGroup, count, err := decodeDefectFeishuMQLItems(text, space)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if page == 1 {
			sessionID, groupID, total = nextSession, nextGroup, count
		}
		if len(items) >= total || len(pageItems) == 0 || sessionID == "" || groupID == "" {
			break
		}
	}
	return items, nil
}

func decodeDefectFeishuMQLItems(text string, space defectFeishuSpaceConfig) ([]defectFeishuRemoteItem, string, string, int, error) {
	var payload struct {
		List []struct {
			Count      int `json:"count"`
			GroupInfos []struct {
				GroupID string `json:"group_id"`
			} `json:"group_infos"`
		} `json:"list"`
		SessionID string `json:"session_id"`
		Data      map[string][]struct {
			Fields []defectFeishuMQLField `json:"moql_field_list"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, "", "", 0, fmt.Errorf("飞书查询结果格式不正确：%w", err)
	}
	groupID, total := "", 0
	if len(payload.List) > 0 {
		total = payload.List[0].Count
		if len(payload.List[0].GroupInfos) > 0 {
			groupID = payload.List[0].GroupInfos[0].GroupID
		}
	}
	rows := payload.Data[groupID]
	if groupID == "" {
		for key, value := range payload.Data {
			groupID, rows = key, value
			break
		}
	}
	items := make([]defectFeishuRemoteItem, 0, len(rows))
	for _, row := range rows {
		item := defectFeishuRemoteItem{ProjectKey: space.ProjectKey, Workspace: space.Workspace}
		for _, field := range row.Fields {
			field.apply(&item)
		}
		if item.WorkItemID != "" {
			items = append(items, item)
		}
	}
	return items, payload.SessionID, groupID, total, nil
}

type defectFeishuMQLField struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

func (field defectFeishuMQLField) apply(item *defectFeishuRemoteItem) {
	var value map[string]json.RawMessage
	if json.Unmarshal(field.Value, &value) != nil {
		return
	}
	stringValue := func(key string) string { var v string; _ = json.Unmarshal(value[key], &v); return strings.TrimSpace(v) }
	longValue := func(key string) string {
		var v json.Number
		if json.Unmarshal(value[key], &v) == nil {
			return v.String()
		}
		return ""
	}
	label := func(key string) string {
		var v struct {
			Label string `json:"label"`
		}
		_ = json.Unmarshal(value[key], &v)
		return strings.TrimSpace(v.Label)
	}
	switch field.Key {
	case "work_item_id":
		item.WorkItemID = longValue("long_value")
	case "name":
		item.Name = stringValue("string_value")
	case "description":
		item.Description = stringValue("string_value")
	case "updated_at":
		item.RemoteUpdatedAt = stringValue("string_value")
	case "start_time":
		item.CreatedDate = stringValue("string_value")
	case "priority":
		item.Priority = label("key_label_value")
	case "business":
		var v struct {
			Label    string `json:"label"`
			Children []struct {
				Label string `json:"label"`
			} `json:"children"`
		}
		_ = json.Unmarshal(value["cascade_key_label_value"], &v)
		item.Business = v.Label
		if len(v.Children) > 0 {
			item.Business += " " + v.Children[0].Label
		}
	case "field_f7ef16", "work_item_status":
		var v []struct {
			Label string `json:"label"`
		}
		_ = json.Unmarshal(value["key_label_value_list"], &v)
		labels := []string{}
		for _, entry := range v {
			labels = append(labels, entry.Label)
		}
		if field.Key == "field_f7ef16" {
			item.Platform = strings.Join(labels, ",")
		} else {
			item.Status = strings.Join(labels, ",")
		}
	case "owner":
		var v struct {
			Email   string `json:"email"`
			NameCN  string `json:"name_cn"`
			NameEN  string `json:"name_en"`
			UserKey string `json:"user_key"`
		}
		_ = json.Unmarshal(value["user_value"], &v)
		item.OwnerEmail, item.OwnerName = v.Email, firstDefectInboundNonEmpty(v.NameCN, v.NameEN)
	case "current_status_operator":
		var v []struct {
			Email  string `json:"email"`
			NameCN string `json:"name_cn"`
			NameEN string `json:"name_en"`
		}
		_ = json.Unmarshal(value["user_value_list"], &v)
		for _, entry := range v {
			item.OperatorEmails = append(item.OperatorEmails, entry.Email)
			item.OperatorNames = append(item.OperatorNames, firstDefectInboundNonEmpty(entry.NameCN, entry.NameEN))
		}
	}
}

func applyDefectFeishuRemoteItem(ctx context.Context, user *User, item defectFeishuRemoteItem, projects []models.Project, selectedProject string) (string, *DefectFeishuRefreshIssue) {
	issue := func(reason string) *DefectFeishuRefreshIssue {
		return &DefectFeishuRefreshIssue{Workspace: item.Workspace, WorkItemID: item.WorkItemID, Title: item.Name, Reason: reason}
	}
	mapping, err := findDefectFeishuInboundMapping(ctx, item.ProjectKey, defectFeishuWorkItemType, item.WorkItemID)
	if err != nil {
		return "failed", issue(err.Error())
	}
	if mapping == nil {
		mapping, err = recoverDefectFeishuInboundMapping(ctx, item)
		if err != nil {
			return "failed", issue(err.Error())
		}
	}
	projectCode := ""
	if mapping != nil {
		defect, getErr := getDefect(ctx, mapping.DefectID)
		if getErr != nil || defect == nil {
			return "failed", issue("平台映射指向的缺陷不存在")
		}
		projectCode = defect.ProjectCode
	} else {
		projectCode = inferDefectFeishuProject(item, projects)
		if projectCode == "" {
			return "unresolved", issue("无法唯一确定所属项目，未自动导入")
		}
	}
	if selectedProject != "" && !strings.EqualFold(projectCode, selectedProject) {
		return "skipped", nil
	}
	hash := defectFeishuRemoteHash(item)
	if mapping != nil {
		if mapping.RemotePayloadHash == hash {
			return "skipped", nil
		}
		defect, _ := getDefect(ctx, mapping.DefectID)
		if defect == nil {
			return "failed", issue("平台缺陷不存在")
		}
		if !defectCanEditRecord(ctx, user, *defect) {
			return "unresolved", issue("当前账号没有修改该平台缺陷的权限")
		}
		if defect.RowVersion > mapping.SyncedRowVersion && remoteChangedAfterMapping(item.RemoteUpdatedAt, mapping.MappingUpdatedAt) {
			return "conflict", issue("平台与飞书均有更新，已保留平台数据")
		}
		if err := updateDefectFromFeishu(ctx, user, *defect, item, hash); err != nil {
			return "failed", issue(err.Error())
		}
		return "updated", nil
	}
	if !defectProjectAllows(ctx, user, projectCode, "create") {
		return "unresolved", issue("当前账号没有在该项目导入缺陷的权限")
	}
	if err := createDefectFromFeishu(ctx, user, projectCode, item, hash); err != nil {
		return "failed", issue(err.Error())
	}
	return "created", nil
}

func findDefectFeishuInboundMapping(ctx context.Context, projectKey, workItemType, workItemID string) (*defectFeishuInboundMapping, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	var m defectFeishuInboundMapping
	err = db.QueryRowContext(ctx, `SELECT defect_id,synced_row_version,remote_payload_hash,remote_updated_at,updated_at
		FROM defect_feishu_syncs WHERE project_key=? AND work_item_type=? AND work_item_id=? LIMIT 1`, projectKey, workItemType, workItemID).
		Scan(&m.DefectID, &m.SyncedRowVersion, &m.RemotePayloadHash, &m.RemoteUpdatedAt, &m.MappingUpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &m, err
}

func recoverDefectFeishuInboundMapping(ctx context.Context, item defectFeishuRemoteItem) (*defectFeishuInboundMapping, error) {
	number := defectNumberRE.FindString(item.Description)
	if number == "" {
		number = defectNumberRE.FindString(item.Name)
	}
	if number == "" {
		return nil, nil
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	var defectID string
	if err := db.QueryRowContext(ctx, "SELECT id FROM defects WHERE defect_no=? LIMIT 1", strings.ToUpper(number)).Scan(&defectID); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339Nano)
	url := fmt.Sprintf("https://project.feishu.cn/%s/%s/detail/%s", item.ProjectKey, defectFeishuWorkItemType, item.WorkItemID)
	_, err = db.ExecContext(ctx, `INSERT INTO defect_feishu_syncs
		(defect_id,project_key,work_item_type,work_item_id,work_item_url,sync_status,synced_row_version,retry_count,last_error,last_attempt_at,next_retry_at,created_at,updated_at)
		VALUES (?,?,?,?,?,'synced',0,0,'','','',?,?) ON DUPLICATE KEY UPDATE project_key=VALUES(project_key),work_item_type=VALUES(work_item_type),work_item_id=VALUES(work_item_id),work_item_url=VALUES(work_item_url),updated_at=VALUES(updated_at)`,
		defectID, item.ProjectKey, defectFeishuWorkItemType, item.WorkItemID, url, now, now)
	if err != nil {
		return nil, err
	}
	return findDefectFeishuInboundMapping(ctx, item.ProjectKey, defectFeishuWorkItemType, item.WorkItemID)
}

func inferDefectFeishuProject(item defectFeishuRemoteItem, projects []models.Project) string {
	combined := item.Name + "\n" + item.Description
	if match := defectProjectCodeRE.FindStringSubmatch(combined); len(match) > 1 {
		return strings.ToUpper(match[1])
	}
	candidates := []string{}
	for _, project := range projects {
		space, ok := defectFeishuSpaceByWorkspace(project.Workspace)
		if !ok || space.ProjectKey != item.ProjectKey {
			continue
		}
		name := normalizeDefectFeishuMatchText(project.ProjectName)
		business := normalizeDefectFeishuMatchText(item.Business)
		platform := normalizeDefectFeishuMatchText(item.Platform)
		productMatch := false
		for _, token := range []string{"shortswave", "mangogo", "meloshort", "reelsdrama", "rushort", "cocoshort", "novelnova", "ttmins"} {
			if strings.Contains(name, token) && strings.Contains(business, token) {
				productMatch = true
				break
			}
		}
		if !productMatch {
			continue
		}
		platformMatch := platform == "" || strings.Contains(name, platform) || (strings.Contains(platform, "ios") && strings.Contains(name, "iphone"))
		if platformMatch {
			candidates = append(candidates, project.ProjectCode)
		}
	}
	sort.Strings(candidates)
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}

func normalizeDefectFeishuMatchText(value string) string {
	value = strings.ToLower(value)
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", "tik tok minis", "ttmins", "tt minis", "ttmins")
	return replacer.Replace(value)
}

func createDefectFromFeishu(ctx context.Context, user *User, projectCode string, item defectFeishuRemoteItem, hash string) error {
	assigneeID := defectFeishuMatchPlatformUser(item.OperatorEmails, item.OperatorNames)
	platform := normalizeDefectFeishuPlatformLabel(item.Platform)
	sync := true
	req := DefectSaveRequest{
		Title: strings.TrimSpace(defectTitlePrefixRE.ReplaceAllString(item.Name, "")), ProjectCode: projectCode,
		DefectType: "function", Severity: defectFeishuInboundSeverity(item.Priority), Priority: defectFeishuInboundSeverity(item.Priority),
		AssigneeID: assigneeID, VerifierID: user.ID, DueDate: firstDefectInboundNonEmpty(item.CreatedDate, time.Now().Format("2006-01-02")),
		Description: item.Description, Environment: DefectEnvironment{Platform: platform, Environment: "测试环境", Network: "WiFi"},
		Tags: []string{"飞书项目导入"}, CustomFields: map[string]any{}, SyncToFeishu: &sync,
	}
	defect, err := createDefect(ctx, user, req)
	if err != nil {
		return err
	}
	if err := setDefectInboundState(ctx, defect.ID, item, hash, defect.RowVersion); err != nil {
		return err
	}
	if status := defectFeishuInboundStatus(item.Status); status != DefectStatusNew {
		return updateDefectInboundStatus(ctx, defect.ID, status)
	}
	return nil
}

func updateDefectFromFeishu(ctx context.Context, user *User, defect Defect, item defectFeishuRemoteItem, hash string) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	before := defect
	defect.Title = strings.TrimSpace(defectTitlePrefixRE.ReplaceAllString(item.Name, ""))
	defect.Status = defectFeishuInboundStatus(item.Status)
	defect.Severity, defect.Priority = defectFeishuInboundSeverity(item.Priority), defectFeishuInboundSeverity(item.Priority)
	if item.Description != "" {
		defect.Description = item.Description
	}
	if platform := normalizeDefectFeishuPlatformLabel(item.Platform); platform != "" {
		defect.Environment.Platform = platform
	}
	if id := defectFeishuMatchPlatformUser(item.OperatorEmails, item.OperatorNames); id != "" {
		defect.AssigneeID = id
		if name, accountErr := defectAccount(id); accountErr == nil {
			defect.AssigneeName = name
		}
	}
	environmentJSON, _ := json.Marshal(defect.Environment)
	now := time.Now().Format(time.RFC3339Nano)
	defect.RowVersion++
	_, err = tx.ExecContext(ctx, `UPDATE defects SET title=?,status=?,severity=?,priority=?,description=?,environment_json=?,assignee_id=?,assignee_name=?,row_version=?,updated_at=? WHERE id=?`,
		defect.Title, defect.Status, defect.Severity, defect.Priority, defect.Description, string(environmentJSON), defect.AssigneeID, defect.AssigneeName, defect.RowVersion, now, defect.ID)
	if err != nil {
		return err
	}
	if err := insertDefectHistory(ctx, tx, defect.ID, "feishu_sync", user, before, defect, "从飞书项目刷新"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return setDefectInboundState(ctx, defect.ID, item, hash, defect.RowVersion)
}

func setDefectInboundState(ctx context.Context, defectID string, item defectFeishuRemoteItem, hash string, rowVersion int) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339Nano)
	url := fmt.Sprintf("https://project.feishu.cn/%s/%s/detail/%s", item.ProjectKey, defectFeishuWorkItemType, item.WorkItemID)
	_, err = db.ExecContext(ctx, `INSERT INTO defect_feishu_syncs
		(defect_id,project_key,work_item_type,work_item_id,work_item_url,sync_status,synced_row_version,retry_count,last_error,last_attempt_at,next_retry_at,created_at,updated_at,remote_payload_hash,remote_updated_at,last_inbound_at)
		VALUES (?,?,?,?,?,'synced',?,0,'','','',?,?,?,?,?)
		ON DUPLICATE KEY UPDATE project_key=VALUES(project_key),work_item_type=VALUES(work_item_type),work_item_id=VALUES(work_item_id),work_item_url=VALUES(work_item_url),sync_status='synced',synced_row_version=VALUES(synced_row_version),retry_count=0,last_error='',next_retry_at='',updated_at=VALUES(updated_at),remote_payload_hash=VALUES(remote_payload_hash),remote_updated_at=VALUES(remote_updated_at),last_inbound_at=VALUES(last_inbound_at)`,
		defectID, item.ProjectKey, defectFeishuWorkItemType, item.WorkItemID, url, rowVersion, now, now, hash, item.RemoteUpdatedAt, now)
	return err
}

func updateDefectInboundStatus(ctx context.Context, defectID, status string) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "UPDATE defects SET status=?,row_version=row_version+1,updated_at=? WHERE id=?", status, time.Now().Format(time.RFC3339Nano), defectID)
	return err
}

func defectFeishuRemoteHash(item defectFeishuRemoteItem) string {
	encoded, _ := json.Marshal(item)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func remoteChangedAfterMapping(remote, mapping string) bool {
	remoteTime, err := time.ParseInLocation("2006-01-02 15:04:05", remote, time.Local)
	if err != nil {
		return true
	}
	mappingTime, err := time.Parse(time.RFC3339Nano, mapping)
	if err != nil {
		return true
	}
	return remoteTime.After(mappingTime.Add(-2 * time.Second))
}

func defectFeishuInboundSeverity(priority string) int {
	switch strings.ToUpper(strings.TrimSpace(priority)) {
	case "P0":
		return 1
	case "P1":
		return 2
	default:
		return 3
	}
}

func defectFeishuInboundStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "CLOSED", "已结束", "已终止":
		return DefectStatusClosed
	case "RESOLVED", "ACCEPTANCE":
		return DefectStatusResolved
	case "IN PROGRESS", "REOPENED":
		return DefectStatusActive
	default:
		return DefectStatusNew
	}
}

func normalizeDefectFeishuPlatformLabel(value string) string {
	upper := strings.ToUpper(strings.TrimSpace(strings.Split(value, ",")[0]))
	switch upper {
	case "IOS":
		return "iOS"
	case "TT MINIS":
		return "TTmins"
	case "ANDROID":
		return "Android"
	case "WEB":
		return "Web"
	default:
		return strings.TrimSpace(value)
	}
}

func defectFeishuMatchPlatformUser(emails, names []string) string {
	accounts, err := ListAccountProfiles()
	if err != nil {
		return ""
	}
	for _, account := range accounts {
		for _, email := range emails {
			if email != "" && strings.EqualFold(strings.TrimSpace(account.Email), strings.TrimSpace(email)) {
				return account.ID
			}
		}
	}
	for _, account := range accounts {
		for _, name := range names {
			if strings.EqualFold(strings.TrimSpace(account.Username), strings.TrimSpace(name)) || strings.EqualFold(strings.TrimSpace(account.Nickname), strings.TrimSpace(name)) {
				return account.ID
			}
		}
	}
	return ""
}

func firstDefectInboundNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
