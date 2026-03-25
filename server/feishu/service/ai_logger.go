package service

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testcenter-server/feishu/model"
)

var (
	logMutex     sync.Mutex
	logPath          = "data/ai_operation_logs.jsonl"
	chatLogPath       = "data/ai_chat_histories.jsonl"
	reportLogPath     = "data/acceptance_reports.jsonl"
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
