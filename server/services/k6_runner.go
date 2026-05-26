package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type dramaRetryCandidateFile struct {
	Candidates []dramaRetryCandidate `json:"candidates"`
}

type dramaRetryCandidate struct {
	DramaID string `json:"dramaId"`
	Reason  string `json:"reason"`
}

type dramaRetryResultFile struct {
	PersistentFailures []dramaRetryFailure `json:"persistentFailures"`
}

type dramaRetryFailure struct {
	Name  string `json:"name"`
	Fails int    `json:"fails"`
}

type dramaInfoFile struct {
	DramaList []string `json:"dramaList"`
}

type StartDramaRunRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	LoginURL     string `json:"loginUrl"`
	DramaListURL string `json:"dramaListUrl"`
	ToolName     string `json:"toolName"`
	Author       string `json:"author"`
	Environment  string `json:"environment"`
}

type dramaRunSnapshot struct {
	RunID     string   `json:"runId"`
	Status    string   `json:"status"`
	Progress  int      `json:"progress"`
	Logs      []string `json:"logs"`
	ReportURL string   `json:"reportUrl"`
	Duration  int      `json:"duration"`
	Error     string   `json:"error"`
}

type dramaRunJob struct {
	mu          sync.Mutex
	id          string
	status      string
	progress    int
	logs        []string
	reportURL   string
	errorText   string
	toolName    string
	author      string
	environment string
	startedAt   time.Time
	finishedAt  time.Time
	cancel      context.CancelFunc
	subscribers map[chan string]struct{}
}

const dramaReportBaseFile = "drama_check_report.html"

var dramaRuns = struct {
	mu      sync.Mutex
	current *dramaRunJob
}{}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all cross-origin connections
	},
}

func StartDramaRunHandler(c *gin.Context) {
	var req StartDramaRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rootDir, _ := filepath.Abs("..")
	env := buildDramaRunEnv(req)

	dramaRuns.mu.Lock()
	if dramaRuns.current != nil && dramaRuns.current.getStatus() == "running" {
		snapshot := dramaRuns.current.snapshot()
		dramaRuns.mu.Unlock()
		c.JSON(http.StatusOK, snapshot)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &dramaRunJob{
		id:          fmt.Sprintf("DRAMA-%d", time.Now().UnixMilli()),
		status:      "running",
		progress:    0,
		logs:        []string{fmt.Sprintf("[%s] 后端任务已创建，准备执行剧集播放接口测试...", time.Now().Format("15:04:05"))},
		toolName:    firstNonEmpty(req.ToolName, "剧集播放接口测试"),
		author:      firstNonEmpty(req.Author, "tester"),
		environment: normalizeReportEnvironment(req.Environment),
		startedAt:   time.Now(),
		cancel:      cancel,
		subscribers: make(map[chan string]struct{}),
	}
	dramaRuns.current = job
	dramaRuns.mu.Unlock()

	go job.execute(ctx, rootDir, env)
	c.JSON(http.StatusOK, job.snapshot())
}

func CurrentDramaRunHandler(c *gin.Context) {
	dramaRuns.mu.Lock()
	job := dramaRuns.current
	dramaRuns.mu.Unlock()
	if job == nil {
		c.JSON(http.StatusOK, gin.H{"status": "idle"})
		return
	}
	c.JSON(http.StatusOK, job.snapshot())
}

func StopDramaRunHandler(c *gin.Context) {
	dramaRuns.mu.Lock()
	job := dramaRuns.current
	dramaRuns.mu.Unlock()
	if job == nil || job.getStatus() != "running" {
		c.JSON(http.StatusOK, gin.H{"status": "idle"})
		return
	}
	job.stopByUser()
	c.JSON(http.StatusOK, job.snapshot())
}

func SubscribeDramaRunHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Drama run WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()

	dramaRuns.mu.Lock()
	job := dramaRuns.current
	dramaRuns.mu.Unlock()
	if job == nil {
		ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:idle"))
		return
	}

	ch := job.subscribe()
	defer job.unsubscribe(ch)
	for _, message := range job.snapshotMessages() {
		if err := ws.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			return
		}
	}

	for message := range ch {
		if err := ws.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			return
		}
	}
}

func (job *dramaRunJob) execute(ctx context.Context, rootDir string, env []string) {
	reportDir := dramaRuntimeReportDir(job.id)
	env = append(env, "DRAMA_REPORT_DIR="+reportDir)
	_ = os.MkdirAll(filepath.Join(rootDir, reportDir), 0755)
	removeDramaRetryArtifacts(rootDir, reportDir)
	job.publishLog("[Step 1/2] 正在初始化前置数据业务 (prepare_data.js)...")

	if err := job.runCommandStream(ctx, rootDir, env, "node", filepath.Join("k6-scripts", "prepare_data.js")); err != nil {
		if ctx.Err() != nil {
			return
		}
		job.fail(fmt.Sprintf("prepare data failed: %v", err))
		return
	}

	job.publishLog("\n[Step 2/2] 数据初始化成功，开始并发检测环节...")
	dramaTotal := readDramaTotal(rootDir)
	dramaCompleted := 0
	var dramaProgressMu sync.Mutex
	handleProgressLine := func(text string) bool {
		if !isDramaProgressMarkerLine(text) {
			return false
		}
		dramaProgressMu.Lock()
		dramaCompleted++
		progress := calculateDramaProgress(dramaCompleted, dramaTotal)
		dramaProgressMu.Unlock()
		job.setProgress(progress)
		return true
	}

	job.publishLog("Ready to execute K6 test: drama_check_flow.js")
	job.publishLog("----------------------------------------")
	if err := job.runK6Stream(ctx, rootDir, env, handleProgressLine); err != nil {
		if ctx.Err() != nil {
			return
		}
		job.fail(fmt.Sprintf("k6 command failed: %v", err))
		return
	}

	if err := rerunDramaRetryCandidates(rootDir, reportDir, env, func(message string) {
		if strings.HasPrefix(message, "DRAMA_PROGRESS:") {
			job.handleControlMessage(message)
			return
		}
		job.publishLog(message)
	}); err != nil {
		if ctx.Err() != nil {
			return
		}
		job.fail(fmt.Sprintf("drama retry failed: %v", err))
		return
	}

	reportFile, reportURL, err := snapshotDramaReport(rootDir, reportDir, job.id)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		job.fail(fmt.Sprintf("snapshot drama report failed: %v", err))
		return
	}

	job.complete(reportFile, reportURL)
}

func (job *dramaRunJob) runCommandStream(ctx context.Context, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		job.publishLog(scanner.Text())
	}
	if scanErr := scanner.Err(); scanErr != nil {
		job.publishLog(fmt.Sprintf("[WARN] 读取命令输出异常: %v", scanErr))
	}
	return cmd.Wait()
}

func (job *dramaRunJob) runK6Stream(ctx context.Context, dir string, env []string, handleProgressLine func(string) bool) error {
	cmd := exec.CommandContext(ctx, "k6", "run", filepath.Join("k6-scripts", "drama_check_flow.js"))
	cmd.Dir = dir
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			text := scanner.Text()
			if handleProgressLine(text) {
				continue
			}
			job.publishLog("[WARN/ERR] " + text)
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			text := scanner.Text()
			if handleProgressLine(text) {
				continue
			}
			job.publishLog(removeANSI(text))
		}
	}()
	wg.Wait()
	return cmd.Wait()
}

func (job *dramaRunJob) subscribe() chan string {
	ch := make(chan string, 128)
	job.mu.Lock()
	job.subscribers[ch] = struct{}{}
	job.mu.Unlock()
	return ch
}

func (job *dramaRunJob) unsubscribe(ch chan string) {
	job.mu.Lock()
	if _, ok := job.subscribers[ch]; ok {
		delete(job.subscribers, ch)
		close(ch)
	}
	job.mu.Unlock()
}

func (job *dramaRunJob) publish(message string) {
	job.mu.Lock()
	subscribers := make([]chan string, 0, len(job.subscribers))
	for ch := range job.subscribers {
		subscribers = append(subscribers, ch)
	}
	job.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- message:
		default:
		}
	}
}

func (job *dramaRunJob) publishLog(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	appendTestRunLog(job.id, message)
	job.mu.Lock()
	job.logs = append(job.logs, message)
	if len(job.logs) > 1200 {
		job.logs = job.logs[len(job.logs)-1200:]
	}
	job.mu.Unlock()
	job.publish(message)
}

func (job *dramaRunJob) setProgress(progress int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	job.mu.Lock()
	if progress < job.progress {
		job.mu.Unlock()
		return
	}
	job.progress = progress
	job.mu.Unlock()
	job.publish(fmt.Sprintf("DRAMA_PROGRESS:%d", progress))
}

func (job *dramaRunJob) handleControlMessage(message string) {
	if strings.HasPrefix(message, "DRAMA_PROGRESS:") {
		var progress int
		if _, err := fmt.Sscanf(message, "DRAMA_PROGRESS:%d", &progress); err == nil {
			job.setProgress(progress)
		}
		return
	}
	job.publishLog(message)
}

func (job *dramaRunJob) complete(reportFile string, reportURL string) {
	job.mu.Lock()
	job.status = "done"
	job.progress = 100
	job.reportURL = reportURL
	job.finishedAt = time.Now()
	job.mu.Unlock()

	rootDir := projectRootDir()
	archive, archiveErr := CreateDramaTestRunArchive(rootDir, job, reportFile, reportURL)
	if archiveErr != nil {
		log.Printf("[DramaRun] create archive failed: %v", archiveErr)
	}
	if archiveErr == nil && len(archive.Artifacts) > 0 {
		reportURL = archive.Artifacts[0].URL
		job.mu.Lock()
		job.reportURL = reportURL
		job.mu.Unlock()
	}

	job.publishLog("\nExecution completed successfully.")
	job.publish("DRAMA_PROGRESS:100")
	job.publish("REPORT_READY_URL:" + reportURL)
	job.publish("EXECUTION_STATUS:success")
	if _, err := AddExecutionReport(ExecutionReport{
		Name:        job.toolName,
		Type:        "K6 压测",
		Status:      "Passed",
		Duration:    formatDuration(time.Since(job.startedAt)),
		Author:      job.author,
		ReportURL:   reportURL,
		RunID:       job.id,
		Environment: job.environment,
	}); err != nil {
		log.Printf("[DramaRun] add report failed: %v", err)
	}
}

func (job *dramaRunJob) fail(reason string) {
	job.mu.Lock()
	job.status = "failed"
	job.errorText = reason
	job.finishedAt = time.Now()
	job.mu.Unlock()
	job.publishLog(fmt.Sprintf("[ERROR] 执行失败: %s", reason))
	rootDir, _ := filepath.Abs("..")
	if err := CreateDramaFailureArchive(rootDir, job); err != nil {
		log.Printf("[DramaRun] create failed archive failed: %v", err)
	}
	job.publish("EXECUTION_STATUS:failed:" + reason)
	if _, err := AddExecutionReport(ExecutionReport{
		Name:        job.toolName,
		Type:        "K6 压测",
		Status:      "Failed",
		Duration:    formatDuration(time.Since(job.startedAt)),
		Author:      job.author,
		RunID:       job.id,
		Environment: job.environment,
	}); err != nil {
		log.Printf("[DramaRun] add failed report failed: %v", err)
	}
}

func (job *dramaRunJob) stopByUser() {
	job.mu.Lock()
	if job.status != "running" {
		job.mu.Unlock()
		return
	}
	job.status = "stopped"
	job.finishedAt = time.Now()
	cancel := job.cancel
	job.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	job.publishLog(fmt.Sprintf("[%s] 用户手动终止执行。", time.Now().Format("15:04:05")))
	job.publish("EXECUTION_STATUS:stopped")
}

func (job *dramaRunJob) getStatus() string {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.status
}

func (job *dramaRunJob) snapshot() dramaRunSnapshot {
	job.mu.Lock()
	defer job.mu.Unlock()
	duration := int(time.Since(job.startedAt).Seconds())
	if !job.finishedAt.IsZero() {
		duration = int(job.finishedAt.Sub(job.startedAt).Seconds())
	}
	logs := append([]string{}, job.logs...)
	return dramaRunSnapshot{
		RunID:     job.id,
		Status:    job.status,
		Progress:  job.progress,
		Logs:      logs,
		ReportURL: job.reportURL,
		Duration:  duration,
		Error:     job.errorText,
	}
}

func (job *dramaRunJob) snapshotMessages() []string {
	snapshot := job.snapshot()
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return []string{"DRAMA_PROGRESS:0"}
	}
	return []string{"DRAMA_SNAPSHOT:" + string(payload)}
}

func buildDramaRunEnv(req StartDramaRunRequest) []string {
	env := os.Environ()
	if req.Email != "" {
		env = append(env, "EMAIL="+req.Email)
	}
	if req.Password != "" {
		env = append(env, "PASSWORD="+req.Password)
	}
	if req.LoginURL != "" {
		env = append(env, "LOGIN_URL="+req.LoginURL)
	}
	if req.DramaListURL != "" {
		env = append(env, "DRAMA_LIST_URL="+req.DramaListURL)
	}
	return env
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// RunK6TestHandler upgrades the HTTP request to a WebSocket and executes the K6 test
func RunK6TestHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()

	// Get parameters from query
	scriptName := c.DefaultQuery("script", "episode.js")
	runID := fmt.Sprintf("WS-%d", time.Now().UnixMilli())
	startedAt := time.Now()
	email := c.Query("email")
	password := c.Query("password")
	loginUrl := c.Query("loginUrl")
	dramaListUrl := c.Query("dramaListUrl")

	// 统一工作目录：由于后端在 server/ 下运行，我们将所有脚本执行的根路径设为项目根目录 (..)
	rootDir, _ := filepath.Abs("..")
	scriptPath := filepath.Join("k6-scripts", scriptName)
	prepareScriptPath := filepath.Join("k6-scripts", "prepare_data.js")

	if _, err := os.Stat(filepath.Join(rootDir, scriptPath)); os.IsNotExist(err) {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: Script %s not found", scriptName)))
		ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:script not found"))
		return
	}

	// Dynamic Env passing
	env := os.Environ()
	if email != "" {
		env = append(env, "EMAIL="+email)
	}
	if password != "" {
		env = append(env, "PASSWORD="+password)
	}
	if loginUrl != "" {
		env = append(env, "LOGIN_URL="+loginUrl)
	}
	if dramaListUrl != "" {
		env = append(env, "DRAMA_LIST_URL="+dramaListUrl)
	}

	// Special orchestration: if running drama_check_flow, run prepare_data FIRST
	reportDir := ""
	if scriptName == "drama_check_flow.js" {
		reportDir = dramaRuntimeReportDir(runID)
		env = append(env, "DRAMA_REPORT_DIR="+reportDir)
		_ = os.MkdirAll(filepath.Join(rootDir, reportDir), 0755)
		removeDramaRetryArtifacts(rootDir, reportDir)
		ws.WriteMessage(websocket.TextMessage, []byte("[Step 1/2] 正在初始化前置数据业务 (prepare_data.js)..."))
		prepCmd := exec.Command("node", prepareScriptPath)
		prepCmd.Env = env
		prepCmd.Dir = rootDir

		stdoutPipe, _ := prepCmd.StdoutPipe()
		prepCmd.Stderr = prepCmd.Stdout

		if err := prepCmd.Start(); err != nil {
			ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\n[❌] 启动数据准备脚本失败: %v", err)))
			ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:prepare data start failed"))
			return
		}

		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			ws.WriteMessage(websocket.TextMessage, []byte(scanner.Text()))
		}

		if err := prepCmd.Wait(); err != nil {
			ws.WriteMessage(websocket.TextMessage, []byte("\n[❌] 前置数据准备出现异常，终止后续执行。"))
			ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:prepare data failed"))
			return
		}
		ws.WriteMessage(websocket.TextMessage, []byte("\n[Step 2/2] 数据初始化成功，开始并发检测环节..."))
	}

	dramaTotal := 0
	dramaCompleted := 0
	var dramaProgressMu sync.Mutex
	handleDramaProgressLine := func(text string) bool {
		if scriptName != "drama_check_flow.js" || !isDramaProgressMarkerLine(text) {
			return false
		}
		dramaProgressMu.Lock()
		dramaCompleted++
		progress := calculateDramaProgress(dramaCompleted, dramaTotal)
		dramaProgressMu.Unlock()
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("DRAMA_PROGRESS:%d", progress)))
		return true
	}
	if scriptName == "drama_check_flow.js" {
		dramaTotal = readDramaTotal(rootDir)
	}

	ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Ready to execute K6 test: %s", scriptName)))
	ws.WriteMessage(websocket.TextMessage, []byte("----------------------------------------"))

	// Execute K6 command
	cmd := exec.Command("k6", "run", scriptPath)
	cmd.Env = env
	cmd.Dir = rootDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to get stdout: %v", err)))
		ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:stdout pipe failed"))
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to get stderr: %v", err)))
		ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:stderr pipe failed"))
		return
	}

	if err := cmd.Start(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to start K6 output: %v", err)))
		ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:k6 start failed"))
		return
	}

	// Read standard output pipeline
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			text := scanner.Text()
			if handleDramaProgressLine(text) {
				continue
			}
			ws.WriteMessage(websocket.TextMessage, []byte("[WARN/ERR] "+text))
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		text := scanner.Text()
		if handleDramaProgressLine(text) {
			continue
		}
		cleanText := removeANSI(text)
		err := ws.WriteMessage(websocket.TextMessage, []byte(cleanText))
		if err != nil {
			log.Println("Write error:", err)
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\nCommand finished with error: %v", err)))
		ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:k6 command failed"))
	} else {
		if scriptName == "drama_check_flow.js" {
			if err := rerunDramaRetryCandidates(rootDir, reportDir, env, func(message string) {
				ws.WriteMessage(websocket.TextMessage, []byte(message))
			}); err != nil {
				ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\n[❌] 720p失败case二次尝试失败: %v", err)))
				ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:drama retry failed"))
				return
			}
		}

		ws.WriteMessage(websocket.TextMessage, []byte("\nExecution completed successfully."))
		if scriptName == "drama_check_flow.js" {
			ws.WriteMessage(websocket.TextMessage, []byte("DRAMA_PROGRESS:100"))
		}
		reportFile := "summary.html"
		reportURL := ""
		if scriptName == "drama_check_flow.js" {
			reportFile, _, err = snapshotDramaReport(rootDir, reportDir, runID)
			if err != nil {
				ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\n[❌] 生成剧集播放报告快照失败: %v", err)))
				ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:report snapshot failed"))
				return
			}
			task := ScheduledTask{
				ID:      runID,
				Name:    "剧集播放接口测试",
				Creator: "tester",
			}
			if archive, archiveErr := CreateScheduledDramaArchive(rootDir, runID, task, "Passed", time.Since(startedAt), startedAt, reportFile); archiveErr != nil {
				ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\n[❌] 归档剧集播放报告失败: %v", archiveErr)))
				ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:failed:report archive failed"))
				return
			} else if len(archive.Artifacts) > 0 {
				reportURL = archive.Artifacts[0].URL
			}
		}
		if reportURL != "" {
			ws.WriteMessage(websocket.TextMessage, []byte("REPORT_READY_URL:"+reportURL))
		} else {
			ws.WriteMessage(websocket.TextMessage, []byte("REPORT_READY:"+reportFile))
		}
		ws.WriteMessage(websocket.TextMessage, []byte("EXECUTION_STATUS:success"))
	}
}

func readDramaTotal(rootDir string) int {
	path := filepath.Join(rootDir, "drama_info.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var payload dramaInfoFile
	if err := json.Unmarshal(content, &payload); err != nil {
		return 0
	}
	return len(payload.DramaList)
}

func calculateDramaProgress(completed int, total int) int {
	if total <= 0 || completed <= 0 {
		return 0
	}
	progress := completed * 95 / total
	if progress < 1 {
		return 1
	}
	if progress > 95 {
		return 95
	}
	return progress
}

func isDramaProgressMarkerLine(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "__DRAMA_CASE_DONE__|") ||
		strings.Contains(trimmed, `msg="__DRAMA_CASE_DONE__|`)
}

func rerunDramaRetryCandidates(rootDir string, reportDir string, env []string, logMessage func(string)) error {
	candidates, err := readDramaRetryCandidates(rootDir, reportDir)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		logIfPresent(logMessage, "\n[Retry] 第一轮没有720p网络失败case，无需二次尝试。")
		return appendDramaRetrySection(rootDir, reportDir, nil, "第一轮没有720p网络失败case，无需二次尝试。")
	}

	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.DramaID != "" {
			ids = append(ids, candidate.DramaID)
		}
	}
	if len(ids) == 0 {
		logIfPresent(logMessage, "\n[Retry] 720p网络失败候选为空，无需二次尝试。")
		return appendDramaRetrySection(rootDir, reportDir, nil, "720p网络失败候选为空，无需二次尝试。")
	}

	logIfPresent(logMessage, fmt.Sprintf("\n[Retry] 发现 %d 个720p网络失败case，开始二次尝试...", len(ids)))
	logIfPresent(logMessage, "DRAMA_PROGRESS:96")
	retryEnv := append([]string{}, env...)
	retryEnv = append(retryEnv, "DRAMA_RETRY_PHASE=1", "DRAMA_RETRY_IDS="+strings.Join(ids, ","))

	cmd := exec.Command("k6", "run", filepath.Join("k6-scripts", "drama_check_flow.js"))
	cmd.Dir = rootDir
	cmd.Env = retryEnv
	output, runErr := cmd.CombinedOutput()
	if len(output) > 0 {
		logIfPresent(logMessage, filterDramaProgressMarkers(removeANSI(string(output))))
	}
	if runErr != nil {
		return runErr
	}

	result, err := readDramaRetryResult(rootDir, reportDir)
	if err != nil {
		return err
	}
	if len(result.PersistentFailures) == 0 {
		logIfPresent(logMessage, "DRAMA_PROGRESS:99")
		logIfPresent(logMessage, fmt.Sprintf("[Retry] %d 个case二次尝试已通过，判定为瞬时网络问题。", len(ids)))
		return appendDramaRetrySection(rootDir, reportDir, nil, fmt.Sprintf("%d 个720p网络失败case二次尝试已通过，判定为瞬时网络问题。", len(ids)))
	}

	logIfPresent(logMessage, "DRAMA_PROGRESS:99")
	logIfPresent(logMessage, fmt.Sprintf("[Retry] %d 个case二次尝试后仍失败，已追加到最终报告。", len(result.PersistentFailures)))
	return appendDramaRetrySection(rootDir, reportDir, result.PersistentFailures, "")
}

func removeDramaRetryArtifacts(rootDir string, reportDir string) {
	reportsDir := filepath.Join(rootDir, reportDir)
	_ = os.Remove(filepath.Join(reportsDir, "drama_retry_candidates.json"))
	_ = os.Remove(filepath.Join(reportsDir, "drama_retry_result.json"))
	_ = os.Remove(filepath.Join(reportsDir, "drama_retry_report.html"))
}

func readDramaRetryCandidates(rootDir string, reportDir string) ([]dramaRetryCandidate, error) {
	path := filepath.Join(rootDir, reportDir, "drama_retry_candidates.json")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var payload dramaRetryCandidateFile
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, err
	}
	return payload.Candidates, nil
}

func readDramaRetryResult(rootDir string, reportDir string) (dramaRetryResultFile, error) {
	path := filepath.Join(rootDir, reportDir, "drama_retry_result.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return dramaRetryResultFile{}, err
	}
	var payload dramaRetryResultFile
	if err := json.Unmarshal(content, &payload); err != nil {
		return dramaRetryResultFile{}, err
	}
	return payload, nil
}

func appendDramaRetrySection(rootDir string, reportDir string, failures []dramaRetryFailure, note string) error {
	reportPath := filepath.Join(rootDir, reportDir, dramaReportBaseFile)
	content, err := os.ReadFile(reportPath)
	if err != nil {
		return err
	}

	section := buildDramaRetrySection(failures, note)
	htmlContent := string(content)
	if strings.Contains(htmlContent, "</body>") {
		htmlContent = strings.Replace(htmlContent, "</body>", section+"</body>", 1)
	} else {
		htmlContent += section
	}
	return os.WriteFile(reportPath, []byte(htmlContent), 0644)
}

func snapshotDramaReport(rootDir string, reportDir string, suffix string) (string, string, error) {
	_ = suffix
	sourceRelPath := filepath.ToSlash(filepath.Join(reportDir, dramaReportBaseFile))
	if _, err := os.Stat(filepath.Join(rootDir, sourceRelPath)); err != nil {
		return "", "", err
	}

	return sourceRelPath, "", nil
}

func dramaRuntimeReportDir(runID string) string {
	safeRunID := sanitizeReportSuffix(runID)
	if safeRunID == "" {
		safeRunID = fmt.Sprintf("%d", time.Now().UnixMilli())
	}
	return filepath.ToSlash(filepath.Join("report", "api_report", ".runtime", safeRunID))
}

func sanitizeReportSuffix(value string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		" ", "_",
	)
	return replacer.Replace(strings.TrimSpace(value))
}

func buildDramaRetrySection(failures []dramaRetryFailure, note string) string {
	var builder strings.Builder
	builder.WriteString(`<section style="margin:24px auto;max-width:1180px;padding:20px 24px;border:1px solid #dbeafe;border-radius:16px;background:#f8fbff;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">`)
	builder.WriteString(`<h2 style="margin:0 0 12px;color:#1f2937;font-size:20px;">720p Failed Case Retry Result</h2>`)
	if len(failures) == 0 {
		builder.WriteString(`<p style="margin:0;color:#047857;font-size:14px;">`)
		builder.WriteString(html.EscapeString(note))
		builder.WriteString(`</p></section>`)
		return builder.String()
	}

	builder.WriteString(`<p style="margin:0 0 12px;color:#b91c1c;font-size:14px;">以下case在第一轮720p网络请求失败后，二次尝试仍未通过，已作为最终异常保留。</p>`)
	builder.WriteString(`<ul style="margin:0;padding-left:20px;color:#991b1b;font-size:13px;line-height:1.7;">`)
	for _, failure := range failures {
		builder.WriteString(`<li>`)
		builder.WriteString(html.EscapeString(failure.Name))
		if failure.Fails > 0 {
			builder.WriteString(fmt.Sprintf(` <strong>(fails: %d)</strong>`, failure.Fails))
		}
		builder.WriteString(`</li>`)
	}
	builder.WriteString(`</ul></section>`)
	return builder.String()
}

func logIfPresent(logMessage func(string), message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	if logMessage != nil {
		logMessage(message)
	}
	log.Print(message)
}

func filterDramaProgressMarkers(output string) string {
	lines := strings.Split(output, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if isDramaProgressMarkerLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// removeANSI removes ANSI escape codes from a string to clean up K6 CLI output
func removeANSI(str string) string {
	// Simple implementation; you might need a regex package for full compliance
	// \x1b\[[0-9;]*m
	replacer := strings.NewReplacer("\x1b[0m", "", "\x1b[31m", "", "\x1b[32m", "", "\x1b[33m", "", "\x1b[34m", "", "\x1b[35m", "", "\x1b[36m", "", "\x1b[90m", "")
	return replacer.Replace(str)
}
