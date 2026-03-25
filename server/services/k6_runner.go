package services

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all cross-origin connections
	},
}

// RunK6TestHandler upgrades the HTTP request to a WebSocket and executes the K6 test
func RunK6TestHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()

	// Get parameters from query
	scriptName := c.DefaultQuery("script", "episode.js")
	email := c.Query("email")
	password := c.Query("password")
	loginUrl := c.Query("loginUrl")
	dramaListUrl := c.Query("dramaListUrl")

	// 统一工作目录：由于后端在 server/ 下运行，我们将所有脚本执行的根路径设为项目根目录 (..)
	rootDir, _ := filepath.Abs("..")
	scriptPath := filepath.Join("k6-scripts", scriptName)
	prepareScriptPath := filepath.Join("k6-scripts", "prepare_data.js")

	if _, err := os.Stat(filepath.Join(rootDir, scriptPath)); os.IsNotExist(err) {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: Script %s not found", scriptName)))
		return
	}

	// Dynamic Env passing
	env := os.Environ()
	if email != "" {
		env = append(env, "EMAIL="+email)
	}
	if password != "" {
		env = append(env, "PASSWORD="+password)
	}
	if loginUrl != "" {
		env = append(env, "LOGIN_URL="+loginUrl)
	}
	if dramaListUrl != "" {
		env = append(env, "DRAMA_LIST_URL="+dramaListUrl)
	}

	// Special orchestration: if running drama_check_flow, run prepare_data FIRST
	if scriptName == "drama_check_flow.js" {
		ws.WriteMessage(websocket.TextMessage, []byte("[Step 1/2] 正在初始化前置数据业务 (prepare_data.js)..."))
		prepCmd := exec.Command("node", prepareScriptPath)
		prepCmd.Env = env
		prepCmd.Dir = rootDir

		stdoutPipe, _ := prepCmd.StdoutPipe()
		prepCmd.Stderr = prepCmd.Stdout

		if err := prepCmd.Start(); err != nil {
			ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\n[❌] 启动数据准备脚本失败: %v", err)))
			return
		}

		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			ws.WriteMessage(websocket.TextMessage, []byte(scanner.Text()))
		}

		if err := prepCmd.Wait(); err != nil {
			ws.WriteMessage(websocket.TextMessage, []byte("\n[❌] 前置数据准备出现异常，终止后续执行。"))
			return
		}
		ws.WriteMessage(websocket.TextMessage, []byte("\n[Step 2/2] 数据初始化成功，开始并发检测环节..."))
	}

	ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Ready to execute K6 test: %s", scriptName)))
	ws.WriteMessage(websocket.TextMessage, []byte("----------------------------------------"))

	// Execute K6 command
	cmd := exec.Command("k6", "run", scriptPath)
	cmd.Env = env
	cmd.Dir = rootDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to get stdout: %v", err)))
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to get stderr: %v", err)))
		return
	}

	if err := cmd.Start(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to start K6 output: %v", err)))
		return
	}

	// Read standard output pipeline
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			text := scanner.Text()
			ws.WriteMessage(websocket.TextMessage, []byte("[WARN/ERR] "+text))
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		text := scanner.Text()
		cleanText := removeANSI(text)
		err := ws.WriteMessage(websocket.TextMessage, []byte(cleanText))
		if err != nil {
			log.Println("Write error:", err)
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\nCommand finished with error: %v", err)))
	} else {
		ws.WriteMessage(websocket.TextMessage, []byte("\nExecution completed successfully."))
	}
}

// removeANSI removes ANSI escape codes from a string to clean up K6 CLI output
func removeANSI(str string) string {
	// Simple implementation; you might need a regex package for full compliance
	// \x1b\[[0-9;]*m
	replacer := strings.NewReplacer("\x1b[0m", "", "\x1b[31m", "", "\x1b[32m", "", "\x1b[33m", "", "\x1b[34m", "", "\x1b[35m", "", "\x1b[36m", "", "\x1b[90m", "")
	return replacer.Replace(str)
}
