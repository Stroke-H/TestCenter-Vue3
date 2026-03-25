package service

import (
	"github.com/sashabaranov/go-openai"
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
	ChatID         string
	State          string
	History        []openai.ChatCompletionMessage
	PendingTool    *openai.ToolCall
	PendingUserID  string // For binding confirmation
	LastToolResult string // Stores the raw execution result of the last tool (e.g., JSON report)
	LastActive     time.Time
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
		"4.【项目智能识别与动态扩建】：支持识别 swa, 1100 等代号。若不确定，请根据工具建议列表询问用户。\n" +
		"5.【全案回复风格】：礼貌、专业、不啰嗦。禁止自行加粗字符串。"
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
