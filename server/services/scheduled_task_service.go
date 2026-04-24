package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	feishumodel "testcenter-server/feishu/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

type anthropicMessageRequest struct {
	Model     string                    `json:"model"`
	MaxTokens int                       `json:"max_tokens"`
	System    string                    `json:"system"`
	Messages  []anthropicMessageContent `json:"messages"`
}

type anthropicMessageContent struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicMessageResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type ScheduledTask struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Function        string `json:"function"`
	ScheduleType    string `json:"schedule_type"`
	Creator         string `json:"creator"`
	NextRun         string `json:"next_run"`
	NextRunAt       string `json:"next_run_at"`
	TestProject     string `json:"test_project"`
	TestProjectCode string `json:"test_project_code"`
	TestEnv         string `json:"test_env"`
	Status          string `json:"status"`
	Description     string `json:"description"`
	LastRunAt       string `json:"last_run_at,omitempty"`
	LastResult      string `json:"last_result,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type CreateScheduledTaskRequest struct {
	Function        string `json:"function" binding:"required"`
	ScheduleType    string `json:"schedule_type" binding:"required"`
	Creator         string `json:"creator" binding:"required"`
	NextRun         string `json:"next_run" binding:"required"`
	TestProject     string `json:"test_project"`
	TestProjectCode string `json:"test_project_code" binding:"required"`
	TestEnv         string `json:"test_env" binding:"required"`
}

type UpdateScheduledTaskRequest struct {
	ScheduleType string `json:"schedule_type" binding:"required"`
	NextRun      string `json:"next_run" binding:"required"`
}

type dramaProfile struct {
	Email        string
	Password     string
	LoginURL     string
	DramaListURL string
}

var (
	scheduledTaskMu   sync.Mutex
	scheduledRunnerOn sync.Once
)

func InitScheduledTaskService() {
	scheduledRunnerOn.Do(func() {
		go scheduledTaskLoop()
	})
}

func ListScheduledTasksHandler(c *gin.Context) {
	tasks, err := loadScheduledTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func CreateScheduledTaskHandler(c *gin.Context) {
	var req CreateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Function != "episode-playback-test" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only episode playback scheduled tasks are supported now"})
		return
	}

	nextRunAt, err := parseScheduledTime(req.NextRun)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid next_run, expected YYYY-MM-DD HH:mm:ss"})
		return
	}

	now := time.Now()
	task := ScheduledTask{
		ID:              "ST-" + uuid.New().String(),
		Name:            "剧集播放接口测试",
		Function:        req.Function,
		ScheduleType:    req.ScheduleType,
		Creator:         req.Creator,
		NextRun:         req.NextRun,
		NextRunAt:       nextRunAt.Format(time.RFC3339),
		TestProject:     req.TestProject,
		TestProjectCode: req.TestProjectCode,
		TestEnv:         req.TestEnv,
		Status:          "active",
		Description:     "Run the drama playback API test flow for the selected project.",
		CreatedAt:       now.Format(time.RFC3339),
		UpdatedAt:       now.Format(time.RFC3339),
	}
	if task.TestProject == "" {
		task.TestProject = lookupProjectName(req.TestProjectCode)
	}

	if err := appendScheduledTask(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func UpdateScheduledTaskHandler(c *gin.Context) {
	taskID := c.Param("id")
	var req UpdateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nextRunAt, err := parseScheduledTime(req.NextRun)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid next_run, expected YYYY-MM-DD HH:mm:ss"})
		return
	}

	tasks, err := loadScheduledTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	for i := range tasks {
		if tasks[i].ID != taskID {
			continue
		}
		if tasks[i].Status == "running" {
			c.JSON(http.StatusConflict, gin.H{"error": "Running scheduled task cannot be edited"})
			return
		}
		tasks[i].ScheduleType = req.ScheduleType
		tasks[i].NextRun = req.NextRun
		tasks[i].NextRunAt = nextRunAt.Format(time.RFC3339)
		tasks[i].UpdatedAt = now.Format(time.RFC3339)
		if tasks[i].Status == "completed" || tasks[i].Status == "paused" {
			tasks[i].Status = "active"
		}
		if err := saveScheduledTasks(tasks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, tasks[i])
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Scheduled task not found"})
}

func scheduledTaskLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	runDueScheduledTasks()
	for range ticker.C {
		runDueScheduledTasks()
	}
}

func runDueScheduledTasks() {
	tasks, err := loadScheduledTasks()
	if err != nil {
		log.Printf("[ScheduledTask] load failed: %v", err)
		return
	}

	now := time.Now()
	changed := false
	for i := range tasks {
		task := &tasks[i]
		if task.Status != "active" || task.Function != "episode-playback-test" {
			continue
		}
		nextRunAt, err := time.Parse(time.RFC3339, task.NextRunAt)
		if err != nil || now.Before(nextRunAt) {
			continue
		}

		task.Status = "running"
		task.UpdatedAt = now.Format(time.RFC3339)
		changed = true
		if err := saveScheduledTasks(tasks); err != nil {
			log.Printf("[ScheduledTask] mark running failed: %v", err)
		}

		go executeScheduledTask(*task)
	}

	if changed {
		_ = saveScheduledTasks(tasks)
	}
}

func executeScheduledTask(task ScheduledTask) {
	start := time.Now()
	log.Printf("[ScheduledTask] executing %s (%s)", task.Name, task.ID)

	err := runDramaPlaybackTask(task.TestEnv)
	duration := time.Since(start)
	result := "success"
	status := "Passed"
	if err != nil {
		result = err.Error()
		status = "Failed"
		log.Printf("[ScheduledTask] execute failed: %v", err)
	}

	reportURL := "http://localhost:8080/reports/drama_check_report.html"
	if _, addErr := AddExecutionReport(ExecutionReport{
		Name:      task.Name,
		Type:      "业务自动化",
		Status:    status,
		Duration:  formatDuration(duration),
		Author:    task.Creator,
		ReportURL: reportURL,
	}); addErr != nil {
		log.Printf("[ScheduledTask] add report failed: %v", addErr)
	}

	notice := buildScheduledTaskNotice(task, status, result, formatDuration(duration), reportURL, start)
	if notifyErr := notifyScheduledTaskCreator(task.Creator, notice); notifyErr != nil {
		log.Printf("[ScheduledTask] notify failed: %v", notifyErr)
	}

	updateScheduledTaskAfterRun(task, start, result)
}

func runDramaPlaybackTask(testEnv string) error {
	profile := getDramaProfile(testEnv)
	rootDir, _ := filepath.Abs("..")
	env := append(os.Environ(),
		"EMAIL="+profile.Email,
		"PASSWORD="+profile.Password,
		"LOGIN_URL="+profile.LoginURL,
		"DRAMA_LIST_URL="+profile.DramaListURL,
	)

	removeDramaRetryArtifacts(rootDir)
	if err := runCommand(rootDir, env, "node", filepath.Join("k6-scripts", "prepare_data.js")); err != nil {
		return fmt.Errorf("prepare data failed: %w", err)
	}
	if err := runCommand(rootDir, env, "k6", "run", filepath.Join("k6-scripts", "drama_check_flow.js")); err != nil {
		return fmt.Errorf("k6 run failed: %w", err)
	}
	if err := rerunDramaRetryCandidates(rootDir, env, func(message string) {
		log.Printf("[ScheduledTask][Retry] %s", message)
	}); err != nil {
		return fmt.Errorf("retry failed cases failed: %w", err)
	}
	return nil
}

func runCommand(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		log.Printf("[ScheduledTask][%s] %s", name, removeANSI(string(output)))
	}
	return err
}

func buildScheduledTaskNotice(task ScheduledTask, status string, result string, duration string, reportURL string, startedAt time.Time) string {
	analysis, err := analyzeScheduledTaskReportWithOpus47(task, status, result, duration, startedAt)
	if err != nil {
		log.Printf("[ScheduledTask] report analysis skipped: %v", err)
		analysis = "报告分析暂未生成，请打开平台查看完整报告。"
	}

	statusText := "通过"
	if status != "Passed" {
		statusText = "失败"
	}

	var builder strings.Builder
	builder.WriteString("定时任务执行完成\n")
	builder.WriteString("任务：" + task.Name + "\n")
	builder.WriteString("项目：" + valueOrFallback(task.TestProject, task.TestProjectCode) + "\n")
	builder.WriteString("环境：" + task.TestEnv + "\n")
	builder.WriteString("结果：" + statusText + "\n")
	builder.WriteString("耗时：" + duration + "\n\n")
	builder.WriteString("报告总结\n")
	builder.WriteString(sanitizeFeishuPlainText(analysis) + "\n\n")
	builder.WriteString("报告地址：" + reportURL)
	return sanitizeFeishuPlainText(builder.String())
}

func analyzeScheduledTaskReportWithOpus47(task ScheduledTask, status string, result string, duration string, startedAt time.Time) (string, error) {
	provider, err := selectOpus47Provider()
	if err != nil {
		return "", err
	}

	reportText, err := readScheduledTaskReportText(startedAt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(reportText) == "" {
		return "", fmt.Errorf("scheduled task report is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	httpClient := &http.Client{Timeout: 90 * time.Second}
	systemPrompt := "你是一名资深测试负责人。请根据用户提供的自动化测试报告内容，生成一段适合飞书机器人私聊发送给任务创建人的中文总结。" +
		"必须只输出纯文本，不要使用 Markdown，不要使用星号、反引号、井号、表格或项目符号。" +
		"不要编造报告里没有的数据。重点说明整体结论、失败点、风险影响和下一步建议。控制在 500 字以内。"
	userPrompt := fmt.Sprintf(
		"任务名称：%s\n项目：%s\n环境：%s\n执行状态：%s\n执行耗时：%s\n执行错误：%s\n\n报告内容：\n%s",
		task.Name,
		valueOrFallback(task.TestProject, task.TestProjectCode),
		task.TestEnv,
		status,
		duration,
		result,
		truncateForAI(reportText, 18000),
	)

	if isAnthropicProvider(provider) {
		return callAnthropicScheduledReport(ctx, httpClient, provider, systemPrompt, userPrompt)
	}

	clientConfig := openai.DefaultConfig(provider.APIKey)
	clientConfig.BaseURL = provider.BaseURL
	clientConfig.HTTPClient = httpClient
	client := openai.NewClientWithConfig(clientConfig)

	req := openai.ChatCompletionRequest{
		Model: provider.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		Temperature: 0.2,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("Opus4.7 returned no choices")
	}

	content := sanitizeFeishuPlainText(resp.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("Opus4.7 returned empty analysis")
	}
	return content, nil
}

func callAnthropicScheduledReport(ctx context.Context, httpClient *http.Client, provider feishumodel.AIProviderConfig, systemPrompt string, userPrompt string) (string, error) {
	payload := anthropicMessageRequest{
		Model:     provider.Model,
		MaxTokens: 800,
		System:    systemPrompt,
		Messages: []anthropicMessageContent{
			{Role: "user", Content: userPrompt},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	apiURL := strings.TrimRight(provider.BaseURL, "/") + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", provider.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Anthropic scheduled report failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var parsed anthropicMessageResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	for _, item := range parsed.Content {
		if strings.TrimSpace(item.Text) != "" {
			return sanitizeFeishuPlainText(item.Text), nil
		}
	}
	return "", fmt.Errorf("Anthropic scheduled report returned empty content")
}

func isAnthropicProvider(provider feishumodel.AIProviderConfig) bool {
	search := strings.ToLower(strings.Join([]string{provider.Name, provider.Model, provider.BaseURL}, " "))
	return strings.Contains(search, "anthropic") || strings.Contains(search, "claude")
}

func selectOpus47Provider() (feishumodel.AIProviderConfig, error) {
	config := feishumodel.GlobalAIConfig
	if config == nil {
		config = feishumodel.LoadAIConfig()
	}
	if config == nil {
		return feishumodel.AIProviderConfig{}, fmt.Errorf("AI config is empty")
	}

	for _, provider := range config.AllEffectiveProviders() {
		search := strings.ToLower(strings.Join([]string{provider.Name, provider.Model, provider.Capability}, " "))
		if strings.Contains(search, "opus") && (strings.Contains(search, "4.7") || strings.Contains(search, "4-7") || strings.Contains(search, "4_7")) {
			return provider, nil
		}
	}
	return feishumodel.AIProviderConfig{}, fmt.Errorf("Opus4.7 provider is not configured")
}

func readScheduledTaskReportText(startedAt time.Time) (string, error) {
	rootDir, _ := filepath.Abs("..")
	reportPath := filepath.Join(rootDir, "k6-scripts", "reports", "drama_check_report.html")
	info, err := os.Stat(reportPath)
	if err != nil {
		return "", err
	}
	if info.ModTime().Before(startedAt.Add(-5 * time.Second)) {
		return "", fmt.Errorf("scheduled task report was not updated by this run")
	}
	content, err := os.ReadFile(reportPath)
	if err != nil {
		return "", err
	}

	text := htmlToPlainText(string(content))
	if retryContent, err := os.ReadFile(filepath.Join(rootDir, "k6-scripts", "reports", "drama_retry_result.json")); err == nil {
		text += "\n\nRetry result JSON:\n" + string(retryContent)
	}
	if candidateContent, err := os.ReadFile(filepath.Join(rootDir, "k6-scripts", "reports", "drama_retry_candidates.json")); err == nil {
		text += "\n\nRetry candidate JSON:\n" + string(candidateContent)
	}
	return text, nil
}

func htmlToPlainText(content string) string {
	replacer := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "</tr>", "\n", "</p>", "\n", "</div>", "\n", "</section>", "\n")
	content = replacer.Replace(content)
	content = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(content, " ")
	content = html.UnescapeString(content)
	content = regexp.MustCompile(`[ \t\r\f\v]+`).ReplaceAllString(content, " ")
	content = regexp.MustCompile(`\n\s+`).ReplaceAllString(content, "\n")
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")
	return strings.TrimSpace(content)
}

func sanitizeFeishuPlainText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.NewReplacer(
		"**", "",
		"`", "",
		"###", "",
		"##", "",
		"#", "",
		"|", " ",
	).Replace(text)
	text = regexp.MustCompile(`(?m)^\s*[-*]\s+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[ \t]+\n`).ReplaceAllString(text, "\n")
	return strings.TrimSpace(text)
}

func truncateForAI(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "\n\n内容过长，已截断。"
}

func valueOrFallback(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func getDramaProfile(testEnv string) dramaProfile {
	if strings.Contains(testEnv, "正式") || strings.EqualFold(testEnv, "prod") {
		return dramaProfile{
			Email:        "test001@wedrama.com",
			Password:     "fb3b2e9961b58",
			LoginURL:     "https://admin.shortswave.com/api/pwd_login",
			DramaListURL: "https://admin.shortswave.com/api/management/drama/all_online_ids",
		}
	}
	return dramaProfile{
		Email:        "test_super_001@shortswave.com",
		Password:     "test123456",
		LoginURL:     "http://35.225.224.94:8080/api/pwd_login",
		DramaListURL: "http://35.225.224.94:8080/api/management/drama/all_online_ids",
	}
}

func updateScheduledTaskAfterRun(task ScheduledTask, runAt time.Time, result string) {
	tasks, err := loadScheduledTasks()
	if err != nil {
		log.Printf("[ScheduledTask] reload after run failed: %v", err)
		return
	}

	for i := range tasks {
		if tasks[i].ID != task.ID {
			continue
		}
		tasks[i].LastRunAt = runAt.Format(time.RFC3339)
		tasks[i].LastResult = result
		tasks[i].UpdatedAt = time.Now().Format(time.RFC3339)
		nextRunAt, _ := time.Parse(time.RFC3339, task.NextRunAt)
		switch task.ScheduleType {
		case "Once":
			tasks[i].Status = "completed"
		case "Daily":
			tasks[i].Status = "active"
			tasks[i].NextRunAt = nextRunAt.AddDate(0, 0, 1).Format(time.RFC3339)
			tasks[i].NextRun = formatScheduleDisplay(tasks[i].NextRunAt)
		case "weekly":
			tasks[i].Status = "active"
			tasks[i].NextRunAt = nextRunAt.AddDate(0, 0, 7).Format(time.RFC3339)
			tasks[i].NextRun = formatScheduleDisplay(tasks[i].NextRunAt)
		default:
			tasks[i].Status = "paused"
		}
		break
	}

	if err := saveScheduledTasks(tasks); err != nil {
		log.Printf("[ScheduledTask] save after run failed: %v", err)
	}
}

func loadScheduledTasks() ([]ScheduledTask, error) {
	scheduledTaskMu.Lock()
	defer scheduledTaskMu.Unlock()
	return sqlListJSON[ScheduledTask]("scheduled_tasks", "`migrated_at` ASC")
}

func appendScheduledTask(task ScheduledTask) error {
	scheduledTaskMu.Lock()
	defer scheduledTaskMu.Unlock()
	return sqlUpsertJSON("scheduled_tasks", task)
}

func saveScheduledTasks(tasks []ScheduledTask) error {
	scheduledTaskMu.Lock()
	defer scheduledTaskMu.Unlock()
	return sqlReplaceAllJSON("scheduled_tasks", tasks)
}

func parseScheduledTime(value string) (time.Time, error) {
	loc := time.Local
	if shanghai, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		loc = shanghai
	}
	return time.ParseInLocation("2006-01-02 15:04:05", value, loc)
}

func formatScheduleDisplay(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.Format("2006-01-02 15:04:05")
}

func lookupProjectName(code string) string {
	if ConfigServiceInstance == nil {
		return code
	}
	projects, err := ConfigServiceInstance.GetAllProjects()
	if err != nil {
		return code
	}
	for _, project := range projects {
		if project.ProjectCode == code {
			return project.ProjectName
		}
	}
	return code
}

func formatDuration(duration time.Duration) string {
	total := int(duration.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func notifyScheduledTaskCreator(username string, text string) error {
	openID := resolveFeishuOpenID(username)
	if openID == "" {
		openID = resolveFeishuOpenID("minghong")
	}
	if openID == "" {
		return fmt.Errorf("no feishu open_id found for %s or minghong", username)
	}
	return sendFeishuOpenIDText(openID, text)
}

func resolveFeishuOpenID(username string) string {
	user, _, err := FindUserByFuzzyName(username)
	if err == nil && user != nil && user.FeishuOpenID != "" {
		return user.FeishuOpenID
	}
	return ""
}

func sendFeishuOpenIDText(openID string, text string) error {
	if feishumodel.GlobalFeishuConfig == nil {
		config, err := feishumodel.LoadConfig("data/feishu_config.json")
		if err != nil {
			return err
		}
		feishumodel.GlobalFeishuConfig = config
	}
	config := feishumodel.GlobalFeishuConfig
	if config == nil || config.AppID == "" || config.AppSecret == "" {
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

	contentBody, _ := json.Marshal(map[string]string{"text": text})
	msgPayload, _ := json.Marshal(map[string]string{
		"receive_id": openID,
		"msg_type":   "text",
		"content":    string(contentBody),
	})

	req, err := http.NewRequest("POST", "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id", bytes.NewBuffer(msgPayload))
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
