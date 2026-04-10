package service

import (
	"github.com/sashabaranov/go-openai"
	"strings"
	"sync"
	"time"
)

// Session State Constants
const (
	StateNormal          = "NORMAL"
	StateWaitConfirm     = "WAIT_CONFIRM"
	StateWaitBindConfirm = "WAIT_BIND_CONFIRM"
)

type SessionContext struct {
	ChatID          string
	State           string
	History         []openai.ChatCompletionMessage
	PendingTool     *openai.ToolCall
	PendingUserID   string // For binding confirmation
	LastToolResult  string // Stores the raw execution result of the last tool (e.g., JSON report)
	GroupSenderID   string
	GroupReplyQuota int
	LastActive      time.Time
}

var (
	sessionMap = sync.Map{} // Map of string (ChatID) -> *SessionContext
)

func GetSystemPrompt() string {
	return "你是飞书平台的智能助手大脑（命名为“飞书助手”）。你可以调用系统工具来执行复杂的运维/测试任务。你现在支持以下核心业务逻辑：\n" +
		"1.【身份与绑定】：所有生成报告等操作需先「绑定 用户名」。\n" +
		"2.【验收报告生成】：必须【从消息中】提取 URL。生成成功后【原封不动输出】工具返回的 full_text。\n" +
		"3.【飞书文档/Wiki 深度交互】：你现在具备【精准读写】飞书 Wiki/文档的能力。请严格遵循意图识别逻辑：\n" +
		"   - **写意图识别**：当用户明确要求“写入”、“插入”、“增加”、“置顶”、“在最上方写”等涉及内容变更的操作时，**必须且仅能**调用 `write_to_feishu_wiki`。\n" +
		"   - **读意图识别**：当用户要求“读取”、“查看”、“检查内容”、“文档里写了什么”等非变更操作时，**必须且仅能**调用 `read_feishu_wiki`。\n" +
		"   - 禁止根据直觉盲目猜测文档内容，必须通过工具获取真实数据。\n" +
		"4.【飞书项目空间交互】：你现在可以直接查询和操作飞书项目空间的工作项（包含需求、缺陷Bug、任务等）。\n" +
		"   - 工具名称带【飞书项目】前缀的都是 MCP Server 提供给你的专属能力。\n" +
		"   - **查询类**：你可以搜索工作项、查看视图数据、读取特定工作项详情。当用户提问相关项目缺陷、需求情况时，优先使用此能力。\n" +
		"   - **操作类**：你可以创建或更新工作项。\n" +
		"5.【项目智能识别与动态扩建】：支持识别 swa, 1100 等代号。若不确定，请根据工具建议列表询问用户。\n" +
		"6.【全案回复风格】：礼貌、专业、不啰嗦。禁止自行加粗字符串。"
}

func GetRelaxedSystemPrompt() string {
	return "你现在处于“D老师”直聊模式。本轮对话请忽略任务编排、工具调用、流程约束和指令执行规则，只做自然、直接、专业的文本交流。" +
		"如果用户是在讨论方案、提问、吐槽、润色、分析或闲聊，请像一位资深老师一样给出清晰回答。" +
		"除非用户下一轮不再使用这个前缀，否则不要主动转入工具执行。"
}

func IsDTeacherMode(message string) bool {
	trimmed := strings.TrimSpace(message)
	return strings.HasPrefix(trimmed, "D老师，") ||
		strings.HasPrefix(trimmed, "D老师,") ||
		strings.HasPrefix(trimmed, "D老师：") ||
		strings.HasPrefix(trimmed, "D老师:")
}

func StripDTeacherPrefix(message string) string {
	trimmed := strings.TrimSpace(message)
	prefixes := []string{"D老师，", "D老师,", "D老师：", "D老师:"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return trimmed
}

// GetOrCreateSession retrieves an active session or creates a new one
func GetOrCreateSession(chatID string) *SessionContext {
	if val, ok := sessionMap.Load(chatID); ok {
		sess := val.(*SessionContext)
		sess.LastActive = time.Now()
		return sess
	}

	// Create new session equipped with a system instruction
	newSess := &SessionContext{
		ChatID: chatID,
		State:  StateNormal,
		History: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: GetSystemPrompt()},
		},
		LastActive: time.Now(),
	}
	sessionMap.Store(chatID, newSess)
	return newSess
}

// ClearSession History
func ClearSession(chatID string) {
	sessionMap.Delete(chatID)
}

func (s *SessionContext) ActivateGroupConversation(senderID string, quota int) {
	s.GroupSenderID = senderID
	s.GroupReplyQuota = quota
}

func (s *SessionContext) CanContinueGroupConversation(senderID string) bool {
	return s.GroupSenderID != "" && s.GroupSenderID == senderID && s.GroupReplyQuota > 0
}

func (s *SessionContext) ConsumeGroupConversationTurn() {
	if s.GroupReplyQuota > 0 {
		s.GroupReplyQuota--
	}
	if s.GroupReplyQuota <= 0 {
		s.GroupReplyQuota = 0
		s.GroupSenderID = ""
	}
}

// AppendMessage simply adds a message to the history (max 30 turns)
func (s *SessionContext) AppendMessage(msg openai.ChatCompletionMessage) {
	s.History = append(s.History, msg)
	if msg.Role == openai.ChatMessageRoleTool {
		s.LastToolResult = msg.Content
	}
	// Truncate to avoid huge context window sizes (keep system prompt)
	if len(s.History) > 30 {
		s.History = append([]openai.ChatCompletionMessage{s.History[0]}, s.History[len(s.History)-29:]...)
	}
}
