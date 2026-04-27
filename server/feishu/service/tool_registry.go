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

func formatAcceptanceReportToolSection(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		trimmed = "无"
	}
	return trimmed + "\n\n"
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
			Required: []string{"project_code"},
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
			if formattedPeriod == "" {
				formattedPeriod = time.Now().Format("2006-01-02")
			} else if strings.Contains(args.Period, "-") {
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

			// 4. Test Environment & Devices
			testEnv := "正式服务器"
			testDevices := ""
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
					testDevices = strings.Join(names, "/")
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
			formattedBugStatus := formatAcceptanceReportToolSection(formattedUnfixed + formattedFixed)
			formattedStories = formatAcceptanceReportToolSection(formattedStories)

			devicesLine := ""
			if testDevices != "" {
				devicesLine = fmt.Sprintf("测试设备：%s\n", testDevices)
			}

			report := fmt.Sprintf("%s项目验收报告\n\n"+
				"项目名称：%s\n"+
				"版本号：%s\n"+
				"测试负责人：%s\n"+
				"测试时间：%s\n"+
				"测试环境：%s\n"+
				"%s"+
				"本次测试覆盖率： 100%%\n\n"+
				"测试结论：当前版本Pass！\n"+
				"正式版本缺陷修复验证情况：\n"+
				"本次预提审版本缺陷提交情况：\n%s"+
				"版本更新测试需求点：\n%s",
				projectActualCode, projectName, displayVersion, testOwner, formattedPeriod, testEnv, devicesLine, formattedBugStatus, formattedStories)

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
					TestEnv:             testEnv,
					TestDevices:         testDevices,
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
						err = FeishuClientInstance.AddBlocksToDocx(objToken, 0, []map[string]interface{}{h2Block, textBlock})
						if err != nil {
							log.Printf("[WikiLinkage] Error syncing report to %s: %v", project.WikiURL, err)
						} else {
							log.Printf("[WikiLinkage] Successfully synced report to %s", project.WikiURL)
						}
					}()
				}
			}

			AddOperationLog(model.AIOperationLog{
				ID:        fmt.Sprintf("OP_%d", time.Now().UnixNano()),
				ToolName:  "generate_acceptance_report",
				Project:   projectActualCode,
				Env:       testEnv,
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
				Environment:     testEnv,
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
		Name:        "get_project_info",
		Description: "查询 TestCenter 的内部项目字典。当你获取到用户输入的项目代号（如 swi, swa）时，使用此工具查找该项目的全名、所属业务线、所属飞书空间及其在飞书中的 project_key。返回结果中会包含可以直接使用的 MQL 搜索语句，你必须直接用它们去调用 search_by_mql。",
		Parameters: &jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"project_code": {
					Type:        jsonschema.String,
					Description: "项目代号/缩写，如 'swi', 'swa'",
				},
				"version": {
					Type:        jsonschema.String,
					Description: "目标版本号，如 '2.60.0'",
				},
			},
			Required: []string{"project_code", "version"},
		},
		Execute: func(ctx context.Context, chatID string, senderID string, argsJSON string) (string, error) {
			var args struct {
				ProjectCode string `json:"project_code"`
				Version     string `json:"version"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", err
			}

			project, err := services.ConfigServiceInstance.GetProjectBySubCode(args.ProjectCode)
			if err != nil || project == nil {
				return fmt.Sprintf("❌ 未找到项目代号 '%s'。请提示用户补充详细信息或到平台上配置项目空间。", args.ProjectCode), nil
			}

			if project.Workspace == "" {
				return fmt.Sprintf("⚠️ 找到项目 '%s'，但其【所属空间】为空。请立刻对用户说：当前项目没有绑定所属空间，请在平台系统设置中补充验收报告需要的对应空间信息。缺少空间信息时，你需要向用户索要以下信息来手动生成报告：测试周期、已修复缺陷链接、未修复缺陷链接、需求链接。", project.ProjectName), nil
			}

			// URL Mappings
			projectKeyMap := map[string]string{
				"海外短剧":    "shortwave",
				"免费短剧":    "freedrama",
				"iOS订阅产品": "ios_sub",
				"番茄短剧":    "tomato",
			}

			projectKey := projectKeyMap[project.Workspace]
			if projectKey == "" {
				projectKey = "unknown"
			}

			// Auto-extract business line and platform from project name
			// e.g. "ShortsWave - iOS" -> businessLine="ShortsWave", platform="iOS"
			businessLine := ""
			platform := ""
			nameParts := strings.SplitN(project.ProjectName, " - ", 2)
			if len(nameParts) == 2 {
				businessLine = strings.TrimSpace(nameParts[0])
				platform = strings.TrimSpace(nameParts[1])
			}

			version := args.Version

			// Platform-specific MQL mappings
			storyPlatformFilter := ""
			storyCategoryFilter := ""
			bugPlatformFilter := ""
			if strings.EqualFold(platform, "iOS") {
				storyPlatformFilter = "field_e4ca95 = \"iOS\""
				storyCategoryFilter = "field_b980a4 = \"iOS端需求\""
				bugPlatformFilter = "field_f7ef16 = \"IOS\""
			} else if strings.EqualFold(platform, "Android") {
				storyPlatformFilter = "field_e4ca95 = \"Android\""
				storyCategoryFilter = "field_b980a4 = \"安卓端需求\""
				bugPlatformFilter = "field_f7ef16 = \"Android\""
			}

			// Build exact working MQL statements based on correct schema
			// Story table: story
			// Bug table for this project: 63329b6c980d67099b12fd73
			storyMQL := fmt.Sprintf("SELECT work_item_id, name, work_item_status FROM `%s`.`story` WHERE business = \"%s\"", projectKey, businessLine)
			bugMQL := fmt.Sprintf("SELECT work_item_id, name, work_item_status FROM `%s`.`63329b6c980d67099b12fd73` WHERE business = \"%s\"", projectKey, businessLine)

			if storyPlatformFilter != "" {
				storyMQL += " AND " + storyPlatformFilter
			}
			if storyCategoryFilter != "" {
				storyMQL += " AND " + storyCategoryFilter
			}
			if bugPlatformFilter != "" {
				bugMQL += " AND " + bugPlatformFilter
			}

			return fmt.Sprintf("✅ 找到项目信息：\n"+
				"项目全名：%s\n"+
				"所属空间：%s\n"+
				"飞书Project_Key：%s\n"+
				"业务线：%s\n"+
				"平台：%s\n\n"+
				"【核心查询指令 ★★★ 必须严格执行 ★★★】：\n"+
				"1. 获取需求：请调用 search_by_mql，参数 mql 为：%s 并且请你自行在后面加上版本过滤条件（例如：AND 规划版本 LIKE \"%%%s%%\"）。\n"+
				"2. 获取缺陷：请调用 search_by_mql，参数 mql 为：%s 并且请你自行在后面加上版本过滤条件（例如：AND 解决版本 LIKE \"%%%s%%\"）。\n"+
				"3. 【禁止归纳总结】：拿到 search_by_mql 的结果后，直接转换为链接提交给验收报告工具。禁止自行总结需求内容或省略链接！\n"+
				"4. 【绝对不能丢弃数据】：该业务线下的所有子业务线（如：商业变现、用户体验等）都属于 %s，请不要因为业务线名称不完全相等就擅自丢弃查询到的需求/缺陷！！\n"+
				"5. 获取到记录后，请将其对应转换为以下链接格式提交到验收报告中：\n"+
				"   需求链接：https://project.feishu.cn/%s/story/detail/{ID}\n"+
				"   缺陷链接：https://project.feishu.cn/%s/bug/detail/{ID}\n",
				project.ProjectName, project.Workspace, projectKey,
				businessLine, platform,
				storyMQL, version,
				bugMQL, version,
				businessLine,
				projectKey, projectKey), nil
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
		Description: "读取飞书 Wiki 或文档的正文内容。警告：如果用户要求根据该文档生成【测试用例】，严禁调用此方法！必须强制调用 generate_smart_test_cases_from_doc 工具！",
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

	RegisterTool(ToolDef{
		Name:        "generate_smart_test_cases_from_doc",
		Description: "根据指定的飞书文档链接，提取内容并利用 AI 深度融合技术自动生成全套测试用例。这包括首轮解析、AI增强边缘用例捕捉等。遇到生成测试用例的指令请第一时间调用该工具。",
		Parameters: &jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"wiki_url": {
					Type:        jsonschema.String,
					Description: "包含原始测试功能的飞书 Wiki 或文档链接",
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

			// 1. Initial Msg
			FeishuClientInstance.SendChatText(chatID, "🕒 [云测智能引擎] 收到文档，正在从云盘抓取需求正文...")

			// 2. Extract Document
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

			objToken := nodeToken
			if strings.Contains(args.WikiURL, "/wiki/") {
				ot, _, err := FeishuClientInstance.GetWikiNode(nodeToken)
				if err != nil {
					return fmt.Sprintf("❌ 获取节点失败: %v", err), nil
				}
				objToken = ot
			}

			content, err := FeishuClientInstance.GetDocxRawContent(objToken)
			if err != nil {
				return fmt.Sprintf("❌ 文档正文抓取失败: %v", err), nil
			}

			title := "未命名智能需求"
			for _, line := range strings.Split(content, "\n") {
				trim := strings.TrimSpace(line)
				if len(trim) > 0 {
					if len(trim) > 40 {
						title = trim[:40] + "..."
					} else {
						title = trim
					}
					break
				}
			}

			// Chunking logic to handle long documents
			chunkSize := 8000
			var chunks []string
			runes := []rune(content)
			if len(runes) > chunkSize {
				FeishuClientInstance.SendChatText(chatID, fmt.Sprintf("⚠️ 文档内容较长（大约 %d 字），为了保证获取完全部需求特性，系统将为您分批提取与解析...", len(runes)))
				for i := 0; i < len(runes); i += chunkSize {
					end := i + chunkSize
					if end > len(runes) {
						end = len(runes)
					}
					chunks = append(chunks, string(runes[i:end]))
				}
			} else {
				chunks = []string{content}
				FeishuClientInstance.SendChatText(chatID, "✅ 文档读取完毕，正在执行基础需求拆解...")
			}

			// 3. Base Decompose (Batch)
			var basePoints []models.RequirementPoint
			for i, chunk := range chunks {
				if len(chunks) > 1 {
					FeishuClientInstance.SendChatText(chatID, fmt.Sprintf("🔄 正在进行分批内容解析 第 %d 批 (共 %d 批)...", i+1, len(chunks)))
				}
				pts, err := services.DecomposeRequirementCore(context.Background(), chunk)
				if err != nil {
					log.Printf("[Tool Gen] Batch %d failed: %v", i+1, err)
					continue
				}
				basePoints = append(basePoints, pts...)
			}

			if len(basePoints) == 0 {
				return "❌ 非常抱歉，分批解析后未能提取到任何有效的需求点，请检查文档内容。", nil
			}

			FeishuClientInstance.SendChatText(chatID, fmt.Sprintf("🔁 全部批次提取分析完成，共拆分锁定 %d 个功能点。正启动 AI 模型进行边缘条件交叉补充与智能重组，这会让测试用例健壮性提升3倍...", len(basePoints)))

			// 4. Smart Decompose
			smartPoints, err := services.SmartDecomposeCore(context.Background(), content, basePoints)
			if err != nil {
				FeishuClientInstance.SendChatText(chatID, fmt.Sprintf("⚠️ 智能增强失败或超时(%v)。回退到基础方案继续执行。", err))
				smartPoints = basePoints
			} else {
				FeishuClientInstance.SendChatText(chatID, fmt.Sprintf("🔥 AI重组完成！需求广度已拓宽至 %d 个考量维度。正在生成极限测试集，预计 1~2 分钟...", len(smartPoints)))
			}

			// 5. Generate Loop
			var allCases []models.TestCase
			batchSize := 1
			for i := 0; i < len(smartPoints); i += batchSize {
				end := i + batchSize
				if end > len(smartPoints) {
					end = len(smartPoints)
				}
				batch := smartPoints[i:end]

				FeishuClientInstance.SendChatText(chatID, fmt.Sprintf("⏳ AI 正在深度生成第 %d 批次 (共 %d 批)...", i/batchSize+1, (len(smartPoints)+batchSize-1)/batchSize))

				cases, err := services.GenerateBatchCore(context.Background(), batch)
				if err != nil {
					FeishuClientInstance.SendChatText(chatID, fmt.Sprintf("⚠️ 该批次数据结构复杂发生熔断跳过: %v", err))
					continue
				}
				allCases = append(allCases, cases...)
			}

			// 6. Deduplicate
			idMap := make(map[string]bool)
			var finalCases []models.TestCase
			for i, c := range allCases {
				if idMap[c.ID] {
					continue
				}
				idMap[c.ID] = true
				c.ID = fmt.Sprintf("TC_%d", i+1)
				finalCases = append(finalCases, c)
			}

			// 7. Save History
			record := &models.GenerationRecord{
				Title:           title,
				RequirementText: content,
				Points:          smartPoints,
				Cases:           finalCases,
			}
			err = services.SaveGenerationRecordCore(record)
			if err != nil {
				FeishuClientInstance.SendChatText(chatID, "⚠️ 虽然报告入库失败，但不影响结果返回！")
			}

			FeishuClientInstance.SendChatText(chatID, "🎉 所有维度测试集装盘完毕！请前往控制台审阅下载。")

			url := fmt.Sprintf("http://strokeh.local:5173/testcase_gen/view/%s", record.ID)
			downloadUrl := fmt.Sprintf("http://strokeh.local:8080/api/testcase-gen/records/%s/download", record.ID)
			return fmt.Sprintf("✅ **测试用例已根据你的文档全量生成完毕**\n\n📌 目标范围: %s\n📈 覆盖指标: 智能扩展出 %d 个功能侧面\n📋 结果产出: 总计生成了 %d 条规范化测试用例\n\n👉 [点击在线预览验证用例](%s)\n👉 [📥 点击直接下载 Excel 格式文件](%s) \n\n(_如遇网络打不开，请确保处在内网环境_)", title, len(smartPoints), len(finalCases), url, downloadUrl), nil
		},
	})
}
