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
	copyDramaRunDiagnosticArtifacts(filepath.Dir(sourcePath), artifactDir)
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
	copyDramaRunDiagnosticArtifacts(filepath.Dir(sourcePath), artifactDir)

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

func CreateScheduledSubtitleArchive(rootDir string, runID string, task ScheduledTask, status string, duration time.Duration, startedAt time.Time, reportFile string) (TestRunArchive, error) {
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
	copySubtitleRunDiagnosticArtifacts(filepath.Dir(sourcePath), artifactDir)

	archive := TestRunArchive{
		RunID:      runID,
		TestType:   "subtitle",
		TestName:   firstNonEmpty(task.Name, "剧集外挂字幕测试"),
		Status:     status,
		StartedAt:  formatArchiveTime(startedAt),
		FinishedAt: formatArchiveTime(finishedAt),
		Duration:   formatDuration(duration),
		Author:     firstNonEmpty(task.Creator, "scheduled-task"),
		Artifacts: []TestRunArtifact{
			{
				Type:        "html",
				DisplayName: "剧集外挂字幕测试报告",
				StoragePath: filepath.ToSlash(filepath.Join("api_report", "test-runs", runID, "artifacts", "report.html")),
				URL:         PlatformBackendURL("/api/test-runs/" + url.PathEscape(runID) + "/artifacts/report"),
				MimeType:    "text/html; charset=utf-8",
			},
		},
		Metrics: DramaRunMetrics{},
	}
	if err := writeJSONFile(filepath.Join(runDir, "metrics.json"), archive.Metrics); err != nil {
		return TestRunArchive{}, err
	}
	if err := writeJSONFile(filepath.Join(runDir, "metadata.json"), archive); err != nil {
		return TestRunArchive{}, err
	}
	return archive, nil
}

func copyDramaRunDiagnosticArtifacts(reportDir string, artifactDir string) {
	for _, fileName := range []string{
		"drama_failure_summary.json",
		"drama_retry_result.json",
		"drama_retry_candidates.json",
	} {
		content, err := os.ReadFile(filepath.Join(reportDir, fileName))
		if err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(artifactDir, fileName), content, 0644)
	}
}

func copySubtitleRunDiagnosticArtifacts(reportDir string, artifactDir string) {
	for _, fileName := range []string{
		"subtitle_queue.jsonl",
		"subtitle_summary.json",
		"subtitle_failures.jsonl",
	} {
		content, err := os.ReadFile(filepath.Join(reportDir, fileName))
		if err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(artifactDir, fileName), content, 0644)
	}
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
	content, err := os.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not found"})
		return
	}
	reportHTML := applyFullscreenReportPresentation(string(content))
	reportHTML = applyDramaIssueOperatingControls(reportHTML, runID)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(reportHTML))
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

func applyFullscreenReportPresentation(reportHTML string) string {
	const styleID = "testcenter-fullscreen-report-presentation"
	if strings.Contains(reportHTML, styleID) {
		return reportHTML
	}

	style := `<style id="` + styleID + `">
  html,
  body {
    width: 100% !important;
    min-height: 100% !important;
    margin: 0 !important;
    padding: 0 !important;
    background: #ffffff !important;
    scrollbar-width: none !important;
  }
  html::-webkit-scrollbar,
  body::-webkit-scrollbar,
  *::-webkit-scrollbar {
    width: 0 !important;
    height: 0 !important;
  }
  body {
    overflow: auto !important;
  }
  .container {
    width: 100% !important;
    max-width: none !important;
    min-height: 100vh !important;
    margin: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    box-shadow: none !important;
    overflow: visible !important;
  }
</style>`

	if strings.Contains(reportHTML, "</head>") {
		return strings.Replace(reportHTML, "</head>", style+"\n</head>", 1)
	}
	return style + reportHTML
}

func applyDramaIssueOperatingControls(reportHTML string, runID string) string {
	const scriptID = "testcenter-drama-issue-operating-controls"
	if !strings.Contains(reportHTML, "drama-other-checks") || strings.Contains(reportHTML, scriptID) {
		return reportHTML
	}
	runIDJSON, err := json.Marshal(runID)
	if err != nil {
		return reportHTML
	}

	style := `<style id="testcenter-drama-issue-operating-style">
  .drama-other-checks {
    position: relative;
    z-index: 12;
    overflow: visible !important;
  }
  .drama-other-checks table {
    position: relative;
    overflow: visible !important;
  }
  .drama-other-checks thead th:first-child {
    border-top-left-radius: 8px;
  }
  .drama-other-checks thead th:last-child {
    border-top-right-radius: 8px;
  }
  .drama-other-checks tbody tr:last-child td:first-child {
    border-bottom-left-radius: 8px;
  }
  .drama-other-checks tbody tr:last-child td:last-child {
    border-bottom-right-radius: 8px;
  }
  .drama-operating-column {
    width: 184px;
    min-width: 184px;
    text-align: left !important;
    vertical-align: middle !important;
    overflow: visible !important;
  }
  .drama-operating-wrap {
    position: relative;
    z-index: 2;
    display: inline-flex;
    align-items: center;
    width: 164px;
    max-width: 100%;
  }
  .drama-operating-wrap[data-open="true"] {
    z-index: 80;
  }
  .drama-operating-trigger {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 9px;
    width: 100%;
    min-height: 38px;
    padding: 7px 11px;
    border: 1px solid rgba(109, 94, 252, 0.28);
    border-radius: 11px;
    background: linear-gradient(135deg, #ffffff 0%, #f5f3ff 100%);
    color: #5145cd;
    box-shadow: 0 4px 12px rgba(91, 69, 214, 0.09), inset 0 1px 0 rgba(255, 255, 255, 0.85);
    font-size: 12px;
    font-weight: 750;
    letter-spacing: 0.01em;
    cursor: pointer;
    transition: border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease, background 160ms ease;
  }
  .drama-operating-trigger:hover:not(:disabled) {
    transform: translateY(-1px);
    border-color: rgba(109, 94, 252, 0.58);
    box-shadow: 0 8px 18px rgba(91, 69, 214, 0.15), inset 0 1px 0 rgba(255, 255, 255, 0.9);
  }
  .drama-operating-trigger:focus-visible {
    outline: none;
    border-color: #6d5efc;
    box-shadow: 0 0 0 4px rgba(109, 94, 252, 0.14), 0 8px 18px rgba(91, 69, 214, 0.13);
  }
  .drama-operating-trigger:disabled {
    cursor: wait;
    opacity: 0.62;
    transform: none;
  }
  .drama-operating-trigger[data-action="data_issue"] {
    border-color: rgba(245, 158, 11, 0.34);
    background: linear-gradient(135deg, #fffdf7 0%, #fff7e6 100%);
    color: #b45309;
    box-shadow: 0 5px 14px rgba(217, 119, 6, 0.1);
  }
  .drama-operating-trigger[data-action="temporarily_ignored"] {
    border-color: rgba(124, 58, 237, 0.34);
    background: linear-gradient(135deg, #faf7ff 0%, #eee8ff 100%);
    color: #6d28d9;
    box-shadow: 0 5px 14px rgba(109, 40, 217, 0.11);
  }
  .drama-operating-trigger[data-state="error"] {
    border-color: rgba(220, 38, 38, 0.34);
    background: linear-gradient(135deg, #fffafa 0%, #fef2f2 100%);
    color: #b91c1c;
  }
  .drama-operating-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    flex: 0 0 22px;
    border-radius: 7px;
    background: linear-gradient(135deg, #806ff8 0%, #6554dc 100%);
    box-shadow: 0 3px 8px rgba(91, 69, 214, 0.24);
  }
  .drama-operating-icon::before {
    content: '';
    width: 7px;
    height: 7px;
    border: 2px solid #fff;
    border-radius: 50%;
    box-sizing: border-box;
  }
  .drama-operating-trigger[data-action="data_issue"] .drama-operating-icon {
    background: linear-gradient(135deg, #fbbf24 0%, #d97706 100%);
    box-shadow: 0 3px 8px rgba(217, 119, 6, 0.22);
  }
  .drama-operating-trigger[data-action="temporarily_ignored"] .drama-operating-icon {
    background: linear-gradient(135deg, #9f7aea 0%, #6d28d9 100%);
    box-shadow: 0 3px 8px rgba(109, 40, 217, 0.22);
  }
  .drama-operating-label {
    min-width: 0;
    flex: 1;
    overflow: hidden;
    text-align: left;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .drama-operating-chevron {
    width: 7px;
    height: 7px;
    flex: 0 0 7px;
    margin: -3px 2px 0 0;
    border-right: 1.8px solid currentColor;
    border-bottom: 1.8px solid currentColor;
    opacity: 0.66;
    transform: rotate(45deg);
    transition: transform 160ms ease, margin 160ms ease;
  }
  .drama-operating-wrap[data-open="true"] .drama-operating-chevron {
    margin: 3px 2px 0 0;
    transform: rotate(225deg);
  }
  .drama-operating-menu {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    display: none;
    width: 232px;
    padding: 7px;
    border: 1px solid rgba(109, 94, 252, 0.16);
    border-radius: 14px;
    background: rgba(255, 255, 255, 0.98);
    box-shadow: 0 18px 45px rgba(51, 37, 122, 0.18), 0 5px 14px rgba(51, 37, 122, 0.08);
    backdrop-filter: blur(14px);
    -webkit-backdrop-filter: blur(14px);
    transform-origin: top right;
    animation: dramaOperatingMenuIn 150ms ease-out;
  }
  .drama-operating-wrap[data-open="true"] .drama-operating-menu {
    display: block;
  }
  .drama-operating-option {
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr);
    gap: 10px;
    align-items: center;
    width: 100%;
    padding: 9px 10px;
    border: 0;
    border-radius: 10px;
    background: transparent;
    color: #334155;
    text-align: left;
    cursor: pointer;
    transition: background 140ms ease, transform 140ms ease;
  }
  .drama-operating-option + .drama-operating-option {
    margin-top: 3px;
  }
  .drama-operating-option:hover,
  .drama-operating-option:focus-visible {
    outline: none;
    background: linear-gradient(135deg, #f8f7ff 0%, #f2efff 100%);
    transform: translateX(2px);
  }
  .drama-operating-option[data-selected="true"] {
    background: linear-gradient(135deg, #f4f1ff 0%, #ebe6ff 100%);
  }
  .drama-operating-option-dot {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: 9px;
  }
  .drama-operating-option[data-action="data_issue"] .drama-operating-option-dot {
    background: #fff3d6;
    color: #d97706;
  }
  .drama-operating-option[data-action="temporarily_ignored"] .drama-operating-option-dot {
    background: #efe9ff;
    color: #7c3aed;
  }
  .drama-operating-option-dot::before {
    content: '';
    width: 8px;
    height: 8px;
    border: 2px solid currentColor;
    border-radius: 50%;
  }
  .drama-operating-option-copy {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 2px;
  }
  .drama-operating-option-title {
    color: #312e81;
    font-size: 12px;
    font-weight: 800;
  }
  .drama-operating-option-description {
    color: #8490a3;
    font-size: 10px;
    font-weight: 500;
    line-height: 1.35;
  }
  @keyframes dramaOperatingMenuIn {
    from { opacity: 0; transform: translateY(-5px) scale(0.98); }
    to { opacity: 1; transform: translateY(0) scale(1); }
  }
  @media (max-width: 860px) {
    .drama-operating-column {
      width: 156px;
      min-width: 156px;
    }
    .drama-operating-wrap {
      width: 138px;
    }
    .drama-operating-menu {
      right: auto;
      left: 0;
    }
  }
</style>`

	script := `<script id="` + scriptID + `">
(function () {
  const runId = ` + string(runIDJSON) + `;
  const actionLabels = {
    data_issue: '数据问题',
    temporarily_ignored: '暂时忽略'
  };

  function getIssueFromRow(row, clientKey) {
    const checkCell = row.cells && row.cells[0];
    if (!checkCell) return null;
    const fullText = checkCell.innerText || checkCell.textContent || '';
    const idMatch = fullText.match(/ID:\s*([0-9a-f]{24})/i);
    const detail = checkCell.querySelector('details > div') || checkCell.querySelector('div');
    if (!idMatch || !detail) return null;
    const detailText = detail.innerText || detail.textContent || '';
    const errors = detailText.split(/\n+/).map(function (line) {
      return line.trim();
    }).filter(Boolean);
    if (!errors.length) return null;
    return { clientKey: clientKey, dramaId: idMatch[1], errors: errors };
  }

  function closeOperatingMenu(control) {
    control.wrap.dataset.open = 'false';
    control.trigger.setAttribute('aria-expanded', 'false');
  }

  function setOperatingState(control, action, stateText, isError) {
    control.trigger.dataset.action = action || '';
    control.trigger.dataset.state = isError ? 'error' : '';
    control.trigger.dataset.savedAction = action || '';
    control.label.textContent = stateText || (actionLabels[action] || '选择处理');
    control.options.forEach(function (option) {
      option.dataset.selected = option.dataset.action === action ? 'true' : 'false';
    });
  }

  async function initializeOperatingColumn() {
    const table = document.querySelector('.drama-other-checks table');
    if (!table || table.dataset.operatingReady === '1') return;
    table.dataset.operatingReady = '1';

    const headerRow = table.querySelector('thead tr');
    const checkHeader = headerRow && headerRow.querySelector('th');
    if (!headerRow || !checkHeader) return;
    const operatingHeader = document.createElement('th');
    operatingHeader.className = 'drama-operating-column';
    operatingHeader.textContent = 'OPERATING';
    checkHeader.insertAdjacentElement('afterend', operatingHeader);

    const rows = Array.from(table.querySelectorAll('tbody tr'));
    const issues = [];
    const controls = new Map();
    rows.forEach(function (row, index) {
      const clientKey = String(index);
      const issue = getIssueFromRow(row, clientKey);
      const cell = document.createElement('td');
      cell.className = 'drama-operating-column';
      const wrap = document.createElement('div');
      wrap.className = 'drama-operating-wrap';
      wrap.dataset.open = 'false';
      const trigger = document.createElement('button');
      trigger.type = 'button';
      trigger.className = 'drama-operating-trigger';
      trigger.setAttribute('aria-haspopup', 'menu');
      trigger.setAttribute('aria-expanded', 'false');
      trigger.innerHTML = '<span class="drama-operating-icon" aria-hidden="true"></span>' +
        '<span class="drama-operating-label"></span>' +
        '<span class="drama-operating-chevron" aria-hidden="true"></span>';
      const label = trigger.querySelector('.drama-operating-label');
      label.textContent = issue ? '读取中…' : '不可标记';
      trigger.disabled = !issue;
      const menu = document.createElement('div');
      menu.className = 'drama-operating-menu';
      menu.setAttribute('role', 'menu');
      menu.innerHTML = '<button type="button" class="drama-operating-option" role="menuitem" data-action="data_issue">' +
        '<span class="drama-operating-option-dot" aria-hidden="true"></span>' +
        '<span class="drama-operating-option-copy"><span class="drama-operating-option-title">数据问题</span><span class="drama-operating-option-description">仅添加标记，仍正常上报</span></span></button>' +
        '<button type="button" class="drama-operating-option" role="menuitem" data-action="temporarily_ignored">' +
        '<span class="drama-operating-option-dot" aria-hidden="true"></span>' +
        '<span class="drama-operating-option-copy"><span class="drama-operating-option-title">暂时忽略</span><span class="drama-operating-option-description">完全相同时不再重复提醒</span></span></button>';
      wrap.appendChild(trigger);
      wrap.appendChild(menu);
      cell.appendChild(wrap);
      row.cells[0].insertAdjacentElement('afterend', cell);
      if (!issue) return;

      issues.push(issue);
      const control = { issue: issue, wrap: wrap, trigger: trigger, label: label, menu: menu, options: Array.from(menu.querySelectorAll('.drama-operating-option')) };
      controls.set(clientKey, control);
      trigger.addEventListener('click', function (event) {
        event.stopPropagation();
        const willOpen = wrap.dataset.open !== 'true';
        controls.forEach(function (otherControl) { closeOperatingMenu(otherControl); });
        wrap.dataset.open = willOpen ? 'true' : 'false';
        trigger.setAttribute('aria-expanded', willOpen ? 'true' : 'false');
      });
      control.options.forEach(function (option) {
        option.addEventListener('click', async function (event) {
        event.stopPropagation();
        const action = option.dataset.action;
        if (!action) return;
        const previousAction = trigger.dataset.savedAction || '';
        closeOperatingMenu(control);
        trigger.disabled = true;
        setOperatingState(control, action, '保存中…', false);
        try {
          const response = await fetch('/api/test-runs/' + encodeURIComponent(runId) + '/issue-dispositions', {
            method: 'PUT',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ dramaId: issue.dramaId, errors: issue.errors, action: action })
          });
          const payload = await response.json().catch(function () { return {}; });
          if (!response.ok) throw new Error(payload.error || '保存失败');
          setOperatingState(control, action, actionLabels[action], false);
        } catch (error) {
          setOperatingState(control, previousAction, error.message || '保存失败', true);
        } finally {
          trigger.disabled = false;
        }
        });
      });
    });

    document.addEventListener('click', function () {
      controls.forEach(function (control) { closeOperatingMenu(control); });
    });

    if (!issues.length) return;
    try {
      const response = await fetch('/api/test-runs/' + encodeURIComponent(runId) + '/issue-dispositions/resolve', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ issues: issues })
      });
      const payload = await response.json().catch(function () { return {}; });
      if (!response.ok) throw new Error(payload.error || '读取失败');
      (payload.items || []).forEach(function (item) {
        const control = controls.get(String(item.clientKey));
        if (!control) return;
        const action = item.action || '';
        control.trigger.disabled = false;
        setOperatingState(control, action, actionLabels[action] || '未标记', false);
      });
    } catch (error) {
      controls.forEach(function (control) {
        control.trigger.disabled = false;
        setOperatingState(control, '', error.message || '读取失败', true);
      });
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializeOperatingColumn);
  } else {
    initializeOperatingColumn();
  }
})();
</script>`

	if strings.Contains(reportHTML, "</head>") {
		reportHTML = strings.Replace(reportHTML, "</head>", style+"\n</head>", 1)
	} else {
		reportHTML = style + reportHTML
	}
	if strings.Contains(reportHTML, "</body>") {
		return strings.Replace(reportHTML, "</body>", script+"\n</body>", 1)
	}
	return reportHTML + script
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
		case strings.Contains(name, "Direct Skips") || strings.Contains(name, "直接跳集剧集数") || strings.Contains(name, "直接跳级剧集数") ||
			strings.Contains(name, "Continuity Failures") || strings.Contains(name, "跳号剧集数"):
			metrics.ContinuityFailures = value
		case strings.Contains(name, "Total Mismatch") || strings.Contains(name, "计数不符剧集数"):
			metrics.TotalMismatch = value
		case strings.Contains(name, "Errored Chapters") || strings.Contains(name, "出错章节总数") ||
			strings.Contains(name, "Unhealthy Chapters") || strings.Contains(name, "下架/异常章节总数"):
			metrics.UnhealthyChapters = value
		case strings.Contains(name, "Converting Chapters") || strings.Contains(name, "转换中章节总数") ||
			strings.Contains(name, "update_status_fail_count") || strings.Contains(name, "转换失败"):
			metrics.UpdateStatusFailCount = value
		}
	}
	return metrics
}

func dramaMetricsBreakdown(metrics DramaRunMetrics) []FailureBreakdownStat {
	return []FailureBreakdownStat{
		{Key: "continuity", Label: "直接跳集", Value: metrics.ContinuityFailures, Color: "#eab308"},
		{Key: "mismatch", Label: "计数不符", Value: metrics.TotalMismatch, Color: "#06b6d4"},
		{Key: "offline", Label: "出错章节", Value: metrics.UnhealthyChapters, Color: "#8b5cf6"},
		{Key: "conversion", Label: "转换中", Value: metrics.UpdateStatusFailCount, Color: "#64748b"},
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
