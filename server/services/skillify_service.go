package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testcenter-server/models"
	"time"

	"testcenter-server/feishu/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sashabaranov/go-openai"
)

// ElementData represents a raw clickable element found on a page
type ElementData struct {
	Selector      string `json:"selector"`
	Text          string `json:"text"`
	TagName       string `json:"tag_name"`
	Role          string `json:"role"`
	AriaLabel     string `json:"aria_label"`
	Placeholder   string `json:"placeholder"`
	ParentContext string `json:"parent_context"` // e.g. "header", "footer", "form"
}

// SkillNode represents an AI-processed, recognizable skill
type SkillNode struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Keyword     string            `json:"keyword"`  // The Playwright keyword to use, e.g. "Click"
	Args        map[string]string `json:"args"`     // e.g. {"selector": "..." }
	Category    string            `json:"category"` // e.g. "Navigation", "Auth"
}

// ScanResults holds the final output of the scanning process
type ScanResults struct {
	URL      string        `json:"url"`
	Elements []ElementData `json:"elements"`
}

// SkillifyService handles the logic for the Node Skillify Tool
type SkillifyService struct{}

var SkillifyServiceInstance = &SkillifyService{}

// ScanPageElements opens a URL and extracts all clickable elements
func (s *SkillifyService) ScanPageElements(targetURL string) (*ScanResults, error) {
	// Initialize Playwright
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("could not start playwright: %v", err)
	}
	defer pw.Stop()

	// Launch headless browser
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("could not create page: %v", err)
	}

	// Navigate to target
	_, err = page.Goto(targetURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return nil, fmt.Errorf("could not navigate to %s: %v", targetURL, err)
	}

	// JS script to find all clickable elements
	// We look for: buttons, links, inputs (button/submit), elements with cursor:pointer, role="button"
	const findElementsScript = `
	() => {
		const results = [];
		const seenSelectors = new Set();

		function getParentContext(el) {
			const parent = el.closest('header, footer, nav, form, aside, main');
			return parent ? parent.tagName.toLowerCase() : 'body';
		}

		function getOptimalSelector(el) {
			if (el.id) return '#' + el.id;
			if (el.getAttribute('data-testid')) return '[data-testid="' + el.getAttribute('data-testid') + '"]';
			if (el.name) return el.tagName.toLowerCase() + '[name="' + el.name + '"]';
			
			// Fallback to a simple path
			const tag = el.tagName.toLowerCase();
			if (el.className && typeof el.className === 'string') {
				const classes = el.className.split(/\s+/).filter(c => c && !c.includes(':')).join('.');
				if (classes) {
					const selector = tag + '.' + classes;
					try {
						if (document.querySelectorAll(selector).length === 1) return selector;
					} catch(e) {}
				}
			}
			return tag + ':text("' + (el.innerText || el.value || '').trim().substring(0, 20) + '")';
		}

		// Detect elements
		const clickables = 'button, a, input[type="button"], input[type="submit"], [role="button"], [onclick]';
		const elements = document.querySelectorAll(clickables);
		
		// Also look for elements with cursor: pointer (computed style)
		const allPotential = document.querySelectorAll('div, span, li, img');
		const pointerElements = Array.from(allPotential).filter(el => {
			// Skip if already captured by selector
			if (el.closest(clickables) && el.tagName.toLowerCase() !== 'img') return false;
			const style = window.getComputedStyle(el);
			return style.cursor === 'pointer';
		});

		const combined = [...Array.from(elements), ...pointerElements];

		combined.forEach(el => {
			let text = (el.innerText || el.value || '').trim();
			const tagName = el.tagName.toLowerCase();
			
			// If it's an image or contains only an image, try to get alt/title
			if (tagName === 'img' || (text === '' && el.querySelector('img'))) {
				const img = tagName === 'img' ? el : el.querySelector('img');
				text = img.getAttribute('alt') || img.getAttribute('title') || img.src.split('/').pop() || '图片核心';
			}

			if (!text && !el.getAttribute('aria-label') && !el.placeholder) return;
			
			const selector = getOptimalSelector(el);
			if (seenSelectors.has(selector)) return;
			seenSelectors.add(selector);

			results.push({
				selector: selector,
				text: text.substring(0, 100),
				tag_name: tagName,
				role: el.getAttribute('role') || (el.closest('a') ? 'link' : (el.closest('button') ? 'button' : '')),
				aria_label: el.getAttribute('aria-label') || '',
				placeholder: el.getAttribute('placeholder') || '',
				parent_context: getParentContext(el)
			});
		});

		return results;
	}
	`

	rawElements, err := page.Evaluate(findElementsScript)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate script: %v", err)
	}

	var elements []ElementData
	// Marshal and Unmarshal to convert interface{} to []ElementData
	data, _ := json.Marshal(rawElements)
	if err := json.Unmarshal(data, &elements); err != nil {
		return nil, fmt.Errorf("failed to parse elements: %v", err)
	}

	return &ScanResults{
		URL:      targetURL,
		Elements: elements,
	}, nil
}

// GenerateSkillsWithAI uses the configured AI providers to transform elements into Skill nodes.
func (s *SkillifyService) GenerateSkillsWithAI(elements []ElementData) ([]SkillNode, error) {
	config := model.GlobalAIConfig
	if config == nil {
		config = model.LoadAIConfig()
	}

	providers := config.EffectiveProviders()
	if config == nil || len(providers) == 0 {
		return nil, fmt.Errorf("AI API Key not configured")
	}

	// Prepare data for AI
	elementsJSON, _ := json.MarshalIndent(elements, "", "  ")

	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: `你是一名自动化测试专家和AI技能管理专家。
用户的输入是一组从网页扫描到的可点击元素数据（Selector, Text, Role等）。
你的任务是将这些原始元素“Skill化”，即把它们归纳成人类和AI都易于识别的“技能节点”。

规则：
1. 命名规范：使用简洁、功能导向的中文名称（如“登录按钮”、“搜索框提交”、“侧边导航-设置”）。
2. 合理分类：根据 parent_context 和功能进行分类（如：导航、表单、页脚、内容操作）。
3. 补充描述：简述该节点的作用。
4. 统一动作：动作（Keyword）目前统一使用 "Click"，除非它明显是输入框（Input）。
5. 输出格式：严格输出 JSON 数组，每个对象包含：name, description, keyword, args, category。
   其中 args 必须包含 selector 字段。

示例输出：
[
  {
    "name": "主页登录",
    "description": "点击顶部的登录按钮进入登录页面",
    "keyword": "Click",
    "args": {"selector": "button.login"},
    "category": "身份验证"
  }
]`,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("请将以下网页元素归纳成 Skill 节点：\n\n%s", string(elementsJSON)),
			},
		},
		Temperature: 0.3,
	}

	var resp openai.ChatCompletionResponse
	var err error
	for _, provider := range providers {
		clientConfig := openai.DefaultConfig(provider.APIKey)
		clientConfig.BaseURL = provider.BaseURL
		clientConfig.HTTPClient = &http.Client{
			Timeout: 90 * time.Second,
		}

		client := openai.NewClientWithConfig(clientConfig)
		req.Model = provider.Model
		resp, err = client.CreateChatCompletion(context.Background(), req)
		if err == nil {
			break
		}
		log.Printf("[Skillify] %s provider failed, trying next AI provider if available: %v", provider.Name, err)
	}
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("AI returned no results")
	}

	content := resp.Choices[0].Message.Content
	// Extract JSON if AI wrapped it in markdown code blocks
	if strings.Contains(content, "```json") {
		content = strings.Split(content, "```json")[1]
		content = strings.Split(content, "```")[0]
	} else if strings.Contains(content, "```") {
		content = strings.Split(content, "```")[1]
		content = strings.Split(content, "```")[0]
	}

	var skills []SkillNode
	if err := json.Unmarshal([]byte(content), &skills); err != nil {
		return nil, fmt.Errorf("failed to parse AI response as JSON: %v. Content: %s", err, content)
	}

	return skills, nil
}

// SaveSkillsToLibrary persists the Skill nodes as User Keywords
func (s *SkillifyService) SaveSkillsToLibrary(skills []SkillNode, suiteName string, sourceURL string) error {
	path := "data/pw_keywords.jsonl"

	// 1. Load existing keywords
	existing, err := loadJSONL[models.UserKeyword](path)
	if err != nil {
		return err
	}

	// 2. Convert SkillNodes to UserKeywords
	now := time.Now().Format(time.RFC3339)
	for _, skill := range skills {
		// Create a unique ID if not exists
		id := "kw-" + strings.ToLower(skill.Name) + "-" + time.Now().Format("05.000") // simple timestamp suffix

		// Map Skill to UserKeyword
		newKW := models.UserKeyword{
			ID:          id,
			Name:        skill.Name,
			Description: skill.Description,
			SuiteName:   suiteName,
			SourceURL:   sourceURL,
			CreatedAt:   now,
			Steps: []models.TestStep{
				{
					ID:          "step-1",
					Keyword:     skill.Keyword,
					Args:        skill.Args,
					Description: fmt.Sprintf("AI Generated Skill: %s", skill.Description),
				},
			},
		}
		existing = append(existing, newKW)
	}

	// 3. Save all back to SQL
	return saveAllJSONL(path, existing)
}
