package service

import (
	"log"
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
	if err := services.SQLUpsertJSONForFeishu("ai_operation_logs", op); err != nil {
		log.Printf("[AI Logger] Failed to open log file: %v", err)
		return err
	}
	return nil
}

// GetOperationLogs returns the last 100 operation logs (or all if fewer)
func GetOperationLogs() ([]model.AIOperationLog, error) {
	logMutex.Lock()
	defer logMutex.Unlock()

	logs, err := services.SQLListJSONForFeishu[model.AIOperationLog]("ai_operation_logs", "`migrated_at` ASC")
	if err != nil {
		return nil, err
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
	messages, err := services.SQLListJSONForFeishu[model.StandardizedMessage]("feishu_messages", "`migrated_at` ASC")
	if err != nil {
		return nil
	}

	var traces []deleteRequestTrace
	for _, msg := range messages {
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
	return services.SQLUpsertJSONForFeishu("ai_chat_histories", session)
}
