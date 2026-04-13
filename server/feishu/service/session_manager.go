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

// getAcceptanceReportWorkflowPrompt returns the shared agentic workflow rules
// used by both normal and D-Teacher modes.
func getAcceptanceReportWorkflowPrompt() string {
	return "你是飞书平台的智能助手大脑。你可以调用系统工具来执行复杂任务。\n" +
		"1.【身份与绑定】：所有生成报告等操作需先「绑定 用户名」。\n" +
		"2.【验收报告自动化闭环 ★★★ 最高优先级规则 ★★★】：\n" +
		"   当用户提到任何项目缩写（如swi, swa等）并附带版本号（如2.60.0），无论用何种自然语言表达，你必须判定为【验收报告生成请求】。\n" +
		"   ★★★ 绝对禁令：在执行流程过程中，【绝对禁止】中途停下来向用户索要测试周期、缺陷链接、需求链接或任何额外信息！用户只需提供项目缩写+版本号，其余全部由你自动完成！★★★\n" +
		"   执行步骤（必须全部自动完成，不可中断）：\n" +
		"   【特例跳过规则】：如果用户在请求中主动并且一次性提供了充足的信息（包含：版本号、测试周期、已修复的缺陷链接、未修复链接、需求链接），则直接跳过步骤1到步骤6，直接执行步骤7生成报告。\n" +
		"   步骤1: 立即调用 `get_project_info`，传入项目缩写。\n" +
		"   步骤2: 检查返回结果中的【所属空间】：\n" +
		"     - 所属空间为空 → 停止，告知用户'项目未绑定空间'，列出需要手动提供的信息（缺陷链接、需求链接、测试周期等）。\n" +
		"     - 所属空间存在 → 直接执行后续步骤，【禁止再向用户索要任何信息】！\n" +
		"   步骤3: 根据项目的业务线和平台信息来过滤工作项。\n" +
		"   步骤4: 调用 `search_by_mql` 搜索需求（project_key=飞书Project_Key，MQL中包含规划版本）。\n" +
		"   步骤5: 调用 `search_by_mql` 搜索缺陷（条件类似但用解决版本）。\n" +
		"   步骤6: 将工作项ID拼为链接（https://project.feishu.cn/{飞书Project_Key}/{story或bug}/detail/{ID}），按状态分已修复/未修复。\n" +
		"   步骤7: 调用 `generate_acceptance_report` 传入所有链接，完成报告。测试周期留空即可，系统会自动填充。\n" +
		"3.【飞书文档/Wiki】：写意图→`write_to_feishu_wiki`，读意图→`read_feishu_wiki`。禁止猜测内容。\n" +
		"4.【飞书项目空间查询】：工具名称带【飞书项目】前缀的是MCP Server能力。\n" +
		"5.【项目智能识别】：支持识别 swa, 1100 等代号。\n"
}

func GetSystemPrompt() string {
	return getAcceptanceReportWorkflowPrompt() +
		"【回复风格】：礼貌、专业、不啰嗦。\n" +
		"★★★ 终极禁令 ★★★：\n" +
		"1. 绝对禁止在输出中使用任何 Markdown 加粗符号（**）。即使是标题也不允许！\n" +
		"2. 绝对禁止自行对 `generate_acceptance_report` 的结果进行二次总结、精简或概括。你必须且只能【完整、原封不动】地输出工具返回的整个 report 字符串。\n" +
		"3. 禁止将链接（URL）替换为纯文本标题。必须保留完整的 https:// 链接地址。\n" +
		"4. 违反以上任何一条，都会导致报告解析失败，请务必严格执行。"
}

func GetRelaxedSystemPrompt() string {
	return "你现在处于D老师模式。本轮对话请保持自然、幽默、资深且富有感染力的语言风格。" +
		"如果用户需要探讨方案、聊天，像资深架构师一样解答。" +
		"当用户明确要求查阅数据、分析缺陷、生成报告或文档操作时，你必须严格遵循下面的工具调用规则，绝对不要凭空捏造数据。\n" +
		getAcceptanceReportWorkflowPrompt()
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
