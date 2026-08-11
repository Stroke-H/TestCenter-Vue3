package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	feishumodel "testcenter-server/feishu/model"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

const projectConfigAICapability = "project_config"

type ProjectConfigMemoHistory struct {
	Content    string `json:"content"`
	Color      string `json:"color"`
	ModifiedAt string `json:"modifiedAt"`
}

type ProjectConfigMemoItem struct {
	ID             string                     `json:"id"`
	Content        string                     `json:"content"`
	Color          string                     `json:"color"`
	UpdatedAt      string                     `json:"updatedAt"`
	History        []ProjectConfigMemoHistory `json:"history,omitempty"`
	Kind           string                     `json:"kind,omitempty"`
	ConfigKey      string                     `json:"configKey,omitempty"`
	SourceReportID string                     `json:"sourceReportId,omitempty"`
	SourceHash     string                     `json:"sourceHash,omitempty"`
}

type ProjectConfigRecord struct {
	ProjectCode string                  `json:"project_code"`
	Items       []ProjectConfigMemoItem `json:"items"`
	UpdatedAt   string                  `json:"updated_at"`
}

type extractedProjectConfig struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

var (
	projectConfigMutex     sync.Mutex
	feishuURLPattern       = regexp.MustCompile(`https?://[^\s<>"'，。；、]+`)
	nonConfigKeyCharacters = regexp.MustCompile(`[^\p{Han}a-zA-Z0-9]+`)
)

func ensureProjectConfigTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS project_config_records (
			project_code VARCHAR(128) NOT NULL,
			items JSON NOT NULL,
			updated_at VARCHAR(64) NOT NULL,
			raw_json JSON NOT NULL,
			migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (project_code)
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
	`)
	return err
}

func listProjectConfigRecords() ([]ProjectConfigRecord, error) {
	if err := ensureProjectConfigTable(); err != nil {
		return nil, err
	}
	return sqlListJSON[ProjectConfigRecord]("project_config_records", "`project_code` ASC")
}

func saveProjectConfigRecord(record ProjectConfigRecord) error {
	if err := ensureProjectConfigTable(); err != nil {
		return err
	}
	record.ProjectCode = strings.TrimSpace(record.ProjectCode)
	if record.ProjectCode == "" {
		return fmt.Errorf("project_code is required")
	}
	if record.Items == nil {
		record.Items = []ProjectConfigMemoItem{}
	}
	record.UpdatedAt = time.Now().Format(time.RFC3339)
	return sqlUpsertJSON("project_config_records", record)
}

func getProjectConfigRecord(projectCode string) (ProjectConfigRecord, error) {
	records, err := listProjectConfigRecords()
	if err != nil {
		return ProjectConfigRecord{}, err
	}
	for _, record := range records {
		if projectCodesMatch(record.ProjectCode, projectCode) {
			return record, nil
		}
	}
	return ProjectConfigRecord{
		ProjectCode: strings.TrimSpace(projectCode),
		Items:       []ProjectConfigMemoItem{},
	}, nil
}

func GetProjectConfigRecordsHandler(c *gin.Context) {
	records, err := listProjectConfigRecords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, records)
}

func SaveProjectConfigRecordHandler(c *gin.Context) {
	var record ProjectConfigRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	for index := range record.Items {
		if record.Items[index].Kind == "ai" {
			record.Items[index].Color = "blue"
		}
	}
	if err := saveProjectConfigRecord(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Project config record saved"})
}

func GetProjectConfigRecord(projectCode string) (ProjectConfigRecord, error) {
	return getProjectConfigRecord(projectCode)
}

func AnalyzeProjectConfigHandler(c *gin.Context) {
	var req struct {
		ProjectCode string `json:"project_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ProjectCode) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_code is required"})
		return
	}
	count, err := AnalyzeProjectReportsForConfig(c.Request.Context(), req.ProjectCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Project config analysis completed", "config_count": count})
}

func queueAcceptanceReportConfigAnalysis(report AcceptanceReport) {
	reportCopy := report
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if _, err := analyzeAcceptanceReportsForConfig(ctx, []AcceptanceReport{reportCopy}); err != nil {
			log.Printf("[ProjectConfigAI] report %s analysis skipped: %v", reportCopy.ID, err)
		}
	}()
}

func AnalyzeProjectReportsForConfig(ctx context.Context, projectCode string) (int, error) {
	reports, err := GetAcceptanceReports()
	if err != nil {
		return 0, err
	}
	matched := make([]AcceptanceReport, 0)
	for _, report := range reports {
		if projectCodesMatch(report.ProjectCode, projectCode) {
			matched = append(matched, report)
		}
	}
	if len(matched) == 0 {
		return 0, fmt.Errorf("project %s has no acceptance reports", projectCode)
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].CreatedAt < matched[j].CreatedAt
	})
	return analyzeAcceptanceReportsForConfig(ctx, matched)
}

func analyzeAcceptanceReportsForConfig(ctx context.Context, reports []AcceptanceReport) (int, error) {
	total := 0
	attempted := 0
	failed := 0
	var lastErr error
	for _, report := range reports {
		attempted++
		configs, sourceHash, err := extractConfigsFromAcceptanceReport(ctx, report)
		if err != nil {
			log.Printf("[ProjectConfigAI] report %s extraction skipped: %v", report.ID, err)
			failed++
			lastErr = err
			continue
		}
		if len(configs) == 0 {
			continue
		}
		if err := upsertAIProjectConfigs(report, sourceHash, configs); err != nil {
			return total, err
		}
		total += len(configs)
	}
	if attempted > 0 && failed == attempted {
		return total, fmt.Errorf("all %d project config analyses failed: %w", attempted, lastErr)
	}
	return total, nil
}

func extractConfigsFromAcceptanceReport(ctx context.Context, report AcceptanceReport) ([]extractedProjectConfig, string, error) {
	sourceParts := []string{
		"项目：" + strings.TrimSpace(report.ProjectCode),
		"版本：" + strings.TrimSpace(report.Version),
	}
	fields := []struct {
		Name    string
		Content string
	}{
		{Name: "版本更新测试需求点", Content: report.UpdateRequirements},
		{Name: "正式版本缺陷修复验证情况", Content: report.BugSubmissionStatus},
		{Name: "本次预提审版本缺陷提交情况", Content: report.BugFixStatus},
		{Name: "测试结论", Content: report.TestConclusion},
	}

	for _, field := range fields {
		content := strings.TrimSpace(field.Content)
		if content == "" {
			continue
		}
		urls := feishuURLPattern.FindAllString(content, -1)
		if len(urls) > 0 {
			content = strings.TrimSpace(feishuURLPattern.ReplaceAllString(content, ""))
			log.Printf("[ProjectConfigAI] report %s links removed, analyzing remaining text: %s", report.ID, field.Name)
			if content == "" {
				continue
			}
		}
		sourceParts = append(sourceParts, field.Name+"：\n"+content)
	}

	source := strings.TrimSpace(strings.Join(sourceParts, "\n\n"))
	if source == "" {
		return nil, "", nil
	}
	hashBytes := sha256.Sum256([]byte(source))
	sourceHash := hex.EncodeToString(hashBytes[:])

	systemPrompt := `你是软件测试项目的配置情报分析助手。请从验收记录中，只提取明确的项目配置变化或配置要求。
配置包括但不限于：开关、环境参数、域名、接口地址、应用包、渠道、账号、权限、广告策略、支付/解锁方式、App Group、版本兼容配置。
不要提取普通需求描述、缺陷现象、测试结论、排期或人员信息。
每一项配置单独输出。key 必须是稳定、简短、可用于识别同一配置的中文或英文键；同一配置后续值变化时必须使用相同 key。
content 必须是可独立阅读的中文配置摘要，包含配置对象和明确值/规则。不要猜测。
只输出 JSON 数组，不要 Markdown。格式：[{"key":"配置键","content":"配置摘要"}]。没有明确配置时输出 []。`
	aiContent, err := callProjectConfigAI(ctx, systemPrompt, truncateRunes(source, 30000))
	if err != nil {
		return nil, sourceHash, err
	}

	configs, err := parseExtractedProjectConfigs(aiContent)
	if err != nil {
		return nil, sourceHash, err
	}
	return configs, sourceHash, nil
}

func parseExtractedProjectConfigs(content string) ([]extractedProjectConfig, error) {
	text := strings.TrimSpace(content)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start < 0 || end < start {
		return nil, fmt.Errorf("AI project config response is not a JSON array")
	}

	var raw []extractedProjectConfig
	if err := json.Unmarshal([]byte(text[start:end+1]), &raw); err != nil {
		return nil, fmt.Errorf("parse AI project config response: %w", err)
	}
	seen := map[string]bool{}
	configs := make([]extractedProjectConfig, 0, len(raw))
	for _, item := range raw {
		item.Key = normalizeConfigKey(item.Key)
		item.Content = strings.TrimSpace(item.Content)
		if item.Key == "" || item.Content == "" || seen[item.Key] {
			continue
		}
		seen[item.Key] = true
		configs = append(configs, item)
	}
	return configs, nil
}

func upsertAIProjectConfigs(report AcceptanceReport, sourceHash string, configs []extractedProjectConfig) error {
	projectConfigMutex.Lock()
	defer projectConfigMutex.Unlock()

	record, err := getProjectConfigRecord(report.ProjectCode)
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)

	for _, config := range configs {
		index := -1
		for itemIndex, item := range record.Items {
			if item.Kind == "ai" && normalizeConfigKey(item.ConfigKey) == config.Key {
				index = itemIndex
				break
			}
		}
		if index < 0 {
			record.Items = append(record.Items, ProjectConfigMemoItem{
				ID:             fmt.Sprintf("ai-config-%d-%s", time.Now().UnixNano(), config.Key),
				Content:        config.Content,
				Color:          "blue",
				UpdatedAt:      now,
				History:        []ProjectConfigMemoHistory{},
				Kind:           "ai",
				ConfigKey:      config.Key,
				SourceReportID: report.ID,
				SourceHash:     sourceHash,
			})
			continue
		}

		current := record.Items[index]
		if current.Content == config.Content {
			current.SourceReportID = report.ID
			current.SourceHash = sourceHash
			current.Color = "blue"
			current.Kind = "ai"
			record.Items[index] = current
			continue
		}
		current.History = append(current.History, ProjectConfigMemoHistory{
			Content:    current.Content,
			Color:      "blue",
			ModifiedAt: now,
		})
		current.Content = config.Content
		current.Color = "blue"
		current.Kind = "ai"
		current.ConfigKey = config.Key
		current.SourceReportID = report.ID
		current.SourceHash = sourceHash
		current.UpdatedAt = now
		record.Items[index] = current
	}

	return saveProjectConfigRecord(record)
}

func normalizeProjectCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func projectCodesMatch(left string, right string) bool {
	leftCode := normalizeProjectCode(left)
	rightCode := normalizeProjectCode(right)
	if leftCode == rightCode {
		return true
	}
	return strings.TrimPrefix(leftCode, "A") == strings.TrimPrefix(rightCode, "A")
}

func normalizeConfigKey(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	key = nonConfigKeyCharacters.ReplaceAllString(key, "")
	return key
}

func callProjectConfigAI(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	config := feishumodel.GlobalAIConfig
	if config == nil {
		config = feishumodel.LoadAIConfig()
	}
	providers := config.EffectiveProvidersByCapability(projectConfigAICapability)
	if len(providers) == 0 {
		providers = config.EffectiveProviders()
	}
	if len(providers) == 0 {
		return "", fmt.Errorf("未配置 AI 接口 Key")
	}

	httpClient := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		Timeout:   180 * time.Second,
	}
	var lastErr error
	for _, provider := range providers {
		clientConfig := openai.DefaultConfig(provider.APIKey)
		clientConfig.BaseURL = provider.BaseURL
		clientConfig.HTTPClient = httpClient
		client := openai.NewClientWithConfig(clientConfig)
		request := openai.ChatCompletionRequest{
			Model: provider.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userPrompt},
			},
			Temperature: 0.05,
			MaxTokens:   4096,
		}
		for attempt := 1; attempt <= 3; attempt++ {
			resp, err := client.CreateChatCompletion(ctx, request)
			if err == nil && len(resp.Choices) > 0 {
				return resp.Choices[0].Message.Content, nil
			}
			if err == nil {
				err = fmt.Errorf("AI 未返回任何内容")
			}
			lastErr = err
			if attempt < 3 && isRetryableProjectConfigAIError(err) {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			break
		}
	}
	return "", lastErr
}

func isRetryableProjectConfigAIError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "eof") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "temporarily unavailable")
}
