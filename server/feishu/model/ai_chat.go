package model

import "time"

// AIChatSession represents a full conversation session record
type AIChatSession struct {
	SessionID string        `json:"session_id"`
	UserID    string        `json:"user_id"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	History   []ChatMessage `json:"history"`
}

type ChatMessage struct {
	Role      string    `json:"role"` // "user" | "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}
