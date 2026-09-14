package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	feishumodel "testcenter-server/feishu/model"

	"github.com/gin-gonic/gin"
)

// --- Models ---

type AcceptanceReport struct {
	ID                  string `json:"id"`
	ProjectName         string `json:"project_name"`
	ProjectCode         string `json:"project_code"`
	Version             string `json:"version"`
	TestOwner           string `json:"test_owner"`
	Reporter            string `json:"reporter"` // Alias for TestOwner
	TestTime            string `json:"test_time"`
	TestEnv             string `json:"test_env"`
	TestDevices         string `json:"test_devices"`
	TestConclusion      string `json:"test_conclusion"`
	BugFixStatus        string `json:"bug_fix_status"`
	BugSubmissionStatus string `json:"bug_submission_status"`
	UpdateRequirements  string `json:"update_requirements"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
	Status              string `json:"status"`
}

var acceptanceReportNumericVersionPattern = regexp.MustCompile(`\d+(?:\.\d+)*`)

func normalizeAcceptanceReportVersion(value string) string {
	return acceptanceReportNumericVersionPattern.FindString(strings.TrimSpace(value))
}

func acceptanceReportDefectTitles(ctx context.Context, projectCode, version string) ([]string, error) {
	targetVersion := normalizeAcceptanceReportVersion(version)
	if targetVersion == "" {
		return []string{}, nil
	}
	project, err := ConfigServiceInstance.GetProjectBySubCode(projectCode)
	if err != nil || project == nil {
		return nil, fmt.Errorf("project not found for: %s", projectCode)
	}
	if err := ensureDefectSchema(ctx); err != nil {
		return nil, err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT title, found_version FROM defects
		WHERE archived = 0 AND project_code = ? ORDER BY created_at ASC, defect_no ASC`, project.ProjectCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	titles := []string{}
	for rows.Next() {
		var title, foundVersion string
		if err := rows.Scan(&title, &foundVersion); err != nil {
			return nil, err
		}
		if normalizeAcceptanceReportVersion(foundVersion) == targetVersion {
			titles = append(titles, strings.TrimSpace(title))
		}
	}
	return titles, rows.Err()
}

// --- Service Logic ---

var (
	reportMutex sync.Mutex
)

func SaveAcceptanceReport(report AcceptanceReport) error {
	reportMutex.Lock()
	err := sqlUpsertJSON("acceptance_reports", report)
	reportMutex.Unlock()
	if err != nil {
		return err
	}
	if err := QueueAcceptanceTodoReminder(report); err != nil {
		log.Printf("[AcceptanceTodo] queue report %s failed: %v", report.ID, err)
	}
	return nil
}

func GetAcceptanceReports() ([]AcceptanceReport, error) {
	reportMutex.Lock()
	defer reportMutex.Unlock()

	reports, err := sqlListJSON[AcceptanceReport]("acceptance_reports", "`migrated_at` ASC")
	if err != nil {
		return nil, err
	}

	// Reverse to show newest first
	for i, j := 0, len(reports)-1; i < j; i, j = i+1, j-1 {
		reports[i], reports[j] = reports[j], reports[i]
	}

	return reports, nil
}

func GetAcceptanceReportByID(id string) (*AcceptanceReport, error) {
	reports, err := GetAcceptanceReports()
	if err != nil {
		return nil, err
	}

	for _, report := range reports {
		if report.ID == id {
			r := report
			return &r, nil
		}
	}

	return nil, fmt.Errorf("acceptance report not found")
}

func FormatAcceptanceReportText(report AcceptanceReport) string {
	formattedCurrentVersionBugs := formatAcceptanceReportSection(report.BugFixStatus)
	formattedHistoricalBugFixes := formatAcceptanceReportSection(report.BugSubmissionStatus)
	formattedStories := formatAcceptanceReportSection(report.UpdateRequirements)

	devicesBlock := ""
	if report.TestDevices != "" {
		devicesBlock = fmt.Sprintf("测试设备：%s\n", report.TestDevices)
	}

	conclusion := report.TestConclusion
	if conclusion == "" {
		conclusion = "Pass"
	}

	return fmt.Sprintf("%s项目验收报告\n\n"+
		"项目名称：%s\n"+
		"版本号：%s\n"+
		"测试负责人：%s\n"+
		"测试时间：%s\n"+
		"测试环境：%s\n"+
		"%s"+
		"本次测试覆盖率： 100%%\n\n"+
		"测试结论：当前版本%s！\n"+
		"正式版本缺陷修复验证情况：\n%s"+
		"本次预提审版本缺陷提交情况：\n%s"+
		"版本更新测试需求点：\n%s",
		report.ProjectCode,
		report.ProjectName,
		report.Version,
		report.TestOwner,
		report.TestTime,
		report.TestEnv,
		devicesBlock,
		conclusion,
		formattedHistoricalBugFixes,
		formattedCurrentVersionBugs,
		formattedStories,
	)
}

func formatAcceptanceReportSection(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		trimmed = "无"
	}
	return trimmed + "\n\n"
}

func sendFeishuGroupText(text string) error {
	return sendFeishuGroupMessage("text", map[string]string{"text": text})
}

func sendFeishuGroupInteractiveCard(card map[string]any) error {
	return sendFeishuGroupMessage("interactive", card)
}

func chunkRunes(text string, limit int) []string {
	if limit <= 0 {
		return []string{text}
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return []string{text}
	}

	chunks := make([]string, 0, (len(runes)/limit)+1)
	for start := 0; start < len(runes); start += limit {
		end := start + limit
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func buildAcceptanceReportFeishuCard(report AcceptanceReport, senderName string) map[string]any {
	elements := []map[string]any{
		{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": buildAcceptanceReportSummaryBlock(report, senderName),
			},
		},
	}

	appendAcceptanceCardSection(&elements, "正式版本缺陷修复验证情况", report.BugSubmissionStatus)
	appendAcceptanceCardSection(&elements, "本次预提审版本缺陷提交情况", report.BugFixStatus)
	appendAcceptanceCardSection(&elements, "版本更新测试需求点", report.UpdateRequirements)

	return map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"template": "blue",
			"title": map[string]any{
				"tag":     "plain_text",
				"content": truncateRunes(formatAcceptanceReportFeishuTitle(report), 80),
			},
		},
		"elements": elements,
	}
}

func buildAcceptanceReportSummaryBlock(report AcceptanceReport, senderName string) string {
	conclusion := strings.TrimSpace(report.TestConclusion)
	if conclusion == "" {
		conclusion = "Pass"
	}

	lines := []string{
		formatAcceptanceCardField("提交人", senderName),
		formatAcceptanceCardField("项目编号", valueOrFallback(report.ProjectCode, report.ProjectName)),
		formatAcceptanceCardField("版本号", report.Version),
		formatAcceptanceCardField("测试负责人", report.TestOwner),
		formatAcceptanceCardField("测试时间", report.TestTime),
		formatAcceptanceCardField("测试环境", report.TestEnv),
	}
	if strings.TrimSpace(report.TestDevices) != "" {
		lines = append(lines, formatAcceptanceCardField("测试设备", report.TestDevices))
	}
	lines = append(lines,
		formatAcceptanceCardField("本次测试覆盖率", "100%"),
		fmt.Sprintf("**测试结论：** 当前版本%s！", escapeLarkMarkdown(conclusion)),
	)
	return strings.Join(lines, "\n")
}

func formatAcceptanceCardField(label string, value string) string {
	return fmt.Sprintf("**%s：** %s", label, escapeLarkMarkdown(valueOrFallback(value, "无")))
}

func appendAcceptanceCardSection(elements *[]map[string]any, title string, content string) {
	*elements = append(*elements, map[string]any{"tag": "hr"})

	sectionText := escapeLarkMarkdown(strings.TrimSpace(content))
	if sectionText == "" {
		sectionText = "无"
	}

	chunks := chunkRunes(sectionText, 2500)
	for index, chunk := range chunks {
		blockContent := chunk
		if index == 0 {
			blockContent = fmt.Sprintf("**%s**\n%s", escapeLarkMarkdown(title), chunk)
		}
		*elements = append(*elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": blockContent,
			},
		})
	}
}

func formatAcceptanceReportFeishuTitle(report AcceptanceReport) string {
	projectName := strings.TrimSpace(report.ProjectName)
	if projectName == "" {
		projectName = strings.TrimSpace(report.ProjectCode)
	}
	if projectName == "" {
		projectName = "未命名项目"
	}

	version := strings.TrimSpace(report.Version)
	if version == "" {
		return projectName + " 验收报告"
	}
	return projectName + " " + version + " 验收报告"
}

func sendFeishuGroupMessage(msgType string, content any) error {
	if feishumodel.GlobalFeishuConfig == nil {
		config, err := feishumodel.LoadConfig("data/feishu_config.json")
		if err != nil {
			return err
		}
		feishumodel.GlobalFeishuConfig = config
	}

	config := feishumodel.GlobalFeishuConfig
	if config == nil || config.AppID == "" || config.AppSecret == "" || config.GroupID == "" {
		return fmt.Errorf("feishu config incomplete")
	}

	authBody, _ := json.Marshal(map[string]string{
		"app_id":     config.AppID,
		"app_secret": config.AppSecret,
	})
	authResp, err := http.Post("https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", "application/json", bytes.NewBuffer(authBody))
	if err != nil {
		return err
	}
	defer authResp.Body.Close()

	var authResult struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.NewDecoder(authResp.Body).Decode(&authResult); err != nil {
		return err
	}
	if authResult.Code != 0 || authResult.TenantAccessToken == "" {
		return fmt.Errorf("feishu auth failed: %s", authResult.Msg)
	}

	contentBody, err := json.Marshal(content)
	if err != nil {
		return err
	}

	msgPayload, err := json.Marshal(map[string]string{
		"receive_id": config.GroupID,
		"msg_type":   msgType,
		"content":    string(contentBody),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id", bytes.NewBuffer(msgPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+authResult.TenantAccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu send failed: %s", string(respBody))
	}

	var sendResult struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &sendResult); err == nil && sendResult.Code != 0 {
		return fmt.Errorf("feishu send failed: %s", sendResult.Msg)
	}

	return nil
}

func getFeishuTenantAccessToken() (string, error) {
	if feishumodel.GlobalFeishuConfig == nil {
		config, err := feishumodel.LoadConfig("data/feishu_config.json")
		if err != nil {
			return "", err
		}
		feishumodel.GlobalFeishuConfig = config
	}

	config := feishumodel.GlobalFeishuConfig
	if config == nil || config.AppID == "" || config.AppSecret == "" {
		return "", fmt.Errorf("feishu config incomplete")
	}

	authBody, _ := json.Marshal(map[string]string{
		"app_id":     config.AppID,
		"app_secret": config.AppSecret,
	})
	authResp, err := http.Post("https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", "application/json", bytes.NewBuffer(authBody))
	if err != nil {
		return "", err
	}
	defer authResp.Body.Close()

	var authResult struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.NewDecoder(authResp.Body).Decode(&authResult); err != nil {
		return "", err
	}
	if authResult.Code != 0 || authResult.TenantAccessToken == "" {
		return "", fmt.Errorf("feishu auth failed: %s", authResult.Msg)
	}

	return authResult.TenantAccessToken, nil
}

func callFeishuAPI(method, apiURL, token string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(payload)
	}

	req, err := http.NewRequest(method, apiURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return respBody, fmt.Errorf("feishu api error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func parseFeishuDocTarget(wikiURL string) (string, string, bool) {
	parts := strings.Split(wikiURL, "/")
	for i, part := range parts {
		if i+1 >= len(parts) {
			continue
		}

		if part == "wiki" {
			return strings.Split(parts[i+1], "?")[0], "wiki", true
		}

		if part == "docx" {
			return strings.Split(parts[i+1], "?")[0], "docx", true
		}

		if part == "docs" {
			return strings.Split(parts[i+1], "?")[0], "doc", true
		}
	}

	return "", "", false
}

func resolveFeishuReadableDocTarget(wikiURL string, token string) (string, string, error) {
	rawToken, linkType, ok := parseFeishuDocTarget(wikiURL)
	if !ok || rawToken == "" {
		return "", "", fmt.Errorf("unable to parse Feishu wiki/docx URL")
	}

	if linkType == "docx" || linkType == "doc" {
		return rawToken, linkType, nil
	}

	apiURL := fmt.Sprintf("https://open.feishu.cn/open-apis/wiki/v2/spaces/get_node?token=%s", url.QueryEscape(rawToken))
	respBody, err := callFeishuAPI("GET", apiURL, token, nil)
	if err != nil {
		return "", "", err
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Node struct {
				ObjToken string `json:"obj_token"`
				ObjType  string `json:"obj_type"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", err
	}
	if result.Code != 0 {
		return "", "", fmt.Errorf("wiki node mapping failed: %s", result.Msg)
	}
	if result.Data.Node.ObjToken == "" {
		return "", "", fmt.Errorf("wiki node mapping returned empty doc token")
	}

	docType := strings.TrimSpace(strings.ToLower(result.Data.Node.ObjType))
	switch docType {
	case "docx":
		return result.Data.Node.ObjToken, "docx", nil
	case "doc":
		return result.Data.Node.ObjToken, "doc", nil
	default:
		return "", "", fmt.Errorf("only doc/docx cloud documents are supported, current type: %s", result.Data.Node.ObjType)
	}
}

func resolveFeishuDocxToken(wikiURL string, token string) (string, error) {
	docToken, docType, err := resolveFeishuReadableDocTarget(wikiURL, token)
	if err != nil {
		return "", err
	}
	if docType != "docx" {
		return "", fmt.Errorf("only docx cloud documents are supported, current type: %s", docType)
	}
	return docToken, nil
}

func formatAcceptanceReportForCloudDoc(report AcceptanceReport) string {
	reportText := FormatAcceptanceReportText(report)
	lines := strings.Split(reportText, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		lineTrim := strings.TrimSpace(line)
		if strings.HasSuffix(lineTrim, "项目验收报告") {
			continue
		}
		if strings.HasPrefix(lineTrim, "项目名称：") {
			continue
		}
		if strings.HasPrefix(lineTrim, "版本号：") {
			continue
		}
		filtered = append(filtered, line)
	}

	return strings.TrimSpace(strings.Join(filtered, "\n")) + "\n\n"
}

func formatAcceptanceReportVersionTitle(report AcceptanceReport) string {
	version := strings.TrimSpace(report.Version)
	if version == "" {
		return "未填写版本号"
	}
	if strings.HasPrefix(strings.ToLower(version), "v") {
		return version
	}
	return "v" + version
}

func syncAcceptanceReportToCloudDoc(report AcceptanceReport, wikiURL string) error {
	tenantToken, err := getFeishuTenantAccessToken()
	if err != nil {
		return err
	}

	docToken, err := resolveFeishuDocxToken(wikiURL, tenantToken)
	if err != nil {
		return err
	}

	textBlock := map[string]interface{}{
		"block_type": 2,
		"text": map[string]interface{}{
			"style": map[string]interface{}{},
			"elements": []map[string]interface{}{
				{
					"text_run": map[string]interface{}{
						"content": formatAcceptanceReportForCloudDoc(report),
					},
				},
			},
		},
	}
	h2Block := map[string]interface{}{
		"block_type": 4,
		"heading2": map[string]interface{}{
			"style": map[string]interface{}{},
			"elements": []map[string]interface{}{
				{
					"text_run": map[string]interface{}{
						"content": formatAcceptanceReportVersionTitle(report),
					},
				},
			},
		},
	}

	apiURL := fmt.Sprintf("https://open.feishu.cn/open-apis/docx/v1/documents/%s/blocks/%s/children", url.PathEscape(docToken), url.PathEscape(docToken))
	payload := map[string]interface{}{
		"index":    0,
		"children": []map[string]interface{}{h2Block, textBlock},
	}
	respBody, err := callFeishuAPI("POST", apiURL, tenantToken, payload)
	if err != nil {
		return err
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err == nil && result.Code != 0 {
		return fmt.Errorf("sync cloud document failed: %s", result.Msg)
	}

	return nil
}

// --- Handlers ---

func SaveAcceptanceReportHandler(c *gin.Context) {
	currentUser, err := currentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
		return
	}

	var req AcceptanceReport
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	req.ID = strings.TrimSpace(req.ID)
	req.Reporter = strings.TrimSpace(req.Reporter)
	req.TestOwner = strings.TrimSpace(req.TestOwner)
	isNewReport := true

	if req.ID != "" {
		existing, existingErr := GetAcceptanceReportByID(req.ID)
		if existingErr == nil && existing != nil {
			isNewReport = false
			if !strings.EqualFold(strings.TrimSpace(existing.Reporter), strings.TrimSpace(currentUser.Username)) {
				c.JSON(http.StatusForbidden, gin.H{"error": "仅报告创建者可修改该验收报告"})
				return
			}
			if req.CreatedAt == "" {
				req.CreatedAt = existing.CreatedAt
			}
			// Preserve original creator to avoid ownership drift during edit.
			req.Reporter = existing.Reporter
		}
	}

	if req.Reporter == "" {
		req.Reporter = currentUser.Username
	}
	if req.TestOwner == "" {
		req.TestOwner = req.Reporter
	}
	if req.CreatedAt == "" {
		req.CreatedAt = time.Now().Format(time.RFC3339)
	}
	req.UpdatedAt = time.Now().Format(time.RFC3339)
	if req.Status == "" {
		req.Status = "Completed"
	}

	if err := SaveAcceptanceReport(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if isNewReport {
		queueAcceptanceReportConfigAnalysis(req)
	}
	versionSyncWarning := ""
	if err := SyncAcceptanceProjectVersion(req.ProjectCode); err != nil {
		versionSyncWarning = "验收报告已保存，但项目版本同步失败：" + err.Error()
		log.Printf("[AcceptanceVersion] %s: %v", req.ProjectCode, err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Acceptance report saved successfully",
		"version_sync_warning": versionSyncWarning,
		"config_ai_queued":     isNewReport,
		"config_project_code":  strings.TrimSpace(req.ProjectCode),
	})
}

func GetAcceptanceReportsHandler(c *gin.Context) {
	reports, err := GetAcceptanceReports()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reports)
}

func FetchAcceptanceReportDefectTitlesHandler(c *gin.Context) {
	var req struct {
		ProjectCode string `json:"project_code"`
		Version     string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	projectCode := strings.TrimSpace(req.ProjectCode)
	version := strings.TrimSpace(req.Version)
	if projectCode == "" || version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_code and version are required"})
		return
	}
	titles, err := acceptanceReportDefectTitles(c.Request.Context(), projectCode, version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"titles": titles})
}

func SendAcceptanceReportToFeishuHandler(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	currentUser, err := currentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user"})
		return
	}

	report, err := GetAcceptanceReportByID(req.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if report.Reporter != currentUser.Username {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the report owner can send this report to Feishu"})
		return
	}

	card := buildAcceptanceReportFeishuCard(*report, currentUser.Username)
	if err := sendFeishuGroupInteractiveCard(card); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Acceptance report sent to Feishu successfully"})
}

func SyncAcceptanceReportToCloudDocHandler(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	currentUser, err := currentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user"})
		return
	}

	report, err := GetAcceptanceReportByID(req.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if report.Reporter != currentUser.Username {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the report owner can sync this report to cloud document"})
		return
	}

	project, err := ConfigServiceInstance.GetProjectBySubCode(report.ProjectCode)
	if err != nil || project == nil || strings.TrimSpace(project.WikiURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current report project has not configured a cloud document link"})
		return
	}

	if err := syncAcceptanceReportToCloudDoc(*report, strings.TrimSpace(project.WikiURL)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Acceptance report synced to cloud document successfully"})
}
