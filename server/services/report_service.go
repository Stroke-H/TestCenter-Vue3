package services

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// --- Models ---

type AcceptanceReport struct {
	ID                   string    `json:"id"`
	ProjectName          string    `json:"project_name"`
	ProjectCode          string    `json:"project_code"`
	Version              string    `json:"version"`
	TestOwner            string    `json:"test_owner"`
	Reporter             string    `json:"reporter"` // Alias for TestOwner
	TestTime             string    `json:"test_time"`
	TestEnv              string    `json:"test_env"`
	TestConclusion       string    `json:"test_conclusion"`
	BugFixStatus         string    `json:"bug_fix_status"`
	BugSubmissionStatus  string    `json:"bug_submission_status"`
	UpdateRequirements   string    `json:"update_requirements"`
	CreatedAt            string    `json:"created_at"`
	UpdatedAt            string    `json:"updated_at"`
	Status               string    `json:"status"`
}

// --- Service Logic ---

var (
	reportMutex   sync.Mutex
	reportLogPath = "data/acceptance_reports.jsonl"
)

func SaveAcceptanceReport(report AcceptanceReport) error {
	reportMutex.Lock()
	defer reportMutex.Unlock()

	dir := filepath.Dir(reportLogPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}

	f, err := os.OpenFile(reportLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(report)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}
	_, _ = f.WriteString("\n")

	return nil
}

func GetAcceptanceReports() ([]AcceptanceReport, error) {
	reportMutex.Lock()
	defer reportMutex.Unlock()

	f, err := os.Open(reportLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []AcceptanceReport{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var reports []AcceptanceReport
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var r AcceptanceReport
		if err := decoder.Decode(&r); err == nil {
			reports = append(reports, r)
		}
	}

	// Reverse to show newest first
	for i, j := 0, len(reports)-1; i < j; i, j = i+1, j-1 {
		reports[i], reports[j] = reports[j], reports[i]
	}

	return reports, nil
}

// --- Handlers ---

func SaveAcceptanceReportHandler(c *gin.Context) {
	var req AcceptanceReport
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.CreatedAt == "" {
		req.CreatedAt = time.Now().Format(time.RFC3339)
	}
	if req.Status == "" {
		req.Status = "Completed"
	}

	if err := SaveAcceptanceReport(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Acceptance report saved successfully"})
}

func GetAcceptanceReportsHandler(c *gin.Context) {
	reports, err := GetAcceptanceReports()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reports)
}
