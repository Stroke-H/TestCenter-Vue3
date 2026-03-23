package model

import "time"

// StandardizedMessage is the output of the Feishu Bot Bridge
type StandardizedMessage struct {
	EventID    string    `json:"event_id"`    // 事件唯一 ID
	MsgID      string    `json:"msg_id"`      // 消息 ID
	ChatID     string    `json:"chat_id"`     // 会话 ID
	ChatType   string    `json:"chat_type"`   // "p2p" | "group"
	SenderID   string    `json:"sender_id"`   // 发送者 Open ID
	SenderName string    `json:"sender_name"` // 发送者名称（如可获取）
	MsgType    string    `json:"msg_type"`    // "text" | "post" | "image" | "file" 等
	Content    string    `json:"content"`     // 消息文本内容（已解析）
	RawContent string    `json:"raw_content"` // 原始 JSON 内容
	Mentions   []Mention `json:"mentions"`    // @提及列表
	Timestamp  int64     `json:"timestamp"`   // 消息时间戳
	ReceivedAt time.Time `json:"received_at"` // 服务端接收时间
}

// Mention represents a user mentioned in a message
type Mention struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Key    string `json:"key"` // @_user_1 等标识
}
