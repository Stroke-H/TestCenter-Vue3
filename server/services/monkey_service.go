package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type MonkeyDevice struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Model       string `json:"model,omitempty"`
	Product     string `json:"product,omitempty"`
	TransportID string `json:"transportId,omitempty"`
}

type MonkeyPackage struct {
	Name string `json:"name"`
}

type MonkeyRunRequest struct {
	PackageName          string `json:"packageName"`
	DeviceID             string `json:"deviceId"`
	PrecisionMode        string `json:"precisionMode"`
	DurationSec          int    `json:"durationSec"`
	EventTotal           int    `json:"eventTotal"`
	ThrottleMs           int    `json:"throttleMs"`
	Seed                 string `json:"seed"`
	Strategy             string `json:"strategy"`
	DenseSamplingEnabled bool   `json:"denseSamplingEnabled"`
	ContinueAfterCrash   bool   `json:"continueAfterCrash"`
	Author               string `json:"author"`
	Environment          string `json:"environment"`
}

type MonkeyRunSummary struct {
	RunID                     string `json:"runId"`
	Status                    string `json:"status"`
	PackageName               string `json:"packageName"`
	DeviceID                  string `json:"deviceId"`
	PrecisionMode             string `json:"precisionMode"`
	BaseScreenshotIntervalSec int    `json:"baseScreenshotIntervalSec"`
	DenseSamplingEnabled      bool   `json:"denseSamplingEnabled"`
	DenseSamplingTriggered    bool   `json:"denseSamplingTriggered"`
	BatchCount                int    `json:"batchCount"`
	StartedAt                 string `json:"startedAt"`
	FinishedAt                string `json:"finishedAt,omitempty"`
	Duration                  string `json:"duration"`
	ScreenshotCount           int    `json:"screenshotCount"`
	NormalCount               int    `json:"normalCount"`
	WarningCount              int    `json:"warningCount"`
	CriticalCount             int    `json:"criticalCount"`
	UnknownCount              int    `json:"unknownCount"`
	FirstCriticalNodeID       string `json:"firstCriticalNodeId,omitempty"`
	AppCrashDetected          bool   `json:"appCrashDetected"`
	TerminationReason         string `json:"terminationReason,omitempty"`
	IgnoredSystemLogCount     int    `json:"ignoredSystemLogCount"`
	ForegroundGuardEnabled    bool   `json:"foregroundGuardEnabled"`
	ForegroundRestartCount    int    `json:"foregroundRestartCount"`
	SystemOverlayDismissCount int    `json:"systemOverlayDismissCount"`
	AdDismissCount            int    `json:"adDismissCount"`
	AdRestartCount            int    `json:"adRestartCount"`
	LastForeignPackage        string `json:"lastForeignPackage,omitempty"`
	Error                     string `json:"error,omitempty"`
}

type MonkeyRiskEvidence struct {
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp,omitempty"`
}

type MonkeyGraphEvent struct {
	ID                 string               `json:"id"`
	RunID              string               `json:"runId"`
	Title              string               `json:"title"`
	Event              string               `json:"event"`
	EventIndex         int                  `json:"eventIndex"`
	Timestamp          string               `json:"timestamp"`
	Activity           string               `json:"activity"`
	Risk               string               `json:"risk"`
	Summary            string               `json:"summary"`
	ImageURL           string               `json:"imageUrl,omitempty"`
	ImageFile          string               `json:"imageFile,omitempty"`
	ImageStatus        string               `json:"imageStatus"`
	ImageDeletedReason string               `json:"imageDeletedReason,omitempty"`
	DenseSampling      bool                 `json:"denseSampling"`
	Evidence           []MonkeyRiskEvidence `json:"evidence"`
}

type MonkeyStreamEvent struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Data      any    `json:"data"`
	Timestamp string `json:"timestamp"`
}

type MonkeyForegroundGuardEvent struct {
	Action            string `json:"action"`
	ForegroundPackage string `json:"foregroundPackage,omitempty"`
	TargetPackage     string `json:"targetPackage"`
	RestartCount      int    `json:"restartCount"`
	Message           string `json:"message"`
	Timestamp         string `json:"timestamp"`
}

type monkeyRunState struct {
	mu                sync.Mutex
	cancel            context.CancelFunc
	stopOnce          sync.Once
	stopped           chan struct{}
	adbPath           string
	deviceID          string
	summary           MonkeyRunSummary
	streamSubscribers map[chan MonkeyStreamEvent]struct{}
	streamHistory     []MonkeyStreamEvent
	nextStreamEventID int64
}

var (
	monkeyRunsMu       sync.Mutex
	monkeyRuns         = map[string]*monkeyRunState{}
	monkeyPackagesMu   sync.Mutex
	monkeyPackagesTTL  = 5 * time.Minute
	monkeyPackages     = map[string]monkeyPackageCache{}
	packageNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)
	logcatPIDPattern   = regexp.MustCompile(`\(\s*(\d+)\)`)
	activityPattern    = regexp.MustCompile(`([A-Za-z0-9_.$]+/[A-Za-z0-9_.$]+)`)
	nodeBoundsPattern  = regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)
)

type monkeyUIHierarchy struct {
	Nodes []monkeyUINode `xml:"node"`
}

type monkeyUINode struct {
	Text        string         `xml:"text,attr"`
	ResourceID  string         `xml:"resource-id,attr"`
	Class       string         `xml:"class,attr"`
	ContentDesc string         `xml:"content-desc,attr"`
	Bounds      string         `xml:"bounds,attr"`
	Clickable   string         `xml:"clickable,attr"`
	Nodes       []monkeyUINode `xml:"node"`
}

type monkeyAdInspection struct {
	Detected       bool
	Signature      string
	CloseTargetX   int
	CloseTargetY   int
	HasCloseTarget bool
}

type monkeyPackageCache struct {
	Packages  []MonkeyPackage
	ExpiresAt time.Time
}

func InitMonkeyRetentionService() {
	go func() {
		applyMonkeyRetentionPolicy()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			applyMonkeyRetentionPolicy()
		}
	}()
}

func ListMonkeyDevicesHandler(c *gin.Context) {
	if _, ok := requireMonkeyPermission(c, "monkey.run.view"); !ok {
		return
	}
	adbPath, err := resolveADBPath()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"adbAvailable": false,
			"adbPath":      "",
			"devices":      []MonkeyDevice{},
			"message":      err.Error(),
		})
		return
	}
	devices, err := listMonkeyDevices(c.Request.Context(), adbPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "adbAvailable": true, "adbPath": adbPath})
		return
	}
	c.JSON(http.StatusOK, gin.H{"adbAvailable": true, "adbPath": adbPath, "devices": devices})
}

func ListMonkeyPackagesHandler(c *gin.Context) {
	if _, ok := requireMonkeyPermission(c, "monkey.run.view"); !ok {
		return
	}
	deviceID := strings.TrimSpace(c.Param("deviceId"))
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId is required"})
		return
	}
	adbPath, err := resolveADBPath()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"adbAvailable": false,
			"adbPath":      "",
			"packages":     []MonkeyPackage{},
			"message":      err.Error(),
		})
		return
	}
	packages, cached, err := listThirdPartyPackages(c.Request.Context(), adbPath, deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "adbAvailable": true, "adbPath": adbPath})
		return
	}
	c.JSON(http.StatusOK, gin.H{"adbAvailable": true, "adbPath": adbPath, "packages": packages, "cached": cached})
}

func StartMonkeyRunHandler(c *gin.Context) {
	user, ok := requireMonkeyPermission(c, "monkey.run.start")
	if !ok {
		return
	}
	var req MonkeyRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Author) == "" {
		req.Author = user.Username
	}
	run, err := startMonkeyRun(context.Background(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run.summary)
}

func ListMonkeyRunsHandler(c *gin.Context) {
	if _, ok := requireMonkeyPermission(c, "monkey.run.view"); !ok {
		return
	}
	runs, err := listArchivedMonkeyRuns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, runs)
}

func GetMonkeyRunHandler(c *gin.Context) {
	if _, ok := requireMonkeyPermission(c, "monkey.run.view"); !ok {
		return
	}
	runID := c.Param("runId")
	summary, err := loadMonkeySummary(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monkey run not found"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func GetMonkeyRunEventsHandler(c *gin.Context) {
	if _, ok := requireMonkeyPermission(c, "monkey.run.view"); !ok {
		return
	}
	runID := c.Param("runId")
	events, err := loadMonkeyEvents(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monkey events not found"})
		return
	}
	c.JSON(http.StatusOK, events)
}

func GetMonkeyRunLogsHandler(c *gin.Context) {
	if _, ok := requireMonkeyPermission(c, "monkey.run.view"); !ok {
		return
	}
	runID := c.Param("runId")
	kind := c.DefaultQuery("kind", "monkey")
	if kind != "logcat" {
		kind = "monkey"
	}
	path := filepath.Join(monkeyRunDir(runID), kind+".log")
	content, err := os.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "log not found"})
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
}

func SubscribeMonkeyRunHandler(c *gin.Context) {
	if _, ok := requireMonkeyPermission(c, "monkey.run.view"); !ok {
		return
	}
	runID := c.Param("runId")
	summary, err := loadMonkeySummary(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Monkey run not found"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	lastEventID := parseMonkeyStreamEventID(c.GetHeader("Last-Event-ID"))
	monkeyRunsMu.Lock()
	run := monkeyRuns[runID]
	monkeyRunsMu.Unlock()
	if run == nil {
		writeMonkeySSE(c.Writer, MonkeyStreamEvent{
			Type: "summary", Data: summary, Timestamp: time.Now().Format(time.RFC3339),
		})
		writeMonkeySSE(c.Writer, MonkeyStreamEvent{
			Type: "complete", Data: summary, Timestamp: time.Now().Format(time.RFC3339),
		})
		c.Writer.Flush()
		return
	}

	subscriber, replay := subscribeMonkeyStream(run, lastEventID)
	defer unsubscribeMonkeyStream(run, subscriber)
	writeMonkeySSE(c.Writer, MonkeyStreamEvent{
		Type: "summary", Data: summary, Timestamp: time.Now().Format(time.RFC3339),
	})
	for _, event := range replay {
		writeMonkeySSE(c.Writer, event)
	}
	c.Writer.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case event, ok := <-subscriber:
			if !ok {
				return
			}
			writeMonkeySSE(c.Writer, event)
			c.Writer.Flush()
		case <-heartbeat.C:
			_, _ = fmt.Fprint(c.Writer, ": heartbeat\n\n")
			c.Writer.Flush()
		}
	}
}

func GetMonkeyRunScreenshotHandler(c *gin.Context) {
	if _, ok := requireMonkeyPermission(c, "monkey.run.view"); !ok {
		return
	}
	runID := c.Param("runId")
	fileName := filepath.Base(c.Param("fileName"))
	path := filepath.Join(monkeyRunDir(runID), "screenshots", fileName)
	c.File(path)
}

func StopMonkeyRunHandler(c *gin.Context) {
	if _, ok := requireMonkeyPermission(c, "monkey.run.stop"); !ok {
		return
	}
	runID := c.Param("runId")
	monkeyRunsMu.Lock()
	run := monkeyRuns[runID]
	monkeyRunsMu.Unlock()
	if run == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "running Monkey task not found"})
		return
	}
	requestMonkeyStop(run)
	select {
	case <-run.stopped:
		run.mu.Lock()
		summary := run.summary
		run.mu.Unlock()
		c.JSON(http.StatusOK, summary)
	case <-time.After(15 * time.Second):
		c.JSON(http.StatusAccepted, gin.H{"message": "stop requested, report finalization is still running"})
	}
}

func requestMonkeyStop(run *monkeyRunState) {
	run.stopOnce.Do(func() {
		run.cancel()
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = stopADBMonkeyProcess(stopCtx, run.adbPath, run.deviceID)
	})
}

func stopADBMonkeyProcess(ctx context.Context, adbPath string, deviceID string) error {
	var lastErr error
	for _, args := range [][]string{
		{"-s", deviceID, "shell", "pkill", "-f", "com.android.commands.monkey"},
		{"-s", deviceID, "shell", "killall", "com.android.commands.monkey"},
	} {
		output, err := exec.CommandContext(ctx, adbPath, args...).CombinedOutput()
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return lastErr
}

func subscribeMonkeyStream(run *monkeyRunState, lastEventID int64) (chan MonkeyStreamEvent, []MonkeyStreamEvent) {
	subscriber := make(chan MonkeyStreamEvent, 64)
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.streamSubscribers == nil {
		run.streamSubscribers = map[chan MonkeyStreamEvent]struct{}{}
	}
	run.streamSubscribers[subscriber] = struct{}{}
	replay := []MonkeyStreamEvent{}
	for _, event := range run.streamHistory {
		if event.ID > lastEventID {
			replay = append(replay, event)
		}
	}
	return subscriber, replay
}

func unsubscribeMonkeyStream(run *monkeyRunState, subscriber chan MonkeyStreamEvent) {
	run.mu.Lock()
	defer run.mu.Unlock()
	delete(run.streamSubscribers, subscriber)
}

func publishMonkeyStreamEvent(run *monkeyRunState, eventType string, data any) {
	run.mu.Lock()
	defer run.mu.Unlock()
	run.nextStreamEventID++
	event := MonkeyStreamEvent{
		ID: run.nextStreamEventID, Type: eventType, Data: data, Timestamp: time.Now().Format(time.RFC3339),
	}
	run.streamHistory = append(run.streamHistory, event)
	if len(run.streamHistory) > 256 {
		run.streamHistory = append([]MonkeyStreamEvent(nil), run.streamHistory[len(run.streamHistory)-256:]...)
	}
	for subscriber := range run.streamSubscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func closeMonkeyStreamSubscribers(run *monkeyRunState) {
	run.mu.Lock()
	defer run.mu.Unlock()
	for subscriber := range run.streamSubscribers {
		close(subscriber)
		delete(run.streamSubscribers, subscriber)
	}
}

func parseMonkeyStreamEventID(value string) int64 {
	id, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return id
}

func writeMonkeySSE(writer io.Writer, event MonkeyStreamEvent) {
	if event.ID > 0 {
		_, _ = fmt.Fprintf(writer, "id: %d\n", event.ID)
	}
	if event.Type != "" {
		_, _ = fmt.Fprintf(writer, "event: %s\n", event.Type)
	}
	payload, err := json.Marshal(event.Data)
	if err != nil {
		payload = []byte(`{"error":"failed to encode SSE payload"}`)
	}
	_, _ = fmt.Fprintf(writer, "data: %s\n\n", payload)
}

func requireMonkeyPermission(c *gin.Context, key string) (*User, bool) {
	user, err := CurrentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return nil, false
	}
	record, err := getPermissionRecordByUserID(user.ID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		c.Abort()
		return nil, false
	}
	if record == nil {
		record = &UserPermissionRecord{
			UserID:      user.ID,
			Username:    user.Username,
			Permissions: DefaultDashboardPermissions(user.Username),
		}
	}
	if record.Permissions[key] || isPermissionAdminUsername(user.Username) {
		return user, true
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
	c.Abort()
	return nil, false
}

func resolveADBPath() (string, error) {
	candidates := []string{}
	if value := strings.TrimSpace(os.Getenv("ANDROID_ADB_PATH")); value != "" {
		candidates = append(candidates, value)
	}
	if path, err := exec.LookPath("adb"); err == nil {
		candidates = append(candidates, path)
	}
	home, _ := os.UserHomeDir()
	for _, base := range []string{os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT"), filepath.Join(home, "Library", "Android", "sdk")} {
		if strings.TrimSpace(base) != "" {
			candidates = append(candidates, filepath.Join(base, "platform-tools", "adb"))
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到 adb。请确认 Android Studio 已安装 SDK Platform-Tools，或设置 ANDROID_ADB_PATH/ANDROID_HOME")
}

func listMonkeyDevices(ctx context.Context, adbPath string) ([]MonkeyDevice, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	output, err := exec.CommandContext(cmdCtx, adbPath, "devices", "-l").CombinedOutput()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("adb devices 超时，请检查设备连接、USB 调试授权或 ADB server 状态")
		}
		return nil, fmt.Errorf("adb devices failed: %s", strings.TrimSpace(string(output)))
	}
	lines := strings.Split(string(output), "\n")
	devices := []MonkeyDevice{}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		device := MonkeyDevice{ID: fields[0], Status: fields[1]}
		for _, field := range fields[2:] {
			if key, value, ok := strings.Cut(field, ":"); ok {
				switch key {
				case "model":
					device.Model = value
				case "product":
					device.Product = value
				case "transport_id":
					device.TransportID = value
				}
			}
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func listThirdPartyPackages(ctx context.Context, adbPath string, deviceID string) ([]MonkeyPackage, bool, error) {
	monkeyPackagesMu.Lock()
	if cached, ok := monkeyPackages[deviceID]; ok && time.Now().Before(cached.ExpiresAt) {
		packages := append([]MonkeyPackage(nil), cached.Packages...)
		monkeyPackagesMu.Unlock()
		return packages, true, nil
	}
	monkeyPackagesMu.Unlock()

	cmdCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	output, err := exec.CommandContext(cmdCtx, adbPath, "-s", deviceID, "shell", "pm", "list", "packages", "-3").CombinedOutput()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return nil, false, fmt.Errorf("读取三方 App 包名超时，请确认设备未锁屏、USB 调试连接稳定，或稍后使用缓存结果")
		}
		return nil, false, fmt.Errorf("读取三方 App 包名失败: %s", strings.TrimSpace(string(output)))
	}
	packages := []MonkeyPackage{}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(output), "\n") {
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "package:"))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		packages = append(packages, MonkeyPackage{Name: name})
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})
	monkeyPackagesMu.Lock()
	monkeyPackages[deviceID] = monkeyPackageCache{
		Packages:  append([]MonkeyPackage(nil), packages...),
		ExpiresAt: time.Now().Add(monkeyPackagesTTL),
	}
	monkeyPackagesMu.Unlock()
	return packages, false, nil
}

func startMonkeyRun(parent context.Context, req MonkeyRunRequest) (*monkeyRunState, error) {
	adbPath, err := resolveADBPath()
	if err != nil {
		return nil, err
	}
	req.PackageName = strings.TrimSpace(req.PackageName)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if !packageNamePattern.MatchString(req.PackageName) {
		return nil, fmt.Errorf("请输入合法 Android 包名")
	}
	if req.DeviceID == "" {
		return nil, fmt.Errorf("请选择 Android 设备")
	}
	if req.DurationSec <= 0 {
		req.DurationSec = 3600
	}
	if req.DurationSec > 6*3600 {
		req.DurationSec = 6 * 3600
	}
	if req.ThrottleMs < 0 {
		req.ThrottleMs = 300
	}
	if req.ThrottleMs == 0 {
		req.ThrottleMs = 300
	}
	if req.EventTotal <= 0 {
		req.EventTotal = maxInt(100, req.DurationSec*1000/req.ThrottleMs)
	}
	durationEventTotal := maxInt(1, req.DurationSec*1000/req.ThrottleMs)
	if req.EventTotal < durationEventTotal {
		req.EventTotal = durationEventTotal
	}
	baseInterval := 30
	if req.PrecisionMode == "high" {
		baseInterval = 10
	} else {
		req.PrecisionMode = "low"
	}
	if req.Seed == "" {
		req.Seed = time.Now().Format("20060102150405")
	}
	req.Seed = normalizeMonkeySeed(req.Seed)

	runID := "MONKEY-" + time.Now().Format("20060102150405")
	ctx, cancel := context.WithTimeout(parent, time.Duration(req.DurationSec+30)*time.Second)
	run := &monkeyRunState{
		cancel:   cancel,
		stopped:  make(chan struct{}),
		adbPath:  adbPath,
		deviceID: req.DeviceID,
		summary: MonkeyRunSummary{
			RunID: runID, Status: "running", PackageName: req.PackageName, DeviceID: req.DeviceID,
			PrecisionMode: req.PrecisionMode, BaseScreenshotIntervalSec: baseInterval,
			DenseSamplingEnabled: req.DenseSamplingEnabled, ForegroundGuardEnabled: true,
			StartedAt: time.Now().Format(time.RFC3339),
		},
		streamSubscribers: map[chan MonkeyStreamEvent]struct{}{},
	}

	if err := os.MkdirAll(filepath.Join(monkeyRunDir(runID), "screenshots"), 0755); err != nil {
		cancel()
		return nil, err
	}
	_ = writeJSON(filepath.Join(monkeyRunDir(runID), "metadata.json"), req)
	_ = writeJSON(filepath.Join(monkeyRunDir(runID), "summary.json"), run.summary)

	monkeyRunsMu.Lock()
	monkeyRuns[runID] = run
	monkeyRunsMu.Unlock()

	go runMonkeyPipeline(ctx, adbPath, req, run)
	return run, nil
}

func runMonkeyPipeline(ctx context.Context, adbPath string, req MonkeyRunRequest, run *monkeyRunState) {
	startedAt, _ := time.Parse(time.RFC3339, run.summary.StartedAt)
	runID := run.summary.RunID
	runDir := monkeyRunDir(runID)
	monkeyLog, _ := os.Create(filepath.Join(runDir, "monkey.log"))
	logcatLog, _ := os.Create(filepath.Join(runDir, "logcat.log"))
	defer func() {
		if monkeyLog != nil {
			_ = monkeyLog.Close()
		}
		if logcatLog != nil {
			_ = logcatLog.Close()
		}
		monkeyRunsMu.Lock()
		delete(monkeyRuns, runID)
		monkeyRunsMu.Unlock()
		closeMonkeyStreamSubscribers(run)
		close(run.stopped)
	}()

	var evidenceMu sync.Mutex
	pendingEvidence := []MonkeyRiskEvidence{}
	denseUntil := time.Time{}
	denseLevel := ""
	criticalEvidenceDetected := false
	recordEvidence := func(e MonkeyRiskEvidence) {
		e.Timestamp = time.Now().Format(time.RFC3339)
		evidenceMu.Lock()
		pendingEvidence = append(pendingEvidence, e)
		if req.DenseSamplingEnabled {
			if e.Level == "critical" {
				criticalEvidenceDetected = true
				denseLevel = "critical"
				denseUntil = time.Now().Add(120 * time.Second)
			} else if e.Level == "warning" && denseLevel != "critical" {
				denseLevel = "warning"
				denseUntil = time.Now().Add(60 * time.Second)
			}
		}
		evidenceMu.Unlock()
		publishMonkeyStreamEvent(run, "risk-evidence", e)
	}
	consumeEvidence := func() ([]MonkeyRiskEvidence, bool) {
		evidenceMu.Lock()
		defer evidenceMu.Unlock()
		items := append([]MonkeyRiskEvidence(nil), pendingEvidence...)
		pendingEvidence = nil
		return items, time.Now().Before(denseUntil)
	}
	currentDenseInterval := func() time.Duration {
		evidenceMu.Lock()
		defer evidenceMu.Unlock()
		if time.Now().After(denseUntil) {
			denseLevel = ""
			return time.Duration(run.summary.BaseScreenshotIntervalSec) * time.Second
		}
		run.mu.Lock()
		run.summary.DenseSamplingTriggered = true
		run.mu.Unlock()
		if denseLevel == "critical" {
			return 2 * time.Second
		}
		return 5 * time.Second
	}

	recordIgnoredSystemLog := func() {
		run.mu.Lock()
		run.summary.IgnoredSystemLogCount++
		run.mu.Unlock()
	}

	_ = exec.CommandContext(ctx, adbPath, "-s", req.DeviceID, "logcat", "-c").Run()
	logcatCmd := exec.CommandContext(ctx, adbPath, "-s", req.DeviceID, "logcat", "-v", "time")
	if logcatPipe, err := logcatCmd.StdoutPipe(); err == nil {
		_ = logcatCmd.Start()
		go scanMonkeyOutput(logcatPipe, logcatLog, "logcat", req.PackageName, recordEvidence, recordIgnoredSystemLog)
		defer func() {
			_ = logcatCmd.Process.Kill()
			_ = logcatCmd.Wait()
		}()
	}

	var monkeyCmd *exec.Cmd
	var done chan error
	startNextBatch := func() error {
		run.mu.Lock()
		nextBatch := run.summary.BatchCount + 1
		run.mu.Unlock()
		batchReq := monkeyBatchRequest(req, nextBatch)
		command := exec.CommandContext(ctx, adbPath, buildMonkeyArgs(batchReq)...)
		stdout, _ := command.StdoutPipe()
		stderr, _ := command.StderrPipe()
		if err := command.Start(); err != nil {
			return err
		}
		go scanMonkeyOutput(stdout, monkeyLog, "monkey", req.PackageName, recordEvidence, recordIgnoredSystemLog)
		go scanMonkeyOutput(stderr, monkeyLog, "monkey", req.PackageName, recordEvidence, recordIgnoredSystemLog)
		monkeyCmd = command
		done = make(chan error, 1)
		go func() { done <- command.Wait() }()
		run.mu.Lock()
		run.summary.BatchCount = nextBatch
		_ = writeJSON(filepath.Join(runDir, "summary.json"), run.summary)
		summary := run.summary
		run.mu.Unlock()
		if monkeyLog != nil {
			_, _ = fmt.Fprintf(monkeyLog, "[BATCH] started #%d seed=%s targetDurationSec=%d\n", nextBatch, batchReq.Seed, req.DurationSec)
		}
		publishMonkeyStreamEvent(run, "summary", summary)
		return nil
	}
	if err := startNextBatch(); err != nil {
		updateMonkeySummaryFromEvents(run, nil, fmt.Sprintf("启动 Monkey 失败: %v", err), true)
		saveMonkeyExecutionReport(req, monkeyRunSummarySnapshot(run))
		return
	}
	guardCtx, guardCancel := context.WithCancel(ctx)
	defer guardCancel()
	go monitorMonkeyForegroundApp(guardCtx, adbPath, req, run, recordEvidence)

	events := []MonkeyGraphEvent{}
	durationTimer := time.NewTimer(time.Until(startedAt.Add(time.Duration(req.DurationSec) * time.Second)))
	defer durationTimer.Stop()

	capture := func(captureCtx context.Context) {
		evidence, dense := consumeEvidence()
		event, err := captureMonkeyScreenshot(captureCtx, adbPath, req, runID, len(events)+1, startedAt, evidence, dense)
		if err != nil {
			if monkeyLog != nil {
				_, _ = fmt.Fprintf(monkeyLog, "[SCREENSHOT][ERROR] %s\n", err.Error())
			}
			run.mu.Lock()
			run.summary.Error = err.Error()
			_ = writeJSON(filepath.Join(runDir, "summary.json"), run.summary)
			run.mu.Unlock()
			recordEvidence(MonkeyRiskEvidence{Level: "warning", Source: "screenshot", Message: err.Error()})
			return
		}
		events = append(events, event)
		_ = writeJSON(filepath.Join(runDir, "events.json"), events)
		publishMonkeyStreamEvent(run, "graph-event", event)
		updateMonkeySummaryFromEvents(run, events, "", false)
	}
	capture(ctx)
	for {
		select {
		case err := <-done:
			evidenceMu.Lock()
			canContinueAfterEvidence := req.ContinueAfterCrash || !criticalEvidenceDetected
			evidenceMu.Unlock()
			if shouldContinueMonkeyBatch(err, ctx.Err(), time.Now(), startedAt.Add(time.Duration(req.DurationSec)*time.Second), canContinueAfterEvidence) {
				if monkeyLog != nil {
					_, _ = fmt.Fprintf(monkeyLog, "[BATCH] completed early; continuing until configured duration=%ds\n", req.DurationSec)
				}
				if nextErr := startNextBatch(); nextErr == nil {
					continue
				} else {
					err = fmt.Errorf("续跑 Monkey 批次失败: %w", nextErr)
					recordEvidence(MonkeyRiskEvidence{Level: "critical", Source: "monkey", Message: err.Error()})
				}
			}
			if err != nil && ctx.Err() == nil {
				recordEvidence(MonkeyRiskEvidence{Level: "critical", Source: "monkey", Message: err.Error()})
			}
			capture(finalCaptureContext(ctx))
			finalErr := ""
			if ctx.Err() != nil {
				finalErr = ctx.Err().Error()
			} else if err != nil {
				finalErr = err.Error()
			}
			updateMonkeySummaryFromEvents(run, events, finalErr, true)
			saveMonkeyExecutionReport(req, monkeyRunSummarySnapshot(run))
			return
		case <-durationTimer.C:
			if monkeyCmd != nil && monkeyCmd.Process != nil {
				_ = monkeyCmd.Process.Kill()
			}
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = stopADBMonkeyProcess(stopCtx, adbPath, req.DeviceID)
			cancel()
			capture(context.Background())
			updateMonkeySummaryFromEvents(run, events, "", true)
			saveMonkeyExecutionReport(req, monkeyRunSummarySnapshot(run))
			return
		case <-ctx.Done():
			capture(finalCaptureContext(ctx))
			updateMonkeySummaryFromEvents(run, events, ctx.Err().Error(), true)
			saveMonkeyExecutionReport(req, monkeyRunSummarySnapshot(run))
			return
		case <-time.After(currentDenseInterval()):
			capture(ctx)
		}
	}
}

func shouldContinueMonkeyBatch(batchErr error, runErr error, now time.Time, deadline time.Time, canContinueAfterEvidence bool) bool {
	return batchErr == nil && runErr == nil && now.Before(deadline) && canContinueAfterEvidence
}

func monkeyBatchRequest(req MonkeyRunRequest, batch int) MonkeyRunRequest {
	if batch <= 1 {
		return req
	}
	seed, err := strconv.ParseInt(req.Seed, 10, 64)
	if err != nil {
		return req
	}
	req.Seed = strconv.FormatInt(seed+int64(batch-1), 10)
	return req
}

func finalCaptureContext(runCtx context.Context) context.Context {
	if runCtx.Err() == nil {
		return runCtx
	}
	return context.Background()
}

func monkeyRunSummarySnapshot(run *monkeyRunState) MonkeyRunSummary {
	run.mu.Lock()
	defer run.mu.Unlock()
	return run.summary
}

func buildMonkeyArgs(req MonkeyRunRequest) []string {
	touch, motion, nav, majorNav, appSwitch := "60", "30", "4", "3", "3"
	switch req.Strategy {
	case "tap-heavy":
		touch, motion, nav, majorNav, appSwitch = "80", "15", "2", "1", "2"
	case "scroll-heavy":
		touch, motion, nav, majorNav, appSwitch = "45", "50", "2", "1", "2"
	case "navigation-heavy":
		touch, motion, nav, majorNav, appSwitch = "45", "25", "15", "10", "5"
	}
	args := []string{
		"-s", req.DeviceID, "shell", "monkey",
		"-p", req.PackageName,
		"--pct-touch", touch,
		"--pct-motion", motion,
		"--pct-trackball", "0",
		"--pct-syskeys", "0",
		"--pct-nav", nav,
		"--pct-majornav", majorNav,
		"--pct-appswitch", appSwitch,
		"--pct-anyevent", "0",
		"--throttle", fmt.Sprintf("%d", req.ThrottleMs),
	}
	if req.ContinueAfterCrash {
		args = append(args, "--ignore-crashes")
	}
	return append(args,
		"-s", req.Seed,
		"-v", "-v", "-v", fmt.Sprintf("%d", req.EventTotal),
	)
}

func normalizeMonkeySeed(seed string) string {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		return time.Now().Format("20060102150405")
	}
	for _, char := range seed {
		if char < '0' || char > '9' {
			hasher := fnv.New32a()
			_, _ = hasher.Write([]byte(seed))
			return fmt.Sprintf("%d", hasher.Sum32())
		}
	}
	return seed
}

func captureMonkeyScreenshot(ctx context.Context, adbPath string, req MonkeyRunRequest, runID string, index int, startedAt time.Time, evidence []MonkeyRiskEvidence, dense bool) (MonkeyGraphEvent, error) {
	fileName := fmt.Sprintf("screen-%04d.png", index)
	filePath := filepath.Join(monkeyRunDir(runID), "screenshots", fileName)
	captureCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(captureCtx, adbPath, "-s", req.DeviceID, "exec-out", "screencap", "-p")
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		if captureCtx.Err() == context.DeadlineExceeded {
			return MonkeyGraphEvent{}, fmt.Errorf("截图采集超时，请检查设备是否锁屏或 USB 调试是否稳定")
		}
		return MonkeyGraphEvent{}, fmt.Errorf("截图采集失败: %v", err)
	}
	if err := os.WriteFile(filePath, output, 0644); err != nil {
		return MonkeyGraphEvent{}, err
	}
	activity := resolveCurrentActivity(ctx, adbPath, req.DeviceID)
	risk := resolveRisk(evidence)
	elapsed := int(time.Since(startedAt).Seconds())
	return MonkeyGraphEvent{
		ID: indexedScreenID(index), RunID: runID, Title: fmt.Sprintf("采样截图 %d", index),
		Event: fmt.Sprintf("screencap #%d", index), EventIndex: maxInt(0, elapsed*1000/req.ThrottleMs),
		Timestamp: time.Now().Format(time.RFC3339), Activity: activity, Risk: risk,
		Summary: buildEvidenceSummary(risk, evidence), ImageURL: PlatformBackendURL("/api/monkey/runs/" + runID + "/screenshots/" + fileName),
		ImageFile: fileName, ImageStatus: "original", DenseSampling: dense, Evidence: evidence,
	}, nil
}

func scanMonkeyOutput(reader io.Reader, writer io.Writer, source string, packageName string, record func(MonkeyRiskEvidence), recordIgnoredSystemLog func()) {
	if reader == nil {
		return
	}
	classifier := monkeyLogClassifier{
		targetPackage: strings.ToLower(strings.TrimSpace(packageName)),
	}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if writer != nil {
			_, _ = fmt.Fprintln(writer, line)
		}
		if evidence, ok, ignored := classifier.classify(line, source); ok {
			record(evidence)
		} else if ignored && recordIgnoredSystemLog != nil {
			recordIgnoredSystemLog()
		}
	}
}

type monkeyLogClassifier struct {
	targetPackage string
	targetPID     string
}

func (classifier *monkeyLogClassifier) classify(line string, source string) (MonkeyRiskEvidence, bool, bool) {
	lower := strings.ToLower(line)
	criticalTokens := []string{"fatal exception", "crash:", "anr in", "application not responding", "force finishing activity", "has died", "outofmemoryerror", "native crash", "sigsegv", "sigabrt", "monkey aborted"}
	warningTokens := []string{"exception", "securityexception", "activitynotfoundexception", "illegalstateexception", "illegalargumentexception", "permission denied", "unable to start activity", "window leaked", "skipped frames", "slow operation", "failed to inflate", "rejecting start of intent", "activity not started"}
	isTargetAppLine := true
	if source == "logcat" {
		isTargetAppLine = classifier.isTargetAppLine(line, lower)
	}

	level := ""
	for _, token := range criticalTokens {
		if strings.Contains(lower, token) {
			level = "critical"
			break
		}
	}
	if level == "" {
		for _, token := range warningTokens {
			if strings.Contains(lower, token) {
				level = "warning"
				break
			}
		}
	}
	if level == "" {
		return MonkeyRiskEvidence{}, false, false
	}

	if source == "logcat" && !isTargetAppLine {
		return MonkeyRiskEvidence{}, false, true
	}

	return MonkeyRiskEvidence{Level: level, Source: source, Message: strings.TrimSpace(line)}, true, false
}

func (classifier *monkeyLogClassifier) isTargetAppLine(line string, lower string) bool {
	linePID := extractLogcatPID(line)
	if classifier.targetPackage != "" && strings.Contains(lower, classifier.targetPackage) {
		if linePID != "" {
			classifier.targetPID = linePID
		}
		return true
	}
	if linePID != "" && classifier.targetPID != "" && linePID == classifier.targetPID {
		return true
	}
	return false
}

func extractLogcatPID(line string) string {
	match := logcatPIDPattern.FindStringSubmatch(line)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func resolveRisk(evidence []MonkeyRiskEvidence) string {
	risk := "normal"
	for _, item := range evidence {
		if item.Level == "critical" {
			return "critical"
		}
		if item.Level == "warning" {
			risk = "warning"
		}
	}
	return risk
}

func buildEvidenceSummary(risk string, evidence []MonkeyRiskEvidence) string {
	if len(evidence) == 0 {
		return "采样窗口未发现异常"
	}
	return summarizeMonkeyEvidenceMessage(evidence[0].Message)
}

func summarizeMonkeyEvidenceMessage(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "aconfigstoragereadexception") && strings.Contains(lower, "fail to mmap storage"):
		return "Android 系统配置映射读取失败，可能是设备系统组件的临时存储异常。建议观察目标 App 功能是否受到影响。"
	case strings.Contains(lower, "error_package_not_found") && strings.Contains(lower, "android.xr"):
		return "设备缺少 Android XR 系统组件。通常与当前目标 App 无关，可结合实际功能表现判断。"
	case strings.Contains(lower, "firebasecrashlytics") && strings.Contains(lower, "filenotfoundexception"):
		return "Crashlytics 临时日志文件不存在，可能与日志组件初始化或清理时序有关。建议结合后续异常节点确认影响。"
	case strings.Contains(lower, "permission denied"):
		return "检测到权限拒绝。建议检查目标 App 是否缺少运行所需权限。"
	case strings.Contains(lower, "activitynotfoundexception") || strings.Contains(lower, "unable to start activity"):
		return "页面拉起失败。建议检查目标页面是否存在、是否已注册，以及当前跳转参数是否正确。"
	case strings.Contains(lower, "skipped frames") || strings.Contains(lower, "slow operation"):
		return "检测到界面卡顿信号。建议结合对应截图和性能日志继续排查。"
	case strings.Contains(lower, "fatal exception") || strings.Contains(lower, "crash:"):
		return "检测到目标 App 崩溃信号。请优先查看 Detail 中的原始日志。"
	case strings.Contains(lower, "anr in") || strings.Contains(lower, "application not responding"):
		return "检测到目标 App 无响应信号。请优先查看 Detail 中的原始日志。"
	default:
		return "采样窗口检测到异常日志。请悬浮 Detail 查看原始日志并结合截图判断影响。"
	}
}

func resolveCurrentActivity(ctx context.Context, adbPath string, deviceID string) string {
	dumpsysCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, args := range [][]string{
		{"-s", deviceID, "shell", "dumpsys", "activity", "activities"},
		{"-s", deviceID, "shell", "dumpsys", "window", "windows"},
	} {
		output, err := exec.CommandContext(dumpsysCtx, adbPath, args...).CombinedOutput()
		if err != nil {
			continue
		}
		if activity := parseCurrentActivity(string(output)); activity != "" {
			return activity
		}
	}
	return "UnknownActivity"
}

func parseCurrentActivity(output string) string {
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "mResumedActivity") ||
			strings.Contains(line, "topResumedActivity") ||
			strings.Contains(line, "ResumedActivity") ||
			strings.Contains(line, "mCurrentFocus") ||
			strings.Contains(line, "mFocusedApp") {
			if activity := activityPattern.FindString(line); activity != "" {
				return activity
			}
		}
	}
	return ""
}

func monitorMonkeyForegroundApp(ctx context.Context, adbPath string, req MonkeyRunRequest, run *monkeyRunState, recordEvidence func(MonkeyRiskEvidence)) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastForeignPackage := ""
	consecutiveForeignChecks := 0
	restoreCooldownUntil := time.Time{}
	lastAdInspectionAt := time.Time{}
	adSignature := ""
	adDetectedAt := time.Time{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Now().Before(restoreCooldownUntil) {
				continue
			}
			if overlay := resolveMonkeySystemOverlay(ctx, adbPath, req.DeviceID); overlay != "" {
				err := dismissMonkeySystemOverlay(ctx, adbPath, req.DeviceID)
				action := "system-overlay-dismissed"
				message := fmt.Sprintf("检测到系统菜单覆盖层 %s，已收起并返回目标 App", overlay)
				if err != nil {
					action = "system-overlay-dismiss-failed"
					message = fmt.Sprintf("检测到系统菜单覆盖层 %s，但自动收起失败: %v", overlay, err)
					recordEvidence(MonkeyRiskEvidence{Level: "warning", Source: "foreground-guard", Message: message})
				}
				summary := updateMonkeyGuardSummary(run, func(summary *MonkeyRunSummary) {
					if err == nil {
						summary.SystemOverlayDismissCount++
					}
				})
				publishMonkeyGuardEvent(run, action, "", req.PackageName, summary.ForegroundRestartCount, message)
				publishMonkeyStreamEvent(run, "summary", summary)
				restoreCooldownUntil = time.Now().Add(2 * time.Second)
				continue
			}

			activity := resolveCurrentActivity(ctx, adbPath, req.DeviceID)
			if time.Since(lastAdInspectionAt) >= 3*time.Second {
				lastAdInspectionAt = time.Now()
				inspection, err := inspectMonkeyAdUI(ctx, adbPath, req.DeviceID)
				if isMonkeyAdActivity(activity) {
					if !inspection.Detected {
						inspection = monkeyAdInspection{Detected: true, Signature: strings.ToLower(activity)}
					}
					err = nil
				}
				if err == nil && inspection.Detected {
					if inspection.Signature != adSignature {
						adSignature = inspection.Signature
						adDetectedAt = time.Now()
						publishMonkeyGuardEvent(run, "ad-waiting", "", req.PackageName, monkeyRunSummarySnapshot(run).ForegroundRestartCount, "检测到疑似广告页，等待 5 秒后尝试自动关闭")
						continue
					}
					if time.Since(adDetectedAt) >= 5*time.Second {
						dismissed := dismissMonkeyAd(ctx, adbPath, req.DeviceID, inspection)
						action := "ad-dismissed"
						message := "检测到广告页，已尝试点击关闭控件或返回键恢复测试"
						summary := updateMonkeyGuardSummary(run, func(summary *MonkeyRunSummary) {
							if dismissed {
								summary.AdDismissCount++
							}
						})
						if !dismissed {
							restartErr := restoreMonkeyTargetApp(ctx, adbPath, req.DeviceID, "", req.PackageName)
							action = "ad-restarted"
							message = fmt.Sprintf("广告页无法自动关闭，已重启目标 App %s", req.PackageName)
							if restartErr != nil {
								action = "ad-restart-failed"
								message = fmt.Sprintf("广告页无法自动关闭，重启目标 App 失败: %v", restartErr)
								recordEvidence(MonkeyRiskEvidence{Level: "warning", Source: "foreground-guard", Message: message})
							}
							summary = updateMonkeyGuardSummary(run, func(summary *MonkeyRunSummary) {
								if restartErr == nil {
									summary.ForegroundRestartCount++
									summary.AdRestartCount++
								}
							})
						}
						publishMonkeyGuardEvent(run, action, "", req.PackageName, summary.ForegroundRestartCount, message)
						publishMonkeyStreamEvent(run, "summary", summary)
						adSignature = ""
						adDetectedAt = time.Time{}
						restoreCooldownUntil = time.Now().Add(3 * time.Second)
						continue
					}
				} else {
					adSignature = ""
					adDetectedAt = time.Time{}
				}
			}

			foregroundPackage := foregroundPackageFromActivity(activity)
			if foregroundPackage == "" || foregroundPackage == req.PackageName {
				lastForeignPackage = ""
				consecutiveForeignChecks = 0
				continue
			}
			if foregroundPackage != lastForeignPackage {
				lastForeignPackage = foregroundPackage
				consecutiveForeignChecks = 1
				continue
			}
			consecutiveForeignChecks++
			if consecutiveForeignChecks < 2 {
				continue
			}
			consecutiveForeignChecks = 0

			err := restoreMonkeyTargetApp(ctx, adbPath, req.DeviceID, foregroundPackage, req.PackageName)
			summary := updateMonkeyGuardSummary(run, func(summary *MonkeyRunSummary) {
				summary.LastForeignPackage = foregroundPackage
				if err == nil {
					summary.ForegroundRestartCount++
				}
			})

			action := "restored"
			message := fmt.Sprintf("检测到前台偏离 %s，已重新拉起目标 App %s", foregroundPackage, req.PackageName)
			if err != nil {
				action = "restore-failed"
				message = fmt.Sprintf("检测到前台偏离 %s，但重新拉起目标 App 失败: %v", foregroundPackage, err)
				recordEvidence(MonkeyRiskEvidence{Level: "warning", Source: "foreground-guard", Message: message})
				restoreCooldownUntil = time.Now().Add(4 * time.Second)
			} else {
				restoreCooldownUntil = time.Now().Add(8 * time.Second)
			}
			publishMonkeyGuardEvent(run, action, foregroundPackage, req.PackageName, summary.ForegroundRestartCount, message)
			publishMonkeyStreamEvent(run, "summary", summary)
		}
	}
}

func updateMonkeyGuardSummary(run *monkeyRunState, update func(*MonkeyRunSummary)) MonkeyRunSummary {
	run.mu.Lock()
	defer run.mu.Unlock()
	update(&run.summary)
	_ = writeJSON(filepath.Join(monkeyRunDir(run.summary.RunID), "summary.json"), run.summary)
	return run.summary
}

func publishMonkeyGuardEvent(run *monkeyRunState, action string, foregroundPackage string, targetPackage string, restartCount int, message string) {
	publishMonkeyStreamEvent(run, "foreground-guard", MonkeyForegroundGuardEvent{
		Action: action, ForegroundPackage: foregroundPackage, TargetPackage: targetPackage,
		RestartCount: restartCount, Message: message, Timestamp: time.Now().Format(time.RFC3339),
	})
}

func resolveMonkeySystemOverlay(ctx context.Context, adbPath string, deviceID string) string {
	overlayCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(overlayCtx, adbPath, "-s", deviceID, "shell", "dumpsys", "window", "windows").CombinedOutput()
	if err != nil {
		return ""
	}
	return detectMonkeySystemOverlay(string(output))
}

func detectMonkeySystemOverlay(output string) string {
	for _, line := range strings.Split(output, "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "mcurrentfocus") && !strings.Contains(lower, "mfocusedapp") {
			continue
		}
		switch {
		case strings.Contains(lower, "notificationshade") || strings.Contains(lower, "notificationpanel"):
			return "通知栏"
		case strings.Contains(lower, "quicksettings"):
			return "快捷设置"
		case strings.Contains(lower, "com.android.systemui"):
			return "系统侧边栏"
		}
	}
	return ""
}

func dismissMonkeySystemOverlay(ctx context.Context, adbPath string, deviceID string) error {
	dismissCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	output, err := exec.CommandContext(dismissCtx, adbPath, "-s", deviceID, "shell", "cmd", "statusbar", "collapse").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	_ = exec.CommandContext(dismissCtx, adbPath, "-s", deviceID, "shell", "input", "keyevent", "KEYCODE_BACK").Run()
	return nil
}

func inspectMonkeyAdUI(ctx context.Context, adbPath string, deviceID string) (monkeyAdInspection, error) {
	inspectCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	output, err := exec.CommandContext(inspectCtx, adbPath, "-s", deviceID, "shell", "uiautomator", "dump", "/dev/tty").CombinedOutput()
	if err != nil {
		return monkeyAdInspection{}, err
	}
	return parseMonkeyAdInspection(string(output))
}

func parseMonkeyAdInspection(output string) (monkeyAdInspection, error) {
	hierarchyStart := strings.Index(output, "<hierarchy")
	if hierarchyStart < 0 {
		return monkeyAdInspection{}, fmt.Errorf("uiautomator output does not contain hierarchy")
	}
	hierarchyEnd := strings.Index(output[hierarchyStart:], "</hierarchy>")
	if hierarchyEnd < 0 {
		return monkeyAdInspection{}, fmt.Errorf("uiautomator hierarchy is incomplete")
	}
	hierarchyEnd += hierarchyStart + len("</hierarchy>")
	var hierarchy monkeyUIHierarchy
	if err := xml.Unmarshal([]byte(output[hierarchyStart:hierarchyEnd]), &hierarchy); err != nil {
		return monkeyAdInspection{}, err
	}
	nodes := flattenMonkeyUINodes(hierarchy.Nodes)
	signal := ""
	for _, node := range nodes {
		value := monkeyUINodeSearchText(node)
		if token := firstMonkeyToken(value, []string{
			"interstitial", "rewarded", "reward_ad", "app_open_ad", "splash_ad", "ad_container", "ad_close", "admob", "applovin",
			"unityads", "ironsource", "vungle", "mintegral", "pangle", "广告", "advertisement", "sponsored",
		}); token != "" {
			signal = token
			break
		}
	}
	if signal == "" {
		return monkeyAdInspection{}, nil
	}
	inspection := monkeyAdInspection{Detected: true, Signature: signal}
	for _, node := range nodes {
		value := monkeyUINodeSearchText(node)
		if !containsAnyMonkeyToken(value, []string{"关闭", "跳过", "skip", "close", "dismiss", "cross", "btn_close", "iv_close", "ad_close"}) {
			continue
		}
		if x, y, ok := monkeyNodeCenter(node.Bounds); ok {
			inspection.CloseTargetX = x
			inspection.CloseTargetY = y
			inspection.HasCloseTarget = true
			return inspection, nil
		}
	}
	return inspection, nil
}

func flattenMonkeyUINodes(nodes []monkeyUINode) []monkeyUINode {
	items := make([]monkeyUINode, 0, len(nodes))
	for _, node := range nodes {
		items = append(items, node)
		items = append(items, flattenMonkeyUINodes(node.Nodes)...)
	}
	return items
}

func monkeyUINodeSearchText(node monkeyUINode) string {
	return strings.ToLower(strings.Join([]string{node.Text, node.ResourceID, node.Class, node.ContentDesc}, " "))
}

func containsAnyMonkeyToken(value string, tokens []string) bool {
	return firstMonkeyToken(value, tokens) != ""
}

func firstMonkeyToken(value string, tokens []string) string {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return token
		}
	}
	return ""
}

func isMonkeyAdActivity(activity string) bool {
	return containsAnyMonkeyToken(strings.ToLower(activity), []string{
		"adactivity", "interstitial", "rewarded", "appopenad", "splashad", "applovin", "unityads",
		"ironsource", "vungle", "mintegral", "pangle", "gdtad", "kwad",
	})
}

func monkeyNodeCenter(bounds string) (int, int, bool) {
	match := nodeBoundsPattern.FindStringSubmatch(bounds)
	if len(match) != 5 {
		return 0, 0, false
	}
	values := make([]int, 0, 4)
	for _, value := range match[1:] {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, 0, false
		}
		values = append(values, parsed)
	}
	return (values[0] + values[2]) / 2, (values[1] + values[3]) / 2, true
}

func dismissMonkeyAd(ctx context.Context, adbPath string, deviceID string, inspection monkeyAdInspection) bool {
	dismissCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if inspection.HasCloseTarget {
		_ = exec.CommandContext(dismissCtx, adbPath, "-s", deviceID, "shell", "input", "tap",
			strconv.Itoa(inspection.CloseTargetX), strconv.Itoa(inspection.CloseTargetY)).Run()
		time.Sleep(800 * time.Millisecond)
		if current, err := inspectMonkeyAdUI(dismissCtx, adbPath, deviceID); err == nil && !current.Detected && !isMonkeyAdActivity(resolveCurrentActivity(dismissCtx, adbPath, deviceID)) {
			return true
		}
	}
	_ = exec.CommandContext(dismissCtx, adbPath, "-s", deviceID, "shell", "input", "keyevent", "KEYCODE_BACK").Run()
	time.Sleep(800 * time.Millisecond)
	current, err := inspectMonkeyAdUI(dismissCtx, adbPath, deviceID)
	return err == nil && !current.Detected && !isMonkeyAdActivity(resolveCurrentActivity(dismissCtx, adbPath, deviceID))
}

func foregroundPackageFromActivity(activity string) string {
	if activity == "" || activity == "UnknownActivity" {
		return ""
	}
	packageName, _, ok := strings.Cut(activity, "/")
	if !ok {
		return ""
	}
	return strings.TrimSpace(packageName)
}

func restoreMonkeyTargetApp(ctx context.Context, adbPath string, deviceID string, foreignPackage string, targetPackage string) error {
	restoreCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if shouldForceStopForegroundPackage(foreignPackage, targetPackage) {
		_ = exec.CommandContext(restoreCtx, adbPath, "-s", deviceID, "shell", "am", "force-stop", foreignPackage).Run()
	}
	_ = exec.CommandContext(restoreCtx, adbPath, "-s", deviceID, "shell", "am", "force-stop", targetPackage).Run()
	output, err := exec.CommandContext(
		restoreCtx, adbPath, "-s", deviceID, "shell", "monkey",
		"-p", targetPackage, "-c", "android.intent.category.LAUNCHER", "1",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := validateMonkeyLaunchOutput(string(output)); err != nil {
		return err
	}
	return nil
}

func validateMonkeyLaunchOutput(output string) error {
	lower := strings.ToLower(output)
	for _, failure := range []string{"no activities found", "monkey aborted", "error:"} {
		if strings.Contains(lower, failure) {
			return fmt.Errorf("目标 App 拉起失败: %s", strings.TrimSpace(output))
		}
	}
	return nil
}

func shouldForceStopForegroundPackage(foregroundPackage string, targetPackage string) bool {
	packageName := strings.ToLower(strings.TrimSpace(foregroundPackage))
	if packageName == "" || packageName == strings.ToLower(strings.TrimSpace(targetPackage)) {
		return false
	}
	for _, prefix := range []string{"android", "com.android.", "com.google.android.permissioncontroller", "com.google.android.apps.nexuslauncher"} {
		if packageName == prefix || strings.HasPrefix(packageName, prefix) {
			return false
		}
	}
	return true
}

func updateMonkeySummaryFromEvents(run *monkeyRunState, events []MonkeyGraphEvent, errText string, finished bool) {
	run.mu.Lock()
	run.summary.ScreenshotCount = len(events)
	run.summary.NormalCount, run.summary.WarningCount, run.summary.CriticalCount, run.summary.UnknownCount = 0, 0, 0, 0
	run.summary.FirstCriticalNodeID = ""
	run.summary.AppCrashDetected = false
	for _, event := range events {
		if eventContainsAppCrash(event) {
			run.summary.AppCrashDetected = true
		}
		switch event.Risk {
		case "critical":
			run.summary.CriticalCount++
			if run.summary.FirstCriticalNodeID == "" {
				run.summary.FirstCriticalNodeID = event.ID
			}
		case "warning":
			run.summary.WarningCount++
		case "unknown":
			run.summary.UnknownCount++
		default:
			run.summary.NormalCount++
		}
	}
	if errText != "" {
		run.summary.Error = errText
	}
	if finished {
		run.summary.Status = "passed"
		if run.summary.WarningCount > 0 {
			run.summary.Status = "warning"
		}
		if run.summary.CriticalCount > 0 || errText != "" {
			run.summary.Status = "failed"
		}
		if errText == context.Canceled.Error() || errText == context.DeadlineExceeded.Error() {
			run.summary.Status = "stopped"
		}
		if len(events) == 0 {
			run.summary.Status = "incomplete"
		}
		run.summary.TerminationReason = resolveMonkeyTerminationReason(run.summary, errText)
		run.summary.FinishedAt = time.Now().Format(time.RFC3339)
	}
	if started, err := time.Parse(time.RFC3339, run.summary.StartedAt); err == nil {
		run.summary.Duration = formatDurationHMS(int(time.Since(started).Seconds()))
	}
	_ = writeJSON(filepath.Join(monkeyRunDir(run.summary.RunID), "summary.json"), run.summary)
	summary := run.summary
	run.mu.Unlock()
	publishMonkeyStreamEvent(run, "summary", summary)
	if finished {
		publishMonkeyStreamEvent(run, "complete", summary)
	}
}

func eventContainsAppCrash(event MonkeyGraphEvent) bool {
	for _, evidence := range event.Evidence {
		lower := strings.ToLower(evidence.Message)
		if strings.Contains(lower, "crash:") ||
			strings.Contains(lower, "fatal exception") ||
			strings.Contains(lower, "force finishing activity") ||
			strings.Contains(lower, "monkey aborted") {
			return true
		}
	}
	return false
}

func resolveMonkeyTerminationReason(summary MonkeyRunSummary, errText string) string {
	if summary.AppCrashDetected {
		return "target_app_crash"
	}
	if errText == context.Canceled.Error() || errText == context.DeadlineExceeded.Error() {
		return "stopped_or_duration_reached"
	}
	if errText != "" {
		return "monkey_process_error"
	}
	if summary.WarningCount > 0 || summary.CriticalCount > 0 {
		return "completed_with_risk"
	}
	return "completed"
}

func saveMonkeyExecutionReport(req MonkeyRunRequest, summary MonkeyRunSummary) {
	status := monkeyExecutionReportStatus(summary.Status)
	analysis := fmt.Sprintf("Monkey 真机测试\n包名: %s\n设备: %s\n模式: %s\n执行批次: %d\n截图: %d\nNormal: %d, Warning: %d, Critical: %d, Unknown: %d\n真实 App Crash: %t\n结束原因: %s\n已过滤系统噪声: %d\n前台守护重启: %d\n系统菜单自动收起: %d\n广告自动关闭: %d\n广告失败重启: %d\n最近偏离包名: %s",
		summary.PackageName, summary.DeviceID, summary.PrecisionMode, summary.BatchCount, summary.ScreenshotCount, summary.NormalCount, summary.WarningCount, summary.CriticalCount, summary.UnknownCount,
		summary.AppCrashDetected, summary.TerminationReason, summary.IgnoredSystemLogCount, summary.ForegroundRestartCount, summary.SystemOverlayDismissCount, summary.AdDismissCount, summary.AdRestartCount, summary.LastForeignPackage)
	_, _ = AddExecutionReport(ExecutionReport{
		ID: "MONKEY-" + time.Now().Format("20060102150405"), RunID: summary.RunID,
		Name: "Monkey测试", Type: "Monkey 测试", Status: status, Duration: summary.Duration,
		Author: req.Author, Environment: normalizeReportEnvironment(req.Environment), AnalysisResult: analysis,
		ReportURL: PlatformBackendURL("/api/monkey/runs/" + summary.RunID + "/events"),
	})
}

func monkeyExecutionReportStatus(summaryStatus string) string {
	switch summaryStatus {
	case "warning":
		return "Warning"
	case "failed", "incomplete":
		return "Failed"
	case "stopped":
		return "Stopped"
	default:
		return "Passed"
	}
}

func listArchivedMonkeyRuns() ([]MonkeyRunSummary, error) {
	root := monkeyStorageRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []MonkeyRunSummary{}, nil
		}
		return nil, err
	}
	runs := []MonkeyRunSummary{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if summary, err := loadMonkeySummary(entry.Name()); err == nil {
			runs = append(runs, summary)
		}
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].RunID > runs[j].RunID })
	return runs, nil
}

func loadMonkeySummary(runID string) (MonkeyRunSummary, error) {
	var summary MonkeyRunSummary
	err := readJSON(filepath.Join(monkeyRunDir(runID), "summary.json"), &summary)
	return summary, err
}

func loadMonkeyEvents(runID string) ([]MonkeyGraphEvent, error) {
	var events []MonkeyGraphEvent
	if err := readJSON(filepath.Join(monkeyRunDir(runID), "events.json"), &events); err != nil {
		return nil, err
	}
	for index := range events {
		if len(events[index].Evidence) > 0 {
			events[index].Summary = buildEvidenceSummary(events[index].Risk, events[index].Evidence)
		}
	}
	return events, nil
}

func applyMonkeyRetentionPolicy() {
	root := monkeyStorageRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		summary, err := loadMonkeySummary(runID)
		if err != nil {
			continue
		}
		started, err := time.Parse(time.RFC3339, summary.StartedAt)
		if err != nil {
			continue
		}
		age := now.Sub(started)
		events, err := loadMonkeyEvents(runID)
		if err != nil {
			continue
		}
		changed := false
		for index := range events {
			event := &events[index]
			if age > 30*24*time.Hour && event.Risk == "normal" && event.ImageFile != "" && event.ImageStatus != "deleted" {
				_ = os.Remove(filepath.Join(monkeyRunDir(runID), "screenshots", event.ImageFile))
				event.ImageURL = ""
				event.ImageStatus = "deleted"
				event.ImageDeletedReason = "已按 30 天保留策略清理正常节点图片"
				changed = true
				continue
			}
			if age > 7*24*time.Hour && event.ImageStatus == "original" && event.ImageFile != "" {
				if compressed, err := compressMonkeyScreenshot(runID, event.ImageFile); err == nil {
					_ = os.Remove(filepath.Join(monkeyRunDir(runID), "screenshots", event.ImageFile))
					event.ImageFile = compressed
					event.ImageURL = PlatformBackendURL("/api/monkey/runs/" + runID + "/screenshots/" + compressed)
					event.ImageStatus = "compressed"
					changed = true
				}
			}
		}
		if changed {
			_ = writeJSON(filepath.Join(monkeyRunDir(runID), "events.json"), events)
		}
	}
}

func compressMonkeyScreenshot(runID string, fileName string) (string, error) {
	src := filepath.Join(monkeyRunDir(runID), "screenshots", fileName)
	content, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	target := strings.TrimSuffix(fileName, filepath.Ext(fileName)) + ".jpg"
	dst, err := os.Create(filepath.Join(monkeyRunDir(runID), "screenshots", target))
	if err != nil {
		return "", err
	}
	defer dst.Close()
	return target, jpeg.Encode(dst, img, &jpeg.Options{Quality: 75})
}

func monkeyStorageRoot() string {
	return filepath.Join(projectRootDir(), "report", "monkey_report")
}

func monkeyRunDir(runID string) string {
	return filepath.Join(monkeyStorageRoot(), filepath.Base(runID))
}

func indexedScreenID(index int) string {
	return fmt.Sprintf("screen-%04d", index)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatDurationHMS(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
