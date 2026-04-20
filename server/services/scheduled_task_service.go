package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	feishumodel "testcenter-server/feishu/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
	scheduledTasksFile = filepath.Join("data", "scheduled_tasks.jsonl")
	scheduledTaskMu    sync.Mutex
	scheduledRunnerOn  sync.Once
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

	notice := fmt.Sprintf("您的定时任务%s已执行完成，报告已经生成，请去平台查看。", task.Name)
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
	return readScheduledTasksUnlocked()
}

func appendScheduledTask(task ScheduledTask) error {
	scheduledTaskMu.Lock()
	defer scheduledTaskMu.Unlock()

	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(scheduledTasksFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	data, _ := json.Marshal(task)
	_, err = file.WriteString(string(data) + "\n")
	return err
}

func saveScheduledTasks(tasks []ScheduledTask) error {
	scheduledTaskMu.Lock()
	defer scheduledTaskMu.Unlock()

	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	file, err := os.Create(scheduledTasksFile)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, task := range tasks {
		data, _ := json.Marshal(task)
		if _, err := writer.WriteString(string(data) + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func readScheduledTasksUnlocked() ([]ScheduledTask, error) {
	file, err := os.Open(scheduledTasksFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []ScheduledTask{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var tasks []ScheduledTask
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var task ScheduledTask
		if err := json.Unmarshal(scanner.Bytes(), &task); err == nil {
			tasks = append(tasks, task)
		}
	}
	return tasks, scanner.Err()
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
