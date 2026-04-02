package service

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testcenter-server/feishu/model"
	"testcenter-server/services"
	"time"
)

var (
	logMutex      sync.Mutex
	logPath       = "data/ai_operation_logs.jsonl"
	chatLogPath   = "data/ai_chat_histories.jsonl"
	reportLogPath = "data/acceptance_reports.jsonl"
)

// AddOperationLog appends a new operation record to the persistent log file
func AddOperationLog(op model.AIOperationLog) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(logPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[AI Logger] Failed to open log file: %v", err)
		return err
	}
	defer f.Close()

	data, err := json.Marshal(op)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}
	_, _ = f.WriteString("\n")

	return nil
}

// GetOperationLogs returns the last 100 operation logs (or all if fewer)
func GetOperationLogs() ([]model.AIOperationLog, error) {
	logMutex.Lock()
	defer logMutex.Unlock()

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.AIOperationLog{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var logs []model.AIOperationLog
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var l model.AIOperationLog
		if err := decoder.Decode(&l); err == nil {
			logs = append(logs, l)
		}
	}

	enrichLegacyDeleteOperationLogs(logs)

	// Reverse to show newest first
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	// Limit to 100
	if len(logs) > 100 {
		return logs[:100], nil
	}

	return logs, nil
}

type deleteRequestTrace struct {
	SenderID  string
	Timestamp time.Time
}

func enrichLegacyDeleteOperationLogs(logs []model.AIOperationLog) {
	traces := loadDeleteRequestTraces()
	if len(traces) == 0 {
		return
	}

	userCache := map[string]*services.User{}
	for i := range logs {
		logEntry := &logs[i]
		if logEntry.ToolName != "delete_account" {
			continue
		}
		if logEntry.UserName != "" {
			continue
		}
		if !strings.Contains(logEntry.Detail, "Account "+logEntry.UserID+" deleted") {
			continue
		}

		trace := findNearestDeleteTraceBefore(traces, logEntry.Timestamp)
		if trace == nil {
			continue
		}

		boundUser, ok := userCache[trace.SenderID]
		if !ok {
			boundUser, _ = services.FindUserByFeishuOpenID(trace.SenderID)
			userCache[trace.SenderID] = boundUser
		}
		if boundUser == nil {
			continue
		}

		logEntry.UserID = boundUser.ID
		logEntry.UserName = boundUser.Username
	}
}

func loadDeleteRequestTraces() []deleteRequestTrace {
	f, err := os.Open("data/feishu_messages.jsonl")
	if err != nil {
		return nil
	}
	defer f.Close()

	type feishuMessage struct {
		SenderID  string `json:"sender_id"`
		Content   string `json:"content"`
		Timestamp int64  `json:"timestamp"`
	}

	var traces []deleteRequestTrace
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var msg feishuMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		if msg.Timestamp == 0 {
			continue
		}
		if !strings.Contains(msg.Content, "删除这个账号") && !strings.Contains(msg.Content, "删除账号") {
			continue
		}

		traces = append(traces, deleteRequestTrace{
			SenderID:  msg.SenderID,
			Timestamp: time.UnixMilli(msg.Timestamp),
		})
	}

	return traces
}

func findNearestDeleteTraceBefore(traces []deleteRequestTrace, operationTime time.Time) *deleteRequestTrace {
	const matchWindow = 5 * time.Minute

	var best *deleteRequestTrace
	for i := range traces {
		trace := &traces[i]
		if trace.Timestamp.After(operationTime) {
			continue
		}
		if operationTime.Sub(trace.Timestamp) > matchWindow {
			continue
		}
		if best == nil || trace.Timestamp.After(best.Timestamp) {
			best = trace
		}
	}

	return best
}

// SaveChatSession saves a completed chat session to the persistent log file
func SaveChatSession(session model.AIChatSession) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	dir := filepath.Dir(chatLogPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}

	f, err := os.OpenFile(chatLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}
	_, _ = f.WriteString("\n")

	return nil
}
