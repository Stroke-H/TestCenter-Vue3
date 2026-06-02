package services

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ExecutionReport 定义执行历史记录的结构
type ExecutionReport struct {
	ID             string `json:"id"`
	RunID          string `json:"runId,omitempty"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Duration       string `json:"duration"`
	CreatedAt      string `json:"createdAt"`
	Author         string `json:"author,omitempty"`
	ReportURL      string `json:"reportUrl,omitempty"`
	AnalysisResult string `json:"analysisResult,omitempty"`
	Environment    string `json:"environment,omitempty"`
}

var (
	execReportMu sync.Mutex
)

// GetExecutionReportsHandler 获取所有执行历史记录
func GetExecutionReportsHandler(c *gin.Context) {
	execReportMu.Lock()
	defer execReportMu.Unlock()

	reports, err := sqlListJSON[ExecutionReport]("execution_reports", "`id` ASC")
	if err != nil {
		log.Println("[ERROR] Open exec reports failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load execution reports"})
		return
	}

	// 精准排序：按 ID 倒序排列 (ID 格式包含 YYYYMMDDHHMMSS，天然支持时间排序)
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ID > reports[j].ID
	})
	for index := range reports {
		reports[index].ReportURL = resolveReportURLForResponse(reports[index])
		reports[index].Status = normalizeMonkeyExecutionReportStatus(reports[index])
	}

	c.JSON(http.StatusOK, reports)
}

func normalizeMonkeyExecutionReportStatus(report ExecutionReport) string {
	if report.Type == "Monkey 测试" &&
		report.Status == "Failed" &&
		strings.Contains(report.AnalysisResult, "结束原因: stopped_or_duration_reached") {
		return "Stopped"
	}
	return report.Status
}

// AddExecutionReportHandler 保存一条新的执行记录
func AddExecutionReportHandler(c *gin.Context) {
	var r ExecutionReport
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	saved, err := AddExecutionReport(r)
	if err != nil {
		log.Println("[ERROR] Save exec report failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, saved)
}

func AddExecutionReport(r ExecutionReport) (ExecutionReport, error) {
	if r.ID == "" {
		r.ID = "REP-" + time.Now().Format("20060102150405")
	}
	if r.CreatedAt == "" {
		r.CreatedAt = time.Now().Format("2006/01/02 15:04:05")
	}
	r.ReportURL = normalizeStoredReportURL(r.ReportURL)

	execReportMu.Lock()
	defer execReportMu.Unlock()

	if err := sqlUpsertJSON("execution_reports", r); err != nil {
		return r, err
	}
	return r, nil
}

func normalizeReportEnvironment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "prod", "production", "正式服", "正式", "正服":
		return "prod"
	case "test", "testing", "测试服", "测试", "测服":
		return "test"
	default:
		return value
	}
}

func normalizeStoredReportURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	if strings.HasPrefix(parsed.Path, "/reports/") || strings.HasPrefix(parsed.Path, "/performance-reports/") {
		pathWithQuery := parsed.Path
		if parsed.RawQuery != "" {
			pathWithQuery += "?" + parsed.RawQuery
		}
		return PlatformBackendURL(pathWithQuery)
	}
	return rawURL
}

func resolveReportURLForResponse(report ExecutionReport) string {
	rootDir := projectRootDir()
	for _, runID := range []string{
		report.RunID,
	} {
		if strings.TrimSpace(runID) == "" {
			continue
		}
		artifactPath := filepath.Join(testRunStorageRoot(rootDir), runID, "artifacts", "report.html")
		if _, err := os.Stat(artifactPath); err == nil {
			return PlatformBackendURL("/api/test-runs/" + url.PathEscape(runID) + "/artifacts/report")
		}
	}
	return normalizeStoredReportURL(report.ReportURL)
}

// ClearExecutionReportsHandler 清空所有执行记录
func ClearExecutionReportsHandler(c *gin.Context) {
	execReportMu.Lock()
	defer execReportMu.Unlock()

	if err := sqlClearJSON("execution_reports"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear execution reports"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All execution records cleared"})
}
