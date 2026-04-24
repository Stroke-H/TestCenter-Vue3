package services

import (
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ExecutionReport 定义执行历史记录的结构
type ExecutionReport struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Duration       string `json:"duration"`
	CreatedAt      string `json:"createdAt"`
	Author         string `json:"author,omitempty"`
	ReportURL      string `json:"reportUrl,omitempty"`
	AnalysisResult string `json:"analysisResult,omitempty"`
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

	c.JSON(http.StatusOK, reports)
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

	execReportMu.Lock()
	defer execReportMu.Unlock()

	if err := sqlUpsertJSON("execution_reports", r); err != nil {
		return r, err
	}
	return r, nil
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
