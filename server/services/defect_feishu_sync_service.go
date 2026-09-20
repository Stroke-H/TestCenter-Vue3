package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	feishumodel "testcenter-server/feishu/model"
)

const (
	defectFeishuWorkspace       = "海外短剧"
	defectFeishuTomatoWorkspace = "番茄短剧"
	defectFeishuWorkItemType    = "63329b6c980d67099b12fd73"

	defectFeishuProjectKey      = "66cd876aaf05322e1b3d6aca"
	defectFeishuTemplateID      = "1752744"
	defectFeishuBusinessSWOther = "67f71d9246fea409f8fc1344"
	defectFeishuBusinessTT      = "6aab5eba95672116f4974f2e"

	defectFeishuTomatoProjectKey    = "687f004b800ff4f8087e00a4"
	defectFeishuTomatoTemplateID    = "3365448"
	defectFeishuTomatoBusinessTT    = "687f005893c29a184c13b872"
	defectFeishuTomatoBusinessOther = "687f005893c29a184c13b873"
)

type defectFeishuSpaceConfig struct {
	Workspace               string
	ProjectKey              string
	WorkItemType            string
	TemplateID              string
	BusinessTT              string
	BusinessShortsWaveOther string
	DefaultBusiness         string
}

var (
	defectFeishuOverseasConfig = defectFeishuSpaceConfig{
		Workspace:               defectFeishuWorkspace,
		ProjectKey:              defectFeishuProjectKey,
		WorkItemType:            defectFeishuWorkItemType,
		TemplateID:              defectFeishuTemplateID,
		BusinessTT:              defectFeishuBusinessTT,
		BusinessShortsWaveOther: defectFeishuBusinessSWOther,
	}
	defectFeishuTomatoConfig = defectFeishuSpaceConfig{
		Workspace:       defectFeishuTomatoWorkspace,
		ProjectKey:      defectFeishuTomatoProjectKey,
		WorkItemType:    defectFeishuWorkItemType,
		TemplateID:      defectFeishuTomatoTemplateID,
		BusinessTT:      defectFeishuTomatoBusinessTT,
		DefaultBusiness: defectFeishuTomatoBusinessOther,
	}
)

type DefectFeishuSync struct {
	DefectID         string `json:"defect_id"`
	ProjectKey       string `json:"project_key"`
	WorkItemType     string `json:"work_item_type"`
	WorkItemID       string `json:"work_item_id"`
	WorkItemURL      string `json:"work_item_url"`
	SyncStatus       string `json:"sync_status"`
	SyncedRowVersion int    `json:"synced_row_version"`
	RetryCount       int    `json:"retry_count"`
	LastError        string `json:"last_error"`
	LastAttemptAt    string `json:"last_attempt_at"`
	NextRetryAt      string `json:"next_retry_at"`
	UpdatedAt        string `json:"updated_at"`
}

type defectMCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type defectMCPResponse struct {
	Result struct {
		Content []defectMCPContent `json:"content"`
		IsError bool               `json:"isError"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type defectFeishuTransition struct {
	ID        any    `json:"id"`
	StateKey  string `json:"state_key"`
	StateName string `json:"state_name"`
}

type defectFeishuTransitionState struct {
	StateKey   string                   `json:"state_key"`
	StateName  string                   `json:"state_name"`
	Transition []defectFeishuTransition `json:"transition"`
}

type defectFeishuUserCandidate struct {
	UserKey  string
	Username string
	NameCN   string
	NameEN   string
	Email    string
}

var (
	defectFeishuSyncOnce sync.Once
	defectFeishuSyncWake = make(chan struct{}, 1)
	defectFeishuURLRE    = regexp.MustCompile(`https?://[^\s"']+`)
	defectFeishuIDRE     = regexp.MustCompile(`(?:work_item_id|workItemId)["'\s:=]+([0-9]+)`)
)

func InitDefectFeishuSyncService() {
	defectFeishuSyncOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := ensureDefectSchema(ctx); err != nil {
			log.Printf("[DefectFeishuSync] schema initialization deferred: %v", err)
			return
		}
		db, _, err := DatabaseManager.DB(ctx)
		if err != nil {
			log.Printf("[DefectFeishuSync] database unavailable: %v", err)
			return
		}
		now := time.Now().Format(time.RFC3339Nano)
		_, _ = db.ExecContext(ctx, `UPDATE defect_feishu_syncs SET sync_status='failed',
			last_error='服务重启后恢复未完成同步', next_retry_at=?, updated_at=? WHERE sync_status='syncing'`, now, now)
		go runDefectFeishuSyncWorker()
		wakeDefectFeishuSyncWorker()
	})
}

func runDefectFeishuSyncWorker() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-defectFeishuSyncWake:
		case <-ticker.C:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if err := processDueDefectFeishuSyncs(ctx, 10); err != nil {
			log.Printf("[DefectFeishuSync] worker failed: %v", err)
		}
		cancel()
	}
}

func wakeDefectFeishuSyncWorker() {
	select {
	case defectFeishuSyncWake <- struct{}{}:
	default:
	}
}

func defectUsesFeishuSync(projectCode string) bool {
	_, ok := defectFeishuSpaceForProject(projectCode)
	return ok
}

func defectFeishuSpaceByWorkspace(workspace string) (defectFeishuSpaceConfig, bool) {
	switch {
	case strings.EqualFold(strings.TrimSpace(workspace), defectFeishuWorkspace):
		return defectFeishuOverseasConfig, true
	case strings.EqualFold(strings.TrimSpace(workspace), defectFeishuTomatoWorkspace):
		return defectFeishuTomatoConfig, true
	default:
		return defectFeishuSpaceConfig{}, false
	}
}

func defectFeishuSpaceForProject(projectCode string) (defectFeishuSpaceConfig, bool) {
	if ConfigServiceInstance == nil {
		return defectFeishuSpaceConfig{}, false
	}
	project, err := ConfigServiceInstance.GetProjectBySubCode(projectCode)
	if err != nil || project == nil {
		return defectFeishuSpaceConfig{}, false
	}
	return defectFeishuSpaceByWorkspace(project.Workspace)
}

func queueDefectFeishuSync(ctx context.Context, defectID string) (bool, error) {
	defect, err := getDefect(ctx, defectID)
	if err != nil || defect == nil {
		return false, err
	}
	space, supported := defectFeishuSpaceForProject(defect.ProjectCode)
	if !defect.SyncToFeishu || !supported {
		return false, nil
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return false, err
	}
	now := time.Now().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx, `INSERT INTO defect_feishu_syncs
		(defect_id, project_key, work_item_type, work_item_id, work_item_url, sync_status,
		 synced_row_version, retry_count, last_error, last_attempt_at, next_retry_at, created_at, updated_at)
		VALUES (?, ?, ?, '', '', 'pending', 0, 0, '', '', ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		 work_item_id=IF(project_key=VALUES(project_key) AND work_item_type=VALUES(work_item_type), work_item_id, ''),
		 work_item_url=IF(project_key=VALUES(project_key) AND work_item_type=VALUES(work_item_type), work_item_url, ''),
		 project_key=VALUES(project_key), work_item_type=VALUES(work_item_type),
		 sync_status='pending', retry_count=0, last_error='', next_retry_at=VALUES(next_retry_at), updated_at=VALUES(updated_at)`,
		defect.ID, space.ProjectKey, space.WorkItemType, now, now, now)
	if err != nil {
		return false, err
	}
	wakeDefectFeishuSyncWorker()
	return true, nil
}

func hydrateDefectFeishuSyncStatuses(ctx context.Context, defects []Defect) error {
	eligibleIDs := make([]string, 0, len(defects))
	indexes := make(map[string]int, len(defects))
	for index := range defects {
		defects[index].FeishuSyncStatus = "not_synced"
		if defects[index].SyncToFeishu {
			eligibleIDs = append(eligibleIDs, defects[index].ID)
			indexes[defects[index].ID] = index
		}
	}
	if len(eligibleIDs) == 0 {
		return nil
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(eligibleIDs)), ",")
	args := make([]any, len(eligibleIDs))
	for index, id := range eligibleIDs {
		args[index] = id
	}
	rows, err := db.QueryContext(ctx, `SELECT defect_id, sync_status FROM defect_feishu_syncs WHERE defect_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var defectID, status string
		if err := rows.Scan(&defectID, &status); err != nil {
			return err
		}
		index, ok := indexes[defectID]
		if !ok {
			continue
		}
		switch status {
		case "synced":
			defects[index].FeishuSyncStatus = "synced"
		case "failed":
			defects[index].FeishuSyncStatus = "failed"
		default:
			defects[index].FeishuSyncStatus = "not_synced"
		}
	}
	return rows.Err()
}

func getDefectFeishuSync(ctx context.Context, defectID string) (*DefectFeishuSync, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	var item DefectFeishuSync
	err = db.QueryRowContext(ctx, `SELECT defect_id, project_key, work_item_type, work_item_id,
		work_item_url, sync_status, synced_row_version, retry_count, last_error, last_attempt_at,
		next_retry_at, updated_at FROM defect_feishu_syncs WHERE defect_id=?`, defectID).Scan(
		&item.DefectID, &item.ProjectKey, &item.WorkItemType, &item.WorkItemID, &item.WorkItemURL,
		&item.SyncStatus, &item.SyncedRowVersion, &item.RetryCount, &item.LastError,
		&item.LastAttemptAt, &item.NextRetryAt, &item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func processDueDefectFeishuSyncs(ctx context.Context, limit int) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339Nano)
	rows, err := db.QueryContext(ctx, `SELECT defect_id FROM defect_feishu_syncs
		WHERE sync_status IN ('pending','failed') AND next_retry_at<=? ORDER BY updated_at ASC LIMIT ?`, now, limit)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		result, err := db.ExecContext(ctx, `UPDATE defect_feishu_syncs SET sync_status='syncing',
			last_attempt_at=?, updated_at=? WHERE defect_id=? AND sync_status IN ('pending','failed')`, now, now, id)
		if err != nil {
			continue
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			continue
		}
		if err := syncDefectToFeishu(ctx, id); err != nil {
			markDefectFeishuSyncFailed(ctx, id, err)
			log.Printf("[DefectFeishuSync] defect %s failed: %v", id, err)
		}
	}
	return nil
}

func syncDefectToFeishu(ctx context.Context, defectID string) error {
	defect, err := getDefect(ctx, defectID)
	if err != nil || defect == nil {
		if err == nil {
			err = errors.New("缺陷不存在")
		}
		return err
	}
	syncItem, err := getDefectFeishuSync(ctx, defectID)
	if err != nil || syncItem == nil {
		if err == nil {
			err = errors.New("同步任务不存在")
		}
		return err
	}
	space, supported := defectFeishuSpaceForProject(defect.ProjectCode)
	if !supported {
		return fmt.Errorf("项目 %s 的所属空间不支持飞书缺陷同步", defect.ProjectCode)
	}
	ctx, err = withDefectFeishuOAuthForReporter(ctx, *defect)
	if err != nil {
		return err
	}
	fields := defectFeishuFields(*defect, space)
	workItemID, workItemURL := syncItem.WorkItemID, syncItem.WorkItemURL
	if syncItem.ProjectKey != space.ProjectKey || syncItem.WorkItemType != space.WorkItemType {
		// The project was moved to another Feishu workspace after the task was queued.
		// Never reuse a work item ID from the previous workspace.
		workItemID, workItemURL = "", ""
		db, _, dbErr := DatabaseManager.DB(ctx)
		if dbErr != nil {
			return dbErr
		}
		if _, dbErr = db.ExecContext(ctx, `UPDATE defect_feishu_syncs SET project_key=?, work_item_type=?,
			work_item_id='', work_item_url='', updated_at=? WHERE defect_id=?`,
			space.ProjectKey, space.WorkItemType, time.Now().Format(time.RFC3339Nano), defectID); dbErr != nil {
			return dbErr
		}
	}
	if workItemID == "" {
		workItemID, workItemURL, err = findExistingDefectFeishuWorkItem(ctx, *defect, space)
		if err != nil {
			return err
		}
		if workItemID == "" {
			result, createErr := callDefectFeishuMCP(ctx, "create_workitem", map[string]any{
				"project_key":    space.ProjectKey,
				"work_item_type": space.WorkItemType,
				"fields":         fields,
			})
			if createErr != nil {
				return createErr
			}
			workItemID, workItemURL = extractDefectFeishuWorkItem(result)
			if workItemID == "" {
				return fmt.Errorf("飞书创建成功但未返回工作项 ID: %s", truncateDefectFeishuText(result, 500))
			}
		} else {
			if _, updateErr := callDefectFeishuMCP(ctx, "update_field", map[string]any{
				"project_key":  space.ProjectKey,
				"work_item_id": workItemID,
				"fields":       fieldsWithoutTemplate(fields),
			}); updateErr != nil {
				return updateErr
			}
		}
	} else {
		result, err := callDefectFeishuMCP(ctx, "update_field", map[string]any{
			"project_key":  space.ProjectKey,
			"work_item_id": workItemID,
			"fields":       fieldsWithoutTemplate(fields),
		})
		if err != nil {
			return err
		}
		if workItemURL == "" {
			_, workItemURL = extractDefectFeishuWorkItem(result)
		}
	}

	if err := syncDefectFeishuRoles(ctx, workItemID, *defect, space); err != nil {
		return fmt.Errorf("经办人同步失败：%w", err)
	}
	warnings := []string{}
	if err := syncDefectFeishuStatus(ctx, workItemID, defect.Status, space); err != nil {
		warnings = append(warnings, "状态未同步："+err.Error())
	}
	latest, err := getDefect(ctx, defectID)
	if err != nil {
		return err
	}
	if latest != nil && latest.RowVersion != defect.RowVersion {
		if err := markDefectFeishuSyncPending(ctx, defectID, workItemID, workItemURL, defect.RowVersion); err != nil {
			return err
		}
		wakeDefectFeishuSyncWorker()
		return nil
	}
	return markDefectFeishuSyncSucceeded(ctx, defectID, workItemID, workItemURL, defect.RowVersion, strings.Join(warnings, "；"))
}

func findExistingDefectFeishuWorkItem(ctx context.Context, defect Defect, space defectFeishuSpaceConfig) (string, string, error) {
	prefixes := []string{defectFeishuNamePrefix(defect)}
	legacyPrefix := fmt.Sprintf("[%s]", defect.DefectNo)
	if prefixes[0] != legacyPrefix {
		prefixes = append(prefixes, legacyPrefix)
	}
	for _, prefix := range prefixes {
		escapedPrefix := strings.ReplaceAll(strings.ReplaceAll(prefix, `\`, `\\`), `'`, `\'`)
		mql := fmt.Sprintf("SELECT `work_item_id`, `name` FROM `%s`.`%s` WHERE `name` LIKE '%%%s%%' LIMIT 1",
			space.ProjectKey, space.WorkItemType, escapedPrefix)
		result, err := callDefectFeishuMCP(ctx, "search_by_mql", map[string]any{
			"project_key": space.ProjectKey,
			"mql":         mql,
		})
		if err != nil {
			return "", "", err
		}
		var decoded any
		if err := json.Unmarshal([]byte(result), &decoded); err != nil {
			return "", "", err
		}
		workItemID := findDefectFeishuMQLField(decoded, "work_item_id")
		if workItemID != "" {
			return workItemID, fmt.Sprintf("https://project.feishu.cn/%s/%s/detail/%s",
				space.ProjectKey, space.WorkItemType, workItemID), nil
		}
	}
	return "", "", nil
}

func defectFeishuFields(defect Defect, space defectFeishuSpaceConfig) []map[string]string {
	fields := []map[string]string{
		{"field_key": "name", "field_value": fmt.Sprintf("%s %s", defectFeishuNamePrefix(defect), defect.Title)},
		{"field_key": "template", "field_value": space.TemplateID},
		{"field_key": "priority", "field_value": defectFeishuPriority(defect.Severity)},
		{"field_key": "description", "field_value": defectFeishuDescription(defect)},
	}
	if business := defectFeishuBusiness(defect, space); business != "" {
		fields = append(fields, map[string]string{"field_key": "business", "field_value": business})
	}
	platform := defectFeishuPlatform(defect.Environment.Platform)
	if platform == "" {
		platform = defectFeishuPlatform(defect.ProjectName)
	}
	if platform != "" {
		value, _ := json.Marshal([]map[string]string{{"option_id": platform}})
		fields = append(fields, map[string]string{"field_key": "field_f7ef16", "field_value": string(value)})
	}
	return fields
}

func defectFeishuNamePrefix(defect Defect) string {
	if !defectFeishuIsTTmins(defect) {
		return fmt.Sprintf("[%s]", defect.DefectNo)
	}
	projectCode := strings.TrimSpace(defect.ProjectCode)
	sequence := strings.TrimPrefix(strings.TrimSpace(defect.DefectNo), "BUG-")
	if projectCode == "" || sequence == "" || sequence == defect.DefectNo {
		return fmt.Sprintf("[%s]", defect.DefectNo)
	}
	return fmt.Sprintf("[%s-%s]", projectCode, sequence)
}

func defectFeishuIsTTmins(defect Defect) bool {
	product := strings.ToLower(strings.Join([]string{
		defect.ProjectCode,
		defect.ProjectName,
		defect.Environment.Platform,
	}, " "))
	return strings.Contains(product, "ttmins") || strings.Contains(product, "tt mins")
}

func defectFeishuBusiness(defect Defect, space defectFeishuSpaceConfig) string {
	// A1138 is named "ShortsWave - TTmins", so TTmins must take precedence.
	if defectFeishuIsTTmins(defect) {
		// The Feishu MCP _business field accepts the final leaf option ID.
		return space.BusinessTT
	}
	product := strings.ToLower(strings.Join([]string{defect.ProjectCode, defect.ProjectName}, " "))
	if (strings.Contains(product, "shortswave") || strings.Contains(product, "shorts wave")) && space.BusinessShortsWaveOther != "" {
		// ShortsWave is a parent option, so use its generic leaf rather than the parent ID.
		return space.BusinessShortsWaveOther
	}
	return space.DefaultBusiness
}

func fieldsWithoutTemplate(fields []map[string]string) []map[string]string {
	result := make([]map[string]string, 0, len(fields))
	for _, field := range fields {
		if field["field_key"] != "template" {
			result = append(result, field)
		}
	}
	return result
}

func defectFeishuPriority(severity int) string {
	switch severity {
	case 1:
		return "option_1"
	case 2:
		return "option_2"
	default:
		return "option_3"
	}
}

func defectFeishuPlatform(platform string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(platform), " ", ""))
	switch {
	case strings.Contains(normalized, "android"):
		return "bhbxi3k4r"
	case strings.Contains(normalized, "ios"):
		return "v_j6_91mp"
	case strings.Contains(normalized, "ttmin") || strings.Contains(normalized, "tiktok"):
		return "trckpxtp2"
	case strings.Contains(normalized, "web") || strings.Contains(normalized, "h5"):
		return "ww3m1nibc"
	case strings.Contains(normalized, "后台"):
		return "sm8n44had"
	default:
		return ""
	}
}

func defectFeishuDescription(defect Defect) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### TestCenter 缺陷信息\n\n")
	fmt.Fprintf(&b, "- **缺陷编号**：%s\n", defect.DefectNo)
	fmt.Fprintf(&b, "- **所属项目**：%s · %s\n", defect.ProjectCode, defect.ProjectName)
	fmt.Fprintf(&b, "- **发现版本**：%s\n", fallbackDefectFeishuText(defect.FoundVersion))
	fmt.Fprintf(&b, "- **缺陷类型**：%s\n", fallbackDefectFeishuText(defect.DefectType))
	fmt.Fprintf(&b, "- **严重程度**：S%d\n", defect.Severity)
	fmt.Fprintf(&b, "- **平台优先级**：P%d\n", defect.Priority)
	fmt.Fprintf(&b, "- **当前状态**：%s\n", defectStatusLabel(defect.Status))
	fmt.Fprintf(&b, "- **提交人**：%s\n", fallbackDefectFeishuText(defect.ReporterName))
	fmt.Fprintf(&b, "- **处理人**：%s\n", fallbackDefectFeishuText(defect.AssigneeName))
	fmt.Fprintf(&b, "- **验证人**：%s\n", fallbackDefectFeishuText(defect.VerifierName))
	fmt.Fprintf(&b, "- **期望解决日期**：%s\n\n", fallbackDefectFeishuText(defect.DueDate))
	fmt.Fprintf(&b, "### 问题现场\n\n")
	fmt.Fprintf(&b, "**前置条件**\n\n%s\n\n", fallbackDefectFeishuText(defect.Precondition))
	b.WriteString("**重现步骤**\n\n")
	if len(defect.Steps) == 0 {
		b.WriteString("未填写\n\n")
	} else {
		for index, step := range defect.Steps {
			fmt.Fprintf(&b, "%d. %s\n", index+1, step.Content)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "**实际结果**\n\n%s\n\n", fallbackDefectFeishuText(defect.ActualResult))
	fmt.Fprintf(&b, "**预期结果**\n\n%s\n\n", fallbackDefectFeishuText(defect.ExpectedResult))
	fmt.Fprintf(&b, "**补充说明**\n\n%s\n\n", fallbackDefectFeishuText(defect.Description))
	fmt.Fprintf(&b, "### 测试环境\n\n- 平台：%s\n- 环境：%s\n- App 版本：%s\n- 系统版本：%s\n- 设备：%s\n- 网络：%s\n",
		fallbackDefectFeishuText(defect.Environment.Platform), fallbackDefectFeishuText(defect.Environment.Environment),
		fallbackDefectFeishuText(defect.Environment.AppVersion), fallbackDefectFeishuText(defect.Environment.OSVersion),
		fallbackDefectFeishuText(defect.Environment.DeviceModel), fallbackDefectFeishuText(defect.Environment.Network))
	return b.String()
}

func fallbackDefectFeishuText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未填写"
	}
	return strings.TrimSpace(value)
}

func syncDefectFeishuRoles(ctx context.Context, workItemID string, defect Defect, space defectFeishuSpaceConfig) error {
	// Reporter synchronization remains best-effort and must not prevent the
	// stricter operator replacement below.
	if reporterName := strings.TrimSpace(defect.ReporterName); reporterName != "" {
		if userKey, err := searchDefectFeishuUserKey(ctx, reporterName, space); err == nil && userKey != "" {
			_, _ = callDefectFeishuMCP(ctx, "update_field", map[string]any{
				"project_key":  space.ProjectKey,
				"work_item_id": workItemID,
				"role_operate": []map[string]any{{"role_key": "reporter", "op": "add", "user_keys": []string{userKey}}},
			})
		}
	}

	assigneeName := strings.TrimSpace(defect.AssigneeName)
	if assigneeName == "" {
		return nil
	}
	userKey, err := searchExactDefectFeishuUserKey(ctx, assigneeName, space)
	if err != nil {
		return err
	}

	// Replace rather than append: a changed platform assignee must not leave
	// the previous Feishu operator on the work item.
	if _, err := callDefectFeishuMCP(ctx, "update_field", map[string]any{
		"project_key":  space.ProjectKey,
		"work_item_id": workItemID,
		"role_operate": []map[string]any{{"role_key": "operator", "op": "remove", "user_keys": []string{"_all"}}},
	}); err != nil {
		return fmt.Errorf("移除当前全部经办人失败：%w", err)
	}
	if _, err := callDefectFeishuMCP(ctx, "update_field", map[string]any{
		"project_key":  space.ProjectKey,
		"work_item_id": workItemID,
		"role_operate": []map[string]any{{"role_key": "operator", "op": "add", "user_keys": []string{userKey}}},
	}); err != nil {
		return fmt.Errorf("指派经办人“%s”失败：%w", assigneeName, err)
	}
	return nil
}

func searchExactDefectFeishuUserKey(ctx context.Context, value string, space defectFeishuSpaceConfig) (string, error) {
	result, err := callDefectFeishuMCP(ctx, "search_user_info", map[string]any{
		"project_key": space.ProjectKey,
		"user_keys":   []string{value},
	})
	if err != nil {
		return "", fmt.Errorf("查询处理人“%s”失败：%w", value, err)
	}
	userKey, err := exactDefectFeishuUserKey(result, value)
	if err != nil {
		return "", err
	}
	return userKey, nil
}

func exactDefectFeishuUserKey(result, value string) (string, error) {
	var decoded any
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		return "", fmt.Errorf("处理人“%s”的飞书用户查询结果无法解析：%w", value, err)
	}
	candidates := collectDefectFeishuUsers(decoded)
	query := strings.TrimSpace(value)
	matches := map[string]bool{}
	for _, candidate := range candidates {
		for _, comparable := range []string{candidate.NameCN, candidate.NameEN, candidate.Username, candidate.Email, candidate.UserKey} {
			if strings.EqualFold(strings.TrimSpace(comparable), query) && candidate.UserKey != "" {
				matches[candidate.UserKey] = true
				break
			}
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("飞书空间未找到与平台处理人“%s”同名的在职用户", value)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("飞书空间找到多个与平台处理人“%s”同名的用户，无法唯一指派", value)
	}
	for userKey := range matches {
		return userKey, nil
	}
	return "", fmt.Errorf("飞书空间未找到处理人“%s”", value)
}

func collectDefectFeishuUsers(value any) []defectFeishuUserCandidate {
	result := []defectFeishuUserCandidate{}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			if userKey, _ := typed["user_key"].(string); strings.TrimSpace(userKey) != "" {
				result = append(result, defectFeishuUserCandidate{
					UserKey:  userKey,
					Username: stringDefectFeishuValue(typed["username"]),
					NameCN:   stringDefectFeishuValue(typed["name_cn"]),
					NameEN:   stringDefectFeishuValue(typed["name_en"]),
					Email:    stringDefectFeishuValue(typed["email"]),
				})
				return
			}
			for _, nested := range typed {
				walk(nested)
			}
		case []any:
			for _, nested := range typed {
				walk(nested)
			}
		}
	}
	walk(value)
	return result
}

func stringDefectFeishuValue(value any) string {
	text, _ := value.(string)
	return text
}

func searchDefectFeishuUserKey(ctx context.Context, value string, space defectFeishuSpaceConfig) (string, error) {
	result, err := callDefectFeishuMCP(ctx, "search_user_info", map[string]any{
		"project_key": space.ProjectKey,
		"user_keys":   []string{value},
	})
	if err != nil {
		return "", err
	}
	var decoded any
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		return "", err
	}
	return findDefectFeishuString(decoded, "user_key"), nil
}

func syncDefectFeishuStatus(ctx context.Context, workItemID, localStatus string, space defectFeishuSpaceConfig) error {
	desired := map[string]string{
		DefectStatusNew: "OPEN", DefectStatusActive: "IN PROGRESS",
		DefectStatusResolved: "RESOLVED", DefectStatusClosed: "CLOSED",
	}[localStatus]
	if desired == "" {
		return nil
	}
	userKey, err := searchDefectFeishuUserKey(ctx, "current_login_user()", space)
	if err != nil || userKey == "" {
		return fmt.Errorf("无法确定飞书项目操作人")
	}
	for attempt := 0; attempt < 3; attempt++ {
		result, err := callDefectFeishuMCP(ctx, "get_transitable_states", map[string]any{
			"project_key":    space.ProjectKey,
			"work_item_type": space.WorkItemType,
			"work_item_id":   workItemID,
			"user_key":       userKey,
		})
		if err != nil {
			return err
		}
		var state defectFeishuTransitionState
		if err := json.Unmarshal([]byte(result), &state); err != nil {
			return err
		}
		if strings.EqualFold(state.StateName, desired) {
			return nil
		}
		transition := chooseDefectFeishuTransition(state, desired)
		if transition == nil {
			return fmt.Errorf("当前状态 %s 无法流转到 %s", state.StateName, desired)
		}
		transitionID := fmt.Sprint(transition.ID)
		if number, ok := transition.ID.(float64); ok {
			transitionID = strconv.FormatInt(int64(number), 10)
		}
		if _, err := callDefectFeishuMCP(ctx, "transition_state", map[string]any{
			"project_key":   space.ProjectKey,
			"work_item_id":  workItemID,
			"transition_id": transitionID,
		}); err != nil {
			return err
		}
	}
	return fmt.Errorf("状态流转超过最大步骤")
}

func chooseDefectFeishuTransition(state defectFeishuTransitionState, desired string) *defectFeishuTransition {
	for index := range state.Transition {
		if strings.EqualFold(state.Transition[index].StateName, desired) {
			return &state.Transition[index]
		}
	}
	if desired == "CLOSED" {
		for _, intermediate := range []string{"RESOLVED", "IN PROGRESS"} {
			for index := range state.Transition {
				if strings.EqualFold(state.Transition[index].StateName, intermediate) {
					return &state.Transition[index]
				}
			}
		}
	}
	return nil
}

func callDefectFeishuMCP(ctx context.Context, toolName string, arguments map[string]any) (string, error) {
	config, err := feishumodel.LoadConfig("data/feishu_config.json")
	if err != nil {
		return "", err
	}
	if config.MCP == nil || !config.MCP.Enabled || strings.TrimSpace(config.MCP.ServerURL) == "" {
		return "", errors.New("飞书项目 MCP 未配置")
	}
	if accessToken := defectFeishuOAuthTokenFromContext(ctx); accessToken != "" {
		return callDefectFeishuMCPRequest(ctx, config.MCP.ServerURL, "Bearer "+accessToken, "", toolName, arguments)
	}
	if strings.TrimSpace(config.MCP.Token) == "" {
		return "", errors.New("飞书项目 MCP Token 未配置")
	}
	return callDefectFeishuMCPRequest(ctx, config.MCP.ServerURL, "", config.MCP.Token, toolName, arguments)
}

func callDefectFeishuMCPRequest(ctx context.Context, serverURL, authorization, mcpToken, toolName string, arguments map[string]any) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": time.Now().UnixNano(), "method": "tools/call",
		"params": map[string]any{"name": toolName, "arguments": arguments},
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	if mcpToken != "" {
		request.Header.Set("X-Mcp-Token", mcpToken)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("飞书项目 MCP HTTP %d: %s", response.StatusCode, truncateDefectFeishuText(string(responseBody), 500))
	}
	var result defectMCPResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", err
	}
	if result.Error != nil {
		return "", fmt.Errorf("飞书项目 MCP %d: %s", result.Error.Code, result.Error.Message)
	}
	if result.Result.IsError {
		message := "飞书项目工具执行失败"
		if len(result.Result.Content) > 0 {
			message = result.Result.Content[0].Text
		}
		return "", errors.New(message)
	}
	for _, content := range result.Result.Content {
		if content.Type == "text" && !strings.HasPrefix(content.Text, "log_id:") {
			return content.Text, nil
		}
	}
	return "{}", nil
}

func extractDefectFeishuWorkItem(value string) (string, string) {
	var decoded any
	workItemID, workItemURL := "", ""
	if json.Unmarshal([]byte(value), &decoded) == nil {
		workItemID = findDefectFeishuString(decoded, "work_item_id", "workItemId")
		workItemURL = findDefectFeishuString(decoded, "url", "work_item_url", "detail_url")
	}
	if workItemID == "" {
		if match := defectFeishuIDRE.FindStringSubmatch(value); len(match) > 1 {
			workItemID = match[1]
		}
	}
	if workItemURL == "" {
		workItemURL = defectFeishuURLRE.FindString(value)
	}
	return strings.TrimSpace(workItemID), strings.TrimSpace(workItemURL)
}

func findDefectFeishuString(value any, keys ...string) string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[key] = true
	}
	var walk func(any) string
	walk = func(current any) string {
		switch typed := current.(type) {
		case map[string]any:
			for key, nested := range typed {
				if wanted[key] {
					switch scalar := nested.(type) {
					case string:
						return scalar
					case float64:
						return strconv.FormatInt(int64(scalar), 10)
					}
				}
			}
			for _, nested := range typed {
				if found := walk(nested); found != "" {
					return found
				}
			}
		case []any:
			for _, nested := range typed {
				if found := walk(nested); found != "" {
					return found
				}
			}
		}
		return ""
	}
	return walk(value)
}

func findDefectFeishuMQLField(value any, fieldKey string) string {
	var walk func(any) string
	walk = func(current any) string {
		switch typed := current.(type) {
		case map[string]any:
			if key, _ := typed["key"].(string); key == fieldKey {
				if fieldValue, ok := typed["value"].(map[string]any); ok {
					for _, valueKey := range []string{"long_value", "string_value"} {
						switch scalar := fieldValue[valueKey].(type) {
						case string:
							return scalar
						case float64:
							return strconv.FormatInt(int64(scalar), 10)
						}
					}
				}
			}
			for _, nested := range typed {
				if found := walk(nested); found != "" {
					return found
				}
			}
		case []any:
			for _, nested := range typed {
				if found := walk(nested); found != "" {
					return found
				}
			}
		}
		return ""
	}
	return walk(value)
}

func markDefectFeishuSyncSucceeded(ctx context.Context, defectID, workItemID, workItemURL string, rowVersion int, warning string) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx, `UPDATE defect_feishu_syncs SET work_item_id=?, work_item_url=?,
		sync_status='synced', synced_row_version=?, retry_count=0, last_error=?, next_retry_at='', updated_at=? WHERE defect_id=?`,
		workItemID, workItemURL, rowVersion, warning, now, defectID)
	return err
}

func markDefectFeishuSyncPending(ctx context.Context, defectID, workItemID, workItemURL string, rowVersion int) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx, `UPDATE defect_feishu_syncs SET work_item_id=?, work_item_url=?,
		sync_status='pending', synced_row_version=?, retry_count=0, last_error='', next_retry_at=?, updated_at=? WHERE defect_id=?`,
		workItemID, workItemURL, rowVersion, now, now, defectID)
	return err
}

func markDefectFeishuSyncFailed(ctx context.Context, defectID string, syncErr error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return
	}
	var retryCount int
	_ = db.QueryRowContext(ctx, "SELECT retry_count FROM defect_feishu_syncs WHERE defect_id=?", defectID).Scan(&retryCount)
	retryCount++
	delay := time.Duration(1<<minDefectFeishuInt(retryCount-1, 6)) * time.Minute
	now := time.Now()
	_, _ = db.ExecContext(ctx, `UPDATE defect_feishu_syncs SET sync_status='failed', retry_count=?,
		last_error=?, next_retry_at=?, updated_at=? WHERE defect_id=?`, retryCount,
		truncateDefectFeishuText(syncErr.Error(), 2000), now.Add(delay).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), defectID)
}

func minDefectFeishuInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateDefectFeishuText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
