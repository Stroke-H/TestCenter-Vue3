package services

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type TestRunArchive struct {
	RunID      string            `json:"runId"`
	TestType   string            `json:"testType"`
	TestName   string            `json:"testName"`
	Status     string            `json:"status"`
	StartedAt  string            `json:"startedAt"`
	FinishedAt string            `json:"finishedAt"`
	Duration   string            `json:"duration"`
	Author     string            `json:"author"`
	Artifacts  []TestRunArtifact `json:"artifacts"`
	Metrics    DramaRunMetrics   `json:"metrics"`
	Legacy     bool              `json:"legacy,omitempty"`
}

type TestRunArtifact struct {
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
	StoragePath string `json:"storagePath"`
	URL         string `json:"url"`
	MimeType    string `json:"mimeType"`
}

type DramaRunMetrics struct {
	FailedChecks          int `json:"failedChecks"`
	FailedRequests        int `json:"failedRequests"`
	ContinuityFailures    int `json:"continuityFailures"`
	TotalMismatch         int `json:"totalMismatch"`
	UnhealthyChapters     int `json:"unhealthyChapters"`
	UpdateStatusFailCount int `json:"updateStatusFailCount"`
}

type DramaAnalyticsPoint struct {
	ID              string                 `json:"id"`
	RunID           string                 `json:"runId"`
	Name            string                 `json:"name"`
	CreatedAt       string                 `json:"createdAt"`
	ReportURL       string                 `json:"reportUrl"`
	FailedChecks    int                    `json:"failedChecks"`
	FailedRequests  int                    `json:"failedRequests"`
	Breakdown       []FailureBreakdownStat `json:"breakdown"`
	Legacy          bool                   `json:"legacy,omitempty"`
	ArtifactMissing bool                   `json:"artifactMissing,omitempty"`
}

type FailureBreakdownStat struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value int    `json:"value"`
	Color string `json:"color"`
}

var metricCardRegexp = regexp.MustCompile(`(?is)<div class="metric-card[^"]*"[^>]*>.*?<h4>\s*([^<]+?)\s*</h4>.*?<div class="metric-value">\s*([^<]+?)\s*</div>`)
var counterRowRegexp = regexp.MustCompile(`(?is)<tr>\s*<td><b>\s*([^<]+?)\s*</b></td>.*?<td[^>]*>\s*([0-9.,-]+)\s*</td>.*?</tr>`)

func CreateDramaTestRunArchive(rootDir string, job *dramaRunJob, reportFile string, reportURL string) (TestRunArchive, error) {
	job.mu.Lock()
	logs := append([]string{}, job.logs...)
	startedAt := job.startedAt
	finishedAt := job.finishedAt
	status := job.status
	job.mu.Unlock()

	if finishedAt.IsZero() {
		finishedAt = time.Now()
	}

	sourcePath := filepath.Join(rootDir, reportFile)
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return TestRunArchive{}, err
	}

	runDir := filepath.Join(testRunStorageRoot(rootDir), job.id)
	artifactDir := filepath.Join(runDir, "artifacts")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return TestRunArchive{}, err
	}

	artifactPath := filepath.Join(artifactDir, "report.html")
	if err := os.WriteFile(artifactPath, content, 0644); err != nil {
		return TestRunArchive{}, err
	}
	if err := ensureLogsFile(filepath.Join(runDir, "logs.ndjson"), logs); err != nil {
		return TestRunArchive{}, err
	}

	metrics := parseDramaMetricsFromHTML(string(content))
	archive := TestRunArchive{
		RunID:      job.id,
		TestType:   "drama",
		TestName:   job.toolName,
		Status:     status,
		StartedAt:  formatArchiveTime(startedAt),
		FinishedAt: formatArchiveTime(finishedAt),
		Duration:   formatDuration(finishedAt.Sub(startedAt)),
		Author:     job.author,
		Artifacts: []TestRunArtifact{
			{
				Type:        "html",
				DisplayName: "剧集播放接口测试报告",
				StoragePath: filepath.ToSlash(filepath.Join("api_report", "test-runs", job.id, "artifacts", "report.html")),
				URL:         PlatformBackendURL("/api/test-runs/" + url.PathEscape(job.id) + "/artifacts/report"),
				MimeType:    "text/html; charset=utf-8",
			},
		},
		Metrics: metrics,
	}

	if err := writeJSONFile(filepath.Join(runDir, "metrics.json"), metrics); err != nil {
		return TestRunArchive{}, err
	}
	if err := writeJSONFile(filepath.Join(runDir, "metadata.json"), archive); err != nil {
		return TestRunArchive{}, err
	}

	_ = reportURL
	return archive, nil
}

func CreateDramaFailureArchive(rootDir string, job *dramaRunJob) error {
	job.mu.Lock()
	startedAt := job.startedAt
	finishedAt := job.finishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now()
	}
	archive := TestRunArchive{
		RunID:      job.id,
		TestType:   "drama",
		TestName:   job.toolName,
		Status:     job.status,
		StartedAt:  formatArchiveTime(startedAt),
		FinishedAt: formatArchiveTime(finishedAt),
		Duration:   formatDuration(finishedAt.Sub(startedAt)),
		Author:     job.author,
		Artifacts:  []TestRunArtifact{},
		Metrics:    DramaRunMetrics{},
	}
	job.mu.Unlock()

	runDir := filepath.Join(testRunStorageRoot(rootDir), job.id)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(runDir, "metrics.json"), archive.Metrics); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(runDir, "metadata.json"), archive)
}

func CreateScheduledDramaArchive(rootDir string, runID string, task ScheduledTask, status string, duration time.Duration, startedAt time.Time, reportFile string) (TestRunArchive, error) {
	finishedAt := startedAt.Add(duration)
	sourcePath := filepath.Join(rootDir, reportFile)
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return TestRunArchive{}, err
	}

	runDir := filepath.Join(testRunStorageRoot(rootDir), runID)
	artifactDir := filepath.Join(runDir, "artifacts")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return TestRunArchive{}, err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "report.html"), content, 0644); err != nil {
		return TestRunArchive{}, err
	}

	metrics := parseDramaMetricsFromHTML(string(content))
	archive := TestRunArchive{
		RunID:      runID,
		TestType:   "drama",
		TestName:   firstNonEmpty(task.Name, "剧集播放接口测试"),
		Status:     status,
		StartedAt:  formatArchiveTime(startedAt),
		FinishedAt: formatArchiveTime(finishedAt),
		Duration:   formatDuration(duration),
		Author:     firstNonEmpty(task.Creator, "scheduled-task"),
		Artifacts: []TestRunArtifact{
			{
				Type:        "html",
				DisplayName: "剧集播放接口测试报告",
				StoragePath: filepath.ToSlash(filepath.Join("api_report", "test-runs", runID, "artifacts", "report.html")),
				URL:         PlatformBackendURL("/api/test-runs/" + url.PathEscape(runID) + "/artifacts/report"),
				MimeType:    "text/html; charset=utf-8",
			},
		},
		Metrics: metrics,
	}
	if err := writeJSONFile(filepath.Join(runDir, "metrics.json"), metrics); err != nil {
		return TestRunArchive{}, err
	}
	if err := writeJSONFile(filepath.Join(runDir, "metadata.json"), archive); err != nil {
		return TestRunArchive{}, err
	}
	return archive, nil
}

func ListDramaRunAnalyticsHandler(c *gin.Context) {
	points := make([]DramaAnalyticsPoint, 0)

	rootDir := projectRootDir()
	archives, _ := listTestRunArchives(rootDir)
	for _, archive := range archives {
		if archive.TestType != "drama" {
			continue
		}
		points = append(points, archiveToDramaAnalyticsPoint(archive))
	}

	sort.Slice(points, func(i, j int) bool {
		return parseArchiveTime(points[i].CreatedAt).Before(parseArchiveTime(points[j].CreatedAt))
	})
	c.JSON(http.StatusOK, points)
}

func GetTestRunArtifactHandler(c *gin.Context) {
	runID := sanitizeReportSuffix(c.Param("runId"))
	artifactName := c.Param("artifact")
	if artifactName != "report" {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not found"})
		return
	}

	rootDir := projectRootDir()
	path := filepath.Join(testRunStorageRoot(rootDir), runID, "artifacts", "report.html")
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not found"})
		return
	}
	c.File(path)
}

func GetTestRunLogsHandler(c *gin.Context) {
	runID := sanitizeReportSuffix(c.Param("runId"))
	rootDir := projectRootDir()
	path := filepath.Join(testRunStorageRoot(rootDir), runID, "logs.ndjson")
	content, err := os.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "logs not found"})
		return
	}
	c.Data(http.StatusOK, "application/x-ndjson; charset=utf-8", content)
}

func archiveToDramaAnalyticsPoint(archive TestRunArchive) DramaAnalyticsPoint {
	reportURL := ""
	if len(archive.Artifacts) > 0 {
		reportURL = archive.Artifacts[0].URL
	}
	return DramaAnalyticsPoint{
		ID:              archive.RunID,
		RunID:           archive.RunID,
		Name:            archive.TestName,
		CreatedAt:       firstNonEmpty(archive.FinishedAt, archive.StartedAt),
		ReportURL:       reportURL,
		FailedChecks:    archive.Metrics.FailedChecks,
		FailedRequests:  archive.Metrics.FailedRequests,
		Breakdown:       dramaMetricsBreakdown(archive.Metrics),
		Legacy:          archive.Legacy,
		ArtifactMissing: false,
	}
}

func listTestRunArchives(rootDir string) ([]TestRunArchive, error) {
	storageRoot := testRunStorageRoot(rootDir)
	entries, err := os.ReadDir(storageRoot)
	if err != nil {
		return nil, err
	}
	archives := make([]TestRunArchive, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(storageRoot, entry.Name(), "metadata.json"))
		if err != nil {
			continue
		}
		var archive TestRunArchive
		if err := json.Unmarshal(content, &archive); err != nil {
			continue
		}
		archives = append(archives, archive)
	}
	return archives, nil
}

func testRunStorageRoot(rootDir string) string {
	return filepath.Join(apiReportStorageRoot(rootDir), "test-runs")
}

func reportStorageRoot(rootDir string) string {
	return filepath.Join(rootDir, "report")
}

func apiReportStorageRoot(rootDir string) string {
	return filepath.Join(reportStorageRoot(rootDir), "api_report")
}

func webTestReportStorageRoot(rootDir string) string {
	return filepath.Join(reportStorageRoot(rootDir), "web_test_report")
}

func projectRootDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		rootDir, _ := filepath.Abs("..")
		return rootDir
	}
	for _, candidate := range []string{
		cwd,
		filepath.Dir(cwd),
		filepath.Dir(filepath.Dir(cwd)),
	} {
		if _, err := os.Stat(filepath.Join(candidate, "k6-scripts")); err == nil {
			return candidate
		}
	}
	rootDir, _ := filepath.Abs("..")
	return rootDir
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func appendTestRunLog(runID string, message string) {
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(message) == "" {
		return
	}
	rootDir := projectRootDir()
	runDir := filepath.Join(testRunStorageRoot(rootDir), sanitizeReportSuffix(runID))
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return
	}
	row := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   message,
	}
	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	file, err := os.OpenFile(filepath.Join(runDir, "logs.ndjson"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(data, '\n'))
}

func ensureLogsFile(path string, logs []string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, line := range logs {
		row := map[string]any{
			"message": line,
		}
		data, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func parseDramaMetricsFromHTML(content string) DramaRunMetrics {
	metrics := DramaRunMetrics{}
	for _, match := range metricCardRegexp.FindAllStringSubmatch(content, -1) {
		name := normalizeHTMLText(match[1])
		value := parseMetricInt(match[2])
		switch name {
		case "Failed Checks":
			metrics.FailedChecks = value
		case "Failed Requests":
			metrics.FailedRequests = value
		}
	}
	for _, match := range counterRowRegexp.FindAllStringSubmatch(content, -1) {
		name := normalizeHTMLText(match[1])
		value := parseMetricInt(match[2])
		switch {
		case strings.Contains(name, "Continuity Failures") || strings.Contains(name, "跳号剧集数"):
			metrics.ContinuityFailures = value
		case strings.Contains(name, "Total Mismatch") || strings.Contains(name, "计数不符剧集数"):
			metrics.TotalMismatch = value
		case strings.Contains(name, "Unhealthy Chapters") || strings.Contains(name, "下架/异常章节总数"):
			metrics.UnhealthyChapters = value
		case strings.Contains(name, "update_status_fail_count") || strings.Contains(name, "转换失败"):
			metrics.UpdateStatusFailCount = value
		}
	}
	return metrics
}

func dramaMetricsBreakdown(metrics DramaRunMetrics) []FailureBreakdownStat {
	return []FailureBreakdownStat{
		{Key: "continuity", Label: "跳号剧集", Value: metrics.ContinuityFailures, Color: "#eab308"},
		{Key: "mismatch", Label: "计数不符", Value: metrics.TotalMismatch, Color: "#06b6d4"},
		{Key: "offline", Label: "下架/异常章节", Value: metrics.UnhealthyChapters, Color: "#8b5cf6"},
		{Key: "conversion", Label: "转换失败", Value: metrics.UpdateStatusFailCount, Color: "#64748b"},
	}
}

func normalizeHTMLText(value string) string {
	replacer := strings.NewReplacer("&nbsp;", " ", "&#34;", `"`, "&gt;", ">", "&lt;", "<", "&amp;", "&")
	return strings.Join(strings.Fields(replacer.Replace(value)), " ")
}

func parseMetricInt(value string) int {
	value = strings.ReplaceAll(normalizeHTMLText(value), ",", "")
	matched := regexp.MustCompile(`-?\d+(\.\d+)?`).FindString(value)
	if matched == "" {
		return 0
	}
	number, _ := strconv.ParseFloat(matched, 64)
	return int(number)
}

func formatArchiveTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006/01/02 15:04:05")
}

func parseArchiveTime(value string) time.Time {
	for _, layout := range []string{"2006/01/02 15:04:05", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
