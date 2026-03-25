package controller

import (
	"context"
	"net/http"
	"testcenter-server/feishu/model"
	"testcenter-server/feishu/service"
	"time"

	"github.com/gin-gonic/gin"
)

// GetOperationLogsHandler returns the recent AI operation logs
func GetOperationLogsHandler(c *gin.Context) {
	logs, err := service.GetOperationLogs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}

// WebChatHandler handles AI chat requests from the dashboard
func WebChatHandler(c *gin.Context) {
	var req struct {
		Message   string `json:"message"`
		SessionID string `json:"session_id"`
		UserID    string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.SessionID == "" {
		req.SessionID = "WEB_DEFAULT"
	}

	// Process chat using the existing AI brain
	// We prefix session ID to distinguish from Feishu bot sessions
	fullSessionID := "WEB_" + req.SessionID
	response := service.ProcessChat(context.Background(), fullSessionID, req.UserID, req.Message)

	// Check if any tool result (like the acceptance report JSON) was generated in this turn
	var toolResult string
	session := service.GetOrCreateSession(fullSessionID)
	if session != nil {
		toolResult = session.LastToolResult
		session.LastToolResult = "" // Clear after use
	}

	c.JSON(http.StatusOK, gin.H{
		"reply":       string(response),
		"tool_result": toolResult,
	})
}

// EndWebChatHandler finalizes a web chat session and saves its history
func EndWebChatHandler(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id"`
		UserID    string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	fullSessionID := "WEB_" + req.SessionID
	session := service.GetOrCreateSession(fullSessionID)

	if session != nil {
		// Convert service.SessionContext to model.AIChatSession
		chatSession := model.AIChatSession{
			SessionID: req.SessionID,
			UserID:    req.UserID,
			StartTime: session.LastActive.Add(-10 * time.Minute), // Approximation
			EndTime:   time.Now(),
			History:   []model.ChatMessage{},
		}

		for _, m := range session.History {
			chatSession.History = append(chatSession.History, model.ChatMessage{
				Role:      string(m.Role),
				Content:   m.Content,
				Timestamp: time.Now(),
			})
		}

		service.SaveChatSession(chatSession)
		service.ClearSession(fullSessionID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session saved and cleared"})
}

// GetTestPhonesHandler returns all migrated test phones
func GetTestPhonesHandler(c *gin.Context) {
	phones := model.GetTestPhones()
	c.JSON(http.StatusOK, phones)
}
