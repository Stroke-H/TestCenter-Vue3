package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// AnalyzeRequest 接收分析请求的参数
type AnalyzeRequest struct {
	Filename string `json:"filename"` // 例如: "google.report.json"
}

// LighthouseSummary 用于发送给 AI 的简要数据
type LighthouseSummary struct {
	URL          string             `json:"url"`
	Scores       map[string]float64 `json:"scores"`
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

	reportPath := filepath.Join("..", "report", req.Filename)
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

	// TODO: 在这里集成真正的 DeepSeek API
	// 目前先使用 Mock AI 进行流程验证
	analysisResult := mockDeepSeekAnalysis(summary)

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

// mockDeepSeekAnalysis 模拟 AI 的回复逻辑
func mockDeepSeekAnalysis(s *LighthouseSummary) string {
	perf := s.Scores["performance"]
	
	result := fmt.Sprintf("### 🚀 AI 性能报告分析报告\n\n**目标地址**：%s\n\n", s.URL)
	result += fmt.Sprintf("#### 1. 核心看板分数\n- **性能评分**: %.0f/100\n- **无障碍**: %.0f/100\n- **最佳实践**: %.0f/100\n- **SEO**: %.0f/100\n\n", 
		s.Scores["performance"], s.Scores["accessibility"], s.Scores["best-practices"], s.Scores["seo"])

	result += "#### 2. 专业总结\n"
	if perf >= 90 {
		result += "本次压测表现优秀，关键渲染路径极其顺滑，符合 2026 高端验收标准（如 FPS > 60）。核心路径加载策略合理，已具备极佳的用户体验。\n\n"
	} else if perf >= 50 {
		result += "本次压测表现一般。虽然基本逻辑能跑通，但在高并发首屏加载时存在明显的“重资源阻塞”。部分图片或 JS 脚本未能异步化，导致用户在加载阶段会有约 1-2s 的白屏感。\n\n"
	} else {
		result += "警告：本次压测结果不合格。存在严重的渲染链路阻塞。核心性能指标（如 LCP 和 FCP）远超行业健康阈值，建议从 CDN 加速、代码粉碎（Tree Shaking）和按需加载三个维度强制优化。\n\n"
	}

	result += "#### 3. 通用验收标准改进建议\n"
	if len(s.Opportunities) > 0 {
		for _, o := range s.Opportunities {
			result += fmt.Sprintf("- **【重点优化】%s**：目前预计可节省 %s。建议参考描述进行调整：*%s*\n", o.Title, o.Savings, o.Description)
		}
	} else {
		result += "- 未发现明显可自动识别的优化机会，建议深入审查三方脚本注入耗时。\n"
	}

	result += "\n---\n*注：以上结论由 DeepSeek AI 模型智能生成，仅供参考。*"
	return result
}
