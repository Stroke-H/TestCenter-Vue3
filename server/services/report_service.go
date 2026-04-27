package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	feishumodel "testcenter-server/feishu/model"

	"github.com/gin-gonic/gin"
)

// --- Models ---

type AcceptanceReport struct {
	ID                  string `json:"id"`
	ProjectName         string `json:"project_name"`
	ProjectCode         string `json:"project_code"`
	Version             string `json:"version"`
	TestOwner           string `json:"test_owner"`
	Reporter            string `json:"reporter"` // Alias for TestOwner
	TestTime            string `json:"test_time"`
	TestEnv             string `json:"test_env"`
	TestDevices         string `json:"test_devices"`
	TestConclusion      string `json:"test_conclusion"`
	BugFixStatus        string `json:"bug_fix_status"`
	BugSubmissionStatus string `json:"bug_submission_status"`
	UpdateRequirements  string `json:"update_requirements"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
	Status              string `json:"status"`
}

// --- Service Logic ---

var (
	reportMutex sync.Mutex
)

func SaveAcceptanceReport(report AcceptanceReport) error {
	reportMutex.Lock()
	defer reportMutex.Unlock()
	return sqlUpsertJSON("acceptance_reports", report)
}

func GetAcceptanceReports() ([]AcceptanceReport, error) {
	reportMutex.Lock()
	defer reportMutex.Unlock()

	reports, err := sqlListJSON[AcceptanceReport]("acceptance_reports", "`migrated_at` ASC")
	if err != nil {
		return nil, err
	}

	// Reverse to show newest first
	for i, j := 0, len(reports)-1; i < j; i, j = i+1, j-1 {
		reports[i], reports[j] = reports[j], reports[i]
	}

	return reports, nil
}

func GetAcceptanceReportByID(id string) (*AcceptanceReport, error) {
	reports, err := GetAcceptanceReports()
	if err != nil {
		return nil, err
	}

	for _, report := range reports {
		if report.ID == id {
			r := report
			return &r, nil
		}
	}

	return nil, fmt.Errorf("acceptance report not found")
}

func FormatAcceptanceReportText(report AcceptanceReport) string {
	formattedFixed := formatAcceptanceReportSection(report.BugFixStatus)
	formattedUnfixed := formatAcceptanceReportSection(report.BugSubmissionStatus)
	formattedStories := formatAcceptanceReportSection(report.UpdateRequirements)

	devicesBlock := ""
	if report.TestDevices != "" {
		devicesBlock = fmt.Sprintf("测试设备：%s\n", report.TestDevices)
	}

	conclusion := report.TestConclusion
	if conclusion == "" {
		conclusion = "Pass"
	}

	return fmt.Sprintf("%s项目验收报告\n\n"+
		"项目名称：%s\n"+
		"版本号：%s\n"+
		"测试负责人：%s\n"+
		"测试时间：%s\n"+
		"测试环境：%s\n"+
		"%s"+
		"本次测试覆盖率： 100%%\n\n"+
		"测试结论：当前版本%s！\n"+
		"正式版本缺陷修复验证情况：\n%s"+
		"本次预提审版本缺陷提交情况：\n%s"+
		"版本更新测试需求点：\n%s",
		report.ProjectCode,
		report.ProjectName,
		report.Version,
		report.TestOwner,
		report.TestTime,
		report.TestEnv,
		devicesBlock,
		conclusion,
		formattedFixed,
		formattedUnfixed,
		formattedStories,
	)
}

func formatAcceptanceReportSection(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		trimmed = "无"
	}
	return trimmed + "\n\n"
}

func sendFeishuGroupText(text string) error {
	if feishumodel.GlobalFeishuConfig == nil {
		config, err := feishumodel.LoadConfig("data/feishu_config.json")
		if err != nil {
			return err
		}
		feishumodel.GlobalFeishuConfig = config
	}

	config := feishumodel.GlobalFeishuConfig
	if config == nil || config.AppID == "" || config.AppSecret == "" || config.GroupID == "" {
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

	contentBody, err := json.Marshal(map[string]string{
		"text": text,
	})
	if err != nil {
		return err
	}

	msgPayload, err := json.Marshal(map[string]string{
		"receive_id": config.GroupID,
		"msg_type":   "text",
		"content":    string(contentBody),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id", bytes.NewBuffer(msgPayload))
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

func SendAcceptanceReportToFeishuHandler(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	currentUser, err := currentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user"})
		return
	}

	report, err := GetAcceptanceReportByID(req.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if report.Reporter != currentUser.Username {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the report owner can send this report to Feishu"})
		return
	}

	reportText := FormatAcceptanceReportText(*report)
	messageText := fmt.Sprintf("【%s 提交的验收报告】\n\n%s", currentUser.Username, reportText)
	if err := sendFeishuGroupText(messageText); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Acceptance report sent to Feishu successfully"})
}
