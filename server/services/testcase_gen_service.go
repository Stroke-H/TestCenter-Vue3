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
	Text string `json:"text" binding:"required"`
}

// GenerateReq 接收需求点列表
type GenerateReq struct {
	Points []models.RequirementPoint `json:"points" binding:"required"`
}

// ExportReq 接收用例列表
type ExportReq struct {
	Cases []models.TestCase `json:"cases" binding:"required"`
}

var (
	historyLock sync.RWMutex
)

var testcaseBoundaryKeywords = []string{
	"边界", "极限", "最大", "最小", "超长", "过长", "上限", "下限", "空值", "为空",
	"null", "nil", "超出", "临界", "最短", "最长", "大数据量", "边缘",
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供需求描述文本"})
		return
	}

	systemPrompt := `你是一名资深的业务分析师和需求工程师。
用户的输入是一段原始的需求文档、功能描述或用户故事。
你的任务是将其拆解为结构化的需求点。

拆解原则：
1. 默认聚焦 App 本身的业务功能、页面流程、状态变化、核心规则和必要边界。
2. 只有当原始需求明确提到，或该功能属于高风险核心链路（如登录、支付、下单、账号、订阅、权限）时，才补充性能、安全、兼容性等非功能点。
3. 不要为了“覆盖更全”而机械拆出浏览器兼容、无障碍、老旧设备适配等泛化需求，除非原文明确要求。
4. feature 必须是适合落成功能测试用例的功能点，不要把同一断言拆成多个近义点。

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

	aiResult, err := callDeepSeek(c.Request.Context(), systemPrompt, req.Text)
	if err != nil {
		log.Printf("[ERROR] AI 需求拆解失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI 拆解失败: %v", err)})
		return
	}

	var points []models.RequirementPoint
	if err := parseAIJSON(aiResult, &points); err != nil {
		log.Printf("[ERROR] 解析 AI 拆解结果失败: %v, 原文: %s", err, aiResult)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析 AI 响应失败", "raw": aiResult})
		return
	}

	c.JSON(http.StatusOK, gin.H{"points": points})
}

// SmartDecomposeHandler AI 智能增强拆解：基于原始文本进行第二轮更细粒度的需求拆解
// 返回增强后的需求点列表、与原始列表的相似度、以及合并后的结果
func SmartDecomposeHandler(c *gin.Context) {
	var req struct {
		Text           string                    `json:"text" binding:"required"`
		ExistingPoints []models.RequirementPoint `json:"existing_points"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供需求描述文本"})
		return
	}

	// 将现有拆解结果序列化，给 AI 作为参考
	existingJSON, _ := json.Marshal(req.ExistingPoints)

	systemPrompt := `你是一名资深的业务分析师和需求工程师，擅长发现隐含需求和边界条件。
用户提供了一段原始需求文档，以及团队已有的初步拆解结果。

你的任务是进行第二轮深度拆解，重点关注：
1. 发现初步拆解中遗漏的隐含需求、边界条件、异常场景。
2. 将粒度过大的需求点拆分为更细的子功能点。
3. 默认仍聚焦 App 功能测试；只有原始需求明确提到，或该点属于登录、支付、订单、账号、权限等高风险核心链路时，才补充安全性、性能、兼容性等非功能性需求。
4. 每个需求点应足够细粒度，但要避免把同一功能拆成大量同义或近义点。

输出格式：必须严格输出一个 JSON 数组。
- 不要包含任何解释性文字。
- 不要在 JSON 结尾添加句号等标点。
- 每个对象包含：module, feature, description, rules。
- feature 必须足够具体，不要泛泛而谈。`

	userContent := fmt.Sprintf("原始需求文档：\n%s\n\n团队初步拆解结果（仅供参考，请独立分析）：\n%s", req.Text, string(existingJSON))

	aiResult, err := callDeepSeek(c.Request.Context(), systemPrompt, userContent)
	if err != nil {
		log.Printf("[ERROR] AI 智能拆解失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI 智能拆解失败: %v", err)})
		return
	}

	var enhancedPoints []models.RequirementPoint
	if err := parseAIJSON(aiResult, &enhancedPoints); err != nil {
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

		cases, err := generateBatch(c.Request.Context(), batch)
		if err != nil {
			log.Printf("[ERROR] 第 %d 批次生成失败: %v", i/batchSize+1, err)
			if len(allCases) > 0 {
				c.JSON(http.StatusOK, gin.H{
					"cases":   allCases,
					"error":   fmt.Sprintf("第 %d 批次生成失败: %v", i/batchSize+1, err),
					"partial": true,
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
		"cases":       finalCases,
		"batch_count": (totalPoints + batchSize - 1) / batchSize,
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
func generateBatch(ctx context.Context, points []models.RequirementPoint) ([]models.TestCase, error) {
	pointsJSON, _ := json.Marshal(points)

	systemPrompt := `你是一名资深的软件测试专家。
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

	aiResult, err := callDeepSeek(ctx, systemPrompt, string(pointsJSON))
	if err != nil {
		return nil, err
	}

	var cases []models.TestCase
	if err := parseAIJSON(aiResult, &cases); err != nil {
		// 解析失败时记录完整的 AI 原始响应以便调试
		log.Printf("[TestCaseGen] AI 原始响应 (前500字符): %.500s", aiResult)
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

// callDeepSeek 调用 DeepSeek API
func callDeepSeek(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	config := model.GlobalAIConfig
	if config == nil {
		config = model.LoadAIConfig()
	}

	providers := config.EffectiveProviders()
	if config == nil || len(providers) == 0 {
		return "", fmt.Errorf("未配置 AI 接口 Key")
	}

	// 显式配置 Transport 以支持环境变量代理和更长的连接生命周期
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   180 * time.Second,
	}

	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   8192, // DeepSeek API 允许的最大值
	}

	var resp openai.ChatCompletionResponse
	var err error
	var lastErr error
	maxRetries := 3

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
				break
			}
			lastErr = err
			log.Printf("[TestCaseGen] %s AI API 异常 (第 %d 次尝试): %v", provider.Name, i+1, err)

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
		log.Printf("[TestCaseGen] %s provider failed, trying next AI provider if available.", provider.Name)
	}

	if err != nil {
		if lastErr != nil {
			return "", fmt.Errorf("AI 生成请求最终失败: %v", lastErr)
		}
		return "", fmt.Errorf("AI 生成请求最终失败: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI 未返回任何内容")
	}

	return resp.Choices[0].Message.Content, nil
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
	systemPrompt := `你是一名资深的业务分析师和需求工程师。
用户的输入是一段原始的需求文档、功能描述或用户故事。
你的任务是将其拆解为结构化的需求点。

拆解原则：
1. 默认聚焦 App 本身的业务功能、页面流程、状态变化、核心规则和必要边界。
2. 只有当原始需求明确提到，或该功能属于高风险核心链路（如登录、支付、下单、账号、订阅、权限）时，才补充性能、安全、兼容性等非功能点。
3. 不要为了“覆盖更全”而机械拆出浏览器兼容、无障碍、老旧设备适配等泛化需求，除非原文明确要求。
4. feature 必须是适合落成功能测试用例的功能点，不要把同一断言拆成多个近义点。

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

	aiResult, err := callDeepSeek(ctx, systemPrompt, text)
	if err != nil {
		return nil, fmt.Errorf("AI 拆解失败: %w", err)
	}

	var points []models.RequirementPoint
	if err := parseAIJSON(aiResult, &points); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w\nRaw: %s", err, aiResult)
	}
	return points, nil
}

// SmartDecomposeCore enhances the initial requirement points
func SmartDecomposeCore(ctx context.Context, text string, existingPoints []models.RequirementPoint) ([]models.RequirementPoint, error) {
	existingJSON, _ := json.Marshal(existingPoints)
	systemPrompt := `你是一名资深的业务分析师和需求工程师，擅长发现隐含需求和边界条件。
用户提供了一段原始需求文档，以及团队已有的初步拆解结果。

你的任务是进行第二轮深度拆解，重点关注：
1. 发现初步拆解中遗漏的隐含需求、边界条件、异常场景。
2. 将粒度过大的需求点拆分为更细的子功能点。
3. 默认仍聚焦 App 功能测试；只有原始需求明确提到，或该点属于登录、支付、订单、账号、权限等高风险核心链路时，才补充安全性、性能、兼容性等非功能性需求。
4. 每个需求点应足够细粒度，但要避免把同一功能拆成大量同义或近义点。

输出格式：必须严格输出一个 JSON 数组。
- 不要包含任何解释性文字。
- 不要在 JSON 结尾添加句号等标点。
- 每个对象包含：module, feature, description, rules。
- feature 必须足够具体，不要泛泛而谈。

示例格式：
[
  {
    "module": "用户中心",
    "feature": "手机号注册",
    "description": "用户通过输入手机号和短信验证码完成注册",
    "rules": ["手机号必须为11位数字", "验证码5分钟内有效"]
  }
]`

	userContent := fmt.Sprintf("原始需求文档：\n%s\n\n团队初步拆解结果（仅供参考，请独立分析）：\n%s", text, string(existingJSON))
	aiResult, err := callDeepSeek(ctx, systemPrompt, userContent)
	if err != nil {
		return nil, fmt.Errorf("AI 智能拆解失败: %w", err)
	}

	var enhancedPoints []models.RequirementPoint
	if err := parseAIJSON(aiResult, &enhancedPoints); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w\nRaw: %s", err, aiResult)
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
	return generateBatch(ctx, points)
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
