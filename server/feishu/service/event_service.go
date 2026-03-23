package service

import (
	"context"
	"log"
	"strconv"
	"testcenter-server/feishu/model"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"time"
)

// NewEventDispatcher creates a dispatcher for handle events
func NewEventDispatcher(verifyToken, encryptKey string) *dispatcher.EventDispatcher {
	return dispatcher.NewEventDispatcher(verifyToken, encryptKey).
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			handleMessageReceive(ctx, event)
			return nil
		})
}

// handleMessageReceive processes the incoming message and converts it to standardized output
func handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) {
	msg := event.Event.Message
	if msg == nil {
		return
	}

	// 1. Basic Info
	createTime, _ := strconv.ParseInt(*msg.CreateTime, 10, 64)
	stdMsg := &model.StandardizedMessage{
		EventID:    event.RequestId(),
		MsgID:      *msg.MessageId,
		ChatID:     *msg.ChatId,
		ChatType:   *msg.ChatType,
		SenderID:   *event.Event.Sender.SenderId.OpenId,
		MsgType:    *msg.MessageType,
		RawContent: *msg.Content,
		Timestamp:  createTime,
		ReceivedAt: time.Now(),
	}

	// 2. Parse Content
	content, err := ParseMessageContent(stdMsg.MsgType, stdMsg.RawContent)
	if err == nil {
		stdMsg.Content = content
	}

	// 3. Extract Mentions
	stdMsg.Mentions = ExtractMentions(msg.Mentions)

	// 4. Output (For Phase 1, we just log it as a standardized JSON)
	log.Printf("[Feishu Standardized Output] User: %s, Content: %s, ChatID: %s\n", 
		stdMsg.SenderID, stdMsg.Content, stdMsg.ChatID)
	
	// Prettified JSON log for verification
	log.Println(larkcore.Prettify(stdMsg))
}
