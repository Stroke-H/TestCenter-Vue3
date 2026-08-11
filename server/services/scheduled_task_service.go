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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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

type scheduledTaskAttemptResult struct {
	Attempt   int
	ReportDir string
	StartedAt time.Time
	Duration  time.Duration
	Err       error
}

type scheduledTaskRunControl struct {
	cancel context.CancelFunc
}

const (
	scheduledFunctionEpisodePlayback  = "episode-playback-test"
	scheduledFunctionExternalSubtitle = "external-subtitle-test"
	defaultScheduledTaskMaxRuntime    = 3 * time.Hour
	scheduledTaskSafeResultRunes      = 60
	scheduledTaskDetailedResultRunes  = 8000
)

var (
	scheduledTaskMu   sync.Mutex
	scheduledRunnerOn sync.Once
	scheduledRunMu    sync.Mutex
	scheduledRunIDs   = map[string]scheduledTaskRunControl{}
)

var scheduledTaskRetryDelays = []time.Duration{
	5 * time.Minute,
	10 * time.Minute,
	15 * time.Minute,
}

var scheduledTaskSaveRetryDelays = []time.Duration{
	0,
	500 * time.Millisecond,
	2 * time.Second,
}

func isSupportedScheduledTaskFunction(function string) bool {
	switch function {
	case scheduledFunctionEpisodePlayback, scheduledFunctionExternalSubtitle:
		return true
	default:
		return false
	}
}

func scheduledTaskDisplayName(function string) string {
	switch function {
	case scheduledFunctionExternalSubtitle:
		return "剧集外挂字幕测试"
	default:
		return "剧集播放接口测试"
	}
}

func scheduledTaskDescription(function string) string {
	switch function {
	case scheduledFunctionExternalSubtitle:
		return "Run the external subtitle validation flow for online dramas."
	default:
		return "Run the drama playback API test flow for the selected project."
	}
}

func scheduledTaskReportType(function string) string {
	switch function {
	case scheduledFunctionExternalSubtitle:
		return "接口验证"
	default:
		return "K6 压测"
	}
}

func InitScheduledTaskService() {
	scheduledRunnerOn.Do(func() {
		go scheduledTaskLoop()
	})
}

func ListScheduledTasksHandler(c *gin.Context) {
	recoverAbandonedScheduledTasks()
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

	if !isSupportedScheduledTaskFunction(req.Function) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported scheduled task function"})
		return
	}

	nextRunAt, err := parseScheduledTime(req.NextRun)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid next_run, expected YYYY-MM-DD HH:mm:ss"})
		return
	}
	if !isScheduledTimeAllowed(nextRunAt, time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "下次执行时间不能早于当前时间"})
		return
	}

	now := time.Now()
	task := ScheduledTask{
		ID:              "ST-" + uuid.New().String(),
		Name:            scheduledTaskDisplayName(req.Function),
		Function:        req.Function,
		ScheduleType:    req.ScheduleType,
		Creator:         req.Creator,
		NextRun:         req.NextRun,
		NextRunAt:       nextRunAt.Format(time.RFC3339),
		TestProject:     req.TestProject,
		TestProjectCode: req.TestProjectCode,
		TestEnv:         req.TestEnv,
		Status:          "active",
		Description:     scheduledTaskDescription(req.Function),
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
	recoverAbandonedScheduledTasks()
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
	if !isScheduledTimeAllowed(nextRunAt, time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "下次执行时间不能早于当前时间"})
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

func DeleteScheduledTaskHandler(c *gin.Context) {
	recoverAbandonedScheduledTasks()
	taskID := c.Param("id")
	tasks, err := loadScheduledTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for i := range tasks {
		if tasks[i].ID != taskID {
			continue
		}
		if tasks[i].Status == "running" && isScheduledTaskActivelyRunning(taskID) {
			c.JSON(http.StatusConflict, gin.H{"error": "Running scheduled task cannot be deleted"})
			return
		}
		tasks = append(tasks[:i], tasks[i+1:]...)
		if err := saveScheduledTasks(tasks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Scheduled task not found"})
}

func PauseScheduledTaskHandler(c *gin.Context) {
	taskID := c.Param("id")
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
			cancelScheduledTaskRun(taskID)
		}
		tasks[i].Status = "paused"
		tasks[i].LastResult = "任务已手动暂停"
		tasks[i].UpdatedAt = now.Format(time.RFC3339)
		if err := saveScheduledTasks(tasks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, tasks[i])
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Scheduled task not found"})
}

func StopCurrentScheduledTaskHandler(c *gin.Context) {
	taskID := c.Param("id")
	tasks, err := loadScheduledTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for i := range tasks {
		if tasks[i].ID != taskID {
			continue
		}
		if tasks[i].Status != "running" || !isScheduledTaskActivelyRunning(taskID) {
			c.JSON(http.StatusConflict, gin.H{"error": "Scheduled task is not currently running"})
			return
		}
		cancelScheduledTaskRun(taskID)
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Scheduled task not found"})
}

func scheduledTaskLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	runScheduledTaskCycle()
	for range ticker.C {
		runScheduledTaskCycle()
	}
}

func runScheduledTaskCycle() {
	// Recovery is part of every scheduler cycle rather than a one-time startup
	// action. A failed persistence attempt must never leave a task permanently
	// stuck in the running state.
	recoverAbandonedScheduledTasks()
	runDueScheduledTasks()
}

func runDueScheduledTasks() {
	tasks, err := loadScheduledTasks()
	if err != nil {
		log.Printf("[ScheduledTask] load failed: %v", err)
		return
	}

	now := time.Now()
	for i := range tasks {
		task := &tasks[i]
		if task.Status != "active" || !isSupportedScheduledTaskFunction(task.Function) {
			continue
		}
		nextRunAt, err := time.Parse(time.RFC3339, task.NextRunAt)
		if err != nil || now.Before(nextRunAt) {
			continue
		}

		task.Status = "running"
		task.UpdatedAt = now.Format(time.RFC3339)
		runCtx, cancel := context.WithTimeout(context.Background(), scheduledTaskExecutionTimeout())
		registerScheduledTaskRun(task.ID, cancel)
		if err := upsertScheduledTaskWithRetry(*task); err != nil {
			unregisterScheduledTaskRun(task.ID)
			cancel()
			task.Status = "active"
			log.Printf("[ScheduledTask] mark running failed: %v", err)
			continue
		}

		go func(ctx context.Context, scheduledTask ScheduledTask, cancel context.CancelFunc) {
			defer cancel()
			executeScheduledTask(ctx, scheduledTask)
		}(runCtx, *task, cancel)
	}
}

func registerScheduledTaskRun(taskID string, cancel context.CancelFunc) {
	scheduledRunMu.Lock()
	defer scheduledRunMu.Unlock()
	scheduledRunIDs[taskID] = scheduledTaskRunControl{cancel: cancel}
}

func unregisterScheduledTaskRun(taskID string) {
	scheduledRunMu.Lock()
	defer scheduledRunMu.Unlock()
	delete(scheduledRunIDs, taskID)
}

func isScheduledTaskActivelyRunning(taskID string) bool {
	scheduledRunMu.Lock()
	defer scheduledRunMu.Unlock()
	_, ok := scheduledRunIDs[taskID]
	return ok
}

func cancelScheduledTaskRun(taskID string) bool {
	scheduledRunMu.Lock()
	control, ok := scheduledRunIDs[taskID]
	scheduledRunMu.Unlock()
	if !ok || control.cancel == nil {
		return false
	}
	control.cancel()
	return true
}

func recoverAbandonedScheduledTasks() {
	tasks, err := loadScheduledTasks()
	if err != nil {
		log.Printf("[ScheduledTask] recover abandoned running tasks failed: %v", err)
		return
	}

	now := time.Now()
	for i := range tasks {
		if tasks[i].Status != "running" || isScheduledTaskActivelyRunning(tasks[i].ID) {
			continue
		}
		prepareAbandonedScheduledTaskForRetry(&tasks[i], now)
		if err := upsertScheduledTaskWithRetry(tasks[i]); err != nil {
			log.Printf("[ScheduledTask] save recovered scheduled task %s failed: %v", tasks[i].ID, err)
		}
	}
}

func prepareAbandonedScheduledTaskForRetry(task *ScheduledTask, now time.Time) {
	task.LastResult = "任务中断，调度器将在恢复后自动补跑"
	task.UpdatedAt = now.Format(time.RFC3339)
	if _, err := time.Parse(time.RFC3339, task.NextRunAt); err != nil {
		task.Status = "paused"
		return
	}
	switch task.ScheduleType {
	case "Once", "Daily", "weekly":
		// Preserve NextRunAt. If it is overdue, runDueScheduledTasks executes one
		// catch-up run immediately, and normal rescheduling then moves it to the
		// next future slot without replaying every missed day.
		task.Status = "active"
	default:
		task.Status = "paused"
	}
}

func scheduledTaskExecutionTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SCHEDULED_TASK_MAX_RUNTIME"))
	if raw == "" {
		return defaultScheduledTaskMaxRuntime
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		log.Printf("[ScheduledTask] invalid SCHEDULED_TASK_MAX_RUNTIME=%q; using %s", raw, defaultScheduledTaskMaxRuntime)
		return defaultScheduledTaskMaxRuntime
	}
	return duration
}

func executeScheduledTask(ctx context.Context, task ScheduledTask) {
	start := time.Now()
	runID := fmt.Sprintf("ST-%s-%d", sanitizeReportSuffix(task.ID), start.UnixMilli())
	log.Printf("[ScheduledTask] 开始执行：%s (%s)", task.Name, task.ID)
	defer unregisterScheduledTaskRun(task.ID)
	defer func() {
		if recovered := recover(); recovered != nil {
			result := fmt.Sprintf("panic recovered: %v", recovered)
			log.Printf("[ScheduledTask] execute panic recovered for %s (%s): %v", task.Name, task.ID, recovered)
			updateScheduledTaskAfterRun(task, start, result)
		}
	}()

	attempts := runScheduledTaskWithRetries(ctx, task, runID)
	finalAttempt := attempts[len(attempts)-1]
	duration := time.Since(start)
	result := "success"
	status := "Passed"
	reportDir := finalAttempt.ReportDir
	if finalAttempt.Err != nil {
		result = buildScheduledTaskAttemptSummary(attempts)
		status = "Failed"
		log.Printf("[ScheduledTask] execute failed after %d attempt(s): %v", len(attempts), finalAttempt.Err)
		if ctx.Err() != nil {
			result = "任务已手动停止"
		}
	} else if len(attempts) > 1 {
		result = fmt.Sprintf("success after %d attempt(s)", len(attempts))
	}
	if finalAttempt.Err == nil && task.Function == scheduledFunctionExternalSubtitle {
		summary := loadSubtitleScheduledSummary(reportDir)
		if summary.Failures > 0 {
			status = "Failed"
			result = fmt.Sprintf("subtitle check found %d failure(s)", summary.Failures)
		}
	}

	updateScheduledTaskAfterRun(task, start, result)

	reportURL := ""
	rootDir := projectRootDir()
	if status == "Passed" || task.Function == scheduledFunctionExternalSubtitle {
		reportFile, snapshotURL, snapshotErr := scheduledTaskReportSnapshot(rootDir, reportDir, runID, task.Function)
		if snapshotErr != nil {
			log.Printf("[ScheduledTask] snapshot report failed: %v", snapshotErr)
		} else {
			reportURL = snapshotURL
			archive, archiveErr := createScheduledTaskArchive(rootDir, runID, task, status, duration, start, reportFile)
			if archiveErr != nil {
				log.Printf("[ScheduledTask] archive report failed: %v", archiveErr)
			} else if len(archive.Artifacts) > 0 {
				reportURL = archive.Artifacts[0].URL
			}
		}
	}
	if _, addErr := AddExecutionReport(ExecutionReport{
		Name:        task.Name,
		Type:        scheduledTaskReportType(task.Function),
		Status:      status,
		Duration:    formatDuration(duration),
		Author:      task.Creator,
		ReportURL:   reportURL,
		RunID:       runID,
		Environment: normalizeReportEnvironment(task.TestEnv),
	}); addErr != nil {
		log.Printf("[ScheduledTask] add report failed: %v", addErr)
	}

	noticeText, noticeCard := buildScheduledTaskNoticeByFunction(task, status, result, formatDuration(duration), reportURL, start, reportDir)
	if notifyErr := notifyScheduledTaskCreator(task.Creator, noticeText, noticeCard); notifyErr != nil {
		log.Printf("[ScheduledTask] notify failed: %v", notifyErr)
	}

	appendScheduledTaskAuditLog(task, status, result, formatDuration(duration), reportURL, start, reportDir)
	log.Printf("[ScheduledTask] 执行完成：%s (%s)，结果=%s，耗时=%s", task.Name, task.ID, status, formatDuration(duration))
}

func scheduledTaskReportSnapshot(rootDir string, reportDir string, runID string, function string) (string, string, error) {
	if function == scheduledFunctionExternalSubtitle {
		reportFile := filepath.Join(reportDir, "subtitle_report.html")
		if _, err := os.Stat(filepath.Join(rootDir, reportFile)); err != nil {
			return "", "", err
		}
		return reportFile, PlatformBackendURL("/api/test-runs/" + url.PathEscape(runID) + "/artifacts/report"), nil
	}
	return snapshotDramaReport(rootDir, reportDir, runID)
}

func createScheduledTaskArchive(rootDir string, runID string, task ScheduledTask, status string, duration time.Duration, startedAt time.Time, reportFile string) (TestRunArchive, error) {
	if task.Function == scheduledFunctionExternalSubtitle {
		return CreateScheduledSubtitleArchive(rootDir, runID, task, status, duration, startedAt, reportFile)
	}
	return CreateScheduledDramaArchive(rootDir, runID, task, status, duration, startedAt, reportFile)
}

func runScheduledTaskWithRetries(ctx context.Context, task ScheduledTask, runID string) []scheduledTaskAttemptResult {
	maxAttempts := len(scheduledTaskRetryDelays) + 1
	attempts := make([]scheduledTaskAttemptResult, 0, maxAttempts)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			delay := scheduledTaskRetryDelays[attempt-2]
			log.Printf("[ScheduledTask] retrying %s (%s), attempt %d/%d after %s", task.Name, task.ID, attempt, maxAttempts, delay)
			select {
			case <-ctx.Done():
				attempts = append(attempts, scheduledTaskAttemptResult{
					Attempt:   attempt,
					StartedAt: time.Now(),
					Duration:  0,
					Err:       ctx.Err(),
				})
				return attempts
			case <-time.After(delay):
			}
		}
		if ctx.Err() != nil {
			attempts = append(attempts, scheduledTaskAttemptResult{
				Attempt:   attempt,
				StartedAt: time.Now(),
				Duration:  0,
				Err:       ctx.Err(),
			})
			return attempts
		}

		attemptStart := time.Now()
		attemptRunID := runID
		if attempt > 1 {
			attemptRunID = fmt.Sprintf("%s-attempt-%d", runID, attempt)
		}
		reportDir := scheduledTaskRuntimeReportDir(task.Function, attemptRunID)
		err := runScheduledTaskAttempt(ctx, task, reportDir)
		attemptResult := scheduledTaskAttemptResult{
			Attempt:   attempt,
			ReportDir: reportDir,
			StartedAt: attemptStart,
			Duration:  time.Since(attemptStart),
			Err:       err,
		}
		attempts = append(attempts, attemptResult)
		if err == nil {
			return attempts
		}

		log.Printf("[ScheduledTask] %s (%s) attempt %d/%d failed: %v", task.Name, task.ID, attempt, maxAttempts, err)
	}

	return attempts
}

func buildScheduledTaskAttemptSummary(attempts []scheduledTaskAttemptResult) string {
	if len(attempts) == 0 {
		return "task failed without attempt detail"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("failed after %d attempt(s)", len(attempts)))
	for _, attempt := range attempts {
		if attempt.Err == nil {
			builder.WriteString(fmt.Sprintf("\nAttempt %d: success, duration=%s", attempt.Attempt, formatDuration(attempt.Duration)))
			continue
		}
		builder.WriteString(fmt.Sprintf("\nAttempt %d: %v, duration=%s", attempt.Attempt, attempt.Err, formatDuration(attempt.Duration)))
	}
	return builder.String()
}

func runDramaPlaybackTask(ctx context.Context, testEnv string, reportDir string) error {
	profile := getDramaProfile(testEnv)
	rootDir, _ := filepath.Abs("..")
	env := appendDramaProfileEnv(os.Environ(), profile, "", "", "", "")
	env = append(env, "DRAMA_REPORT_DIR="+reportDir)

	_ = os.MkdirAll(filepath.Join(rootDir, reportDir), 0755)
	removeDramaRetryArtifacts(rootDir, reportDir)
	if err := runCommand(ctx, rootDir, env, "node", filepath.Join("k6-scripts", "prepare_data.js")); err != nil {
		return fmt.Errorf("prepare data failed: %w", err)
	}
	if err := runCommand(ctx, rootDir, env, "k6", "run", filepath.Join("k6-scripts", "drama_check_flow.js")); err != nil {
		return fmt.Errorf("k6 run failed: %w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := rerunDramaRetryCandidates(ctx, rootDir, reportDir, env, nil, false); err != nil {
		return fmt.Errorf("retry failed cases failed: %w", err)
	}
	return nil
}

func runScheduledTaskAttempt(ctx context.Context, task ScheduledTask, reportDir string) error {
	switch task.Function {
	case scheduledFunctionExternalSubtitle:
		return runExternalSubtitleTask(ctx, task.TestEnv, reportDir)
	default:
		return runDramaPlaybackTask(ctx, task.TestEnv, reportDir)
	}
}

func scheduledTaskRuntimeReportDir(function string, runID string) string {
	switch function {
	case scheduledFunctionExternalSubtitle:
		return filepath.ToSlash(filepath.Join("report", "api_report", ".runtime", "subtitle_"+runID))
	default:
		return dramaRuntimeReportDir(runID)
	}
}

func runExternalSubtitleTask(ctx context.Context, testEnv string, reportDir string) error {
	profile := getDramaProfile(testEnv)
	rootDir, _ := filepath.Abs("..")
	env := appendDramaProfileEnv(os.Environ(), profile, "", "", "", "")
	env = append(env, "SUBTITLE_REPORT_DIR="+reportDir)

	_ = os.MkdirAll(filepath.Join(rootDir, reportDir), 0755)
	if err := runCommand(ctx, rootDir, env, "node", filepath.Join("scripts", "prepare_subtitle_check.js")); err != nil {
		return fmt.Errorf("prepare subtitle data failed: %w", err)
	}
	if err := runCommand(ctx, rootDir, env, "node", filepath.Join("scripts", "run_subtitle_check.js")); err != nil {
		return fmt.Errorf("subtitle check failed: %w", err)
	}
	if err := runCommand(ctx, rootDir, env, "node", filepath.Join("scripts", "build_subtitle_report.js")); err != nil {
		return fmt.Errorf("build subtitle report failed: %w", err)
	}
	return ctx.Err()
}

func runCommand(ctx context.Context, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
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
	drainCommandOutput := func(reader io.Reader) {
		defer wg.Done()
		_, _ = io.Copy(io.Discard, reader)
	}

	wg.Add(2)
	go drainCommandOutput(stdout)
	go drainCommandOutput(stderr)

	err = cmd.Wait()
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func buildScheduledTaskNotice(task ScheduledTask, status string, result string, duration string, reportURL string, startedAt time.Time, reportDir string) (string, map[string]any) {
	analysis, err := analyzeScheduledTaskReportWithAI(task, status, result, duration, startedAt, reportDir)
	if err != nil {
		log.Printf("[ScheduledTask] report analysis skipped: %v", err)
		analysis = "报告分析暂未生成，请打开平台查看完整报告。"
	}
	failures := loadScheduledDramaFailures(reportDir)
	dramaFailures, groupFailures, classificationFailures, otherFailures := splitScheduledDramaFailures(failures)
	failureDetail := formatScheduledDramaFailureDetail(dramaFailures)
	groupFailureDetail := formatScheduledDramaFailureDetail(groupFailures)
	classificationFailureDetail := formatScheduledDramaFailureDetail(classificationFailures)
	otherFailureDetail := formatScheduledDramaFailureDetail(otherFailures)

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
	if strings.TrimSpace(failureDetail) != "" {
		builder.WriteString("异常剧集明细\n")
		builder.WriteString(failureDetail + "\n\n")
	}
	if strings.TrimSpace(groupFailureDetail) != "" {
		builder.WriteString("分组异常\n")
		builder.WriteString(groupFailureDetail + "\n\n")
	}
	if strings.TrimSpace(classificationFailureDetail) != "" {
		builder.WriteString("分类异常\n")
		builder.WriteString(classificationFailureDetail + "\n\n")
	}
	if strings.TrimSpace(otherFailureDetail) != "" {
		builder.WriteString("其他异常\n")
		builder.WriteString(otherFailureDetail + "\n\n")
	}
	builder.WriteString("报告地址：" + reportURL)
	text := sanitizeFeishuPlainText(builder.String())
	return text, buildScheduledTaskFeishuCard(task, statusText, duration, reportURL, analysis, dramaFailures, groupFailures, classificationFailures, otherFailures)
}

func buildScheduledTaskNoticeByFunction(task ScheduledTask, status string, result string, duration string, reportURL string, startedAt time.Time, reportDir string) (string, map[string]any) {
	if task.Function == scheduledFunctionExternalSubtitle {
		return buildSubtitleScheduledTaskNotice(task, status, result, duration, reportURL, reportDir)
	}
	return buildScheduledTaskNotice(task, status, result, duration, reportURL, startedAt, reportDir)
}

type subtitleScheduledSummary struct {
	TotalDramas             int    `json:"totalDramas"`
	ExternalDramas          int    `json:"externalDramas"`
	TotalSubtitleURLs       int    `json:"totalSubtitleUrls"`
	TotalCheckItems         int    `json:"totalCheckItems"`
	CheckedFiles            int    `json:"checkedFiles"`
	FailedFiles             int    `json:"failedFiles"`
	Failures                int    `json:"failures"`
	TimestampFailures       int    `json:"timestampFailures"`
	FetchFailures           int    `json:"fetchFailures"`
	MissingSubtitleFailures int    `json:"missingSubtitleFailures"`
	SubtitleCountFailures   int    `json:"subtitleCountFailures"`
	FinishedAt              string `json:"finishedAt"`
}

func buildSubtitleScheduledTaskNotice(task ScheduledTask, status string, result string, duration string, reportURL string, reportDir string) (string, map[string]any) {
	summary := loadSubtitleScheduledSummary(reportDir)
	statusText := "通过"
	if status != "Passed" || summary.Failures > 0 {
		statusText = "失败"
	}

	var builder strings.Builder
	builder.WriteString("定时任务执行完成\n")
	builder.WriteString("任务：" + task.Name + "\n")
	builder.WriteString("项目：" + valueOrFallback(task.TestProject, task.TestProjectCode) + "\n")
	builder.WriteString("环境：" + task.TestEnv + "\n")
	builder.WriteString("结果：" + statusText + "\n")
	builder.WriteString("耗时：" + duration + "\n")
	if summary.TotalSubtitleURLs > 0 || summary.CheckedFiles > 0 {
		builder.WriteString(fmt.Sprintf("外挂剧：%d 部\n", summary.ExternalDramas))
		builder.WriteString(fmt.Sprintf("检查项：%d/%d，正片字幕文件：%d\n", summary.CheckedFiles, valueOrDefaultInt(summary.TotalCheckItems, summary.TotalSubtitleURLs), summary.TotalSubtitleURLs))
		builder.WriteString(fmt.Sprintf("异常：%d 条，影响文件：%d 个\n", summary.Failures, summary.FailedFiles))
		builder.WriteString(fmt.Sprintf("字幕数量异常：%d，缺失字幕：%d，时间轴异常：%d，拉取失败：%d\n", summary.SubtitleCountFailures, summary.MissingSubtitleFailures, summary.TimestampFailures, summary.FetchFailures))
	} else if strings.TrimSpace(result) != "" {
		builder.WriteString("执行结果：" + sanitizeFeishuPlainText(result) + "\n")
	}
	builder.WriteString("报告地址：" + reportURL)

	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": task.Name},
			"template": map[string]string{"通过": "green", "失败": "red"}[statusText],
		},
		"elements": []any{
			map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**结果：**%s\n**项目：**%s\n**环境：**%s\n**耗时：**%s", statusText, valueOrFallback(task.TestProject, task.TestProjectCode), task.TestEnv, duration)}},
			map[string]any{"tag": "hr"},
			map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**外挂剧：**%d 部\n**检查项：**%d/%d　**正片字幕文件：**%d\n**异常：**%d 条，影响文件 %d 个\n**字幕数量异常：**%d　**缺失字幕：**%d　**时间轴异常：**%d　**拉取失败：**%d", summary.ExternalDramas, summary.CheckedFiles, valueOrDefaultInt(summary.TotalCheckItems, summary.TotalSubtitleURLs), summary.TotalSubtitleURLs, summary.Failures, summary.FailedFiles, summary.SubtitleCountFailures, summary.MissingSubtitleFailures, summary.TimestampFailures, summary.FetchFailures)}},
		},
	}
	if strings.TrimSpace(reportURL) != "" {
		card["elements"] = append(card["elements"].([]any), map[string]any{
			"tag": "action",
			"actions": []any{
				map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "查看报告"}, "url": reportURL, "type": "primary"},
			},
		})
	}
	return sanitizeFeishuPlainText(builder.String()), card
}

func loadSubtitleScheduledSummary(reportDir string) subtitleScheduledSummary {
	rootDir, _ := filepath.Abs("..")
	content, err := os.ReadFile(filepath.Join(rootDir, reportDir, "subtitle_summary.json"))
	if err != nil {
		return subtitleScheduledSummary{}
	}
	var summary subtitleScheduledSummary
	_ = json.Unmarshal(content, &summary)
	return summary
}

func valueOrDefaultInt(value int, fallback int) int {
	if value != 0 {
		return value
	}
	return fallback
}

func SendDramaRunFeishuReportHandler(c *gin.Context) {
	runID := sanitizeReportSuffix(c.Param("runId"))
	if strings.TrimSpace(runID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runId is required"})
		return
	}

	rootDir := projectRootDir()
	runDir := filepath.Join(testRunStorageRoot(rootDir), runID)
	metadataPath := filepath.Join(runDir, "metadata.json")
	metadataContent, err := os.ReadFile(metadataPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "测试报告归档不存在"})
		return
	}

	var archive TestRunArchive
	if err := json.Unmarshal(metadataContent, &archive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "测试报告归档元信息无效"})
		return
	}
	if archive.TestType != "drama" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅剧集播放接口测试报告支持发送 AI 飞书报告"})
		return
	}

	reportPath := filepath.Join(runDir, "artifacts", "report.html")
	reportContent, err := os.ReadFile(reportPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "测试报告 HTML 不存在"})
		return
	}

	reportURL := firstNonEmpty(archive.ReportURLFromArtifacts(), PlatformBackendURL("/api/test-runs/"+url.PathEscape(runID)+"/artifacts/report"))
	task := scheduledTaskFromDramaArchive(archive)
	status := firstNonEmpty(archive.Status, "Passed")
	duration := firstNonEmpty(archive.Duration, "-")
	reportText := htmlToPlainText(string(reportContent))
	if retryContent, err := os.ReadFile(filepath.Join(runDir, "artifacts", "drama_retry_result.json")); err == nil {
		reportText += "\n\nRetry result JSON:\n" + string(retryContent)
	}

	analysis, err := analyzeScheduledReportTextWithAI(task, status, "", duration, reportText)
	if err != nil {
		log.Printf("[DramaRun] manual Feishu report analysis skipped for %s: %v", runID, err)
		analysis = "报告分析暂未生成，请打开平台查看完整报告。"
	}

	failures := loadArchivedDramaFailures(runDir, string(reportContent))
	dramaFailures, groupFailures, classificationFailures, otherFailures := splitScheduledDramaFailures(failures)
	statusText := "通过"
	if !isDramaArchiveSuccessStatus(status) {
		statusText = "失败"
	}

	card := buildScheduledTaskFeishuCard(task, statusText, duration, reportURL, analysis, dramaFailures, groupFailures, classificationFailures, otherFailures)
	if err := sendFeishuGroupInteractiveCard(card); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "发送飞书失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "剧集播放接口测试 AI 报告已发送到飞书"})
}

func splitScheduledDramaFailures(failures []dramaFailureSummary) ([]dramaFailureSummary, []dramaFailureSummary, []dramaFailureSummary, []dramaFailureSummary) {
	dramaFailures := make([]dramaFailureSummary, 0, len(failures))
	groupFailures := make([]dramaFailureSummary, 0, len(failures))
	classificationFailures := make([]dramaFailureSummary, 0, len(failures))
	otherFailures := make([]dramaFailureSummary, 0, len(failures))

	for _, failure := range failures {
		dramaFailure := failure
		dramaFailure.Errors = nil
		groupFailure := failure
		groupFailure.Errors = nil
		classificationFailure := failure
		classificationFailure.Errors = nil
		otherFailure := failure
		otherFailure.Errors = nil

		for _, line := range failure.Errors {
			if isScheduledDramaClassificationError(line) {
				classificationFailure.Errors = append(classificationFailure.Errors, line)
			} else if isScheduledDramaOtherError(line) {
				otherFailure.Errors = append(otherFailure.Errors, line)
			} else if isScheduledDramaGroupError(line) {
				groupFailure.Errors = append(groupFailure.Errors, line)
			} else {
				dramaFailure.Errors = append(dramaFailure.Errors, line)
			}
		}
		if len(dramaFailure.Errors) > 0 {
			dramaFailures = append(dramaFailures, dramaFailure)
		}
		if len(groupFailure.Errors) > 0 {
			groupFailures = append(groupFailures, groupFailure)
		}
		if len(classificationFailure.Errors) > 0 {
			classificationFailures = append(classificationFailures, classificationFailure)
		}
		if len(otherFailure.Errors) > 0 {
			otherFailures = append(otherFailures, otherFailure)
		}
	}

	return dramaFailures, groupFailures, classificationFailures, otherFailures
}

func isScheduledDramaGroupError(line string) bool {
	normalized := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "•"))
	return strings.HasPrefix(normalized, "[解锁类型]") ||
		strings.HasPrefix(normalized, "[分组规则]") ||
		strings.HasPrefix(normalized, "[App Group元数据]")
}

func isScheduledDramaOtherError(line string) bool {
	normalized := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "•"))
	return strings.HasPrefix(normalized, "[命名规范]")
}

func isScheduledDramaClassificationError(line string) bool {
	normalized := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "•"))
	return strings.HasPrefix(normalized, "[分类规则]")
}

func formatScheduledDramaFailureDetail(failures []dramaFailureSummary) string {
	if len(failures) == 0 {
		return ""
	}

	sort.SliceStable(failures, func(i, j int) bool {
		left := valueOrFallback(failures[i].IntID, failures[i].DramaID)
		right := valueOrFallback(failures[j].IntID, failures[j].DramaID)
		return left < right
	})

	var builder strings.Builder
	for index, failure := range failures {
		if index > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(valueOrFallback(failure.IntID, "-") + "\n")
		builder.WriteString(valueOrFallback(failure.DramaID, "-") + "\n")
		builder.WriteString(valueOrFallback(failure.Title, "-") + "\n")
		builder.WriteString(valueOrFallback(failure.CNTitle, "-") + "\n")
		for _, line := range failure.Errors {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if !strings.HasPrefix(trimmed, "•") {
				trimmed = "• " + trimmed
			}
			builder.WriteString(trimmed + "\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

func buildScheduledTaskFeishuCard(task ScheduledTask, statusText string, duration string, reportURL string, analysis string, dramaFailures []dramaFailureSummary, groupFailures []dramaFailureSummary, classificationFailures []dramaFailureSummary, otherFailures []dramaFailureSummary) map[string]any {
	statusColor := "green"
	if statusText != "通过" || len(dramaFailures) > 0 || len(groupFailures) > 0 || len(classificationFailures) > 0 || len(otherFailures) > 0 {
		statusColor = "orange"
	}
	if statusText == "失败" {
		statusColor = "red"
	}

	elements := []map[string]any{
		{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**任务：** %s\n**项目：** %s\n**环境：** %s\n**结果：** %s\n**耗时：** %s", escapeLarkMarkdown(task.Name), escapeLarkMarkdown(valueOrFallback(task.TestProject, task.TestProjectCode)), escapeLarkMarkdown(task.TestEnv), escapeLarkMarkdown(statusText), escapeLarkMarkdown(duration)),
			},
		},
		{"tag": "hr"},
		{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": "**报告总结**\n" + escapeLarkMarkdown(sanitizeFeishuPlainText(analysis)),
			},
		},
	}

	if len(dramaFailures) > 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**异常剧集明细（%d 部）**", len(dramaFailures)),
			},
		})
		for _, failure := range dramaFailures {
			elements = append(elements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": formatDramaFailureCardBlock(failure),
				},
			})
		}
	}

	if len(groupFailures) > 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**分组异常（%d 部）**", len(groupFailures)),
			},
		})
		for _, failure := range groupFailures {
			elements = append(elements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": formatDramaFailureCardBlock(failure),
				},
			})
		}
	}

	if len(classificationFailures) > 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**分类异常（%d 部）**", len(classificationFailures)),
			},
		})
		for _, failure := range classificationFailures {
			elements = append(elements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": formatDramaFailureCardBlock(failure),
				},
			})
		}
	}

	if len(otherFailures) > 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**其他异常（%d 部）**", len(otherFailures)),
			},
		})
		for _, failure := range otherFailures {
			elements = append(elements, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": formatDramaFailureCardBlock(failure),
				},
			})
		}
	}

	if strings.TrimSpace(reportURL) != "" {
		elements = append(elements, map[string]any{"tag": "hr"})
		elements = append(elements, map[string]any{
			"tag": "action",
			"actions": []map[string]any{
				{
					"tag":  "button",
					"text": map[string]any{"tag": "plain_text", "content": "查看完整报告"},
					"url":  reportURL,
					"type": "primary",
				},
			},
		})
	}

	return map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"template": statusColor,
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "剧集播放接口测试完成",
			},
		},
		"elements": elements,
	}
}

const defaultDramaManagementListURL = "https://admin.shortswave.com/drama/list"

func buildDramaManagementURL(dramaID string) string {
	dramaID = strings.TrimSpace(dramaID)
	if dramaID == "" {
		return ""
	}

	baseURL := strings.TrimSpace(os.Getenv("DRAMA_MANAGEMENT_LIST_URL"))
	if baseURL == "" {
		baseURL = defaultDramaManagementListURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	target, err := json.Marshal(map[string]string{"id": dramaID})
	if err != nil {
		return ""
	}
	query := parsed.Query()
	query.Set("target", string(target))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func formatDramaFailureCardBlock(failure dramaFailureSummary) string {
	lines := []string{
		fmt.Sprintf("**%s  %s**", escapeLarkMarkdown(valueOrFallback(failure.IntID, "-")), escapeLarkMarkdown(valueOrFallback(failure.Title, "-"))),
		escapeLarkMarkdown(valueOrFallback(failure.DramaID, "-")),
		escapeLarkMarkdown(valueOrFallback(failure.CNTitle, "-")),
	}
	dramaURL := buildDramaManagementURL(failure.DramaID)
	for _, line := range failure.Errors {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			formatted := escapeLarkMarkdown(trimmed)
			if dramaURL != "" {
				formatted += "  [查看剧集信息](" + dramaURL + ")"
			}
			lines = append(lines, formatted)
		}
	}
	return strings.Join(lines, "\n")
}

func (archive TestRunArchive) ReportURLFromArtifacts() string {
	for _, artifact := range archive.Artifacts {
		if strings.TrimSpace(artifact.URL) != "" {
			return artifact.URL
		}
	}
	return ""
}

func scheduledTaskFromDramaArchive(archive TestRunArchive) ScheduledTask {
	return ScheduledTask{
		ID:          archive.RunID,
		Name:        firstNonEmpty(archive.TestName, "剧集播放接口测试"),
		Creator:     firstNonEmpty(archive.Author, "tester"),
		TestProject: "剧集播放接口测试",
		TestEnv:     "归档报告",
	}
}

func isDramaArchiveSuccessStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "passed", "success", "done":
		return true
	default:
		return false
	}
}

func loadArchivedDramaFailures(runDir string, reportHTML string) []dramaFailureSummary {
	merged := make(map[string]dramaFailureSummary)
	addFailures := func(items []dramaFailureSummary) {
		for _, item := range items {
			key := strings.TrimSpace(item.DramaID)
			if key == "" {
				key = strings.Join([]string{item.IntID, item.Title, item.CNTitle}, "|")
			}
			if key == "" {
				continue
			}
			existing := merged[key]
			if existing.DramaID == "" {
				existing = item
			} else {
				existing.DramaID = valueOrFallback(existing.DramaID, item.DramaID)
				existing.IntID = valueOrFallback(existing.IntID, item.IntID)
				existing.Title = valueOrFallback(existing.Title, item.Title)
				existing.CNTitle = valueOrFallback(existing.CNTitle, item.CNTitle)
			}
			existing.Errors = mergeDramaFailureErrorLines(existing.Errors, item.Errors)
			merged[key] = existing
		}
	}

	summaryPath := filepath.Join(runDir, "artifacts", "drama_failure_summary.json")
	if content, err := os.ReadFile(summaryPath); err == nil {
		var payload dramaFailureSummaryFile
		if json.Unmarshal(content, &payload) == nil {
			addFailures(payload.Failures)
		}
	}

	retryPath := filepath.Join(runDir, "artifacts", "drama_retry_result.json")
	if content, err := os.ReadFile(retryPath); err == nil {
		var payload dramaRetryResultFile
		if json.Unmarshal(content, &payload) == nil {
			addFailures(payload.StructuredFailures)
			addFailures(parseLegacyRetryFailures(payload.PersistentFailures))
		}
	}

	if len(merged) == 0 {
		addFailures(parseDramaFailuresFromReportHTML(reportHTML))
	}
	if len(merged) == 0 {
		return nil
	}
	failures := make([]dramaFailureSummary, 0, len(merged))
	for _, item := range merged {
		failures = append(failures, item)
	}
	return failures
}

var dramaDetailsRegexp = regexp.MustCompile(`(?is)<details[^>]*>\s*<summary>\s*<b>\s*(.*?)\s*</b>\s*</summary>\s*<div[^>]*>\s*(.*?)\s*</div>\s*</details>`)
var dramaIDLineRegexp = regexp.MustCompile(`(?i)ID:\s*([0-9a-f]{24})(?:\(([^)]*)\))?`)

func parseDramaFailuresFromReportHTML(reportHTML string) []dramaFailureSummary {
	matches := dramaDetailsRegexp.FindAllStringSubmatch(reportHTML, -1)
	failures := make([]dramaFailureSummary, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		summary := htmlToPlainText(strings.ReplaceAll(match[1], "<br>", "\n"))
		detailHTML := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n").Replace(match[2])
		detail := htmlToPlainText(detailHTML)
		lines := splitNonEmptyLines(detail)
		if len(lines) == 0 {
			continue
		}

		failure := dramaFailureSummary{Errors: lines}
		summaryLines := splitNonEmptyLines(summary)
		for _, line := range summaryLines {
			if strings.HasPrefix(strings.TrimSpace(line), "ID:") {
				if idMatch := dramaIDLineRegexp.FindStringSubmatch(line); len(idMatch) >= 2 {
					failure.DramaID = strings.TrimSpace(idMatch[1])
					if len(idMatch) >= 3 {
						failure.IntID = strings.TrimSpace(idMatch[2])
					}
				}
				continue
			}
			if strings.Contains(line, "/CnName:") {
				parts := strings.SplitN(line, "/CnName:", 2)
				failure.Title = strings.TrimSpace(parts[0])
				failure.CNTitle = strings.TrimSpace(parts[1])
			}
		}
		if failure.DramaID == "" && failure.Title == "" && failure.CNTitle == "" {
			continue
		}
		failures = append(failures, failure)
	}
	return failures
}

func splitNonEmptyLines(text string) []string {
	parts := strings.Split(text, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func escapeLarkMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"*", "\\*",
		"`", "\\`",
	)
	return replacer.Replace(strings.TrimSpace(text))
}

func loadScheduledDramaFailures(reportDir string) []dramaFailureSummary {
	rootDir, _ := filepath.Abs("..")
	merged := make(map[string]dramaFailureSummary)
	addFailures := func(items []dramaFailureSummary) {
		for _, item := range items {
			key := strings.TrimSpace(item.DramaID)
			if key == "" {
				continue
			}
			existing := merged[key]
			if existing.DramaID == "" {
				existing = item
			} else {
				existing.IntID = valueOrFallback(existing.IntID, item.IntID)
				existing.Title = valueOrFallback(existing.Title, item.Title)
				existing.CNTitle = valueOrFallback(existing.CNTitle, item.CNTitle)
			}
			existing.Errors = mergeDramaFailureErrorLines(existing.Errors, item.Errors)
			merged[key] = existing
		}
	}

	summaryPath := filepath.Join(rootDir, reportDir, "drama_failure_summary.json")
	if content, err := os.ReadFile(summaryPath); err == nil {
		var payload dramaFailureSummaryFile
		if json.Unmarshal(content, &payload) == nil {
			addFailures(payload.Failures)
		}
	}

	retryPath := filepath.Join(rootDir, reportDir, "drama_retry_result.json")
	if content, err := os.ReadFile(retryPath); err == nil {
		var payload dramaRetryResultFile
		if json.Unmarshal(content, &payload) == nil {
			addFailures(payload.StructuredFailures)
			addFailures(parseLegacyRetryFailures(payload.PersistentFailures))
		}
	}

	if len(merged) == 0 {
		return nil
	}
	failures := make([]dramaFailureSummary, 0, len(merged))
	for _, item := range merged {
		failures = append(failures, item)
	}
	enrichScheduledDramaFailures(rootDir, failures)
	return failures
}

func enrichScheduledDramaFailures(rootDir string, failures []dramaFailureSummary) {
	if len(failures) == 0 {
		return
	}

	info, err := readDramaInfoForMetadata(rootDir)
	if err != nil {
		log.Printf("[ScheduledTask] read drama metadata config skipped: %v", err)
		return
	}
	for index := range failures {
		enrichScheduledDramaFailureFromLocalMeta(&failures[index], info)
	}
	if strings.TrimSpace(info.APIBase) == "" || strings.TrimSpace(info.Auth.XToken) == "" {
		return
	}

	client := &http.Client{Timeout: 12 * time.Second}
	cache := make(map[string]dramaManagementItem)
	for index := range failures {
		failure := &failures[index]
		if hasDramaTitleMetadata(*failure) {
			continue
		}
		item, ok := lookupDramaManagementItem(client, info, *failure, cache)
		if !ok {
			continue
		}
		failure.IntID = valueOrFallback(failure.IntID, item.IntIDString())
		failure.Title = valueOrFallback(failure.Title, item.Title)
		failure.CNTitle = valueOrFallback(failure.CNTitle, valueOrFallback(item.CNTitle, item.CNName))
	}
}

func enrichScheduledDramaFailureFromLocalMeta(failure *dramaFailureSummary, info dramaInfoFile) {
	if failure == nil || info.DramaMeta == nil {
		return
	}
	meta, ok := info.DramaMeta[strings.TrimSpace(failure.DramaID)]
	if !ok {
		return
	}
	failure.IntID = valueOrFallback(failure.IntID, meta.IntID)
	failure.Title = valueOrFallback(failure.Title, meta.Title)
	failure.CNTitle = valueOrFallback(failure.CNTitle, valueOrFallback(meta.CNTitle, meta.CNName))
}

func readDramaInfoForMetadata(rootDir string) (dramaInfoFile, error) {
	content, err := os.ReadFile(filepath.Join(rootDir, "drama_info.json"))
	if err != nil {
		return dramaInfoFile{}, err
	}
	var info dramaInfoFile
	if err := json.Unmarshal(content, &info); err != nil {
		return dramaInfoFile{}, err
	}
	return info, nil
}

func hasDramaTitleMetadata(failure dramaFailureSummary) bool {
	return strings.TrimSpace(failure.IntID) != "" &&
		strings.TrimSpace(failure.Title) != "" &&
		strings.TrimSpace(failure.CNTitle) != ""
}

type dramaManagementListResponse struct {
	Code  int                   `json:"code"`
	Data  []dramaManagementItem `json:"data"`
	Total int                   `json:"total"`
}

type dramaManagementItem struct {
	ID      string `json:"id"`
	IntID   any    `json:"int_id"`
	Title   string `json:"title"`
	CNTitle string `json:"cn_title"`
	CNName  string `json:"cn_name"`
}

func (item dramaManagementItem) IntIDString() string {
	switch value := item.IntID.(type) {
	case json.Number:
		return value.String()
	case float64:
		return fmt.Sprintf("%.0f", value)
	case string:
		return strings.TrimSpace(value)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func lookupDramaManagementItem(client *http.Client, info dramaInfoFile, failure dramaFailureSummary, cache map[string]dramaManagementItem) (dramaManagementItem, bool) {
	if item, ok := cache[failure.DramaID]; ok {
		return item, true
	}

	queryCandidates := []url.Values{}
	if strings.TrimSpace(failure.DramaID) != "" {
		queryCandidates = append(queryCandidates,
			url.Values{"id": {failure.DramaID}},
			url.Values{"keyword": {failure.DramaID}},
			url.Values{"search": {failure.DramaID}},
		)
	}
	if strings.TrimSpace(failure.IntID) != "" {
		queryCandidates = append(queryCandidates,
			url.Values{"int_id": {failure.IntID}},
			url.Values{"keyword": {failure.IntID}},
			url.Values{"search": {failure.IntID}},
		)
	}

	for _, values := range queryCandidates {
		values.Set("page", "1")
		values.Set("page_size", "20")
		if item, ok := fetchMatchingDramaManagementItem(client, info, values, failure); ok {
			cache[failure.DramaID] = item
			return item, true
		}
	}

	const pageSize = 200
	const maxPages = 80
	for page := 1; page <= maxPages; page++ {
		values := url.Values{
			"page":      {fmt.Sprint(page)},
			"page_size": {fmt.Sprint(pageSize)},
			"online":    {"1"},
		}
		item, ok, done := fetchMatchingDramaManagementItemPage(client, info, values, failure)
		if ok {
			cache[failure.DramaID] = item
			return item, true
		}
		if done {
			break
		}
	}
	return dramaManagementItem{}, false
}

func fetchMatchingDramaManagementItem(client *http.Client, info dramaInfoFile, values url.Values, failure dramaFailureSummary) (dramaManagementItem, bool) {
	item, ok, _ := fetchMatchingDramaManagementItemPage(client, info, values, failure)
	return item, ok
}

func fetchMatchingDramaManagementItemPage(client *http.Client, info dramaInfoFile, values url.Values, failure dramaFailureSummary) (dramaManagementItem, bool, bool) {
	endpoint := strings.TrimRight(info.APIBase, "/") + "/api/management/drama/list?" + values.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return dramaManagementItem{}, false, true
	}
	req.Header.Set("Cookie", "x-token="+info.Auth.XToken)
	req.Header.Set("Connection", "close")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[ScheduledTask] drama metadata lookup failed: %v", err)
		return dramaManagementItem{}, false, true
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return dramaManagementItem{}, false, true
	}

	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	var payload dramaManagementListResponse
	if err := decoder.Decode(&payload); err != nil {
		return dramaManagementItem{}, false, true
	}
	for _, item := range payload.Data {
		if dramaManagementItemMatches(item, failure) {
			return item, true, false
		}
	}
	page := parsePositiveInt(values.Get("page"))
	pageSize := parsePositiveInt(values.Get("page_size"))
	done := len(payload.Data) == 0 || (payload.Total > 0 && page*pageSize >= payload.Total)
	return dramaManagementItem{}, false, done
}

func dramaManagementItemMatches(item dramaManagementItem, failure dramaFailureSummary) bool {
	if strings.TrimSpace(failure.DramaID) != "" && strings.EqualFold(strings.TrimSpace(item.ID), strings.TrimSpace(failure.DramaID)) {
		return true
	}
	if strings.TrimSpace(failure.IntID) != "" && item.IntIDString() == strings.TrimSpace(failure.IntID) {
		return true
	}
	return false
}

func parsePositiveInt(value string) int {
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func mergeDramaFailureErrorLines(current []string, next []string) []string {
	seen := make(map[string]bool, len(current)+len(next))
	merged := make([]string, 0, len(current)+len(next))
	for _, line := range append(current, next...) {
		normalized := strings.TrimSpace(line)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		merged = append(merged, normalized)
	}
	return merged
}

func parseLegacyRetryFailures(failures []dramaRetryFailure) []dramaFailureSummary {
	items := make([]dramaFailureSummary, 0, len(failures))
	for _, failure := range failures {
		dramaID := extractDramaIDFromFailureName(failure.Name)
		if dramaID == "" {
			continue
		}
		items = append(items, dramaFailureSummary{
			DramaID: dramaID,
			Errors:  []string{"• [接口抓取失败] 720p 网络请求失败"},
		})
	}
	return items
}

func extractDramaIDFromFailureName(name string) string {
	match := regexp.MustCompile(`剧集\s*ID:\s*([0-9a-fA-F]{24})`).FindStringSubmatch(htmlToPlainText(name))
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func appendScheduledTaskAuditLog(task ScheduledTask, status string, result string, duration string, reportURL string, startedAt time.Time, reportDir string) {
	projectName := valueOrFallback(task.TestProject, task.TestProjectCode)
	detail := fmt.Sprintf(
		"Scheduled task executed: %s | duration=%s | result=%s | next=%s",
		task.Name,
		duration,
		valueOrFallback(strings.TrimSpace(result), status),
		task.NextRun,
	)
	if strings.TrimSpace(reportURL) != "" {
		detail += " | report=" + reportURL
	}
	if task.Function != scheduledFunctionExternalSubtitle {
		failures := loadScheduledDramaFailures(reportDir)
		detail = summarizeScheduledDramaAuditFailures(failures)
		if statusToAuditStatus(status) == "failed" && len(failures) == 0 {
			detail = "执行错误 1"
		}
	}

	op := feishumodel.AIOperationLog{
		ID:        fmt.Sprintf("OP_%d", time.Now().UnixNano()),
		ToolName:  "scheduled_episode_playback_test",
		Project:   projectName,
		Env:       normalizeScheduledTaskAuditEnv(task.TestEnv),
		UserID:    task.Creator,
		UserName:  task.Creator,
		Status:    strings.ToLower(strings.TrimSpace(statusToAuditStatus(status))),
		Detail:    detail,
		Timestamp: time.Now(),
	}

	if err := SQLUpsertJSONForFeishu("ai_operation_logs", op); err != nil {
		log.Printf("[ScheduledTask] append audit log failed: %v", err)
	}
}

func summarizeScheduledDramaAuditFailures(failures []dramaFailureSummary) string {
	dramaFailures, groupFailures, classificationFailures, otherFailures := splitScheduledDramaFailures(failures)
	parts := make([]string, 0, 4)
	if len(dramaFailures) > 0 {
		parts = append(parts, fmt.Sprintf("剧集错误 %d", len(dramaFailures)))
	}
	if len(groupFailures) > 0 {
		parts = append(parts, fmt.Sprintf("分组错误 %d", len(groupFailures)))
	}
	if len(classificationFailures) > 0 {
		parts = append(parts, fmt.Sprintf("分类错误 %d", len(classificationFailures)))
	}
	if len(otherFailures) > 0 {
		parts = append(parts, fmt.Sprintf("其他错误 %d", len(otherFailures)))
	}
	if len(parts) == 0 {
		return "无异常"
	}
	return strings.Join(parts, "、")
}

// ScheduledDramaAuditSummaryForRunID rebuilds the compact audit summary for legacy logs.
func ScheduledDramaAuditSummaryForRunID(runID string) (string, bool) {
	safeRunID := sanitizeReportSuffix(runID)
	if safeRunID == "" || safeRunID == "." || safeRunID == ".." {
		return "", false
	}

	runDir := filepath.Join(testRunStorageRoot(projectRootDir()), safeRunID)
	metadataContent, err := os.ReadFile(filepath.Join(runDir, "metadata.json"))
	if err != nil {
		return "", false
	}
	var archive TestRunArchive
	if json.Unmarshal(metadataContent, &archive) != nil || archive.TestType != "drama" {
		return "", false
	}

	reportHTML := ""
	reportPath := filepath.Join(runDir, "artifacts", "report.html")
	if content, readErr := os.ReadFile(reportPath); readErr == nil {
		reportHTML = string(content)
	}
	failures := loadArchivedDramaFailures(runDir, reportHTML)
	return summarizeScheduledDramaAuditFailures(failures), true
}

func normalizeScheduledTaskAuditEnv(testEnv string) string {
	switch strings.TrimSpace(testEnv) {
	case "正式服务器":
		return "prod"
	case "测试服务器":
		return "test"
	default:
		return strings.TrimSpace(testEnv)
	}
}

func statusToAuditStatus(status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "Passed") {
		return "success"
	}
	return "failed"
}

func analyzeScheduledTaskReportWithAI(task ScheduledTask, status string, result string, duration string, startedAt time.Time, reportDir string) (string, error) {
	reportText, err := readScheduledTaskReportText(startedAt, reportDir)
	if err != nil {
		return "", err
	}
	return analyzeScheduledReportTextWithAI(task, status, result, duration, reportText)
}

func analyzeScheduledReportTextWithAI(task ScheduledTask, status string, result string, duration string, reportText string) (string, error) {
	if strings.TrimSpace(reportText) == "" {
		return "", fmt.Errorf("scheduled task report is empty")
	}
	provider, err := selectScheduledReportProvider()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	httpClient := &http.Client{Timeout: 90 * time.Second}
	systemPrompt := "你是一名资深测试负责人。请根据用户提供的自动化测试报告内容，生成一段适合飞书机器人私聊发送给任务创建人的中文总结。" +
		"必须只输出纯文本，不要使用 Markdown，不要使用星号、反引号、井号、表格或项目符号。" +
		"不要编造报告里没有的数据。重点说明整体结论、风险影响和下一步建议，不要逐条罗列每部剧的异常明细，明细会由系统结构化附加。控制在 300 字以内。"
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
		return "", fmt.Errorf("scheduled report provider returned no choices")
	}

	content := sanitizeFeishuPlainText(resp.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("scheduled report provider returned empty analysis")
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

func selectScheduledReportProvider() (feishumodel.AIProviderConfig, error) {
	config := feishumodel.GlobalAIConfig
	if config == nil {
		config = feishumodel.LoadAIConfig()
	}
	if config == nil {
		return feishumodel.AIProviderConfig{}, fmt.Errorf("AI config is empty")
	}

	if providers := config.EffectiveProvidersByCapability("scheduled_report"); len(providers) > 0 {
		return providers[0], nil
	}

	if providers := config.EffectiveProviders(); len(providers) > 0 {
		return providers[0], nil
	}

	for _, provider := range config.AllEffectiveProviders() {
		search := strings.ToLower(strings.Join([]string{provider.Name, provider.Model, provider.Capability}, " "))
		if strings.Contains(search, "opus") && (strings.Contains(search, "4.7") || strings.Contains(search, "4-7") || strings.Contains(search, "4_7")) {
			return provider, nil
		}
	}
	return feishumodel.AIProviderConfig{}, fmt.Errorf("scheduled_report provider is not configured")
}

func readScheduledTaskReportText(startedAt time.Time, reportDir string) (string, error) {
	rootDir, _ := filepath.Abs("..")
	reportPath := filepath.Join(rootDir, reportDir, "drama_check_report.html")
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
	if retryContent, err := os.ReadFile(filepath.Join(rootDir, reportDir, "drama_retry_result.json")); err == nil {
		text += "\n\nRetry result JSON:\n" + string(retryContent)
	}
	if candidateContent, err := os.ReadFile(filepath.Join(rootDir, reportDir, "drama_retry_candidates.json")); err == nil {
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
			Email:        os.Getenv("DRAMA_PROD_EMAIL"),
			Password:     os.Getenv("DRAMA_PROD_PASSWORD"),
			LoginURL:     valueOrFallback(os.Getenv("DRAMA_PROD_LOGIN_URL"), "https://admin.shortswave.com/api/pwd_login"),
			DramaListURL: valueOrFallback(os.Getenv("DRAMA_PROD_LIST_URL"), "https://admin.shortswave.com/api/management/drama/all_online_ids"),
		}
	}
	if strings.Contains(testEnv, "灰") || strings.EqualFold(testEnv, "gray") {
		return dramaProfile{
			Email:        os.Getenv("DRAMA_GRAY_EMAIL"),
			Password:     os.Getenv("DRAMA_GRAY_PASSWORD"),
			LoginURL:     valueOrFallback(os.Getenv("DRAMA_GRAY_LOGIN_URL"), "http://35.193.183.77:8080/api/pwd_login"),
			DramaListURL: valueOrFallback(os.Getenv("DRAMA_GRAY_LIST_URL"), "http://35.193.183.77:8080/api/management/drama/all_online_ids"),
		}
	}
	return dramaProfile{
		Email:        os.Getenv("DRAMA_TEST_EMAIL"),
		Password:     os.Getenv("DRAMA_TEST_PASSWORD"),
		LoginURL:     valueOrFallback(os.Getenv("DRAMA_TEST_LOGIN_URL"), "http://35.225.224.94:8080/api/pwd_login"),
		DramaListURL: valueOrFallback(os.Getenv("DRAMA_TEST_LIST_URL"), "http://35.225.224.94:8080/api/management/drama/all_online_ids"),
	}
}

func updateScheduledTaskAfterRun(task ScheduledTask, runAt time.Time, result string) {
	tasks, err := loadScheduledTasks()
	if err != nil {
		log.Printf("[ScheduledTask] reload after run failed: %v", err)
		return
	}

	detailedResult := compactScheduledTaskResult(result, scheduledTaskDetailedResultRunes)
	criticalResult := compactScheduledTaskResult(detailedResult, scheduledTaskSafeResultRunes)
	for i := range tasks {
		if tasks[i].ID != task.ID {
			continue
		}
		tasks[i].LastRunAt = runAt.Format(time.RFC3339)
		tasks[i].LastResult = criticalResult
		tasks[i].UpdatedAt = time.Now().Format(time.RFC3339)
		if tasks[i].Status != "paused" {
			if tasks[i].ScheduleType == "Once" {
				if err := deleteScheduledTaskWithRetry(tasks[i].ID); err != nil {
					log.Printf("[ScheduledTask] remove completed one-time task failed: %v", err)
				}
				return
			}
			rescheduleScheduledTaskAfterRun(&tasks, i, time.Now())
		}

		// The compact result is a critical checkpoint that remains compatible
		// with legacy VARCHAR(64) schemas. Persist it before the best-effort
		// detailed result, and update only this row to avoid stale-list races.
		if err := upsertScheduledTaskWithRetry(tasks[i]); err != nil {
			log.Printf("[ScheduledTask] save critical state after run failed: %v", err)
			return
		}
		if detailedResult != criticalResult {
			tasks[i].LastResult = detailedResult
			if err := upsertScheduledTask(tasks[i]); err != nil {
				log.Printf("[ScheduledTask] save detailed result after run failed; compact checkpoint retained: %v", err)
			}
		}
		return
	}
}

func compactScheduledTaskResult(result string, maxRunes int) string {
	result = strings.Join(strings.Fields(strings.TrimSpace(result)), " ")
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(result)
	if len(runes) <= maxRunes {
		return result
	}
	if maxRunes == 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

func retryScheduledTaskPersistence(operation func() error) error {
	var lastErr error
	for _, delay := range scheduledTaskSaveRetryDelays {
		if delay > 0 {
			time.Sleep(delay)
		}
		if err := operation(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func upsertScheduledTaskWithRetry(task ScheduledTask) error {
	return retryScheduledTaskPersistence(func() error {
		return upsertScheduledTask(task)
	})
}

func deleteScheduledTaskWithRetry(taskID string) error {
	return retryScheduledTaskPersistence(func() error {
		return deleteScheduledTaskRecord(taskID)
	})
}

func rescheduleScheduledTaskAfterRun(tasks *[]ScheduledTask, index int, now time.Time) {
	if index < 0 || index >= len(*tasks) {
		return
	}

	task := &(*tasks)[index]
	nextRunAt, err := time.Parse(time.RFC3339, task.NextRunAt)
	if err != nil {
		task.Status = "paused"
		return
	}

	switch task.ScheduleType {
	case "Once":
		*tasks = append((*tasks)[:index], (*tasks)[index+1:]...)
	case "Daily":
		task.Status = "active"
		task.NextRunAt = nextFutureScheduledTime(nextRunAt, 24*time.Hour, now).Format(time.RFC3339)
		task.NextRun = formatScheduleDisplay(task.NextRunAt)
	case "weekly":
		task.Status = "active"
		task.NextRunAt = nextFutureScheduledTime(nextRunAt, 7*24*time.Hour, now).Format(time.RFC3339)
		task.NextRun = formatScheduleDisplay(task.NextRunAt)
	default:
		task.Status = "paused"
	}
}

func nextFutureScheduledTime(base time.Time, interval time.Duration, now time.Time) time.Time {
	next := base.Add(interval)
	for !next.After(now) {
		next = next.Add(interval)
	}
	return next
}

func loadScheduledTasks() ([]ScheduledTask, error) {
	scheduledTaskMu.Lock()
	defer scheduledTaskMu.Unlock()
	return sqlListJSON[ScheduledTask]("scheduled_tasks", "`migrated_at` ASC")
}

func appendScheduledTask(task ScheduledTask) error {
	return upsertScheduledTask(task)
}

func upsertScheduledTask(task ScheduledTask) error {
	scheduledTaskMu.Lock()
	defer scheduledTaskMu.Unlock()
	return sqlUpsertJSON("scheduled_tasks", task)
}

func deleteScheduledTaskRecord(taskID string) error {
	scheduledTaskMu.Lock()
	defer scheduledTaskMu.Unlock()
	return sqlDeleteJSON("scheduled_tasks", taskID)
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

func isScheduledTimeAllowed(nextRunAt time.Time, now time.Time) bool {
	return !nextRunAt.Before(now)
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

func notifyScheduledTaskCreator(username string, text string, card map[string]any) error {
	openID := resolveFeishuOpenID(username)
	if openID == "" {
		openID = resolveFeishuOpenID("minghong")
	}
	if openID == "" {
		return fmt.Errorf("no feishu open_id found for %s or minghong", username)
	}
	if len(card) > 0 {
		if err := sendFeishuOpenIDInteractiveCard(openID, card); err != nil {
			log.Printf("[ScheduledTask] interactive card send failed, falling back to text: %v", err)
		} else {
			return nil
		}
	}
	if err := sendFeishuOpenIDText(openID, text); err != nil {
		return err
	}
	log.Printf("[ScheduledTask] fallback text sent to %s", username)
	return nil
}

func resolveFeishuOpenID(username string) string {
	user, _, err := FindUserByFuzzyName(username)
	if err == nil && user != nil && user.FeishuOpenID != "" {
		return user.FeishuOpenID
	}
	return ""
}

func sendFeishuOpenIDText(openID string, text string) error {
	return sendFeishuOpenIDMessage(openID, "text", map[string]string{"text": text})
}

func sendFeishuOpenIDInteractiveCard(openID string, card map[string]any) error {
	return sendFeishuOpenIDMessage(openID, "interactive", card)
}

func sendFeishuOpenIDMessage(openID string, msgType string, content any) error {
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

	contentBody, err := json.Marshal(content)
	if err != nil {
		return err
	}
	msgPayload, _ := json.Marshal(map[string]string{
		"receive_id": openID,
		"msg_type":   msgType,
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
