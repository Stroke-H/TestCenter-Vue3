package services

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

	// Ensure reports directory exists at project root (relative to server/ dir is ../report)
	reportDir := filepath.Join("..", "report")
	if _, err := os.Stat(reportDir); os.IsNotExist(err) {
		os.MkdirAll(reportDir, 0755)
	}

	// Extract filename from URL (e.g., 'example' from 'https://example.com')
	hostname := "report"
	u, err := url.Parse(targetURL)
	if err == nil {
		host := u.Hostname()
		if host == "" {
			// If parse fails or protocol missing, try a simpler split
			host = strings.Split(targetURL, "/")[0]
		}
		
		// Remove 'www.' if present and take the main segment
		host = strings.TrimPrefix(host, "www.")
		parts := strings.Split(host, ".")
		if len(parts) > 0 {
			hostname = parts[0]
		}
	}
	
	// Sanitize hostname for filename
	hostname = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, hostname)

	// 使用时间戳确保文件唯一性，防止不同次测试报告重名覆盖
	timestamp := time.Now().Format("20060102150405")
	reportBaseName := fmt.Sprintf("%s_%s", hostname, timestamp)
	reportPath := filepath.Join(reportDir, reportBaseName)
	absReportPath, _ := filepath.Abs(reportPath)

	// Execute Lighthouse command with HTML and JSON outputs
	// Lighthouse will append '.report.html' and '.report.json' to the output-path
	cmd := exec.Command("lighthouse", 
		targetURL, 
		"--output", "html", 
		"--output", "json", 
		"--output-path", absReportPath, 
		"--chrome-flags=--no-sandbox --headless --disable-gpu",
		"--quiet",
	)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\n[ERROR] 启动 Lighthouse 失败: %v", err)))
		return
	}

	// Stream logs from both stdout and stderr
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
	
	// Notification for frontend with the dynamic filename
	// Lighthouse adds .report.html
	finalHtmlReport := fmt.Sprintf("%s.report.html", reportBaseName)
	ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("REPORT_READY:%s", finalHtmlReport)))
}
