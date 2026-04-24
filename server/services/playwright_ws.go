package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"regexp"
	"testcenter-server/models"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/playwright-community/playwright-go"
)

// WSMessage defines the message structure sent to the frontend
type WSMessage struct {
	Type       string            `json:"type"` // "step_start", "step_pass", "step_fail", "screenshot", "log", "suite_done"
	StepIndex  int               `json:"step_index"`
	Keyword    string            `json:"keyword"`
	Args       map[string]string `json:"args"`
	Message    string            `json:"message"`
	Screenshot string            `json:"screenshot"` // Base64 image
	Timestamp  string            `json:"timestamp"`
	Error      string            `json:"error"`
}

// PlaywrightWSHandler upgrades the connection and executes the Playwright test case.
func PlaywrightWSHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()

	caseID := c.Query("caseId")
	if caseID == "" {
		sendWSLog(ws, "Error: caseId is required", "error")
		return
	}

	// Load Case
	cs, err := getPlaywrightCaseByID(caseID)
	if err != nil {
		sendWSLog(ws, fmt.Sprintf("Error: %v", err), "error")
		return
	}

	executePlaywrightCase(ws, cs)
}

func getPlaywrightCaseByID(id string) (*models.PlaywrightCase, error) {
	cases, err := loadPlaywrightRecords[models.PlaywrightCase](pwCaseTable)
	if err != nil {
		return nil, err
	}
	for _, cs := range cases {
		if cs.ID == id {
			return &cs, nil
		}
	}
	return nil, fmt.Errorf("case not found")
}

func sendWSLog(ws *websocket.Conn, msg string, msgType string) {
	data := WSMessage{
		Type:      "log",
		Message:   msg,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if msgType == "error" {
		data.Error = msg
	}
	_ = ws.WriteJSON(data)
}

func executePlaywrightCase(ws *websocket.Conn, cs *models.PlaywrightCase) {
	sendWSLog(ws, fmt.Sprintf("Starting test case: %s", cs.Name), "log")

	// Initialize Playwright
	pw, err := playwright.Run()
	if err != nil {
		sendWSLog(ws, fmt.Sprintf("Could not start playwright: %v", err), "error")
		return
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true), // Default to headless
	})
	if err != nil {
		sendWSLog(ws, fmt.Sprintf("Could not launch browser: %v", err), "error")
		return
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		sendWSLog(ws, fmt.Sprintf("Could not create page: %v", err), "error")
		return
	}

	// Execution context for variables
	vars := make(map[string]string)
	for k, v := range cs.Variables {
		vars[k] = v
	}

	keywordMap := GetKeywordMap()

	for i, step := range cs.Steps {
		if step.Disabled {
			_ = ws.WriteJSON(WSMessage{Type: "log", Message: fmt.Sprintf("Skipping disabled step %d: %s", i+1, step.Keyword)})
			continue
		}

		// step_start
		_ = ws.WriteJSON(WSMessage{
			Type:      "step_start",
			StepIndex: i,
			Keyword:   step.Keyword,
			Message:   fmt.Sprintf("Executing step %d: %s", i+1, step.Keyword),
		})

		// Substitute variables in args
		finalArgs := make(map[string]string)
		for k, v := range step.Args {
			finalArgs[k] = substituteVars(v, vars)
		}

		executor, exists := keywordMap[step.Keyword]
		if !exists {
			errMsg := fmt.Sprintf("Error: Unknown keyword '%s'", step.Keyword)
			_ = ws.WriteJSON(WSMessage{Type: "step_fail", StepIndex: i, Error: errMsg})
			break
		}

		// Execute keyword
		resultText, err := executor(context.Background(), page, finalArgs)

		if err != nil {
			_ = ws.WriteJSON(WSMessage{Type: "step_fail", StepIndex: i, Error: err.Error()})
			break
		}

		// Handle ReturnVar
		if step.ReturnVar != "" {
			vars[step.ReturnVar] = resultText
		}

		// Handle Screenshot (Special case for the 'Screenshot' keyword or automatic)
		var screenshotB64 string
		if step.Keyword == "Screenshot" {
			screenshotB64 = base64.StdEncoding.EncodeToString([]byte(resultText))
		} else {
			// Auto screenshot after each step
			img, _ := page.Screenshot()
			screenshotB64 = base64.StdEncoding.EncodeToString(img)
		}

		// step_pass
		_ = ws.WriteJSON(WSMessage{
			Type:       "step_pass",
			StepIndex:  i,
			Message:    resultText,
			Screenshot: screenshotB64,
			Timestamp:  time.Now().Format(time.RFC3339),
		})
	}

	_ = ws.WriteJSON(WSMessage{
		Type:    "suite_done",
		Message: "Test case execution completed",
	})
}

func substituteVars(input string, vars map[string]string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(input, func(m string) string {
		varName := m[2 : len(m)-1]
		if val, ok := vars[varName]; ok {
			return val
		}
		return m
	})
}
