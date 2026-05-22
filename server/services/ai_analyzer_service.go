package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"testcenter-server/feishu/model"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

// AnalyzeRequest 接收分析请求的参数
type AnalyzeRequest struct {
	Filename string `json:"filename"` // 例如: "google.report.json"
}

// LighthouseSummary 用于发送给 AI 的简要数据
type LighthouseSummary struct {
	URL           string             `json:"url"`
	Scores        map[string]float64 `json:"scores"`
	Opportunities []Opportunity      `json:"opportunities"`
}

type Opportunity struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Savings     string `json:"savings"`
}

// AnalyzeLighthouseHandler 处理报告分析请求
func AnalyzeLighthouseHandler(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	reportPath := filepath.Join(webTestReportStorageRoot(projectRootDir()), req.Filename)
	data, err := os.ReadFile(reportPath)
	if err != nil {
		log.Printf("[ERROR] Read JSON report failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Report file not found"})
		return
	}

	// 解析简要指标，避免发送过大数据给 AI
	summary, err := extractLighthouseSummary(data)
	if err != nil {
		log.Printf("[ERROR] Extract summary failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse report data"})
		return
	}

	analysisResult, err := analyzeWithAI(c.Request.Context(), summary)
	if err != nil {
		log.Printf("[ERROR] AI analysis failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI analysis failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"analysis": analysisResult,
	})
}

// extractLighthouseSummary 从复杂的 Lighthouse JSON 中提取 AI 关注的核心点
func extractLighthouseSummary(data []byte) (*LighthouseSummary, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	summary := &LighthouseSummary{
		Scores: make(map[string]float64),
	}

	// 提取 URL
	if url, ok := raw["requestedUrl"].(string); ok {
		summary.URL = url
	}

	// 提取评分 (Categories)
	if categories, ok := raw["categories"].(map[string]interface{}); ok {
		for k, v := range categories {
			if cat, ok := v.(map[string]interface{}); ok {
				if score, ok := cat["score"].(float64); ok {
					summary.Scores[k] = score * 100 // 转为 0-100
				}
			}
		}
	}

	// 提取耗时严重的 Opportunity (前 3 条)
	// 在 Lighthouse JSON 中，许多审计项在 audits 下
	if audits, ok := raw["audits"].(map[string]interface{}); ok {
		count := 0
		for _, v := range audits {
			if count >= 3 {
				break
			}
			audit, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			// 过滤出有节省空间的建议
			details, ok := audit["details"].(map[string]interface{})
			if !ok || details["type"] != "opportunity" {
				continue
			}

			summary.Opportunities = append(summary.Opportunities, Opportunity{
				Title:       fmt.Sprintf("%v", audit["title"]),
				Description: fmt.Sprintf("%v", audit["description"]),
				Savings:     fmt.Sprintf("%v", audit["displayValue"]),
			})
			count++
		}
	}

	return summary, nil
}

func analyzeWithAI(ctx context.Context, summary *LighthouseSummary) (string, error) {
	config := model.GlobalAIConfig
	if config == nil {
		config = model.LoadAIConfig()
	}

	providers := config.EffectiveProviders()
	if config == nil || len(providers) == 0 {
		return "", fmt.Errorf("AI API Key 未配置")
	}

	summaryJSON, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: 90 * time.Second,
	}

	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: "你是一名资深 Web 性能分析专家。用户会给你 Lighthouse JSON 提炼后的 summary。" +
					"请只基于 summary 进行分析，不要编造不存在的数据。" +
					"输出中文，适合直接显示在测试报告页面的 HTML 报告下方。" +
					"结构固定为：1. 核心评分概览；2. 结论；3. 优先级最高的 3 条优化建议；4. 风险提示。" +
					"每条建议尽量对应 summary 中的 opportunities，并说明为什么值得优先处理。",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("请分析这份 Lighthouse summary，并输出适合测试报告页面展示的总结：\n\n%s", string(summaryJSON)),
			},
		},
		Temperature: 0.2,
	}

	var resp openai.ChatCompletionResponse
	var apiErr error
	for _, provider := range providers {
		clientConfig := openai.DefaultConfig(provider.APIKey)
		clientConfig.BaseURL = provider.BaseURL
		clientConfig.HTTPClient = httpClient
		client := openai.NewClientWithConfig(clientConfig)
		req.Model = provider.Model

		resp, apiErr = client.CreateChatCompletion(ctx, req)
		if apiErr == nil {
			break
		}
		log.Printf("[AI Analyzer] %s provider failed, trying next AI provider if available: %v", provider.Name, apiErr)
	}
	if apiErr != nil {
		return "", apiErr
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI returned no choices")
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	if result == "" {
		return "", fmt.Errorf("AI returned empty analysis")
	}

	return result, nil
}
