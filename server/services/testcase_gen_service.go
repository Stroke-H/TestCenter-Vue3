package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"testcenter-server/feishu/model"
	"testcenter-server/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/xuri/excelize/v2"
)

// DecomposeReq 接收原始需求文本
type DecomposeReq struct {
	Text    string `json:"text"`
	WikiURL string `json:"wiki_url"`
}

type SmartDecomposeReq struct {
	Text                 string                    `json:"text" binding:"required"`
	ExistingPoints       []models.RequirementPoint `json:"existing_points"`
	PreviousAnalyzerUsed bool                      `json:"previous_analyzer_used"`
}

// GenerateReq 接收需求点列表
type GenerateReq struct {
	Points       []models.RequirementPoint `json:"points" binding:"required"`
	Text         string                    `json:"text"`
	AnalyzerUsed bool                      `json:"analyzer_used"`
}

type ReviewReq struct {
	RoleKey        string                    `json:"role_key" binding:"required"`
	Text           string                    `json:"text" binding:"required"`
	Points         []models.RequirementPoint `json:"points" binding:"required"`
	Cases          []models.TestCase         `json:"cases" binding:"required"`
	AnalyzerUsed   bool                      `json:"analyzer_used"`
	ExistingReview map[string]any            `json:"existing_review"`
}

type OptimizeCasesReq struct {
	Text          string                                 `json:"text" binding:"required"`
	Points        []models.RequirementPoint              `json:"points" binding:"required"`
	Cases         []models.TestCase                      `json:"cases" binding:"required"`
	ReviewResults map[string]models.TestCaseReviewResult `json:"review_results" binding:"required"`
	AnalyzerUsed  bool                                   `json:"analyzer_used"`
}

// ExportReq 接收用例列表
type ExportReq struct {
	Cases []models.TestCase `json:"cases" binding:"required"`
}

type testcaseAIExecution struct {
	Content  string `json:"content"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type testcaseAIModelMeta struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

var (
	historyLock sync.RWMutex
)

var testcaseBoundaryKeywords = []string{
	"边界", "极限", "最大", "最小", "超长", "过长", "上限", "下限", "空值", "为空",
	"null", "nil", "超出", "临界", "最短", "最长", "大数据量", "边缘",
}

var shortDramaAnalyzerKeywords = []string{
	"短剧", "短剧app", "追剧", "看剧", "剧集", "全集", "选集", "剧架", "收藏", "继续观看",
	"历史记录", "解锁", "金币", "bonus", "coin", "vip", "会员", "订阅", "预约", "for you",
	"more tab", "full episodes", "episode", "episodes", "waterfall feed", "wallet", "奖励页",
	"任务页", "广告任务", "视频任务", "自动解锁", "自动播放", "drama", "short drama", "novel-like",
}

const shortDramaAnalyzerSummary = `【短剧类 App Analyzer 摘要】
这类产品通常围绕以下功能模块展开，请先参考这些模块语义，再理解需求属于哪个功能层面：
1. 首页：搜索、VIP 入口、任务入口、Banner、自动播放版位、VIP 专区、预约位、瀑布流、榜单入口。
2. For You / More：预览流、视频信息覆盖层、全集入口、收藏/分享、分类筛选、资源库。
3. 内容播放：竖屏播放、选集、自动播放、倍速、术语表/字幕、点赞评论分享。
4. 任务奖励：签到、激励任务、广告任务、视频任务、奖励 Banner。
5. 剧架/历史：收藏列表、历史记录、编辑删除、基于行为的推荐。
6. 钱包/VIP：金币充值、订阅会员、消费记录、按集解锁。
7. 个人中心：账号信息、VIP 中心、钱包、继续观看、下载、反馈、语言、自动解锁/自动播放等设置。
核心交互通常包含：沉浸式上下滑、前几集免费后付费/看广告解锁、任务赚金币、继续观看/历史联动。`

const (
	testcaseAICapabilityDecompose      = "testcase_decompose"
	testcaseAICapabilitySmartDecompose = "testcase_smart_decompose"
	testcaseAICapabilityGenerate       = "testcase_generate"
	testcaseAICapabilityReview         = "testcase_review"
	testcaseAICapabilityOptimize       = "testcase_optimize"
)

var testcaseReviewRoles = []models.TestCaseReviewRole{
	{
		Key:         "product_manager",
		Name:        "产品经理",
		Description: "从业务目标、主流程、状态切换和规则覆盖的角度评审测试用例。",
		IdentityMD: `# 产品经理评审身份

你是一名资深产品经理，正在评审一组测试用例。

你的核心目标：
1. 判断这些测试用例是否真实覆盖了需求目标、业务规则和用户主流程。
2. 检查是否遗漏关键产品状态、条件分支、前后置限制和业务边界。
3. 关注“需求是否被测到”，而不是实现细节是否优雅。

你的重点关注：
- 主流程是否完整
- 关键业务规则是否覆盖
- 用户路径是否闭环
- 状态切换是否覆盖
- 配置开关、权益差异、角色差异是否覆盖
- 是否遗漏会影响上线验收的核心场景

你不重点关注：
- 代码实现方式
- 底层接口性能
- 前端像素级视觉细节
- 纯技术框架问题

你的输出结构：
1. 总体结论
2. 已覆盖较好的产品点
3. 当前遗漏的产品需求点
4. 建议新增的测试点
5. 建议删除或合并的低价值用例
6. 风险等级`,
	},
	{
		Key:         "developer",
		Name:        "开发",
		Description: "从边界、异常、状态同步和技术风险覆盖的角度评审测试用例。",
		IdentityMD: `# 开发评审身份

你是一名资深后端/客户端开发工程师，正在评审一组测试用例。

你的核心目标：
1. 判断测试用例是否覆盖了实现层面高风险逻辑。
2. 检查是否遗漏边界条件、异常处理、幂等性、状态同步、接口返回差异等技术风险场景。
3. 从可实现性和系统行为角度评估这批用例是否足够扎实。

你的重点关注：
- 空值、非法值、边界值
- 状态流转异常
- 重复操作、幂等、并发
- 前后端状态不一致
- 接口异常返回
- 配置同步、缓存延迟、落库失败、回滚失败
- 复杂条件组合下的行为差异

你不重点关注：
- 产品文案是否好看
- UI 视觉是否美观
- 运营目标是否达成

你的输出结构：
1. 总体结论
2. 已覆盖较好的技术风险点
3. 缺失的边界/异常/状态类测试点
4. 建议新增的技术风险用例
5. 建议合并或删除的重复用例
6. 风险等级`,
	},
	{
		Key:         "tester",
		Name:        "测试",
		Description: "从覆盖面、执行性、粒度和预期可验证性角度评审测试用例。",
		IdentityMD: `# 测试评审身份

你是一名资深测试工程师/测试负责人，正在评审一组测试用例。

你的核心目标：
1. 判断这批用例是否覆盖主流程、异常流、边界流和高风险流。
2. 评估这批用例是否具备执行价值，而不是只有形式上的完整。
3. 检查是否存在遗漏、重复、粒度失衡、不可执行、不可验证的问题。

你的重点关注：
- 覆盖面是否均衡
- 是否存在明显漏测
- 是否有重复验证目标
- 用例粒度是否合适
- 预期结果是否可验证
- 步骤是否可执行
- 是否缺少前置条件
- 是否缺少优先级和风险排序

你不重点关注：
- 代码内部实现细节
- UI 审美偏好
- 纯业务立项逻辑

你的输出结构：
1. 总体结论
2. 覆盖较好的测试维度
3. 漏测点
4. 重复或低价值测试点
5. 不可执行/不可验证问题
6. 建议新增的测试用例方向
7. 风险等级`,
	},
	{
		Key:         "artist",
		Name:        "美术",
		Description: "从视觉呈现、交互反馈、界面状态和一致性角度评审测试用例。",
		IdentityMD: `# 美术评审身份

你是一名资深 UI/UX / 视觉设计评审角色，正在评审一组测试用例。

你的核心目标：
1. 判断这批用例是否覆盖视觉呈现、交互反馈和界面一致性相关风险。
2. 关注用户看到的结果是否正确、清晰、一致、符合预期。
3. 帮助识别那些虽然功能通了，但体验表现可能有问题的场景。

你的重点关注：
- 页面布局是否完整
- 文案、按钮、状态标签、图标是否正确
- 交互反馈是否清晰
- 选中态、禁用态、加载态、异常态是否覆盖
- 视觉层级是否混乱
- 不同状态切换时 UI 是否一致
- 弹窗、提示、空态、错误态是否合理

你不重点关注：
- 底层代码实现
- 接口技术细节
- 后端幂等性和并发逻辑

你的输出结构：
1. 总体结论
2. 已覆盖较好的视觉/交互点
3. 当前遗漏的界面状态或交互反馈
4. 建议新增的视觉/交互测试点
5. 建议补充的异常态/空态/加载态检查
6. 风险等级`,
	},
}

func normalizeTestCaseText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer(
		" ", "",
		"\n", "",
		"\t", "",
		"，", "",
		",", "",
		"。", "",
		".", "",
		"：", "",
		":", "",
		"（", "",
		"）", "",
		"(", "",
		")", "",
		"'", "",
		"’", "",
		"\"", "",
	)
	return replacer.Replace(text)
}

func isShortDramaAppRequirement(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}

	matchCount := 0
	for _, keyword := range shortDramaAnalyzerKeywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			matchCount++
			if matchCount >= 2 {
				return true
			}
		}
	}

	return false
}

func resolveShortDramaAnalyzerUsage(text string, previousAnalyzerUsed bool) bool {
	if previousAnalyzerUsed {
		return true
	}
	return isShortDramaAppRequirement(text)
}

func buildRequirementDecomposePrompt(useShortDramaAnalyzer bool) string {
	testerRole, _ := getTestcaseReviewRole("tester")
	basePrompt := `你是一名资深的业务分析师和需求工程师。
用户的输入是一段原始的需求文档、功能描述或用户故事。
你的任务是先识别哪些内容属于“项目功能层面的需求”，再将这些功能需求拆解为结构化的需求点。

你还必须同时代入以下测试角色身份，用测试视角约束拆解粒度与稳定性：
` + testerRole.IdentityMD + `

第一步：先做需求筛选，只保留与项目功能直接相关、适合生成测试用例的内容。
- 保留：页面功能、业务流程、交互动作、状态变化、显式业务规则、必要边界、异常处理、与项目功能直接关联的会员/付费/解锁/任务/播放/收藏/分享/筛选/记录等逻辑。
- 排除：投放策略、运营目标、营销话术、文案润色、纯商业结论、数据指标目标、排期描述、纯 UI 风格审美要求、无明确交互的视觉建议、泛泛的“体验更好/性能更高”表述、与项目功能无关的外部流程。
- 如果原文同时包含功能需求和非功能/非项目内容，只提取功能需求部分进行拆解。

第二步：对保留下来的功能需求做结构化拆解。

拆解原则：
1. 默认聚焦 App 本身的业务功能、页面流程、状态变化、核心规则和必要边界。
2. 只有当原始需求明确提到，或该功能属于高风险核心链路（如登录、支付、下单、账号、订阅、权限）时，才补充性能、安全、兼容性等非功能点。
3. 不要为了“覆盖更全”而机械拆出浏览器兼容、无障碍、老旧设备适配等泛化需求，除非原文明确要求。
4. feature 必须是适合落成功能测试用例的功能点，不要把同一断言拆成多个近义点。
5. 如果某条内容无法映射成项目功能测试点，就不要输出。
6. 为了保证同一需求多次拆解结果稳定，请坚持“按独立可验证功能点拆解”的固定粒度：
   - 同一功能下，仅当存在独立前置条件、独立状态切换、独立规则集合、独立异常处理时才拆成多个点。
   - 纯粹的文案润色、同义描述、顺序变化、描述展开，不构成新的需求点。
   - 一个需求点通常应对应 1 组主流程 + 关键规则，不要把单条规则拆成多个几乎等价的点。
7. 输出顺序保持稳定：按业务流程从上到下、从前台到后台、从主流程到补充分支排序。

输出格式：必须严格输出一个 JSON 数组。
- 不要包含任何解释性文字或 Markdown 标记之外的内容。
- 不要输出多余的标点符号或在 JSON 结尾添加句号。
- 每个对象包含：module, feature, description, rules。

示例格式：
[
  {
    "module": "用户中心",
    "feature": "手机号注册",
    "description": "用户通过输入手机号和短信验证码完成注册",
    "rules": ["手机号必须为11位数字", "验证码5分钟内有效"]
  }
]`

	if !useShortDramaAnalyzer {
		return basePrompt
	}

	return basePrompt + "\n\n" + shortDramaAnalyzerSummary + `

额外要求（仅适用于短剧类 App）：
1. 先参考上述短剧产品功能结构，再判断原文提到的内容是不是项目功能需求。
2. 优先将需求归入：首页、For You/More、内容播放、任务奖励、剧架/历史、钱包/VIP、个人中心等模块。
3. 只有和这些产品功能链路直接相关的需求才输出；不要把投放、买量、素材、运营目标、泛化增长策略当成测试用例需求。
4. 如果原文提到“上线新版本”“优化策略”“提升转化”等模糊目标，只有在能落到具体产品功能改动时才拆解。`
}

func buildSmartDecomposePrompt(useShortDramaAnalyzer bool) string {
	testerRole, _ := getTestcaseReviewRole("tester")
	basePrompt := `你是一名资深的业务分析师和需求工程师，擅长发现隐含需求和边界条件。
用户提供了一段原始需求文档，以及团队已有的初步拆解结果。

你的任务是进行第二轮深度拆解，但前提仍然是：只围绕项目功能层面的需求进行补强，不要把非项目功能内容扩写成测试点。

你还必须同时代入以下测试角色身份，用测试视角约束补充方向和拆分粒度：
` + testerRole.IdentityMD + `

重点关注：
1. 发现初步拆解中遗漏的隐含需求、边界条件、异常场景。
2. 将粒度过大的需求点拆分为更细的子功能点。
3. 默认仍聚焦 App 功能测试；只有原始需求明确提到，或该点属于登录、支付、订单、账号、权限等高风险核心链路时，才补充安全性、性能、兼容性等非功能性需求。
4. 每个需求点应足够细粒度，但要避免把同一功能拆成大量同义或近义点。
5. 不要从运营目标、商业指标、泛化体验描述里臆造出功能需求点。
6. 如果已有拆解已经足够稳定和完整，不要为了“看起来更丰富”而强行新增 3-5 个低价值点。
7. 只有在确实存在独立状态机、独立规则集合、独立失败路径时，才新增需求点。

输出格式：必须严格输出一个 JSON 数组。
- 不要包含任何解释性文字。
- 不要在 JSON 结尾添加句号等标点。
- 每个对象包含：module, feature, description, rules。
- feature 必须足够具体，不要泛泛而谈。`

	if !useShortDramaAnalyzer {
		return basePrompt
	}

	return basePrompt + "\n\n" + shortDramaAnalyzerSummary + `

额外要求（仅适用于短剧类 App）：
1. 请用短剧产品结构校正已有拆解结果，优先检查：首页推荐流、For You 预览、全集入口、收藏/分享、选集播放、任务赚金币、会员/金币解锁、剧架/历史、设置开关等链路是否漏拆。
2. 只补充和短剧产品功能直接相关的需求点；不要把运营活动描述、投放策略、增长目标本身补成测试用例。`
}

func buildGeneratePrompt(useShortDramaAnalyzer bool) string {
	basePrompt := `你是一名资深的软件测试专家。
用户提供了一组结构化的需求点。
你的任务是根据这些需求点，生成更聚焦于 App 功能测试、可直接执行的测试用例。

核心规则（必须严格遵守）：
1. 生成目标：优先覆盖用户主流程、核心业务规则、关键状态切换和直接影响体验的功能点。
2. 不要机械地为每个功能点都生成四维用例。请按风险选择最有价值的维度：
   - 默认至少生成 1 条“常规功能测试”。
   - 只有存在明确输入校验、边界限制时，才增加“边界极限测试”。
   - 只有存在失败恢复、异常返回、网络波动、权限拦截等场景时，才增加“异常容错测试”。
   - 只有功能涉及重复点击、并发提交、状态竞争、库存/订单/支付争抢等风险时，才增加“稳定性并发测试”。
3. 单个需求点通常生成 2-4 条高价值用例；只有高风险复杂功能才允许到 5 条。严禁堆砌同义、近义、换壳型用例。
4. 默认聚焦 App 本身功能测试。除非需求明确提到，否则不要生成浏览器兼容性、无障碍、老旧设备、UI 像素级样式、纯安全渗透、纯性能压测类用例。
5. 用例结构：包含 id, category, type, title, precondition, steps, test_data, expected_result, priority, remark。
   - steps：描述具体操作步骤的字符串数组。
   - test_data：测试所需的数据。可以是字符串，也可以是描述数据的 JSON 对象（如果是对象，请确保其结构清晰）。
   - category 只能使用：常规功能测试、边界极限测试、异常容错测试、稳定性并发测试。
6. type 只能使用：POSITIVE、NEGATIVE、EXCEPTION、CONCURRENCY。
7. ID 命名规范：TC-{MODULE_NAME}-{TYPE_CHAR}-{SEQ}
8. 优先级：P0, P1, P2, P3。
9. 每条用例应尽量短而准：
   - 标题避免长句堆砌。
   - steps 一般 3-5 步即可。
   - expected_result 只写关键断言，不要把步骤再复述一遍。
10. 去冗余要求：
   - 不要输出仅改动措辞、但验证目标相同的重复用例。
   - 不要把“空值、非法字符、超长”分别拆成多个低价值重复用例，除非它们对应不同业务结果。
   - 如果多个校验本质一致，请合并成 1 条代表性用例。

输出格式：必须严格输出一个 JSON 数组。
- 【严禁指令】禁止在 JSON 中使用任何编程代码、方法或变量（如 .repeat(), .slice(), str.charAt 等）。
- 【数据规范】所有字段值必须是最终的字面量字符串、数字、布尔值或 null。
- 不要输出任何解释性文本。
- 【重要】不要在 JSON 结尾添加多余的标点符号或句号。
- 确保返回的是纯净的、可以直接被 json.Unmarshal 解析的 JSON 数组。`

	if !useShortDramaAnalyzer {
		return basePrompt
	}

	return basePrompt + "\n\n" + shortDramaAnalyzerSummary + `

额外要求（仅适用于短剧类 App）：
1. 生成用例时，优先参考短剧产品的真实功能链路：首页推荐流、For You 预览、全集入口、选集播放、自动播放、收藏/分享、任务赚金币、会员/金币解锁、剧架/历史、设置开关等。
2. 用例要围绕短剧产品常见状态变化来设计，如试看/解锁、广告解锁、金币扣减、会员态、继续观看、历史同步、自动播放开关等。
3. 不要把投放、买量、运营活动、增长策略本身生成成测试用例；只有能落到具体产品功能改动时才生成。`
}

func getTestcaseReviewRole(roleKey string) (models.TestCaseReviewRole, bool) {
	for _, role := range testcaseReviewRoles {
		if role.Key == roleKey {
			return role, true
		}
	}
	return models.TestCaseReviewRole{}, false
}

func buildReviewPrompt(role models.TestCaseReviewRole, useShortDramaAnalyzer bool) string {
	basePrompt := `你正在扮演指定评审角色，对一组已经生成完成的测试用例进行专业评审。

请严格遵守以下原则：
1. 你必须充分代入“身份说明”里的角色视角，只从该角色关注的重点出发评审。
2. 你的任务是评审测试用例，而不是重新生成完整用例列表。
3. 你需要判断当前测试用例覆盖得好的地方、遗漏点、重复/低价值点，以及建议新增的测试方向。
4. 如果某些用例已经足够好，也要明确指出，不要为了挑问题而挑问题。
5. 结论必须具体，不要泛泛地说“建议补充边界情况”而不给出方向。

输出格式要求：
- 必须严格输出一个 JSON 对象，不要输出 Markdown，不要输出解释性文字。
- 字段必须包含：
  - overall_conclusion: string
  - highlights: string[]
  - missing_coverage: string[]
  - suggested_new_cases: string[]
  - suggested_merge_or_drop: string[]
  - risk_level: string
  - review_focus: string[]
- risk_level 只能是：低 / 中 / 高 / 极高
- 数组字段即使没有内容也必须返回空数组 []`

	if !useShortDramaAnalyzer {
		return basePrompt + "\n\n身份说明：\n" + role.IdentityMD
	}

	return basePrompt + "\n\n身份说明：\n" + role.IdentityMD + "\n\n" + shortDramaAnalyzerSummary + `

额外要求（仅适用于短剧类 App）：
1. 评审时优先关注：首页推荐流、For You 预览、全集入口、选集播放、自动播放、收藏/分享、任务赚金币、会员/金币解锁、剧架/历史、设置开关等短剧核心链路。
2. 如果当前测试用例没有覆盖试看/解锁、广告解锁、金币扣减、会员态、继续观看、历史同步、自动播放开关等关键状态，请明确指出。`
}

func buildReviewUserContent(text string, points []models.RequirementPoint, cases []models.TestCase) string {
	pointsJSON, _ := json.Marshal(points)
	casesJSON, _ := json.Marshal(cases)
	return fmt.Sprintf("需求原文：\n%s\n\n结构化需求点：\n%s\n\n待评审测试用例：\n%s", text, string(pointsJSON), string(casesJSON))
}

func buildOptimizePrompt(useShortDramaAnalyzer bool) string {
	basePrompt := `你是一名资深测试设计专家，当前任务不是从零生成用例，而是基于多角色评审意见对现有测试用例进行智能优化。

优化目标：
1. 保留当前已有的高价值用例。
2. 根据评审意见补足缺失的测试点。
3. 删除或合并明显重复、低价值、粒度失衡的用例。
4. 输出一版更完整、更聚焦、可执行性更强的最终测试用例集。

必须遵守：
1. 你会收到：需求原文、结构化需求点、当前测试用例、多个角色的评审意见。
2. 只能围绕需求和评审意见优化，不要凭空扩展到无关领域。
3. 输出结果必须仍然是测试用例 JSON 数组，而不是评审总结。
4. 尽量保留原有用例中已经合理的 source_module/source_feature 映射；新增用例也必须补齐 source_module/source_feature。
5. 用例结构必须包含：id, source_module, source_feature, category, type, title, precondition, steps, test_data, expected_result, priority, remark。
6. category 只能使用：常规功能测试、边界极限测试、异常容错测试、稳定性并发测试。
7. type 只能使用：POSITIVE、NEGATIVE、EXCEPTION、CONCURRENCY。
8. 优先级只能使用：P0、P1、P2、P3。
9. 不要输出解释性文本，不要输出 Markdown，只能输出纯 JSON 数组。
10. 所有字段必须是最终字面量，不允许出现代码、表达式或伪变量。`

	if !useShortDramaAnalyzer {
		return basePrompt
	}

	return basePrompt + "\n\n" + shortDramaAnalyzerSummary + `

额外要求（仅适用于短剧类 App）：
1. 优先补足短剧关键链路：首页推荐流、For You 预览、全集入口、选集播放、自动播放、收藏/分享、任务赚金币、会员/金币解锁、剧架/历史、设置开关等。
2. 重点检查试看/解锁、广告解锁、金币扣减、会员态、继续观看、历史同步、自动播放开关等状态是否有遗漏。
3. 不要把投放、买量、运营活动、增长策略本身优化成测试用例。`
}

func buildOptimizeUserContent(text string, points []models.RequirementPoint, cases []models.TestCase, reviewResults map[string]models.TestCaseReviewResult) string {
	pointsJSON, _ := json.Marshal(points)
	casesJSON, _ := json.Marshal(cases)
	reviewsJSON, _ := json.Marshal(reviewResults)
	return fmt.Sprintf("需求原文：\n%s\n\n结构化需求点：\n%s\n\n当前测试用例：\n%s\n\n多角色评审意见：\n%s", text, string(pointsJSON), string(casesJSON), string(reviewsJSON))
}

func inferTestCaseCategory(tc models.TestCase) string {
	text := tc.Title + " " + tc.Precondition + " " + tc.ExpectedResult

	switch strings.ToUpper(strings.TrimSpace(tc.Type)) {
	case "CONCURRENCY":
		return "稳定性并发测试"
	case "EXCEPTION":
		return "异常容错测试"
	}

	for _, keyword := range testcaseBoundaryKeywords {
		if strings.Contains(text, keyword) {
			return "边界极限测试"
		}
	}

	return "常规功能测试"
}

func finalizeGeneratedCases(cases []models.TestCase) []models.TestCase {
	seen := make(map[string]bool)
	var unique []models.TestCase

	for _, tc := range cases {
		tc.ID = strings.TrimSpace(tc.ID)
		tc.Type = strings.ToUpper(strings.TrimSpace(tc.Type))
		tc.Title = strings.TrimSpace(tc.Title)
		tc.Precondition = strings.TrimSpace(tc.Precondition)
		tc.ExpectedResult = strings.TrimSpace(tc.ExpectedResult)
		tc.Priority = strings.ToUpper(strings.TrimSpace(tc.Priority))
		tc.Remark = strings.TrimSpace(tc.Remark)

		if tc.Category == "" {
			tc.Category = inferTestCaseCategory(tc)
		}

		cleanSteps := make([]string, 0, len(tc.Steps))
		for _, step := range tc.Steps {
			step = strings.TrimSpace(step)
			if step != "" {
				cleanSteps = append(cleanSteps, step)
			}
		}
		tc.Steps = cleanSteps

		key := strings.Join([]string{
			tc.Category,
			tc.Type,
			normalizeTestCaseText(tc.Title),
			normalizeTestCaseText(strings.Join(tc.Steps, "|")),
			normalizeTestCaseText(tc.ExpectedResult),
		}, "::")
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, tc)
	}

	return deduplicateAndReindex(unique)
}

// DecomposeRequirementHandler 需求拆解处理器
func DecomposeRequirementHandler(c *gin.Context) {
	var req DecomposeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式不正确"})
		return
	}

	req.Text = strings.TrimSpace(req.Text)
	req.WikiURL = strings.TrimSpace(req.WikiURL)
	if req.Text == "" && req.WikiURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供需求描述文本或飞书需求链接"})
		return
	}

	sourceText := req.Text
	if req.WikiURL != "" {
		docContent, err := loadFeishuRequirementContent(req.WikiURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("读取飞书需求文档失败: %v", err)})
			return
		}

		if req.Text != "" {
			sourceText = docContent + "\n\n补充说明：\n" + req.Text
		} else {
			sourceText = docContent
		}
	}

	useShortDramaAnalyzer := isShortDramaAppRequirement(sourceText)
	systemPrompt := buildRequirementDecomposePrompt(useShortDramaAnalyzer)

	aiResult, err := callTestcaseAI(c.Request.Context(), testcaseAICapabilityDecompose, systemPrompt, sourceText)
	if err != nil {
		log.Printf("[ERROR] AI 需求拆解失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI 拆解失败: %v", err)})
		return
	}

	var points []models.RequirementPoint
	if err := parseAIJSON(aiResult.Content, &points); err != nil {
		log.Printf("[ERROR] 解析 AI 拆解结果失败: %v, 原文: %s", err, aiResult.Content)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析 AI 响应失败", "raw": aiResult})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"points":        points,
		"source_text":   sourceText,
		"source_link":   req.WikiURL,
		"used_docx_ai":  req.WikiURL != "",
		"ai_provider":   aiResult.Provider,
		"ai_model":      aiResult.Model,
		"analyzer_used": useShortDramaAnalyzer,
	})
}

// SmartDecomposeHandler AI 智能增强拆解：基于原始文本进行第二轮更细粒度的需求拆解
// 返回增强后的需求点列表、与原始列表的相似度、以及合并后的结果
func SmartDecomposeHandler(c *gin.Context) {
	var req SmartDecomposeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供需求描述文本"})
		return
	}

	// 将现有拆解结果序列化，给 AI 作为参考
	existingJSON, _ := json.Marshal(req.ExistingPoints)

	useShortDramaAnalyzer := resolveShortDramaAnalyzerUsage(req.Text, req.PreviousAnalyzerUsed)
	systemPrompt := buildSmartDecomposePrompt(useShortDramaAnalyzer)

	userContent := fmt.Sprintf("原始需求文档：\n%s\n\n团队初步拆解结果（仅供参考，请独立分析）：\n%s", req.Text, string(existingJSON))

	aiResult, err := callTestcaseAI(c.Request.Context(), testcaseAICapabilitySmartDecompose, systemPrompt, userContent)
	if err != nil {
		log.Printf("[ERROR] AI 智能拆解失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI 智能拆解失败: %v", err)})
		return
	}

	var enhancedPoints []models.RequirementPoint
	if err := parseAIJSON(aiResult.Content, &enhancedPoints); err != nil {
		log.Printf("[ERROR] 解析 AI 智能拆解结果失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析 AI 响应失败", "raw": aiResult})
		return
	}

	// 计算相似度（基于 feature 标题的 Jaccard 相似系数）
	existingFeatures := make(map[string]bool)
	for _, p := range req.ExistingPoints {
		existingFeatures[p.Feature] = true
	}
	enhancedFeatures := make(map[string]bool)
	for _, p := range enhancedPoints {
		enhancedFeatures[p.Feature] = true
	}

	// 交集数量
	intersection := 0
	for f := range existingFeatures {
		if enhancedFeatures[f] {
			intersection++
		}
	}
	// 并集数量
	unionSet := make(map[string]bool)
	for f := range existingFeatures {
		unionSet[f] = true
	}
	for f := range enhancedFeatures {
		unionSet[f] = true
	}
	similarity := 0.0
	if len(unionSet) > 0 {
		similarity = float64(intersection) / float64(len(unionSet)) * 100
	}

	// 合并去重（基于 module+feature 唯一键）
	mergedMap := make(map[string]struct {
		point models.RequirementPoint
		isNew bool
	})

	// 记录初始点
	for _, p := range req.ExistingPoints {
		key := p.Module + "::" + p.Feature
		mergedMap[key] = struct {
			point models.RequirementPoint
			isNew bool
		}{point: p, isNew: false}
	}

	// 合并增强点
	for _, p := range enhancedPoints {
		key := p.Module + "::" + p.Feature
		if existing, ok := mergedMap[key]; ok {
			// 已存在，合并 rules
			rulesSet := make(map[string]bool)
			for _, r := range existing.point.Rules {
				rulesSet[r] = true
			}
			for _, r := range p.Rules {
				if !rulesSet[r] {
					existing.point.Rules = append(existing.point.Rules, r)
				}
			}
			// 补充更长的 description
			if len(p.Description) > len(existing.point.Description) {
				existing.point.Description = p.Description
			}
			mergedMap[key] = existing
		} else {
			// 新增点
			mergedMap[key] = struct {
				point models.RequirementPoint
				isNew bool
			}{point: p, isNew: true}
		}
	}

	var mergedPoints []gin.H
	newPointsCount := 0
	for _, v := range mergedMap {
		if v.isNew {
			newPointsCount++
		}
		mergedPoints = append(mergedPoints, gin.H{
			"module":      v.point.Module,
			"feature":     v.point.Feature,
			"description": v.point.Description,
			"rules":       v.point.Rules,
			"is_new":      v.isNew,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"enhanced_points": enhancedPoints,
		"merged_points":   mergedPoints,
		"similarity":      fmt.Sprintf("%.1f", similarity),
		"analyzer_used":   useShortDramaAnalyzer,
		"stats": gin.H{
			"original_count": len(req.ExistingPoints),
			"enhanced_count": len(enhancedPoints),
			"merged_count":   len(mergedPoints),
			"new_points":     newPointsCount,
		},
	})
}

// GenerateTestCasesHandler 测试用例生成处理器 (分批版)
func GenerateTestCasesHandler(c *gin.Context) {
	var req GenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需求点列表不能为空"})
		return
	}

	useShortDramaAnalyzer := resolveShortDramaAnalyzerUsage(req.Text, req.AnalyzerUsed)

	// 每批仅处理 1 个需求点，避免生成内容过长导致 Token 截断
	batchSize := 1
	var allCases []models.TestCase
	totalPoints := len(req.Points)

	for i := 0; i < totalPoints; i += batchSize {
		end := i + batchSize
		if end > totalPoints {
			end = totalPoints
		}
		batch := req.Points[i:end]

		log.Printf("[INFO] 正在生成第 %d 批次用例 (需求点: %d/%d)", i/batchSize+1, end, totalPoints)

		cases, err := generateBatch(c.Request.Context(), batch, useShortDramaAnalyzer)
		if err != nil {
			log.Printf("[ERROR] 第 %d 批次生成失败: %v", i/batchSize+1, err)
			if len(allCases) > 0 {
				c.JSON(http.StatusOK, gin.H{
					"cases":         allCases,
					"error":         fmt.Sprintf("第 %d 批次生成失败: %v", i/batchSize+1, err),
					"partial":       true,
					"analyzer_used": useShortDramaAnalyzer,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI 生成失败: %v", err)})
			return
		}
		allCases = append(allCases, cases...)
	}

	finalCases := deduplicateAndReindex(allCases)

	c.JSON(http.StatusOK, gin.H{
		"cases":         finalCases,
		"batch_count":   (totalPoints + batchSize - 1) / batchSize,
		"analyzer_used": useShortDramaAnalyzer,
	})
}

func GetTestcaseReviewRolesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"roles": testcaseReviewRoles,
	})
}

func ReviewTestCasesHandler(c *gin.Context) {
	var req ReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评审参数不完整"})
		return
	}

	role, ok := getTestcaseReviewRole(req.RoleKey)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未知评审角色"})
		return
	}

	useShortDramaAnalyzer := resolveShortDramaAnalyzerUsage(req.Text, req.AnalyzerUsed)
	systemPrompt := buildReviewPrompt(role, useShortDramaAnalyzer)
	userContent := buildReviewUserContent(req.Text, req.Points, req.Cases)

	aiResult, err := callTestcaseAI(c.Request.Context(), testcaseAICapabilityReview, systemPrompt, userContent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI 评审失败: %v", err)})
		return
	}

	var parsed struct {
		OverallConclusion    string   `json:"overall_conclusion"`
		Highlights           []string `json:"highlights"`
		MissingCoverage      []string `json:"missing_coverage"`
		SuggestedNewCases    []string `json:"suggested_new_cases"`
		SuggestedMergeOrDrop []string `json:"suggested_merge_or_drop"`
		RiskLevel            string   `json:"risk_level"`
		ReviewFocus          []string `json:"review_focus"`
	}
	if err := parseAIJSON(aiResult.Content, &parsed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析 AI 评审结果失败", "raw": aiResult})
		return
	}

	result := models.TestCaseReviewResult{
		RoleKey:              role.Key,
		RoleName:             role.Name,
		IdentityMD:           role.IdentityMD,
		OverallConclusion:    strings.TrimSpace(parsed.OverallConclusion),
		Highlights:           parsed.Highlights,
		MissingCoverage:      parsed.MissingCoverage,
		SuggestedNewCases:    parsed.SuggestedNewCases,
		SuggestedMergeOrDrop: parsed.SuggestedMergeOrDrop,
		RiskLevel:            strings.TrimSpace(parsed.RiskLevel),
		ReviewFocus:          parsed.ReviewFocus,
		ReviewedAt:           time.Now().Format(time.RFC3339),
		AIProvider:           aiResult.Provider,
		AIModel:              aiResult.Model,
	}

	c.JSON(http.StatusOK, gin.H{
		"role":          role,
		"result":        result,
		"analyzer_used": useShortDramaAnalyzer,
	})
}

func OptimizeReviewedCasesHandler(c *gin.Context) {
	var req OptimizeCasesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "优化参数不完整"})
		return
	}
	if len(req.ReviewResults) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要一份评审结果"})
		return
	}

	useShortDramaAnalyzer := resolveShortDramaAnalyzerUsage(req.Text, req.AnalyzerUsed)
	systemPrompt := buildOptimizePrompt(useShortDramaAnalyzer)
	userContent := buildOptimizeUserContent(req.Text, req.Points, req.Cases, req.ReviewResults)

	aiResult, err := callTestcaseAI(c.Request.Context(), testcaseAICapabilityOptimize, systemPrompt, userContent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI 优化失败: %v", err)})
		return
	}

	var optimizedCases []models.TestCase
	if err := parseAIJSON(aiResult.Content, &optimizedCases); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析 AI 优化结果失败", "raw": aiResult})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cases":         finalizeGeneratedCases(optimizedCases),
		"ai_provider":   aiResult.Provider,
		"ai_model":      aiResult.Model,
		"analyzer_used": useShortDramaAnalyzer,
	})
}

// ListRecordsHandler 获取历史记录列表
func ListRecordsHandler(c *gin.Context) {
	historyLock.RLock()
	defer historyLock.RUnlock()

	recordsData, err := sqlListJSON[models.GenerationRecord]("testcase_history", "`migrated_at` ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取历史记录"})
		return
	}

	var records []gin.H
	for _, record := range recordsData {
		records = append(records, gin.H{
			"id":               record.ID,
			"title":            record.Title,
			"project_code":     record.ProjectCode,
			"module":           record.Module,
			"requirement_text": record.RequirementText,
			"created_at":       record.CreatedAt,
			"case_count":       len(record.Cases),
		})
	}

	// 按时间倒序排列
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// GetRecordHandler 获取单条历史记录详情
func GetRecordHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供记录 ID"})
		return
	}

	historyLock.RLock()
	defer historyLock.RUnlock()

	records, err := sqlListJSON[models.GenerationRecord]("testcase_history", "`migrated_at` ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取历史记录"})
		return
	}
	for _, record := range records {
		if record.ID == id {
			c.JSON(http.StatusOK, record)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "历史记录不存在"})
}

// DownloadRecordHandler 导出整条历史记录为 Excel 下载文件
func DownloadRecordHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供记录 ID"})
		return
	}

	historyLock.RLock()
	defer historyLock.RUnlock()

	records, err := sqlListJSON[models.GenerationRecord]("testcase_history", "`migrated_at` ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取历史记录"})
		return
	}

	var targetRecord models.GenerationRecord
	found := false
	for _, record := range records {
		if record.ID == id {
			targetRecord = record
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "找不到该记录"})
		return
	}

	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
	}()
	sheet := "测试用例"
	index, _ := f.NewSheet(sheet)
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	headers := []string{"用例ID", "分类", "类型", "用例标题", "前置条件", "测试步骤", "测试数据", "预期结果", "优先级", "备注"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for i, tc := range targetRecord.Cases {
		rowIdx := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", rowIdx), tc.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", rowIdx), tc.Type)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", rowIdx), tc.Title)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", rowIdx), tc.Precondition)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", rowIdx), strings.Join(tc.Steps, "\n"))

		testDataStr := ""
		if tc.TestData != nil {
			switch v := tc.TestData.(type) {
			case string:
				testDataStr = v
			default:
				data, _ := json.MarshalIndent(v, "", "  ")
				testDataStr = string(data)
			}
		}
		f.SetCellValue(sheet, fmt.Sprintf("F%d", rowIdx), testDataStr)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", rowIdx), tc.ExpectedResult)
		f.SetCellValue(sheet, fmt.Sprintf("H%d", rowIdx), tc.Priority)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", rowIdx), tc.Remark)
	}

	fileName := fmt.Sprintf("TestCases_%s.xlsx", targetRecord.ID)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件流写入失败"})
	}
}

// SaveRecordHandler 保存生成结果到历史记录
func SaveRecordHandler(c *gin.Context) {
	var record models.GenerationRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记录数据"})
		return
	}

	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.CreatedAt == "" {
		record.CreatedAt = time.Now().Format(time.RFC3339)
	}

	historyLock.Lock()
	defer historyLock.Unlock()

	if err := sqlUpsertJSON("testcase_history", record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法保存历史记录"})
		return
	}

	c.JSON(http.StatusOK, record)
}

// UpdateRecordHandler 更新已存在的历史记录
func UpdateRecordHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供记录 ID"})
		return
	}

	var updatedRecord models.GenerationRecord
	if err := c.ShouldBindJSON(&updatedRecord); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记录数据"})
		return
	}
	updatedRecord.ID = id // 确保 ID 保持不变

	historyLock.Lock()
	defer historyLock.Unlock()

	records, err := sqlListJSON[models.GenerationRecord]("testcase_history", "`migrated_at` ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取历史记录"})
		return
	}

	found := false
	for i := range records {
		if records[i].ID == id {
			updatedRecord.CreatedAt = records[i].CreatedAt
			records[i] = updatedRecord
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}

	if err := sqlReplaceAllJSON("testcase_history", records); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法更新历史记录"})
		return
	}

	c.JSON(http.StatusOK, updatedRecord)
}

// DeleteRecordHandler 删除历史记录
func DeleteRecordHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供记录 ID"})
		return
	}

	historyLock.Lock()
	defer historyLock.Unlock()

	records, err := sqlListJSON[models.GenerationRecord]("testcase_history", "`migrated_at` ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取历史记录"})
		return
	}

	filtered := make([]models.GenerationRecord, 0, len(records))
	found := false
	for _, record := range records {
		if record.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, record)
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}

	if err := sqlReplaceAllJSON("testcase_history", filtered); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法更新历史记录文件"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已成功删除记录"})
}

// generateBatch 生成单个批次的用例
func generateBatch(ctx context.Context, points []models.RequirementPoint, useShortDramaAnalyzer bool) ([]models.TestCase, error) {
	pointsJSON, _ := json.Marshal(points)
	systemPrompt := buildGeneratePrompt(useShortDramaAnalyzer)

	aiResult, err := callTestcaseAI(ctx, testcaseAICapabilityGenerate, systemPrompt, string(pointsJSON))
	if err != nil {
		return nil, err
	}

	var cases []models.TestCase
	if err := parseAIJSON(aiResult.Content, &cases); err != nil {
		// 解析失败时记录完整的 AI 原始响应以便调试
		log.Printf("[TestCaseGen] AI 原始响应 (前500字符): %.500s", aiResult.Content)
		return nil, err
	}
	return finalizeGeneratedCases(cases), nil
}

// deduplicateAndReindex 去重并重新分配 ID 序号
func deduplicateAndReindex(cases []models.TestCase) []models.TestCase {
	seen := make(map[string]bool)
	var unique []models.TestCase

	for _, tc := range cases {
		if tc.Category == "" {
			tc.Category = inferTestCaseCategory(tc)
		}

		// 基于分类 + 标题 + 预期结果进行去重，避免同义重复
		k := strings.Join([]string{
			tc.Category,
			normalizeTestCaseText(tc.Title),
			normalizeTestCaseText(tc.ExpectedResult),
		}, "::")
		if !seen[k] {
			seen[k] = true
			unique = append(unique, tc)
		}
	}

	counters := make(map[string]int)
	for i := range unique {
		parts := strings.Split(unique[i].ID, "-")
		if len(parts) >= 3 {
			module := parts[1]
			typeChar := parts[2]
			key := module + "-" + typeChar
			counters[key]++
			unique[i].ID = fmt.Sprintf("TC-%s-%s-%03d", module, typeChar, counters[key])
		}
	}

	return unique
}

// ExportTestCasesExcelHandler 导出测试用例为 Excel
func ExportTestCasesExcelHandler(c *gin.Context) {
	var req ExportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "待导出的测试用例列表不能为空"})
		return
	}

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	sheet := "测试用例"
	index, _ := f.NewSheet(sheet)
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// 表头
	headers := []string{"用例ID", "类型", "用例标题", "前置条件", "测试步骤", "测试数据", "预期结果", "优先级", "备注"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// 设置表头样式
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})
	f.SetRowStyle(sheet, 1, 1, headerStyle)

	// 填充数据
	for i, tc := range req.Cases {
		rowIdx := i + 2
		if tc.Category == "" {
			tc.Category = inferTestCaseCategory(tc)
		}
		f.SetCellValue(sheet, fmt.Sprintf("A%d", rowIdx), tc.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", rowIdx), tc.Category)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", rowIdx), tc.Type)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", rowIdx), tc.Title)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", rowIdx), tc.Precondition)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", rowIdx), strings.Join(tc.Steps, "\n"))

		// Stringify TestData if it's not already a string
		testDataStr := ""
		if tc.TestData != nil {
			switch v := tc.TestData.(type) {
			case string:
				testDataStr = v
			default:
				data, _ := json.MarshalIndent(v, "", "  ")
				testDataStr = string(data)
			}
		}
		f.SetCellValue(sheet, fmt.Sprintf("G%d", rowIdx), testDataStr)

		f.SetCellValue(sheet, fmt.Sprintf("H%d", rowIdx), tc.ExpectedResult)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", rowIdx), tc.Priority)
		f.SetCellValue(sheet, fmt.Sprintf("J%d", rowIdx), tc.Remark)
	}

	// 自动换行及边框
	normalStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})
	f.SetCellStyle(sheet, "A2", fmt.Sprintf("J%d", len(req.Cases)+1), normalStyle)

	// 列宽
	f.SetColWidth(sheet, "A", "A", 20)
	f.SetColWidth(sheet, "B", "B", 12)
	f.SetColWidth(sheet, "C", "C", 12)
	f.SetColWidth(sheet, "D", "D", 30)
	f.SetColWidth(sheet, "E", "J", 35)

	// 冻结首行
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=Generated_Test_Cases.xlsx")
	c.Header("Content-Transfer-Encoding", "binary")

	if err := f.Write(c.Writer); err != nil {
		log.Printf("[ERROR] 写入 Excel 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出失败"})
	}
}

func resolveTestcaseProviders(config *model.AIConfig, capability string) []model.AIProviderConfig {
	if config == nil {
		return nil
	}

	providers := config.EffectiveProvidersByCapability(capability)
	if len(providers) > 0 {
		return providers
	}
	if capability == testcaseAICapabilityReview || capability == testcaseAICapabilityOptimize {
		providers = config.EffectiveProvidersByCapability(testcaseAICapabilityGenerate)
		if len(providers) > 0 {
			return providers
		}
	}
	return config.EffectiveProviders()
}

func GetTestcaseAIModelsHandler(c *gin.Context) {
	meta := gin.H{
		"decompose":       getTestcaseAIModelMeta(testcaseAICapabilityDecompose),
		"smart_decompose": getTestcaseAIModelMeta(testcaseAICapabilitySmartDecompose),
		"generate":        getTestcaseAIModelMeta(testcaseAICapabilityGenerate),
		"review":          getTestcaseAIModelMeta(testcaseAICapabilityReview),
		"optimize":        getTestcaseAIModelMeta(testcaseAICapabilityOptimize),
	}
	c.JSON(http.StatusOK, meta)
}

func getTestcaseAIModelMeta(capability string) testcaseAIModelMeta {
	config := model.GlobalAIConfig
	if config == nil {
		config = model.LoadAIConfig()
	}
	providers := resolveTestcaseProviders(config, capability)
	if len(providers) == 0 {
		return testcaseAIModelMeta{}
	}
	return testcaseAIModelMeta{
		Provider: providers[0].Name,
		Model:    providers[0].Model,
	}
}

func callTestcaseAI(ctx context.Context, capability string, systemPrompt, userPrompt string) (testcaseAIExecution, error) {
	config := model.GlobalAIConfig
	if config == nil {
		config = model.LoadAIConfig()
	}

	providers := resolveTestcaseProviders(config, capability)
	if config == nil || len(providers) == 0 {
		return testcaseAIExecution{}, fmt.Errorf("未配置 AI 接口 Key")
	}

	// 显式配置 Transport 以支持环境变量代理和更长的连接生命周期
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   180 * time.Second,
	}

	temperature := float32(0.3)
	switch capability {
	case testcaseAICapabilityDecompose, testcaseAICapabilitySmartDecompose:
		temperature = 0.05
	case testcaseAICapabilityReview, testcaseAICapabilityOptimize:
		temperature = 0.2
	}

	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: temperature,
		MaxTokens:   8192, // DeepSeek API 允许的最大值
	}

	var resp openai.ChatCompletionResponse
	var err error
	var lastErr error
	maxRetries := 3
	var usedProvider model.AIProviderConfig

	for _, provider := range providers {
		clientConfig := openai.DefaultConfig(provider.APIKey)
		clientConfig.BaseURL = provider.BaseURL
		clientConfig.HTTPClient = httpClient
		client := openai.NewClientWithConfig(clientConfig)
		req.Model = provider.Model

		// 重试逻辑：针对网络抖动导致的 EOF、超时或连接重置进行重试
		for i := 0; i < maxRetries; i++ {
			resp, err = client.CreateChatCompletion(ctx, req)
			if err == nil {
				usedProvider = provider
				break
			}
			lastErr = err
			log.Printf("[TestCaseGen] %s (%s) AI API 异常 (第 %d 次尝试): %v", provider.Name, capability, i+1, err)

			// 检查是否为可重试错误
			errMsg := strings.ToLower(err.Error())
			isRetryable := strings.Contains(errMsg, "eof") ||
				strings.Contains(errMsg, "timeout") ||
				strings.Contains(errMsg, "connection reset") ||
				strings.Contains(errMsg, "broken pipe")

			if !isRetryable || i == maxRetries-1 {
				break
			}
			// 指数退避等待
			time.Sleep(time.Duration(i+1) * time.Second)
		}

		if err == nil {
			break
		}
		log.Printf("[TestCaseGen] %s (%s) provider failed, trying next AI provider if available.", provider.Name, capability)
	}

	if err != nil {
		if lastErr != nil {
			return testcaseAIExecution{}, fmt.Errorf("AI 生成请求最终失败: %v", lastErr)
		}
		return testcaseAIExecution{}, fmt.Errorf("AI 生成请求最终失败: %v", err)
	}

	if len(resp.Choices) == 0 {
		return testcaseAIExecution{}, fmt.Errorf("AI 未返回任何内容")
	}

	return testcaseAIExecution{
		Content:  resp.Choices[0].Message.Content,
		Provider: usedProvider.Name,
		Model:    usedProvider.Model,
	}, nil
}

// repairTruncatedJSON 修复因 Token 限制导致的 JSON 截断问题
// 如果 JSON 数组/对象没有正确闭合，尝试在最后一个完整元素处截断并补全括号
func repairTruncatedJSON(jsonStr string) string {
	jsonStr = strings.TrimSpace(jsonStr)
	if len(jsonStr) == 0 {
		return jsonStr
	}

	// 检查括号是否平衡
	openBrackets := 0
	openBraces := 0
	inString := false
	escaped := false

	for _, ch := range jsonStr {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '[':
			openBrackets++
		case ']':
			openBrackets--
		case '{':
			openBraces++
		case '}':
			openBraces--
		}
	}

	// 如果括号已平衡，不需要修复
	if openBrackets == 0 && openBraces == 0 {
		return jsonStr
	}

	log.Printf("[TestCaseGen] 检测到 JSON 截断: 未闭合的 [ 数量=%d, 未闭合的 { 数量=%d，正在尝试修复...", openBrackets, openBraces)

	// 策略：找到最后一个完整的 JSON 对象（即最后一个 }），在其后截断
	// 然后补全所需的闭合括号
	lastCompleteObj := strings.LastIndex(jsonStr, "}")
	if lastCompleteObj > 0 {
		// 从最后一个 } 处截断，然后重新计算括号
		truncated := jsonStr[:lastCompleteObj+1]

		// 重新计算截断后的括号平衡
		ob, oc := 0, 0
		strMode := false
		esc := false
		for _, ch := range truncated {
			if esc {
				esc = false
				continue
			}
			if ch == '\\' && strMode {
				esc = true
				continue
			}
			if ch == '"' {
				strMode = !strMode
				continue
			}
			if strMode {
				continue
			}
			switch ch {
			case '[':
				ob++
			case ']':
				ob--
			case '{':
				oc++
			case '}':
				oc--
			}
		}

		// 补全闭合括号
		for oc > 0 {
			truncated += "}"
			oc--
		}
		for ob > 0 {
			truncated += "]"
			ob--
		}

		log.Printf("[TestCaseGen] JSON 截断修复完成，原长度=%d，修复后长度=%d", len(jsonStr), len(truncated))
		return truncated
	}

	return jsonStr
}

// repairAIJSON 修复 AI 在 JSON 内部插入的常见语法错误（第一轮批量修复）
func repairAIJSON(jsonStr string) string {
	// 1. 句号充当逗号的情况: "value". "nextKey" → "value", "nextKey"
	re1 := regexp.MustCompile(`"\s*[.。]+\s*"`)
	jsonStr = re1.ReplaceAllStringFunc(jsonStr, func(m string) string {
		// 判断是否是字段分隔符（句号在两个引号之间）
		// 保留 key 内部的合法句号（如 "v1.0"），只修复跨字段的
		return `", "`
	})

	// 2. 值后面的句号 + 逗号/括号: "value". , 或 "value".}
	re2 := regexp.MustCompile(`"\s*[.。]+\s*([,}\]\n])`)
	jsonStr = re2.ReplaceAllString(jsonStr, `"$1`)

	// 3. 结构关闭后的句号: }. 或 ].
	re3 := regexp.MustCompile(`([}\]])\s*[.。]+`)
	jsonStr = re3.ReplaceAllString(jsonStr, `$1`)

	// 4. 句号在逗号附近: "item"., "next" 或 ,. "next"
	re4 := regexp.MustCompile(`[.。]+\s*,`)
	jsonStr = re4.ReplaceAllString(jsonStr, `,`)
	re5 := regexp.MustCompile(`,\s*[.。]+\s*"`)
	jsonStr = re5.ReplaceAllString(jsonStr, `, "`)

	// 5. 数字后面的句号 + 逗号/括号: 123. , 或 123.}（非小数点）
	re6 := regexp.MustCompile(`(\d)\s*\.\s*([,}\]])`)
	jsonStr = re6.ReplaceAllString(jsonStr, `$1$2`)

	// 6. trailing comma: ,] 或 ,}
	re7 := regexp.MustCompile(`,\s*([}\]])`)
	jsonStr = re7.ReplaceAllString(jsonStr, `$1`)

	return jsonStr
}

// parseAIJSON 提取并解析 AI 返回的 JSON，具备多层防御和迭代自修复能力
func parseAIJSON(content string, v interface{}) error {
	trimmed := strings.TrimSpace(content)

	// 第一层：从 Markdown 代码块中提取 JSON 内容
	if strings.Contains(trimmed, "```") {
		parts := strings.Split(trimmed, "```")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			part = strings.TrimPrefix(part, "json")
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "[") || strings.HasPrefix(part, "{") {
				trimmed = part
				break
			}
		}
	}

	// 第二层：边界提取 — 定位最外层的 [ ] 或 { }
	startIndex := strings.IndexAny(trimmed, "[{")
	endIndex := strings.LastIndexAny(trimmed, "]}")
	if startIndex != -1 && endIndex != -1 && endIndex > startIndex {
		trimmed = trimmed[startIndex : endIndex+1]
	}

	// 第三层：正则批量修复（处理已知的常见模式）
	repaired := repairAIJSON(trimmed)

	// 第 3.5 层：截断修复 — 如果 JSON 不完整（缺少闭合括号），尝试补全
	repaired = repairTruncatedJSON(repaired)

	// 第四层：迭代自修复 — 如果解析失败，根据错误偏移量精确修复问题字符
	maxAttempts := 20
	lastOffset := -1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := json.Unmarshal([]byte(repaired), v)
		if err == nil {
			if attempt > 0 {
				log.Printf("[TestCaseGen] JSON 经过 %d 次自修复后解析成功", attempt)
			}
			return nil
		}

		// 提取错误偏移量
		offset := findJSONErrorOffset(err)

		// 如果偏移量没有变化，说明修复尝试无效，停止以防死循环
		if offset == lastOffset {
			logParseFailure(repaired, err, offset)
			return fmt.Errorf("JSON 自修复在偏移量 %d 处停滞: %v", offset, err)
		}
		lastOffset = offset

		if offset < 0 || offset >= len(repaired) {
			// 无法定位错误位置，放弃修复
			logParseFailure(repaired, err, offset)
			return err
		}

		// 根据错误位置上下文智能修复
		fixed, ok := fixAtOffset(repaired, offset)
		if !ok {
			// 无法修复，放弃
			logParseFailure(repaired, err, offset)
			return err
		}
		log.Printf("[TestCaseGen] 自修复第 %d 次: offset=%d, 修复前='%s', 修复后='%s'",
			attempt+1, offset, safeSubstr(repaired, offset-5, offset+5), safeSubstr(fixed, offset-5, offset+5))
		repaired = fixed
	}

	// 超过最大重试次数
	return fmt.Errorf("JSON 自修复超过 %d 次仍失败", maxAttempts)
}

// fixAtOffset 根据 JSON 解析错误的偏移量，智能修复问题字符
// 返回修复后的字符串和是否成功修复的标志
func fixAtOffset(jsonStr string, offset int) (string, bool) {
	if offset <= 0 || offset >= len(jsonStr) {
		return jsonStr, false
	}

	runes := []rune(jsonStr)
	// offset 是字节偏移，需要转换为字符位置
	bytePos := 0
	runeOffset := 0
	for i, r := range runes {
		if bytePos >= offset {
			runeOffset = i
			break
		}
		bytePos += len(string(r))
	}
	if bytePos < offset {
		runeOffset = len(runes)
	}

	if runeOffset >= len(runes) {
		return jsonStr, false
	}

	ch := runes[runeOffset]

	// 情况 1：问题字符是句号或中文句号 — 判断上下文决定是删除还是替换为逗号
	if ch == '.' || ch == '。' {
		// 向前找上一个非空白字符
		prevNonSpace := findPrevNonSpace(runes, runeOffset)
		// 向后找下一个非空白字符
		nextNonSpace := findNextNonSpace(runes, runeOffset)

		// 如果前面是 " 后面也是 "，说明句号在两个字段之间，替换为逗号
		if prevNonSpace >= 0 && nextNonSpace < len(runes) &&
			runes[prevNonSpace] == '"' && runes[nextNonSpace] == '"' {
			runes[runeOffset] = ','
			return string(runes), true
		}

		// 否则直接删除句号
		result := append(runes[:runeOffset], runes[runeOffset+1:]...)
		return string(result), true
	}

	// 情况 2：问题字符是其他非法字符（如 。；！等中文标点）— 直接删除
	if isIllegalJSONChar(ch) {
		result := append(runes[:runeOffset], runes[runeOffset+1:]...)
		return string(result), true
	}

	// 情况 3：问题字符本身是合法的，但位置不对
	// 比如缺少逗号导致 "value""key" 解析失败 — 在 offset 前插入逗号
	if ch == '"' {
		prevNonSpace := findPrevNonSpace(runes, runeOffset)
		if prevNonSpace >= 0 && (runes[prevNonSpace] == '"' || runes[prevNonSpace] == '}' || runes[prevNonSpace] == ']') {
			// 在 offset 处插入逗号
			newRunes := make([]rune, len(runes)+1)
			copy(newRunes, runes[:runeOffset])
			newRunes[runeOffset] = ','
			copy(newRunes[runeOffset+1:], runes[runeOffset:])
			return string(newRunes), true
		}
	}

	// 无法处理的情况 — 不再盲目删除字符，直接返回失败让调用者知晓
	return jsonStr, false
}

// findPrevNonSpace 从 pos 往前找第一个非空白字符
func findPrevNonSpace(runes []rune, pos int) int {
	for i := pos - 1; i >= 0; i-- {
		if runes[i] != ' ' && runes[i] != '\t' && runes[i] != '\n' && runes[i] != '\r' {
			return i
		}
	}
	return -1
}

// findNextNonSpace 从 pos 往后找第一个非空白字符
func findNextNonSpace(runes []rune, pos int) int {
	for i := pos + 1; i < len(runes); i++ {
		if runes[i] != ' ' && runes[i] != '\t' && runes[i] != '\n' && runes[i] != '\r' {
			return i
		}
	}
	return len(runes)
}

// isIllegalJSONChar 检查是否是 JSON 中不应出现的字符
func isIllegalJSONChar(ch rune) bool {
	illegals := []rune{'。', '；', '！', '？', '，', '：', '、', '…'}
	for _, c := range illegals {
		if ch == c {
			return true
		}
	}
	return false
}

// safeSubstr 安全截取子字符串，防止越界
func safeSubstr(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start >= end {
		return ""
	}
	return s[start:end]
}

// logParseFailure 记录 JSON 解析失败的详细信息
func logParseFailure(repaired string, err error, offset int) {
	if offset >= 0 && offset < len(repaired) {
		start := offset - 80
		if start < 0 {
			start = 0
		}
		end := offset + 80
		if end > len(repaired) {
			end = len(repaired)
		}
		log.Printf("[TestCaseGen] JSON 解析最终失败 (offset %d)。出错位置附近:\n...%s...\n错误: %v", offset, repaired[start:end], err)
	} else {
		sampleLen := 500
		if len(repaired) < sampleLen {
			sampleLen = len(repaired)
		}
		log.Printf("[TestCaseGen] JSON 解析最终失败。内容摘要:\n%s\n错误: %v", repaired[:sampleLen], err)
	}
}

// findJSONErrorOffset 从 json 解析错误中提取字符偏移量
func findJSONErrorOffset(err error) int {
	if synErr, ok := err.(*json.SyntaxError); ok {
		return int(synErr.Offset)
	}
	return -1
}

// --- Feishu Bot Core Export Functions ---

// DecomposeRequirementCore is the core logical function decoupled from gin.Context
func DecomposeRequirementCore(ctx context.Context, text string) ([]models.RequirementPoint, error) {
	systemPrompt := buildRequirementDecomposePrompt(resolveShortDramaAnalyzerUsage(text, false))

	aiResult, err := callTestcaseAI(ctx, testcaseAICapabilityDecompose, systemPrompt, text)
	if err != nil {
		return nil, fmt.Errorf("AI 拆解失败: %w", err)
	}

	var points []models.RequirementPoint
	if err := parseAIJSON(aiResult.Content, &points); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w\nRaw: %s", err, aiResult.Content)
	}
	return points, nil
}

// SmartDecomposeCore enhances the initial requirement points
func SmartDecomposeCore(ctx context.Context, text string, existingPoints []models.RequirementPoint) ([]models.RequirementPoint, error) {
	existingJSON, _ := json.Marshal(existingPoints)
	systemPrompt := buildSmartDecomposePrompt(resolveShortDramaAnalyzerUsage(text, false))

	userContent := fmt.Sprintf("原始需求文档：\n%s\n\n团队初步拆解结果（仅供参考，请独立分析）：\n%s", text, string(existingJSON))
	aiResult, err := callTestcaseAI(ctx, testcaseAICapabilitySmartDecompose, systemPrompt, userContent)
	if err != nil {
		return nil, fmt.Errorf("AI 智能拆解失败: %w", err)
	}

	var enhancedPoints []models.RequirementPoint
	if err := parseAIJSON(aiResult.Content, &enhancedPoints); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w\nRaw: %s", err, aiResult.Content)
	}

	// 合并去重逻辑
	mergedMap := make(map[string]models.RequirementPoint)
	for _, p := range existingPoints {
		key := p.Module + "::" + p.Feature
		mergedMap[key] = p
	}

	for _, p := range enhancedPoints {
		key := p.Module + "::" + p.Feature
		if existing, ok := mergedMap[key]; ok {
			rulesSet := make(map[string]bool)
			for _, r := range existing.Rules {
				rulesSet[r] = true
			}
			for _, r := range p.Rules {
				if !rulesSet[r] {
					existing.Rules = append(existing.Rules, r)
				}
			}
			if len(p.Description) > len(existing.Description) {
				existing.Description = p.Description
			}
			mergedMap[key] = existing
		} else {
			mergedMap[key] = p
		}
	}

	var finalPoints []models.RequirementPoint
	for _, v := range mergedMap {
		finalPoints = append(finalPoints, v)
	}
	return finalPoints, nil
}

// GenerateBatchCore exposes the batch generator
func GenerateBatchCore(ctx context.Context, points []models.RequirementPoint) ([]models.TestCase, error) {
	return generateBatch(ctx, points, false)
}

func loadFeishuRequirementContent(wikiURL string) (string, error) {
	if strings.TrimSpace(wikiURL) == "" {
		return "", fmt.Errorf("empty wiki url")
	}

	tenantToken, err := getFeishuTenantAccessToken()
	if err != nil {
		return "", err
	}

	docToken, docType, err := resolveFeishuReadableDocTarget(wikiURL, tenantToken)
	if err != nil {
		return "", err
	}

	var apiURL string
	switch docType {
	case "docx":
		apiURL = fmt.Sprintf("https://open.feishu.cn/open-apis/docx/v1/documents/%s/raw_content", docToken)
	case "doc":
		apiURL = fmt.Sprintf("https://open.feishu.cn/open-apis/doc/v2/%s/raw_content", docToken)
	default:
		return "", fmt.Errorf("暂不支持读取该类型飞书文档: %s", docType)
	}
	respBody, err := callFeishuAPI("GET", apiURL, tenantToken, nil)
	if err != nil {
		if strings.Contains(err.Error(), "status 403") || strings.Contains(err.Error(), "\"forBidden\"") {
			return "", fmt.Errorf("飞书应用暂无该文档的读取权限，请确认文档已授权给当前飞书应用/机器人，或将链接换成已授权的云文档")
		}
		return "", err
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("read docx failed: %s", result.Msg)
	}

	content := strings.TrimSpace(result.Data.Content)
	if content == "" {
		return "", fmt.Errorf("文档正文为空")
	}

	return content, nil
}

// SaveGenerationRecordCore persists the generation to history
func SaveGenerationRecordCore(record *models.GenerationRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.CreatedAt == "" {
		record.CreatedAt = time.Now().Format(time.RFC3339)
	}

	historyLock.Lock()
	defer historyLock.Unlock()

	return sqlUpsertJSON("testcase_history", record)
}
