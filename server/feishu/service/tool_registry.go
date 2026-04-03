package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testcenter-server/feishu/model"
	"testcenter-server/models"
	"testcenter-server/services"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// Tool execution logic
type ExecuteFunc func(ctx context.Context, chatID string, senderID string, args string) (string, error)

type ToolDef struct {
	Name            string
	Description     string
	Parameters      *jsonschema.Definition
	NeedConfirm     bool
	ConfirmPrompt   string
	BuildConfirmMsg func(ctx context.Context, argsJSON string) (string, error)
	Execute         ExecuteFunc
}

var Registry = map[string]ToolDef{}

func RegisterTool(def ToolDef) {
	Registry[def.Name] = def
}

func GetAvailableTools() []openai.Tool {
	var tools []openai.Tool
	for _, def := range Registry {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			},
		})
	}
	return tools
}

func init() {
	RegisterTool(ToolDef{
		Name:        "delete_account",
		Description: "【高优先级】一键删号功能。支持识别正式服(prod)或测试服(test)环境。当用户贴了请求头并要求删号时，必须立即调用本工具。优先从用户语气中识别环境（如“正式服”、“生产”、“线上”对应 prod，“测试”、“test”对应 test），若未提及则默认 test。",
		Parameters: &jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"headers_json": {
					Type:        jsonschema.Object,
					Description: "A flat JSON object containing exactly the parsed request headers provided by the user.",
				},
				"env": {
					Type:        jsonschema.String,
					Enum:        []string{"prod", "test"},
					Description: "The environment to target: 'prod' (正式服/正式环境) or 'test' (测试服/测试环境). Default is 'test'.",
				},
			},
			Required: []string{"headers_json", "env"},
		},
		NeedConfirm: true,
		BuildConfirmMsg: func(ctx context.Context, argsJSON string) (string, error) {
			var args struct {
				Headers map[string]interface{} `json:"headers_json"`
				Env     string                 `json:"env"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", fmt.Errorf("解析参数失败: %v", err)
			}

			app, _ := args.Headers["app"].(string)
			projectName := "NovelNova"
			envName := "测试环境 (Test)"
			baseDomain := "http://34.10.7.187" // default testing env (NovelNova)

			isProd := args.Env == "prod"
			if isProd {
				envName = "🚀 正式环境 (PROD) 🚀"
			}

			if app == "com.shortswave.android" || app == "com.company.shortsdrama.wave" || strings.Contains(strings.ToLower(app), "shortwave") || strings.Contains(strings.ToLower(app), "shortsdrama") {
				projectName = "ShortsWave"
				if isProd {
					baseDomain = "https://api.shortswave.com"
				} else {
					baseDomain = "http://35.225.224.94"
				}
			} else {
				// NovelNova
				if isProd {
					baseDomain = "https://api.novelnovastory.com"
				} else {
					baseDomain = "http://34.10.7.187"
				}
			}

			// Call login anonymous to preview account
			loginUrl := baseDomain + "/login/anonymous"
			bodyBytes, _ := json.Marshal(args.Headers)
			req, _ := http.NewRequest("POST", loginUrl, bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			for k, v := range args.Headers {
				if str, ok := v.(string); ok {
					if strings.ToLower(k) != "content-length" && strings.ToLower(k) != "accept-encoding" {
						req.Header.Set(k, str)
					}
				}
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()

			var resData struct {
				Data struct {
					UserID   interface{} `json:"user_id"`
					UserName string      `json:"user_name"`
				} `json:"data"`
			}
			json.NewDecoder(resp.Body).Decode(&resData)

			uidStr := fmt.Sprintf("%v", resData.Data.UserID)
			if uidStr == "" || uidStr == "<nil>" {
				return "", fmt.Errorf("未能通过你提供的请求头定位到账号。\n目标项目：%s\n目标环境：%s\n请检查凭据是否过期或应用归属是否有误。", projectName, envName)
			}

			return fmt.Sprintf("⚠️ [敏感操作拦截 - 账号销毁]\n系统已解析请求头并自动提取身份：\n• 目标项目：**%s**\n• 执行环境：**%s**\n• 账号 UID：**%s**\n• 玩家昵称：**%s**\n\n您确定要【彻底删除此账号及所有业务数据】吗？（操作绝对不可逆！）\n请确切回复【是】一键抹除，或回复【否】中止操作。", projectName, envName, uidStr, resData.Data.UserName), nil
		},
		Execute: func(ctx context.Context, chatID string, senderID string, argsJSON string) (string, error) {
			var args struct {
				Headers map[string]interface{} `json:"headers_json"`
				Env     string                 `json:"env"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", err
			}

			app, _ := args.Headers["app"].(string)
			baseDomain := "http://34.10.7.187"
			isProd := args.Env == "prod"

			if app == "com.shortswave.android" || app == "com.company.shortsdrama.wave" || strings.Contains(strings.ToLower(app), "shortwave") || strings.Contains(strings.ToLower(app), "shortsdrama") {
				if isProd {
					baseDomain = "https://api.shortswave.com"
				} else {
					baseDomain = "http://35.225.224.94"
				}
			} else {
				if isProd {
					baseDomain = "https://api.novelnovastory.com"
				} else {
					baseDomain = "http://34.10.7.187"
				}
			}

			log.Printf("[Tool: delete_account] Starting execution. App: %s, Env: %s, BaseDomain: %s", app, args.Env, baseDomain)

			// First, do login anonymous to get the fresh session token to use for deletion
			loginUrl := baseDomain + "/login/anonymous"
			bodyBytes, _ := json.Marshal(args.Headers)
			loginReq, _ := http.NewRequest("POST", loginUrl, bytes.NewBuffer(bodyBytes))
			loginReq.Header.Set("Content-Type", "application/json")
			for k, v := range args.Headers {
				if str, ok := v.(string); ok {
					if strings.ToLower(k) != "content-length" && strings.ToLower(k) != "accept-encoding" {
						loginReq.Header.Set(k, str)
					}
				}
			}

			log.Printf("[Tool: delete_account] Calling login anonymous: %s", loginUrl)
			loginResp, err := http.DefaultClient.Do(loginReq)
			if err != nil {
				log.Printf("[Tool: delete_account] Login request failed: %v", err)
				return "", fmt.Errorf("登录获取凭证阶段失败: %v", err)
			}
			defer loginResp.Body.Close()

			var loginData struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
				Data struct {
					SessionToken string `json:"session_token"`
					UserID       string `json:"user_id"`
				} `json:"data"`
			}
			json.NewDecoder(loginResp.Body).Decode(&loginData)
			log.Printf("[Tool: delete_account] Login response: Code=%d, Msg=%s, UserID=%s", loginData.Code, loginData.Msg, loginData.Data.UserID)

			sessionToken := loginData.Data.SessionToken
			if sessionToken == "" {
				log.Printf("[Tool: delete_account] Failed to get session token from login response. Falling back to original headers.")
				if tok, ok := args.Headers["X-SESSION-TOKEN"].(string); ok {
					sessionToken = tok
				} else {
					return "", fmt.Errorf("无法获取用于操作的 Session Token")
				}
			}
			log.Printf("[Tool: delete_account] Using session token: %s...", sessionToken[:10])

			// Now call delete API
			deleteUrl := baseDomain + "/user/delete"
			// Ensure the headers used for deletion also have the fresh token
			args.Headers["X-SESSION-TOKEN"] = sessionToken

			// As per frontend logic, deletion is a GET request with specific headers
			delReq, _ := http.NewRequest("GET", deleteUrl, nil)
			for k, v := range args.Headers {
				if str, ok := v.(string); ok {
					if strings.ToLower(k) != "content-length" && strings.ToLower(k) != "accept-encoding" {
						delReq.Header.Set(k, str)
					}
				}
			}
			// Explicitly set the token in header too
			delReq.Header.Set("X-SESSION-TOKEN", sessionToken)

			log.Printf("[Tool: delete_account] Calling delete API: %v", deleteUrl)
			delResp, err := http.DefaultClient.Do(delReq)
			if err != nil {
				log.Printf("[Tool: delete_account] Delete request failed: %v", err)
				return "", fmt.Errorf("提交注销请求失败: %v", err)
			}
			defer delResp.Body.Close()

			var delResData struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
			}
			json.NewDecoder(delResp.Body).Decode(&delResData)
			log.Printf("[Tool: delete_account] Delete result: Code=%d, Msg=%s", delResData.Code, delResData.Msg)

			if delResData.Code == 0 || delResData.Msg == "success" {
				// LOG THE OPERATION
				projectName := "NovelNova"
				if app == "com.shortswave.android" || app == "com.company.shortsdrama.wave" || strings.Contains(strings.ToLower(app), "shortwave") || strings.Contains(strings.ToLower(app), "shortsdrama") {
					projectName = "ShortsWave"
				}
				operatorID := senderID
				operatorName := ""
				if boundUser, err := services.FindUserByFeishuOpenID(senderID); err == nil && boundUser != nil {
					operatorID = boundUser.ID
					if boundUser.Username != "" {
						operatorName = boundUser.Username
					}
				}
				AddOperationLog(model.AIOperationLog{
					ID:        fmt.Sprintf("OP_%d", time.Now().UnixNano()),
					ToolName:  "delete_account",
					Project:   projectName,
					Env:       args.Env,
					UserID:    operatorID,
					UserName:  operatorName,
					Status:    "success",
					Detail:    fmt.Sprintf("Account %s deleted in %s env", loginData.Data.UserID, args.Env),
					Timestamp: time.Now(),
				})

				return "✅ 账号注销指令已下发成功！账号已被清除。", nil
			}
			return fmt.Sprintf("❌ 注销失败: API 返回 %s (Code: %d)", delResData.Msg, delResData.Code), nil
		},
	})

	RegisterTool(ToolDef{
		Name:        "generate_acceptance_report",
		Description: "生成标准格式的验收报告。重要：直接从用户消息中提取链接 URL 作为字符串，禁止尝试读取或浏览链接内容。输入包括项目代码(swa/swi)、版本号(可选)、测试周期及 Bug/需求链接。",
		Parameters: &jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"project_code": {
					Type:        jsonschema.String,
					Description: "项目代号、缩写或 ID（例如: 'swa', '1100', 'swi', 'A1100', 'ShortsWave'等）",
				},
				"version": {
					Type:        jsonschema.String,
					Description: "版本号。如果未提供，系统将尝试在前一版本基础上末位+1。",
				},
				"period": {
					Type:        jsonschema.String,
					Description: "测试周期，如 '3.17-3.20'",
				},
				"bug_links_unfixed": {
					Type: jsonschema.Array,
					Items: &jsonschema.Definition{
						Type: jsonschema.String,
					},
					Description: "未修复的缺陷链接列表",
				},
				"bug_links_fixed": {
					Type: jsonschema.Array,
					Items: &jsonschema.Definition{
						Type: jsonschema.String,
					},
					Description: "已修复的缺陷链接列表",
				},
				"story_links": {
					Type: jsonschema.Array,
					Items: &jsonschema.Definition{
						Type: jsonschema.String,
					},
					Description: "测试的需求/故事链接列表",
				},
			},
			Required: []string{"project_code", "period"},
		},
		Execute: func(ctx context.Context, chatID string, senderID string, argsJSON string) (string, error) {
			var args struct {
				ProjectCode string   `json:"project_code"`
				Version     string   `json:"version"`
				Period      string   `json:"period"`
				BugUnfixed  []string `json:"bug_links_unfixed"`
				BugFixed    []string `json:"bug_links_fixed"`
				StoryLinks  []string `json:"story_links"`
				TestOwner   string   `json:"test_owner"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", err
			}

			// 1. Identify Project
			project, err := services.ConfigServiceInstance.GetProjectBySubCode(args.ProjectCode)
			if err != nil {
				// Try fuzzy match for suggestions
				suggestions := services.ConfigServiceInstance.SearchProjectsFuzzy(args.ProjectCode)
				if len(suggestions) > 0 {
					var names []string
					for _, s := range suggestions {
						names = append(names, fmt.Sprintf("- %s (%s / %s)", s.ProjectName, s.ProjectCode, s.ShortCode))
					}
					return fmt.Sprintf("⚠️ 未在现有库中精确匹配到项目 '%s'。您是指以下项目之一吗？\n%s\n\n如果以上都不是，请问这是否是一个【新项目】？如果是，请直接提供项目全称和 ID (如 A1200)，我将为您创建记录并生成报告。",
						args.ProjectCode, strings.Join(names, "\n")), nil
				}
				return fmt.Sprintf("❌ 未找到项目代号 '%s'，且没有找到相似的现有项目。请确认项目代号是否正确。\n若这是一个您需要新增的项目，请提供：【项目全称】和【项目 ID】。", args.ProjectCode), nil
			}

			projectName := project.ProjectName
			projectActualCode := project.ProjectCode

			// 2. Version Logic
			version := args.Version
			if version == "" {
				version = "2.58.0" // Default/Fallback
			}
			displayVersion := "v" + version

			// 3. Date Formatting
			year := time.Now().Year()
			formattedPeriod := args.Period
			if strings.Contains(args.Period, "-") {
				parts := strings.Split(args.Period, "-")
				if len(parts) == 2 {
					formatDate := func(s string) string {
						subParts := strings.Split(strings.TrimSpace(s), ".")
						if len(subParts) == 2 {
							return fmt.Sprintf("%d-%02s-%02s", year, subParts[0], subParts[1])
						}
						return s
					}
					formattedPeriod = fmt.Sprintf("%s~%s", formatDate(parts[0]), formatDate(parts[1]))
				}
			}

			// 4. Test Environment (Filter by OS suffix)
			envStr := "正式服务器"
			devices, _ := services.ConfigServiceInstance.GetDevicesByProjectCode(projectActualCode)

			// Detect target OS from project name suffix
			targetOS := ""
			if strings.Contains(strings.ToLower(projectName), "- ios") {
				targetOS = "iOS"
			} else if strings.Contains(strings.ToLower(projectName), "- android") {
				targetOS = "Android"
			}

			if len(devices) > 0 {
				var names []string
				for _, d := range devices {
					// Only add if OS matches or if targetOS is not specified
					if targetOS == "" || strings.EqualFold(d.OS, targetOS) {
						names = append(names, d.DeviceName)
					}
				}
				if len(names) > 0 {
					envStr = strings.Join(names, "/") + "/正式服务器"
				}
			}

			// 5. Identify Platform User
			testOwner := args.TestOwner
			if testOwner == "" {
				platformUser, _ := services.FindUserByFeishuOpenID(senderID)
				if platformUser != nil {
					if platformUser.Nickname != "" {
						testOwner = platformUser.Nickname
					} else {
						testOwner = platformUser.Username
					}
				}
			}
			if testOwner == "" {
				testOwner = "Tester"
			}

			// 6. Build Report
			formattedFixed := ""
			if len(args.BugFixed) > 0 {
				formattedFixed = "已修复：\n"
				for _, link := range args.BugFixed {
					formattedFixed += link + "\n"
				}
			}
			formattedUnfixed := ""
			if len(args.BugUnfixed) > 0 {
				formattedUnfixed = "未修复：\n"
				for _, link := range args.BugUnfixed {
					formattedUnfixed += link + "\n"
				}
			}
			formattedStories := ""
			for _, link := range args.StoryLinks {
				formattedStories += link + "\n"
			}

			report := fmt.Sprintf("%s项目验收报告\n\n"+
				"项目名称：%s\n"+
				"版本号：%s\n"+
				"测试负责人：%s\n"+
				"测试时间：%s\n"+
				"测试环境：%s\n"+
				"本次测试覆盖率： 100%%\n\n"+
				"测试结论：当前版本Pass！\n"+
				"正式版本缺陷修复验证情况：\n"+
				"本次预提审版本缺陷提交情况：\n%s%s"+
				"版本更新测试需求点：\n%s",
				projectActualCode, projectName, displayVersion, testOwner, formattedPeriod, envStr, formattedUnfixed, formattedFixed, formattedStories)

			operatorName := testOwner
			operatorID := senderID
			if boundUser, err := services.FindUserByFeishuOpenID(senderID); err == nil && boundUser != nil {
				operatorID = boundUser.ID
				if boundUser.Username != "" {
					operatorName = boundUser.Username
				}
			}

			// 7. Persist (Only for non-Web sessions, Web will handle it via confirmation dialog)
			if !strings.HasPrefix(chatID, "WEB_") {
				newReport := services.AcceptanceReport{
					ID:                  fmt.Sprintf("AR_%d", time.Now().Unix()),
					ProjectName:         projectName,
					ProjectCode:         projectActualCode,
					Version:             displayVersion,
					TestOwner:           testOwner,
					Reporter:            testOwner,
					TestTime:            formattedPeriod,
					TestEnv:             envStr,
					TestConclusion:      "Pass",
					BugFixStatus:        formattedFixed,
					BugSubmissionStatus: formattedUnfixed,
					UpdateRequirements:  formattedStories,
					CreatedAt:           time.Now().Format(time.RFC3339),
					UpdatedAt:           time.Now().Format(time.RFC3339),
					Status:              "Completed",
				}
				services.SaveAcceptanceReport(newReport)

				// [NEW] Automatic Sync to Wiki Documentation Link
				if project.WikiURL != "" {
					go func() {
						log.Printf("[WikiLinkage] Syncing report for %s to %s", projectActualCode, project.WikiURL)
						// 1. Extract node token
						parts := strings.Split(project.WikiURL, "/")
						nodeToken := ""
						for i, p := range parts {
							if p == "wiki" && i+1 < len(parts) {
								nodeToken = strings.Split(parts[i+1], "?")[0]
								break
							}
						}
						if nodeToken == "" {
							return
						}

						// 2. Map Wiki Node to Doc Token
						objToken, objType, err := FeishuClientInstance.GetWikiNode(nodeToken)
						if err != nil || objType != "docx" {
							return
						}

						// 3. Clean Report Text
						lines := strings.Split(report, "\n")
						var filtered []string
						for _, line := range lines {
							lineTrim := strings.TrimSpace(line)
							if strings.HasSuffix(lineTrim, "项目验收报告") {
								continue
							}
							if strings.HasPrefix(lineTrim, "项目名称：") {
								continue
							}
							filtered = append(filtered, line)
						}
						cleanedReport := strings.Join(filtered, "\n")

						// 4. Prepare Blocks (H2 for Version + Text for Body)
						h2Block := map[string]interface{}{
							"block_type": 4, // Heading 2
							"heading2": map[string]interface{}{
								"style": map[string]interface{}{},
								"elements": []map[string]interface{}{
									{
										"text_run": map[string]interface{}{
											"content": strings.ToUpper(displayVersion), // V2.58.0
										},
									},
								},
							},
						}
						textBlock := map[string]interface{}{
							"block_type": 2, // Text
							"text": map[string]interface{}{
								"style": map[string]interface{}{},
								"elements": []map[string]interface{}{
									{
										"text_run": map[string]interface{}{
											"content": cleanedReport + "\n",
										},
									},
								},
							},
						}

						// 5. Prepend to Docx
						FeishuClientInstance.AddBlocksToDocx(objToken, 0, []map[string]interface{}{h2Block, textBlock})
					}()
				}
			}

			AddOperationLog(model.AIOperationLog{
				ID:        fmt.Sprintf("OP_%d", time.Now().UnixNano()),
				ToolName:  "generate_acceptance_report",
				Project:   projectActualCode,
				Env:       envStr,
				UserID:    operatorID,
				UserName:  operatorName,
				Status:    "success",
				Detail:    fmt.Sprintf("Acceptance report generated for %s %s", projectName, displayVersion),
				Timestamp: time.Now(),
			})

			// 8. Return structured JSON for web frontend
			type ReportOutput struct {
				ProjectName     string   `json:"project_name"`
				ProjectCode     string   `json:"project_code"`
				Version         string   `json:"version"`
				Period          string   `json:"period"`
				Environment     string   `json:"environment"`
				BugLinksFixed   []string `json:"bug_links_fixed"`
				BugLinksUnfixed []string `json:"bug_links_unfixed"`
				StoryLinks      []string `json:"story_links"`
				FullText        string   `json:"full_text"`
				IsReport        bool     `json:"_is_report"`
			}
			outJSON, _ := json.Marshal(ReportOutput{
				ProjectName:     projectName,
				ProjectCode:     projectActualCode,
				Version:         displayVersion,
				Period:          formattedPeriod,
				Environment:     envStr,
				BugLinksFixed:   args.BugFixed,
				BugLinksUnfixed: args.BugUnfixed,
				StoryLinks:      args.StoryLinks,
				FullText:        report,
				IsReport:        true,
			})
			return string(outJSON), nil
		},
	})

	RegisterTool(ToolDef{
		Name:        "add_new_project",
		Description: "录入新项目映射关系。当用户确认需新增项目时使用。需提供项目 ID (如 A1200) 和全称。",
		Parameters: &jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"project_code": {
					Type:        jsonschema.String,
					Description: "项目官方 ID (如 A1200)",
				},
				"project_name": {
					Type:        jsonschema.String,
					Description: "项目全称 (如 MeloShorts - Android)",
				},
				"short_code": {
					Type:        jsonschema.String,
					Description: "简写代号 (如 msa, swa)",
				},
			},
			Required: []string{"project_code", "project_name"},
		},
		Execute: func(ctx context.Context, chatID string, senderID string, argsJSON string) (string, error) {
			var args struct {
				ProjectCode string `json:"project_code"`
				ProjectName string `json:"project_name"`
				ShortCode   string `json:"short_code"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", err
			}
			newProj := models.Project{
				ID:          args.ProjectCode,
				ProjectCode: args.ProjectCode,
				ProjectName: args.ProjectName,
				ShortCode:   args.ShortCode,
				CreatedAt:   time.Now().Format(time.RFC3339),
			}
			err := services.ConfigServiceInstance.UpdateProject(newProj)
			if err != nil {
				return fmt.Sprintf("❌ 新增项目失败: %v", err), nil
			}
			return fmt.Sprintf("✅ 已成功入库新项目：\n项目 ID: %s\n项目全称: %s\n缩写: %s\n您可以立即重新生成该项目的验收报告了。",
				args.ProjectCode, args.ProjectName, args.ShortCode), nil
		},
	})
	RegisterTool(ToolDef{
		Name:        "write_to_feishu_wiki",
		Description: "在飞书 Wiki 或文档的最上方插入指定文本。适用于记录测试结论、插入临时标记等场景。",
		Parameters: &jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"wiki_url": {
					Type:        jsonschema.String,
					Description: "完整的飞书 Wiki 或文档链接",
				},
				"content": {
					Type:        jsonschema.String,
					Description: "需要写入的文本内容",
				},
			},
			Required: []string{"wiki_url", "content"},
		},
		Execute: func(ctx context.Context, chatID string, senderID string, argsJSON string) (string, error) {
			var args struct {
				WikiURL string `json:"wiki_url"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", err
			}

			// 1. Extract node token from URL
			parts := strings.Split(args.WikiURL, "/")
			nodeToken := ""
			for i, p := range parts {
				if p == "wiki" && i+1 < len(parts) {
					nodeToken = strings.Split(parts[i+1], "?")[0]
					break
				}
				if (p == "docx" || p == "docs") && i+1 < len(parts) {
					nodeToken = strings.Split(parts[i+1], "?")[0]
					break
				}
			}

			if nodeToken == "" {
				return "❌ 无法从提供的链接中解析出 Wiki 节点或文档 Token。请提供格式正确的飞书 Wiki 链接。", nil
			}

			// 2. Map Wiki Node to ObjToken (if it's a wiki)
			objToken := nodeToken
			objType := "docx" // Default to docx for direct doc links

			if strings.Contains(args.WikiURL, "/wiki/") {
				ot, jt, err := FeishuClientInstance.GetWikiNode(nodeToken)
				if err != nil {
					return fmt.Sprintf("❌ 获取 Wiki 节点信息失败: %v。请确认机器人已被添加为该知识库的协作者。", err), nil
				}
				objToken = ot
				objType = jt
			}

			if objType != "docx" {
				return fmt.Sprintf("⚠️ 目前仅支持对 docx 类型的文档进行插入操作（当前识别类型: %s）。", objType), nil
			}

			// 3. Prepend content
			err := FeishuClientInstance.PrependToDocx(objToken, args.Content)
			if err != nil {
				return fmt.Sprintf("❌ 写入文档失败: %v。请检查机器人权限及文档锁定状态。", err), nil
			}

			return fmt.Sprintf("✅ 已成功在文档 (token: %s) 最上方写入内容：\n%s", objToken, args.Content), nil
		},
	})

	RegisterTool(ToolDef{
		Name:        "read_feishu_wiki",
		Description: "读取飞书 Wiki 或文档的正文内容。适用于查看文档状态、核对现有信息等场景。",
		Parameters: &jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"wiki_url": {
					Type:        jsonschema.String,
					Description: "完整的飞书 Wiki 或文档链接",
				},
			},
			Required: []string{"wiki_url"},
		},
		Execute: func(ctx context.Context, chatID string, senderID string, argsJSON string) (string, error) {
			var args struct {
				WikiURL string `json:"wiki_url"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", err
			}

			// 1. Extract node token from URL
			parts := strings.Split(args.WikiURL, "/")
			nodeToken := ""
			for i, p := range parts {
				if p == "wiki" && i+1 < len(parts) {
					nodeToken = strings.Split(parts[i+1], "?")[0]
					break
				}
				if (p == "docx" || p == "docs") && i+1 < len(parts) {
					nodeToken = strings.Split(parts[i+1], "?")[0]
					break
				}
			}

			if nodeToken == "" {
				return "❌ 无法解析文档地址。", nil
			}

			// 2. Map Wiki Node to ObjToken
			objToken := nodeToken
			if strings.Contains(args.WikiURL, "/wiki/") {
				ot, _, err := FeishuClientInstance.GetWikiNode(nodeToken)
				if err != nil {
					return fmt.Sprintf("❌ 获取节点失败: %v", err), nil
				}
				objToken = ot
			}

			// 3. Read content
			content, err := FeishuClientInstance.GetDocxRawContent(objToken)
			if err != nil {
				return fmt.Sprintf("❌ 读取失败: %v", err), nil
			}

			return fmt.Sprintf("--- 文档正文 (Token: %s) ---\n%s", objToken, content), nil
		},
	})
}
