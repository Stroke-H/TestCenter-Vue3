package model

import "time"

// AIOperationLog represents a record of a successful tool execution by the AI
type AIOperationLog struct {
	ID        string    `json:"id"`
	ToolName  string    `json:"tool_name"`
	Project   string    `json:"project"`
	Env       string    `json:"env"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Status    string    `json:"status"` // "success" | "failed"
	Detail    string    `json:"detail"` // Summary of the result
	Timestamp time.Time `json:"timestamp"`
}
