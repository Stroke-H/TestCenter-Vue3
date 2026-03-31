package services

import (
	"bufio"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	ReportURL      string `json:"reportUrl,omitempty"`
	AnalysisResult string `json:"analysisResult,omitempty"`
}

var (
	// 数据文件路径
	execReportsFile = filepath.Join("data", "execution_reports.jsonl")
	execReportMu    sync.Mutex
)

// GetExecutionReportsHandler 获取所有执行历史记录 (从文件读取)
func GetExecutionReportsHandler(c *gin.Context) {
	execReportMu.Lock()
	defer execReportMu.Unlock()

	file, err := os.Open(execReportsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空列表
			c.JSON(http.StatusOK, []ExecutionReport{})
			return
		}
		log.Println("[ERROR] Open exec reports failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open reports file"})
		return
	}
	defer file.Close()

	var reports []ExecutionReport
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var r ExecutionReport
		if err := json.Unmarshal([]byte(scanner.Text()), &r); err == nil {
			reports = append(reports, r)
		}
	}

	// 精准排序：按 ID 倒序排列 (ID 格式包含 YYYYMMDDHHMMSS，天然支持时间排序)
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ID > reports[j].ID
	})

	c.JSON(http.StatusOK, reports)
}

// AddExecutionReportHandler 保存一条新的执行记录到文件
func AddExecutionReportHandler(c *gin.Context) {
	var r ExecutionReport
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 由后端生成 ID 和创建时间（如果前端没传）
	if r.ID == "" {
		r.ID = "REP-" + time.Now().Format("20060102150405")
	}
	if r.CreatedAt == "" {
		// 格式化为：2026/03/30 15:42:56 (使用标准补齐格式有利于排序和展示)
		r.CreatedAt = time.Now().Format("2006/01/02 15:04:05")
	}

	execReportMu.Lock()
	defer execReportMu.Unlock()

	// 确保 data 目录存在
	os.MkdirAll("data", 0755)

	file, err := os.OpenFile(execReportsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("[ERROR] Save exec report failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open reports file for writing"})
		return
	}
	defer file.Close()

	data, _ := json.Marshal(r)
	if _, err := file.WriteString(string(data) + "\n"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write report line"})
		return
	}

	c.JSON(http.StatusOK, r)
}

// ClearExecutionReportsHandler 清空所有执行记录
func ClearExecutionReportsHandler(c *gin.Context) {
	execReportMu.Lock()
	defer execReportMu.Unlock()

	if err := os.Truncate(execReportsFile, 0); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"message": "No reports file to clear"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear reports file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All execution records cleared"})
}
