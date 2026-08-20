package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"testcenter-server/feishu/client"
	"testcenter-server/feishu/model"
	"testcenter-server/services"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
)

const feishuBotDisplayName = "TesterByClaw"

// NewEventDispatcher creates a dispatcher for handle events
func NewEventDispatcher(verifyToken, encryptKey string) *dispatcher.EventDispatcher {
	return dispatcher.NewEventDispatcher(verifyToken, encryptKey).
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			handleMessageReceive(ctx, event)
			return nil
		}).
		OnP2MessageReadV1(func(ctx context.Context, event *larkim.P2MessageReadV1) error {
			// Dummy handler to ignore read receipts
			return nil
		}).
		OnP2CardActionTrigger(handleCardActionTrigger)

}

func handleCardActionTrigger(_ context.Context, event *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error) {
	if event == nil || event.Event == nil || event.Event.Action == nil {
		return nil, nil
	}
	action := event.Event.Action.Value
	actionName, _ := action["action"].(string)
	if actionName != "acceptance_todo_done" {
		return nil, nil
	}

	reminderID, _ := action["reminder_id"].(string)
	groupReminderIDs, _ := action["group_reminder_ids"].(string)
	operatorOpenID := ""
	if event.Event.Operator != nil {
		operatorOpenID = event.Event.Operator.OpenID
	}
	reminder, err := services.CompleteAcceptanceTodoReminder(reminderID, operatorOpenID)
	if err != nil {
		log.Printf("[AcceptanceTodo] Done callback failed: %v", err)
		return &larkcallback.CardActionTriggerResponse{
			Toast: &larkcallback.Toast{Type: "error", Content: err.Error()},
		}, nil
	}

	reminderIDs := []string{reminderID}
	if strings.TrimSpace(groupReminderIDs) != "" {
		reminderIDs = strings.Split(groupReminderIDs, ",")
	}
	group, groupErr := services.GetAcceptanceTodoRemindersForCard(reminderIDs, operatorOpenID)
	if groupErr != nil {
		log.Printf("[AcceptanceTodo] reload card group failed: %v", groupErr)
		group = []services.AcceptanceTodoReminder{reminder}
	}

	log.Printf("[AcceptanceTodo] completed: id=%s project=%s", reminder.ID, reminder.ProjectCode)
	return &larkcallback.CardActionTriggerResponse{
		Toast: &larkcallback.Toast{Type: "success", Content: "已完成，后续不再提醒"},
		Card: &larkcallback.Card{
			Type: "card_json",
			Data: services.BuildAcceptanceTodoReminderGroupCard(group),
		},
	}, nil
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

	// 4. Output
	log.Printf("[Feishu Standardized Output] User: %s, Content: %s, ChatID: %s, ChatType: %s\n",
		stdMsg.SenderID, stdMsg.Content, stdMsg.ChatID, stdMsg.ChatType)

	// Prettified JSON log for verification
	log.Println(larkcore.Prettify(stdMsg))

	// Persist standardized message for audit and later trace enrichment.
	if err := services.SQLUpsertJSONForFeishu("feishu_messages", stdMsg); err != nil {
		log.Printf("[Feishu] persist standardized message failed: %v", err)
	}

	// 0. Handle Multi-turn Confirmation States
	session := GetOrCreateSession(stdMsg.ChatID)

	if stdMsg.ChatType == "group" {
		cleanContent := stripAllMentions(stdMsg)
		isTriggered := hasBotMention(stdMsg) || IsDTeacherMode(cleanContent) || IsUnlockMode(cleanContent)
		if isTriggered {
			session.ActivateGroupConversation(stdMsg.SenderID, 2)
			session.ConsumeGroupConversationTurn()
		} else if session.CanContinueGroupConversation(stdMsg.SenderID) {
			session.ConsumeGroupConversationTurn()
		} else {
			return
		}
	}

	if session.State == StateWaitBindConfirm {
		cleanMsg := strings.TrimSpace(stripAllMentions(stdMsg))
		if isConfirmation(cleanMsg) {
			// Execute pending binding
			err := services.UpdateUserFeishuOpenID(session.PendingUserID, stdMsg.SenderID)
			if err != nil {
				replyText(ctx, stdMsg.MsgID, fmt.Sprintf("❌ 绑定失败: %v", err))
			} else {
				replyText(ctx, stdMsg.MsgID, "✅ 确认成功！绑定已完成。")
			}
		} else {
			replyText(ctx, stdMsg.MsgID, "已取消绑定操作。")
		}
		session.State = StateNormal
		session.PendingUserID = ""
		return
	}

	// 1. Mandatory Binding Check for sensitive operations
	cleanRaw := stripAllMentions(stdMsg)
	isBindingCmd := strings.HasPrefix(cleanRaw, "绑定") || strings.HasPrefix(cleanRaw, "帮我绑定") || strings.HasPrefix(cleanRaw, "解绑") || strings.HasPrefix(cleanRaw, "取消绑定")

	if !isBindingCmd {
		boundUser, err := services.FindUserByFeishuOpenID(stdMsg.SenderID)
		if err != nil || boundUser == nil {
			replyText(ctx, stdMsg.MsgID, fmt.Sprintf("⚠️ 【身份验证提醒】\n您尚未绑定测试平台账号，无法使用删号、生成报告等核心功能。\n\n• 请先发送：「绑定 用户名」完成账号关联。\n• 如果您还没有账号，请前往测试平台新建账户：\n🔗 地址: %s\n\n完成后即可开始您的报告协作！", services.PlatformFrontendURL("/register")))
			return
		}
	}

	if stdMsg.MsgType == "text" && (strings.HasPrefix(stripAllMentions(stdMsg), "绑定") || strings.HasPrefix(stripAllMentions(stdMsg), "帮我绑定")) {
		cleanContent := stripAllMentions(stdMsg)
		// Handle full-width characters
		cleanContent = strings.ReplaceAll(cleanContent, "：", ":")
		cleanContent = strings.ReplaceAll(cleanContent, "　", " ")
		cleanContent = strings.TrimSpace(cleanContent)

		log.Printf("[Feishu Command] Intercepted binding command: %s", cleanContent)

		// 1. Check uniqueness first
		existingUser, err := services.FindUserByFeishuOpenID(stdMsg.SenderID)
		if err == nil && existingUser != nil {
			replyText(ctx, stdMsg.MsgID, fmt.Sprintf("⚠️ 您当前的飞书账号已绑定测试账户: %s。\n如需绑定新账号，请先发送【解绑】以清空当前关系。", existingUser.Username))
			return
		}

		targetName := ""
		if strings.HasPrefix(cleanContent, "帮我绑定") {
			targetName = strings.TrimSpace(strings.TrimPrefix(cleanContent, "帮我绑定"))
		} else {
			targetName = strings.TrimSpace(strings.TrimPrefix(cleanContent, "绑定"))
		}

		targetName = strings.TrimPrefix(targetName, ":")
		targetName = strings.TrimSpace(targetName)

		if targetName == "" {
			replyText(ctx, stdMsg.MsgID, "⚠️ 请输入要绑定的用户名，例如：绑定 minghong")
			return
		}

		user, isExact, err := services.FindUserByFuzzyName(targetName)
		if err != nil || user == nil {
			replyText(ctx, stdMsg.MsgID, fmt.Sprintf("未查询到匹配用户: %s", targetName))
			return
		}

		if isExact {
			// Execute binding immediately for exact match
			err = services.UpdateUserFeishuOpenID(user.ID, stdMsg.SenderID)
			if err != nil {
				replyText(ctx, stdMsg.MsgID, fmt.Sprintf("绑定失败: %v", err))
			} else {
				replyText(ctx, stdMsg.MsgID, fmt.Sprintf("✅ 绑定成功！\n平台用户: %s\n飞书 ID: %s", user.Username, stdMsg.SenderID))
			}
		} else {
			// Fuzzy match found, entering confirmation state
			session.State = StateWaitBindConfirm
			session.PendingUserID = user.ID
			replyText(ctx, stdMsg.MsgID, fmt.Sprintf("❓ 未找到精确匹配。您是指要绑定用户【%s】吗？\n(回复“是/确定”继续，或回复其他内容取消)", user.Username))
		}
	} else if stdMsg.MsgType == "text" && (strings.TrimSpace(stripAllMentions(stdMsg)) == "解绑" || strings.TrimSpace(stripAllMentions(stdMsg)) == "取消绑定") {
		err := services.UnbindFeishuOpenID(stdMsg.SenderID)
		if err != nil {
			replyText(ctx, stdMsg.MsgID, fmt.Sprintf("❌ 解绑失败: %v", err))
		} else {
			replyText(ctx, stdMsg.MsgID, "✅ 解绑成功！您当前的飞书账号已不再关联任何测试账户。")
		}
	} else if stdMsg.MsgType == "text" {
		// UNIFIED LOGIC: Unify P2P and specific Group ID logic
		cfg := model.GlobalFeishuConfig
		isAllowedGroup := cfg != nil && cfg.GroupID != "" && stdMsg.ChatID == cfg.GroupID

		if stdMsg.ChatType == "p2p" || isAllowedGroup {
			// Strip mentions for cleaner AI input (handles both <at> and @_user_1)
			cleanContent := stripAllMentions(stdMsg)

			go func() {
				// Add a "THUMBSUP" reaction to act as a "Processing..." indicator
				var reactionId *string
				cli := client.GetClient()
				if cli != nil {
					reactionReq := larkim.NewCreateMessageReactionReqBuilder().
						MessageId(stdMsg.MsgID).
						Body(larkim.NewCreateMessageReactionReqBodyBuilder().
							ReactionType(larkim.NewEmojiBuilder().EmojiType("OnIt").Build()).
							Build()).
						Build()
					resp, err := cli.Im.MessageReaction.Create(context.Background(), reactionReq)
					if err != nil {
						log.Printf("[Feishu Reaction] Error adding reaction: %v", err)
					} else if !resp.Success() {
						log.Printf("[Feishu Reaction] Failed: Code=%d, Msg=%s", resp.Code, resp.Msg)
					} else {
						log.Printf("[Feishu Reaction] Successfully added processing indicator")
						if resp.Data != nil && resp.Data.ReactionId != nil {
							reactionId = resp.Data.ReactionId
						}
					}
				}

				aiResponse := ProcessChat(ctx, stdMsg.ChatID, stdMsg.SenderID, cleanContent, "work")

				if cli != nil && reactionId != nil {
					cli.Im.MessageReaction.Delete(context.Background(), larkim.NewDeleteMessageReactionReqBuilder().
						MessageId(stdMsg.MsgID).
						ReactionId(*reactionId).
						Build())
				}

				replyText(ctx, stdMsg.MsgID, aiResponse)
			}()
		}
	}
}

func hasBotMention(stdMsg *model.StandardizedMessage) bool {
	for _, mention := range stdMsg.Mentions {
		if strings.TrimSpace(mention.Name) == feishuBotDisplayName {
			return true
		}
	}
	return false
}

// stripAllMentions removes legacy <at> tags AND modern @_user_1 style mentions from content
func stripAllMentions(stdMsg *model.StandardizedMessage) string {
	content := stdMsg.Content
	// 1. Remove XML-style <at> tags
	re := regexp.MustCompile(`<at [^>]*>[^<]*</at>`)
	content = re.ReplaceAllString(content, "")

	// 2. Remove mention keys identified by Feishu (like @_user_1)
	for _, m := range stdMsg.Mentions {
		if m.Key != "" {
			content = strings.ReplaceAll(content, m.Key, "")
		}
	}

	return strings.TrimSpace(content)
}

// replyText sends a basic text reply to the user using the Lark API
func replyText(ctx context.Context, msgId string, text string) {
	cli := client.GetClient()
	if cli == nil {
		return
	}

	msgMap := map[string]string{"text": text}
	b, _ := json.Marshal(msgMap)

	req := larkim.NewReplyMessageReqBuilder().
		MessageId(msgId).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			Content(string(b)).
			MsgType("text").
			Build()).
		Build()

	resp, err := cli.Im.Message.Reply(ctx, req)
	if err != nil {
		log.Printf("[Feishu Reply Error] %v", err)
	} else if !resp.Success() {
		log.Printf("[Feishu Reply Failed] Code: %d, Msg: %s", resp.Code, resp.Msg)
	}
}

// fetchWikiAndReply attempts to read a wiki linking directly and reply the raw text
func fetchWikiAndReply(ctx context.Context, wikiToken string, msgId string) {
	cli := client.GetClient()
	if cli == nil {
		return
	}

	replyText(ctx, msgId, "收到 Wiki 链接，正在尝试通过云文档 API 提取内容... \n(提示: 需确保你已将飞书助手添加为该文档的阅读者)")

	nodeReq := larkwiki.NewGetNodeSpaceReqBuilder().Token(wikiToken).Build()
	nodeResp, err := cli.Wiki.Space.GetNode(ctx, nodeReq)

	if err != nil {
		replyText(ctx, msgId, fmt.Sprintf("API 调用异常设: %v", err))
		return
	}
	if !nodeResp.Success() {
		replyText(ctx, msgId, fmt.Sprintf("权限不足或不存在！API 被拒绝。\n错误码: %d\n错误信息: %s\n这可能意味着你需要把机器人加到文档的分享权限中。", nodeResp.Code, nodeResp.Msg))
		return
	}

	if nodeResp.Data == nil || nodeResp.Data.Node == nil {
		replyText(ctx, msgId, "文档信息为空。")
		return
	}

	objType := *nodeResp.Data.Node.ObjType
	objToken := *nodeResp.Data.Node.ObjToken

	if objType == "docx" || objType == "doc" {
		docReq := larkdocx.NewRawContentDocumentReqBuilder().DocumentId(objToken).Build()
		docResp, err := cli.Docx.Document.RawContent(ctx, docReq)
		if err == nil && docResp.Success() {
			content := *docResp.Data.Content
			if len(content) > 1000 {
				content = content[:1000] + "...\n[内容由于太长被截断]"
			}
			replyText(ctx, msgId, fmt.Sprintf("📖 提取成功（格式 %s），前1000字预览如下：\n%s", objType, content))
		} else {
			if err != nil {
				replyText(ctx, msgId, fmt.Sprintf("请求文档正文发生网络错误: %v", err))
			} else {
				replyText(ctx, msgId, fmt.Sprintf("抽取失败！API 业务报错 Code: %d, Msg: %s\n类型: %s (注: docx API 只能读新版文档，如果提示 Document not found 可能是旧版 doc！另外请确认在文档右上角【分享】里真正把机器人加为了协作者。)", docResp.Code, docResp.Msg, objType))
			}
		}
	} else {
		replyText(ctx, msgId, fmt.Sprintf("抱歉，API 获取成功，但该节点是一个 [%s] 类型的文件，目前我的逻辑仅解析了 docx 纯文本。", objType))
	}
}
