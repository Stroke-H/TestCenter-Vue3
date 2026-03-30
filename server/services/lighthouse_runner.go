package services

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// RunLighthouseHandler upgrades to WebSocket and executes lighthouse scan
func RunLighthouseHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()

	targetURL := c.Query("url")
	if targetURL == "" {
		ws.WriteMessage(websocket.TextMessage, []byte("Error: No URL provided"))
		return
	}

	ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("[INFO] 准备开始 Lighthouse 分析: %s", targetURL)))
	ws.WriteMessage(websocket.TextMessage, []byte("--------------------------------------------------"))

	// Ensure reports directory exists
	reportDir := filepath.Join("..", "k6-scripts", "reports")
	reportName := "lighthouse_report.html"
	reportPath := filepath.Join(reportDir, reportName)
	absReportPath, _ := filepath.Abs(reportPath)

	// Execute Lighthouse command
	// chrome-flags: --no-sandbox is essential for many server environments
	cmd := exec.Command("lighthouse", 
		targetURL, 
		"--output", "html", 
		"--output-path", absReportPath, 
		"--chrome-flags=--no-sandbox --headless --disable-gpu",
		"--quiet", // Avoid too much noise if not needed, but bufio will catch it anyway
	)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\n[ERROR] 启动 Lighthouse 失败: %v", err)))
		return
	}

	// Stream logs from both stdout and stderr (Lighthouse often logs to stderr)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			ws.WriteMessage(websocket.TextMessage, []byte(scanner.Text()))
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		ws.WriteMessage(websocket.TextMessage, []byte(scanner.Text()))
	}

	if err := cmd.Wait(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\n[WARN] Lighthouse 执行过程中出现警告或错误: %v", err)))
	}

	ws.WriteMessage(websocket.TextMessage, []byte("\n--------------------------------------------------"))
	ws.WriteMessage(websocket.TextMessage, []byte("[SUCCESS] Lighthouse 分析任务执行完成。"))
	
	// Special token for frontend to identify report completion
	ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("REPORT_READY:%s", reportName)))
}
