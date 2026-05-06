package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type acceptanceReportWorkItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Link   string `json:"link"`
}

type acceptanceReportProjectItems struct {
	ProjectName     string                     `json:"project_name"`
	ProjectCode     string                     `json:"project_code"`
	Version         string                     `json:"version"`
	StoryLinks      []string                   `json:"story_links"`
	BugLinksUnfixed []string                   `json:"bug_links_unfixed"`
	BugLinksFixed   []string                   `json:"bug_links_fixed"`
	Stories         []acceptanceReportWorkItem `json:"stories"`
	BugsUnfixed     []acceptanceReportWorkItem `json:"bugs_unfixed"`
	BugsFixed       []acceptanceReportWorkItem `json:"bugs_fixed"`
	StoryMQL        string                     `json:"story_mql"`
	BugMQL          string                     `json:"bug_mql"`
}

// --- Service Logic ---

var (
	reportMutex sync.Mutex
)

func SaveAcceptanceReport(report AcceptanceReport) error {
	reportMutex.Lock()
	defer reportMutex.Unlock()
	return sqlUpsertJSON("acceptance_reports", report)
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
	formattedFixed := formatAcceptanceReportSection(report.BugFixStatus)
	formattedUnfixed := formatAcceptanceReportSection(report.BugSubmissionStatus)
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
		formattedFixed,
		formattedUnfixed,
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

	contentBody, err := json.Marshal(map[string]string{
		"text": text,
	})
	if err != nil {
		return err
	}

	msgPayload, err := json.Marshal(map[string]string{
		"receive_id": config.GroupID,
		"msg_type":   "text",
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

func getFeishuProjectKey(workspace string) string {
	projectKeyMap := map[string]string{
		"海外短剧":    "shortwave",
		"免费短剧":    "freedrama",
		"iOS订阅产品": "ios_sub",
		"番茄短剧":    "tomato",
	}
	return projectKeyMap[workspace]
}

func splitProjectBusinessAndPlatform(projectName string) (string, string) {
	parts := strings.SplitN(projectName, " - ", 2)
	if len(parts) != 2 {
		return strings.TrimSpace(projectName), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func quoteMQLString(value string) string {
	return strings.ReplaceAll(value, "'", "\\'")
}

func acceptanceReportVersionCandidates(version string) []string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return nil
	}

	seen := map[string]bool{}
	add := func(value string, candidates *[]string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		*candidates = append(*candidates, value)
	}

	withoutPrefix := strings.TrimLeft(strings.TrimLeft(trimmed, "v"), "V")
	var candidates []string
	add("V"+withoutPrefix, &candidates)
	add(trimmed, &candidates)
	add("v"+withoutPrefix, &candidates)
	add(withoutPrefix, &candidates)

	return candidates
}

func buildAcceptanceReportProjectItemMQL(projectKey string, businessLine string, platform string, version string) (string, string) {
	versionValue := quoteMQLString(version)
	storyConditions := []string{fmt.Sprintf("`planning_version` = '%s'", versionValue)}
	bugConditions := []string{fmt.Sprintf("`解决版本` = '%s'", versionValue)}

	if strings.EqualFold(platform, "iOS") {
		storyConditions = append(storyConditions, "`field_b980a4` = 'iOS端需求'")
		bugConditions = append(bugConditions, "`field_f7ef16` = 'IOS'")
	} else if strings.EqualFold(platform, "Android") {
		storyConditions = append(storyConditions, "`field_b980a4` = '安卓端需求'")
		bugConditions = append(bugConditions, "`field_f7ef16` = 'Android'")
	}

	storyMQL := fmt.Sprintf("SELECT `work_item_id`, `name`, `work_item_status` FROM `%s`.`story` WHERE %s", projectKey, strings.Join(storyConditions, " AND "))
	bugMQL := fmt.Sprintf("SELECT `work_item_id`, `name`, `work_item_status` FROM `%s`.`63329b6c980d67099b12fd73` WHERE %s", projectKey, strings.Join(bugConditions, " AND "))

	return storyMQL, bugMQL
}

func isMQLVersionLabelNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "attrValueLabel not found") ||
		strings.Contains(message, "attribute key or value error")
}

func fetchAcceptanceReportMCPItemsStrict(ctx context.Context, projectKey string, businessLine string, platform string, version string, itemType string) ([]acceptanceReportWorkItem, string, error) {
	candidates := acceptanceReportVersionCandidates(version)
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("version is required")
	}

	var lastErr error
	for _, candidate := range candidates {
		storyMQL, bugMQL := buildAcceptanceReportProjectItemMQL(projectKey, businessLine, platform, candidate)
		mql := storyMQL
		if itemType == "bug" {
			mql = bugMQL
		}

		raw, err := callAcceptanceReportMCPTool(ctx, "search_by_mql", map[string]interface{}{
			"project_key": projectKey,
			"mql":         mql,
		})
		if err != nil {
			lastErr = fmt.Errorf("fetch %s items failed with version %s: %w", itemType, candidate, err)
			if isMQLVersionLabelNotFoundError(err) {
				continue
			}
			return nil, mql, lastErr
		}

		return parseMCPWorkItems(raw, projectKey, itemType), mql, nil
	}

	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", nil
}

func callAcceptanceReportMCPTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	if feishumodel.GlobalFeishuConfig == nil {
		config, err := feishumodel.LoadConfig("data/feishu_config.json")
		if err != nil {
			return "", err
		}
		feishumodel.GlobalFeishuConfig = config
	}

	config := feishumodel.GlobalFeishuConfig
	if config == nil || config.MCP == nil || !config.MCP.Enabled || config.MCP.ServerURL == "" || config.MCP.Token == "" {
		return "", fmt.Errorf("feishu MCP config incomplete")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"id":      time.Now().UnixNano(),
		"params": map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.MCP.ServerURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mcp-Token", config.MCP.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("MCP server returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse MCP response: %v", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("MCP error %d: %s", result.Error.Code, result.Error.Message)
	}
	if result.Result.IsError {
		if len(result.Result.Content) > 0 {
			return "", fmt.Errorf("MCP tool error: %s", result.Result.Content[0].Text)
		}
		return "", fmt.Errorf("MCP tool error")
	}
	if len(result.Result.Content) == 0 {
		return "", nil
	}

	return result.Result.Content[0].Text, nil
}

func parseMCPWorkItems(raw string, projectKey string, itemType string) []acceptanceReportWorkItem {
	items := parseMCPWorkItemsFromJSON(raw, projectKey, itemType)
	if len(items) > 0 {
		return dedupeAcceptanceReportWorkItems(items)
	}
	return dedupeAcceptanceReportWorkItems(parseMCPWorkItemsFromText(raw, projectKey, itemType))
}

func parseMCPWorkItemsFromJSON(raw string, projectKey string, itemType string) []acceptanceReportWorkItem {
	var payload interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}

	var items []acceptanceReportWorkItem
	var walk func(interface{})
	walk = func(value interface{}) {
		switch typed := value.(type) {
		case map[string]interface{}:
			if fieldList, ok := typed["moql_field_list"].([]interface{}); ok {
				if item, ok := parseMCPMoqlFieldList(fieldList, projectKey, itemType); ok {
					items = append(items, item)
					return
				}
			}

			id := firstStringValue(typed, "work_item_id", "workItemID", "工作项ID", "工作项 ID")
			if id != "" {
				status := firstStringValue(typed, "work_item_status", "workItemStatus", "status", "状态")
				name := firstStringValue(typed, "name", "title", "名称", "标题")
				items = append(items, newAcceptanceReportWorkItem(id, name, status, projectKey, itemType))
				return
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(payload)
	return items
}

func parseMCPMoqlFieldList(fieldList []interface{}, projectKey string, itemType string) (acceptanceReportWorkItem, bool) {
	var id string
	var name string
	var status string

	for _, entry := range fieldList {
		fieldMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		fieldKey := firstStringValue(fieldMap, "key")
		valueMap, _ := fieldMap["value"].(map[string]interface{})

		switch fieldKey {
		case "work_item_id":
			id = firstStringValue(valueMap, "long_value", "string_value", "value")
		case "name":
			name = firstStringValue(valueMap, "string_value", "value")
		case "work_item_status":
			status = parseMCPStatusValue(valueMap)
		}
	}

	if strings.TrimSpace(id) == "" {
		return acceptanceReportWorkItem{}, false
	}

	return newAcceptanceReportWorkItem(id, name, status, projectKey, itemType), true
}

func parseMCPStatusValue(valueMap map[string]interface{}) string {
	if valueMap == nil {
		return ""
	}

	if text := firstStringValue(valueMap, "label", "string_value", "value"); text != "" {
		return text
	}

	if list, ok := valueMap["key_label_value_list"].([]interface{}); ok {
		for _, entry := range list {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			if label := firstStringValue(entryMap, "label", "key"); label != "" {
				return label
			}
		}
	}

	return ""
}

func parseMCPWorkItemsFromText(raw string, projectKey string, itemType string) []acceptanceReportWorkItem {
	idPattern := regexp.MustCompile(`(?:work_item_id|工作项ID|工作项 ID)["'\s:=：]+([0-9]{3,})`)
	statusPattern := regexp.MustCompile(`(?:work_item_status|状态|status)["'\s:=：]+([^,\n\r}\]]+)`)
	matches := idPattern.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return nil
	}

	var items []acceptanceReportWorkItem
	for index, match := range matches {
		id := raw[match[2]:match[3]]
		end := len(raw)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		segment := raw[match[0]:end]
		status := ""
		if statusMatch := statusPattern.FindStringSubmatch(segment); len(statusMatch) > 1 {
			status = strings.Trim(statusMatch[1], ` "'，,`)
		}
		items = append(items, newAcceptanceReportWorkItem(id, "", status, projectKey, itemType))
	}

	return items
}

func firstStringValue(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			switch typed := value.(type) {
			case string:
				return strings.TrimSpace(typed)
			case float64:
				return strings.TrimSpace(fmt.Sprintf("%.0f", typed))
			case json.Number:
				return strings.TrimSpace(typed.String())
			case map[string]interface{}:
				if text := firstStringValue(typed, "label", "name", "value", "text"); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func newAcceptanceReportWorkItem(id string, name string, status string, projectKey string, itemType string) acceptanceReportWorkItem {
	id = strings.TrimSpace(id)
	return acceptanceReportWorkItem{
		ID:     id,
		Name:   strings.TrimSpace(name),
		Status: strings.TrimSpace(status),
		Link:   fmt.Sprintf("https://project.feishu.cn/%s/%s/detail/%s", projectKey, itemType, id),
	}
}

func dedupeAcceptanceReportWorkItems(items []acceptanceReportWorkItem) []acceptanceReportWorkItem {
	seen := map[string]bool{}
	deduped := make([]acceptanceReportWorkItem, 0, len(items))
	for _, item := range items {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		deduped = append(deduped, item)
	}
	return deduped
}

func acceptanceReportWorkItemLinks(items []acceptanceReportWorkItem) []string {
	links := make([]string, 0, len(items))
	for _, item := range items {
		if item.Link != "" {
			links = append(links, item.Link)
		}
	}
	return links
}

func isClosedAcceptanceBugStatus(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	closedKeywords := []string{"close", "closed", "done", "已关闭", "已完成", "已解决", "关闭", "完成"}
	for _, keyword := range closedKeywords {
		if strings.Contains(normalized, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func fetchAcceptanceReportProjectItems(ctx context.Context, projectCode string, version string) (*acceptanceReportProjectItems, error) {
	project, err := ConfigServiceInstance.GetProjectBySubCode(projectCode)
	if err != nil || project == nil {
		return nil, fmt.Errorf("project not found for: %s", projectCode)
	}
	if strings.TrimSpace(project.Workspace) == "" {
		return nil, fmt.Errorf("current project has not configured workspace")
	}

	projectKey := getFeishuProjectKey(project.Workspace)
	if projectKey == "" {
		return nil, fmt.Errorf("unsupported project workspace: %s", project.Workspace)
	}

	businessLine, platform := splitProjectBusinessAndPlatform(project.ProjectName)
	if businessLine == "" {
		return nil, fmt.Errorf("unable to parse project business line")
	}

	stories, storyMQL, err := fetchAcceptanceReportMCPItemsStrict(ctx, projectKey, businessLine, platform, version, "story")
	if err != nil {
		return nil, err
	}
	bugs, bugMQL, err := fetchAcceptanceReportMCPItemsStrict(ctx, projectKey, businessLine, platform, version, "bug")
	if err != nil {
		return nil, err
	}

	var bugsFixed []acceptanceReportWorkItem
	var bugsUnfixed []acceptanceReportWorkItem
	for _, bug := range bugs {
		if isClosedAcceptanceBugStatus(bug.Status) {
			bugsFixed = append(bugsFixed, bug)
		} else {
			bugsUnfixed = append(bugsUnfixed, bug)
		}
	}

	return &acceptanceReportProjectItems{
		ProjectName:     project.ProjectName,
		ProjectCode:     project.ProjectCode,
		Version:         version,
		StoryLinks:      acceptanceReportWorkItemLinks(stories),
		BugLinksUnfixed: acceptanceReportWorkItemLinks(bugsUnfixed),
		BugLinksFixed:   acceptanceReportWorkItemLinks(bugsFixed),
		Stories:         stories,
		BugsUnfixed:     bugsUnfixed,
		BugsFixed:       bugsFixed,
		StoryMQL:        storyMQL,
		BugMQL:          bugMQL,
	}, nil
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

	if req.ID != "" {
		existing, existingErr := GetAcceptanceReportByID(req.ID)
		if existingErr == nil && existing != nil {
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

	c.JSON(http.StatusOK, gin.H{"message": "Acceptance report saved successfully"})
}

func GetAcceptanceReportsHandler(c *gin.Context) {
	reports, err := GetAcceptanceReports()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reports)
}

func FetchAcceptanceReportProjectItemsHandler(c *gin.Context) {
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

	items, err := fetchAcceptanceReportProjectItems(c.Request.Context(), projectCode, version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
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

	reportText := FormatAcceptanceReportText(*report)
	messageText := fmt.Sprintf("【%s 提交的验收报告】\n\n%s", currentUser.Username, reportText)
	if err := sendFeishuGroupText(messageText); err != nil {
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
