package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"testcenter-server/feishu/model"
	"testcenter-server/services"

	"github.com/sashabaranov/go-openai"
)

const (
	chatProviderRaceLimit     = 10
	chatProviderStaggerDelay  = 5 * time.Second
	chatProviderRequestTimout = 60 * time.Second
)

func shouldPassthroughToolResult(toolName string) bool {
	switch toolName {
	case "generate_acceptance_report", "generate_smart_test_cases_from_doc":
		return true
	default:
		return false
	}
}

func detectAIUseCase(message string) string {
	text := strings.ToLower(strings.TrimSpace(message))
	checks := []struct {
		capability string
		keywords   []string
	}{
		{"embedding", []string{"embedding", "embed", "向量", "嵌入", "语义检索", "语义向量", "相似度检索"}},
		{"reward", []string{"reward", "奖励模型", "回复打分", "回答打分", "评分模型", "helpfulness", "correctness"}},
		{"safety", []string{"安全分类", "内容安全", "安全检测", "违规检测", "moderation", "safety", "guardrail", "风控分类"}},
		{"vision", []string{"视觉", "图像", "图片", "看图", "识图", "image", "vision", "多模态"}},
		{"parse", []string{"解析", "文档解析", "pdf解析", "版面解析", "表格抽取", "ocr", "parse"}},
	}

	for _, check := range checks {
		for _, keyword := range check.keywords {
			if strings.Contains(text, strings.ToLower(keyword)) {
				return check.capability
			}
		}
	}
	return ""
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
	if config == nil || !config.HasConfiguredProvider() {
		return "⚠️ AI API Key 未配置或为空，请先检查 AI 配置。"
	}

	unlockMode := IsUnlockMode(userMessage)
	bypassConstraints := unlockMode || IsDTeacherMode(userMessage)
	normalizedMessage := userMessage
	if unlockMode {
		normalizedMessage = StripUnlockPrefix(userMessage)
		if normalizedMessage == "" {
			normalizedMessage = "请直接自然回答当前问题。"
		}
	} else if bypassConstraints {
		normalizedMessage = StripDTeacherPrefix(userMessage)
		if normalizedMessage == "" {
			normalizedMessage = "请和我自然聊聊，不要调用工具。"
		}
	}

	// 1. Get Session
	session := GetOrCreateSession(chatID)

	if capability := detectAIUseCase(normalizedMessage); capability != "" {
		session.AppendMessage(openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: normalizedMessage,
		})
		result := callSpecialAIModel(ctx, capability, normalizedMessage)
		session.AppendMessage(openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: result,
		})
		return result
	}

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
			return callDeepSeek(ctx, senderID, session, false, false)
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
			return callDeepSeek(ctx, senderID, session, bypassConstraints, unlockMode)
		}
	}

	// 3. Normal turn
	session.AppendMessage(openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: normalizedMessage,
	})

	return callDeepSeek(ctx, senderID, session, bypassConstraints, unlockMode)
}

func callDeepSeek(ctx context.Context, senderID string, session *SessionContext, bypassConstraints bool, unlockMode bool) string {
	config := model.GlobalAIConfig
	if config == nil {
		config = model.LoadAIConfig()
	}
	providers := config.EffectiveProviders()
	if len(providers) == 0 {
		return "AI 大脑思考失败：AI API Key 未配置或为空。"
	}
	raceProviders, fallbackProviders := splitChatProvidersForRacing(providers)

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   chatProviderRequestTimout,
	}

	activeSystemPrompt := GetSystemPrompt()
	if bypassConstraints {
		activeSystemPrompt = GetRelaxedSystemPrompt()
	}
	if unlockMode {
		activeSystemPrompt = ""
	}
	systemMessages := make([]openai.ChatCompletionMessage, 0, 2)
	if !unlockMode {
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

		systemMessages = append(systemMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemContext,
		})
		systemMessages = append(systemMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: activeSystemPrompt,
		})
	}

	history := session.History
	if len(history) > 0 && history[0].Role == openai.ChatMessageRoleSystem {
		history = history[1:]
	}
	history = normalizeMessagesForDeepSeek(history)
	if unlockMode {
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == openai.ChatMessageRoleUser {
				history = []openai.ChatCompletionMessage{history[i]}
				break
			}
		}
	}

	// Build temporary messages including identity context
	messages := make([]openai.ChatCompletionMessage, 0, len(history)+2)
	messages = append(messages, systemMessages...)
	messages = append(messages, history...)

	req := openai.ChatCompletionRequest{
		Messages:    messages,
		Temperature: 0.1,
	}
	if !unlockMode {
		req.Tools = GetAvailableTools()
	}

	resp, provider, err := raceChatProviders(ctx, httpClient, raceProviders, req)
	if err != nil && shouldResetChatHistoryForToolError(err) {
		log.Printf("[Bot Brain] Detected corrupted history (orphaned tool messages). Resetting session.")
		lastUserMsg := findLastUserMessage(session.History)
		session.History = []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: GetSystemPrompt()},
		}
		if lastUserMsg != "" {
			session.AppendMessage(openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: lastUserMsg,
			})
		}
		history = normalizeMessagesForDeepSeek(session.History)
		messages = make([]openai.ChatCompletionMessage, 0, len(history)+2)
		messages = append(messages, systemMessages...)
		messages = append(messages, history...)
		req.Messages = messages
		resp, provider, err = raceChatProviders(ctx, httpClient, raceProviders, req)
	}
	if err != nil && len(fallbackProviders) > 0 {
		log.Printf("[Bot Brain] NVIDIA racing failed, trying fallback providers: %v", err)
		resp, provider, err = callFallbackChatProviders(ctx, httpClient, fallbackProviders, req)
	}

	if err != nil {
		return fmt.Sprintf("AI 大脑思考失败 (网络/API异常): %v", err)
	}
	log.Printf("[Bot Brain] AI provider succeeded: %s (model: %s)", provider.Name, provider.Model)

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
		return callDeepSeek(ctx, senderID, session, false, false)
	}

	// Normal text reply
	return msg.Content
}

func callSpecialAIModel(ctx context.Context, capability string, userPrompt string) string {
	config := model.GlobalAIConfig
	if config == nil {
		config = model.LoadAIConfig()
	}

	providers := config.EffectiveProvidersByCapability(capability)
	if len(providers) == 0 {
		return fmt.Sprintf("未配置 %s 能力模型，请先检查 AI provider 配置。", capability)
	}

	if capability == "embedding" {
		return callEmbeddingModel(ctx, providers, userPrompt)
	}

	return callSpecialChatModel(ctx, capability, providers, userPrompt)
}

type chatProviderResult struct {
	Provider model.AIProviderConfig
	Response openai.ChatCompletionResponse
	Error    error
}

func splitChatProvidersForRacing(providers []model.AIProviderConfig) ([]model.AIProviderConfig, []model.AIProviderConfig) {
	raceProviders := make([]model.AIProviderConfig, 0, len(providers))
	fallbackProviders := make([]model.AIProviderConfig, 0, len(providers))
	for _, provider := range providers {
		if strings.Contains(strings.ToLower(provider.Name), "nvidia") ||
			strings.Contains(strings.ToLower(provider.BaseURL), "nvidia.com") {
			raceProviders = append(raceProviders, provider)
			continue
		}
		fallbackProviders = append(fallbackProviders, provider)
	}
	if len(raceProviders) == 0 {
		return providers, nil
	}
	return raceProviders, fallbackProviders
}

func raceChatProviders(ctx context.Context, httpClient *http.Client, providers []model.AIProviderConfig, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, model.AIProviderConfig, error) {
	if len(providers) == 0 {
		return openai.ChatCompletionResponse{}, model.AIProviderConfig{}, fmt.Errorf("no AI providers configured")
	}

	limit := len(providers)
	if limit > chatProviderRaceLimit {
		limit = chatProviderRaceLimit
	}

	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan chatProviderResult, limit)
	var wg sync.WaitGroup
	started := 0
	var lastErr error

	startProvider := func(provider model.AIProviderConfig) {
		started++
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := callSingleChatProvider(raceCtx, httpClient, provider, req)
			select {
			case results <- chatProviderResult{Provider: provider, Response: response, Error: err}:
			case <-raceCtx.Done():
			}
		}()
	}

	startProvider(providers[0])
	ticker := time.NewTicker(chatProviderStaggerDelay)
	defer ticker.Stop()

	for completed := 0; completed < limit; {
		select {
		case result := <-results:
			completed++
			if result.Error == nil {
				cancel()
				wg.Wait()
				return result.Response, result.Provider, nil
			}
			lastErr = result.Error
			log.Printf("[Bot Brain] %s provider failed during race: %v", result.Provider.Name, result.Error)
		case <-ticker.C:
			if started < limit {
				startProvider(providers[started])
			}
		case <-ctx.Done():
			cancel()
			wg.Wait()
			return openai.ChatCompletionResponse{}, model.AIProviderConfig{}, ctx.Err()
		}
	}

	cancel()
	wg.Wait()
	if lastErr != nil {
		return openai.ChatCompletionResponse{}, model.AIProviderConfig{}, lastErr
	}
	return openai.ChatCompletionResponse{}, model.AIProviderConfig{}, fmt.Errorf("all racing providers failed")
}

func callFallbackChatProviders(ctx context.Context, httpClient *http.Client, providers []model.AIProviderConfig, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, model.AIProviderConfig, error) {
	var lastErr error
	for _, provider := range providers {
		resp, err := callSingleChatProvider(ctx, httpClient, provider, req)
		if err == nil {
			return resp, provider, nil
		}
		lastErr = err
		log.Printf("[Bot Brain] %s fallback provider failed: %v", provider.Name, err)
	}
	if lastErr != nil {
		return openai.ChatCompletionResponse{}, model.AIProviderConfig{}, lastErr
	}
	return openai.ChatCompletionResponse{}, model.AIProviderConfig{}, fmt.Errorf("all fallback providers failed")
}

func callSingleChatProvider(ctx context.Context, httpClient *http.Client, provider model.AIProviderConfig, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	providerCtx, cancel := context.WithTimeout(ctx, chatProviderRequestTimout)
	defer cancel()

	clientConfig := openai.DefaultConfig(provider.APIKey)
	clientConfig.BaseURL = provider.BaseURL
	clientConfig.HTTPClient = httpClient
	client := openai.NewClientWithConfig(clientConfig)

	localReq := req
	localReq.Model = provider.Model
	resp, err := client.CreateChatCompletion(providerCtx, localReq)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return openai.ChatCompletionResponse{}, fmt.Errorf("%s returned no choices", provider.Name)
	}
	return resp, nil
}

func shouldResetChatHistoryForToolError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "400") && strings.Contains(err.Error(), "tool")
}

func findLastUserMessage(messages []openai.ChatCompletionMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == openai.ChatMessageRoleUser {
			return messages[i].Content
		}
	}
	return ""
}

func callEmbeddingModel(ctx context.Context, providers []model.AIProviderConfig, userPrompt string) string {
	type embeddingRequest struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	type embeddingResponse struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	client := &http.Client{Timeout: 90 * time.Second}
	var lastErr error
	for _, provider := range providers {
		body, _ := json.Marshal(embeddingRequest{
			Model: provider.Model,
			Input: []string{userPrompt},
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(provider.BaseURL, "/")+"/embeddings", bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("[Bot Brain] %s embedding provider failed: %v", provider.Name, err)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
			log.Printf("[Bot Brain] %s embedding provider failed: %v", provider.Name, lastErr)
			continue
		}

		var parsed embeddingResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			lastErr = err
			continue
		}
		if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
			lastErr = fmt.Errorf("embedding response is empty")
			continue
		}

		vector := parsed.Data[0].Embedding
		previewSize := 8
		if len(vector) < previewSize {
			previewSize = len(vector)
		}
		log.Printf("[Bot Brain] AI provider succeeded: %s (model: %s, capability: embedding)", provider.Name, provider.Model)
		return fmt.Sprintf("Embedding 已生成。\n模型：%s\n维度：%d\n向量预览：%v\n提示：完整向量较长，当前只展示前 %d 个数值。", provider.Model, len(vector), vector[:previewSize], previewSize)
	}

	if lastErr != nil {
		return fmt.Sprintf("Embedding 模型调用失败：%v", lastErr)
	}
	return "Embedding 模型调用失败：没有可用 provider。"
}

func callSpecialChatModel(ctx context.Context, capability string, providers []model.AIProviderConfig, userPrompt string) string {
	systemPrompt := map[string]string{
		"reward": "你正在调用 reward/reward-like 模型能力。请根据用户提供的提示、回答或内容进行质量评分，并优先输出可读的评分维度和改进建议。",
		"safety": "你正在调用安全分类模型能力。请对用户提供的内容做安全、违规、越狱、敏感风险分类，并给出风险等级、命中原因和建议处理方式。",
		"vision": "你正在调用视觉/多模态模型能力。请尽量分析用户提供的图片、图像 URL 或视觉任务；如果缺少图片输入，请明确说明需要图片 URL 或上传图片。",
		"parse":  "你正在调用文档解析/版面解析模型能力。请抽取用户提供内容中的结构化信息、表格、标题、段落和关键字段；如果需要图片或 PDF 输入，请明确说明。",
	}[capability]
	if systemPrompt == "" {
		systemPrompt = "请调用当前特殊能力模型完成用户请求。"
	}

	httpClient := &http.Client{Timeout: 120 * time.Second}
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.1,
	}

	var resp openai.ChatCompletionResponse
	var err error
	for _, provider := range providers {
		clientConfig := openai.DefaultConfig(provider.APIKey)
		clientConfig.BaseURL = provider.BaseURL
		clientConfig.HTTPClient = httpClient
		client := openai.NewClientWithConfig(clientConfig)
		req.Model = provider.Model

		resp, err = client.CreateChatCompletion(ctx, req)
		if err == nil && len(resp.Choices) > 0 {
			log.Printf("[Bot Brain] AI provider succeeded: %s (model: %s, capability: %s)", provider.Name, provider.Model, capability)
			return strings.TrimSpace(resp.Choices[0].Message.Content)
		}
		if err != nil {
			log.Printf("[Bot Brain] %s %s provider failed: %v", provider.Name, capability, err)
		}
	}

	if err != nil {
		return fmt.Sprintf("%s 能力模型调用失败：%v", capability, err)
	}
	return fmt.Sprintf("%s 能力模型未返回有效内容。", capability)
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
