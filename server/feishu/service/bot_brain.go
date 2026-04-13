package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"testcenter-server/feishu/model"
	"testcenter-server/services"

	"github.com/sashabaranov/go-openai"
)

func shouldPassthroughToolResult(toolName string) bool {
	switch toolName {
	case "generate_acceptance_report", "generate_smart_test_cases_from_doc":
		return true
	default:
		return false
	}
}

func isConfirmation(msg string) bool {
	m := strings.TrimSpace(strings.ToLower(msg))

	// 1. Explicit Negation Check (Higher priority)
	negationWords := []string{"不", "取消", "算了", "不要", "拒绝", "no", "stop", "cancel", "n"}
	for _, word := range negationWords {
		if strings.Contains(m, word) {
			// Special case: "不得不" or "确定不取消" are complex, but for simplicity here we treat any "no" as non-confirmation
			// Check if "不" is part of the confirmation, e.g., "不得不删" (rare in chat)
			// But "先不删除了" clearly contains "不" and should return false.
			return false
		}
	}

	// 2. Simple Direct Confirmations
	confirmWords := []string{"是", "确认", "确定", "对", "嗯", "好的", "执行", "yes", "y", "ok", "go", "sure"}
	for _, word := range confirmWords {
		if m == word {
			return true
		}
	}

	// 3. Destructive Confirmation Phrases
	// Must contain both a confirmation verb and the action keyword
	if (strings.Contains(m, "删") || strings.Contains(m, "执行")) &&
		(strings.Contains(m, "确认") || strings.Contains(m, "帮我") || strings.Contains(m, "确定") || strings.Contains(m, "立即")) {
		return true
	}

	return false
}

// ProcessChat handles the whole conversation turn for a user message
// ProcessChat handles the whole conversation turn for a user message
func ProcessChat(ctx context.Context, chatID string, senderID string, userMessage string) string {
	config := model.GlobalAIConfig
	if config == nil || config.APIKey == "" {
		return "⚠️ DeepSeek API Key 未配置或为空，请先 ... "
	}

	bypassConstraints := IsDTeacherMode(userMessage)
	normalizedMessage := userMessage
	if bypassConstraints {
		normalizedMessage = StripDTeacherPrefix(userMessage)
		if normalizedMessage == "" {
			normalizedMessage = "请和我自然聊聊，不要调用工具。"
		}
	}

	// 1. Get Session
	session := GetOrCreateSession(chatID)

	// 2. Check if waiting for confirmation
	if session.State == StateWaitConfirm && session.PendingTool != nil {
		if isConfirmation(userMessage) {
			// Execute the tool
			toolCall := session.PendingTool
			toolDef, exists := Registry[toolCall.Function.Name]
			if !exists {
				session.State = StateNormal
				session.PendingTool = nil
				return "工具执行失败：找不到对应的工具定义。"
			}

			log.Printf("[Bot Brain] User confirmed execution of %s", toolDef.Name)
			result, err := toolDef.Execute(ctx, session.ChatID, senderID, toolCall.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("执行失败: %v", err)
			}

			// Feed result back to LLM
			session.AppendMessage(openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				Name:       toolCall.Function.Name,
				ToolCallID: toolCall.ID,
			})

			// Reset state
			session.State = StateNormal
			session.PendingTool = nil

			if shouldPassthroughToolResult(toolCall.Function.Name) {
				session.AppendMessage(openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: result,
				})
				return result
			}

			// Call LLM again for final answer
			return callDeepSeek(ctx, senderID, session, false)
		} else {
			// Treat as cancellation or new topic
			log.Printf("[Bot Brain] User did not confirm, canceling %s safely", session.PendingTool.Function.Name)

			session.AppendMessage(openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    "用户未给予明确操作确认或者直接发起了新话题，刚才的工具调用已自动取消无害化处理。请直接自然地响应用户接下来的这段最新对话即可。",
				Name:       session.PendingTool.Function.Name,
				ToolCallID: session.PendingTool.ID,
			})
			session.State = StateNormal
			session.PendingTool = nil

			// Process this message as a NORMAL turn
			session.AppendMessage(openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: normalizedMessage,
			})
			return callDeepSeek(ctx, senderID, session, bypassConstraints)
		}
	}

	// 3. Normal turn
	session.AppendMessage(openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: normalizedMessage,
	})

	return callDeepSeek(ctx, senderID, session, bypassConstraints)
}

func callDeepSeek(ctx context.Context, senderID string, session *SessionContext, bypassConstraints bool) string {
	config := model.GlobalAIConfig

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second,
	}

	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = config.BaseURL
	clientConfig.HTTPClient = httpClient
	cli := openai.NewClientWithConfig(clientConfig)

	// Resolve Identity Context
	systemContext := "【系统状态报告】未在库中找到当前用户的绑定信息。如果用户尝试执行需要权限的操作，请引导其发送“绑定 用户名”。"

	// Try Feishu lookup first
	platformUser, _ := services.FindUserByFeishuOpenID(senderID)
	if platformUser == nil {
		// If not found, try direct platform ID lookup (common for Web Chat)
		platformUser, _ = services.GetUserByID(senderID)
	}

	if platformUser != nil {
		name := platformUser.Nickname
		if name == "" {
			name = platformUser.Username
		}
		// If it's a web session, we can be even more assertive about the identity
		isWeb := strings.HasPrefix(session.ChatID, "WEB_")
		channelMsg := "飞书已验证用户"
		if isWeb {
			channelMsg = "平台 Web 端已登录用户"
		}

		systemContext = fmt.Sprintf("【系统状态报告】当前对话用户已验证身份 (%s)。其平台用户名为: %s，昵称是: %s。你已拥有完整执行权限，请直接协助其完成任务，严禁再次询问其是否已完成绑定。",
			channelMsg, platformUser.Username, name)
	}

	activeSystemPrompt := GetSystemPrompt()
	if bypassConstraints {
		activeSystemPrompt = GetRelaxedSystemPrompt()
	}

	history := session.History
	if len(history) > 0 && history[0].Role == openai.ChatMessageRoleSystem {
		history = history[1:]
	}
	history = normalizeMessagesForDeepSeek(history)

	// Build temporary messages including identity context
	messages := make([]openai.ChatCompletionMessage, 0, len(history)+2)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemContext,
	})
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: activeSystemPrompt,
	})
	messages = append(messages, history...)

	req := openai.ChatCompletionRequest{
		Model:       config.Model,
		Messages:    messages,
		Temperature: 0.1,
	}
	req.Tools = GetAvailableTools()

	var resp openai.ChatCompletionResponse
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		resp, err = cli.CreateChatCompletion(ctx, req)
		if err == nil {
			break
		}
		log.Printf("[Bot Brain] API Error on attempt %d: %v", i+1, err)

		// On 400 errors (broken history), clear session and rebuild with just the last user message
		if strings.Contains(err.Error(), "400") && strings.Contains(err.Error(), "tool") {
			log.Printf("[Bot Brain] Detected corrupted history (orphaned tool messages). Resetting session.")
			// Find the last user message
			lastUserMsg := ""
			for j := len(session.History) - 1; j >= 0; j-- {
				if session.History[j].Role == openai.ChatMessageRoleUser {
					lastUserMsg = session.History[j].Content
					break
				}
			}
			// Reset the session history
			session.History = []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: GetSystemPrompt()},
			}
			if lastUserMsg != "" {
				session.AppendMessage(openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: lastUserMsg,
				})
			}
			// Rebuild request with cleaned history
			history = normalizeMessagesForDeepSeek(session.History)
			messages = make([]openai.ChatCompletionMessage, 0, len(history)+2)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemContext,
			})
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: activeSystemPrompt,
			})
			messages = append(messages, history...)
			req.Messages = messages
			time.Sleep(time.Second)
			continue
		}

		if !strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "connection reset") {
			break
		}
		time.Sleep(time.Second * time.Duration(i+1))
	}

	if err != nil {
		return fmt.Sprintf("AI 大脑思考失败 (网络/API异常): %v", err)
	}

	choice := resp.Choices[0]
	msg := choice.Message

	// Add assistant's message to history
	session.AppendMessage(msg)

	// Check if tool call requested
	if len(msg.ToolCalls) > 0 {
		// First pass: look for any tool that requires confirmation
		for _, toolCall := range msg.ToolCalls {
			toolDef, exists := Registry[toolCall.Function.Name]
			if exists && toolDef.NeedConfirm {
				// We currently only support confirming one tool at a time logic-wise
				session.State = StateWaitConfirm
				// Make a copy to avoid pointer scope issues
				tc := toolCall 
				session.PendingTool = &tc

				prompt := toolDef.ConfirmPrompt
				if toolDef.BuildConfirmMsg != nil {
					if dynPrompt, err := toolDef.BuildConfirmMsg(ctx, toolCall.Function.Arguments); err == nil {
						prompt = dynPrompt
					} else {
						prompt = fmt.Sprintf("⚠️ [操作预览失败] %v\n您可以回复【是】继续强行执行，或回复【否】中止操作。", err)
					}
				} else if prompt != "" {
					prompt = fmt.Sprintf(prompt, toolCall.Function.Arguments)
				}
				return prompt
			}
		}

		// Execute all tools if none need confirmation
		for _, toolCall := range msg.ToolCalls {
			toolDef, exists := Registry[toolCall.Function.Name]
			result := ""
			if !exists {
				result = fmt.Sprintf("AI 请求了一个不存在的系统工具: %s", toolCall.Function.Name)
			} else {
				res, err := toolDef.Execute(ctx, session.ChatID, senderID, toolCall.Function.Arguments)
				if err != nil {
					result = fmt.Sprintf("执行失败: %v", err)
				} else {
					result = res
				}
			}

			session.AppendMessage(openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				Name:       toolCall.Function.Name,
				ToolCallID: toolCall.ID,
			})

			if len(msg.ToolCalls) == 1 && shouldPassthroughToolResult(toolCall.Function.Name) {
				session.AppendMessage(openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: result,
				})
				return result
			}
		}
		
		// Recursive call to get the final text response after ALL tool results are appended
		return callDeepSeek(ctx, senderID, session, false)
	}

	// Normal text reply
	return msg.Content
}

// normalizeMessagesForDeepSeek sanitizes the message history to ensure it is valid:
// 1. Ensures assistant messages always have non-empty content.
// 2. Ensures tool messages always have non-empty content.
// 3. Removes orphaned tool messages (tool messages without a preceding assistant message with matching tool_calls).
func normalizeMessagesForDeepSeek(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	normalized := make([]openai.ChatCompletionMessage, 0, len(messages))

	// Build a set of valid tool_call IDs from assistant messages
	validToolCallIDs := make(map[string]bool)

	for _, msg := range messages {
		fixed := msg

		if fixed.Role == openai.ChatMessageRoleAssistant {
			if fixed.Content == "" {
				fixed.Content = " "
			}
			// Track all tool_call IDs this assistant message declares
			for _, tc := range fixed.ToolCalls {
				validToolCallIDs[tc.ID] = true
			}
			normalized = append(normalized, fixed)
		} else if fixed.Role == openai.ChatMessageRoleTool {
			if fixed.Content == "" {
				fixed.Content = " "
			}
			// Only include this tool message if its ToolCallID is valid
			if fixed.ToolCallID != "" && validToolCallIDs[fixed.ToolCallID] {
				normalized = append(normalized, fixed)
			} else {
				log.Printf("[Bot Brain] Dropping orphaned tool message (ID=%s, Name=%s)", fixed.ToolCallID, fixed.Name)
			}
		} else {
			normalized = append(normalized, fixed)
		}
	}
	return normalized
}
