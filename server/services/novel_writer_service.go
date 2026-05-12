package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	feishumodel "testcenter-server/feishu/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openai "github.com/sashabaranov/go-openai"
)

type NovelProject struct {
	ID             string             `json:"id"`
	Title          string             `json:"title"`
	Genre          string             `json:"genre"`
	TargetWords    int                `json:"target_words"`
	TargetChapters int                `json:"target_chapters"`
	Status         string             `json:"status"`
	CurrentStage   string             `json:"current_stage"`
	CreatedBy      string             `json:"created_by"`
	CreatedAt      string             `json:"created_at"`
	UpdatedAt      string             `json:"updated_at"`
	Materials      NovelMaterials     `json:"materials"`
	Extracted      NovelExtractedInfo `json:"extracted"`
	Outline        NovelOutline       `json:"outline"`
	StyleProfile   NovelStyleProfile  `json:"style_profile"`
	Chapters       []NovelChapter     `json:"chapters"`
	Memory         NovelMemory        `json:"memory"`
}

type NovelMaterials struct {
	RawText      string `json:"raw_text"`
	CharacterRaw string `json:"character_raw"`
	WorldRaw     string `json:"world_raw"`
	ConflictRaw  string `json:"conflict_raw"`
	ReferenceRaw string `json:"reference_raw"`
}

type NovelExtractedInfo struct {
	Characters    []NovelInfoCard `json:"characters"`
	WorldRules    []NovelInfoCard `json:"world_rules"`
	Conflicts     []NovelInfoCard `json:"conflicts"`
	KeyEvents     []NovelInfoCard `json:"key_events"`
	OpenQuestions []string        `json:"open_questions"`
}

type NovelInfoCard struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type NovelOutline struct {
	Logline  string                `json:"logline"`
	Acts     []NovelInfoCard       `json:"acts"`
	Chapters []NovelChapterOutline `json:"chapters"`
}

type NovelChapterOutline struct {
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	Goal         string              `json:"goal"`
	Conflict     string              `json:"conflict"`
	Hook         string              `json:"hook"`
	Summary      string              `json:"summary"`
	BeforeState  NovelChapterState   `json:"before_state"`
	AfterState   NovelChapterState   `json:"after_state"`
	MustHappen   []string            `json:"must_happen"`
	TensionCurve []NovelTensionPoint `json:"tension_curve"`
	KeyScenes    []string            `json:"key_scenes"`
	NewHooks     []string            `json:"new_hooks"`
}

type NovelChapterState struct {
	Characters   []NovelCharacterState `json:"characters"`
	PlotHooks    []string              `json:"plot_hooks"`
	PlotAdvances []string              `json:"plot_advances"`
}

type NovelCharacterState struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Location string `json:"location"`
}

type NovelTensionPoint struct {
	Position int    `json:"position"`
	Value    int    `json:"value"`
	Note     string `json:"note"`
}

type NovelStyleProfile struct {
	Summary    string   `json:"summary"`
	Narration  string   `json:"narration"`
	Sentence   string   `json:"sentence"`
	Dialogue   string   `json:"dialogue"`
	Rhythm     string   `json:"rhythm"`
	DoRules    []string `json:"do_rules"`
	AvoidRules []string `json:"avoid_rules"`
}

type NovelChapter struct {
	ID        string                `json:"id"`
	OutlineID string                `json:"outline_id"`
	Title     string                `json:"title"`
	Status    string                `json:"status"`
	Content   string                `json:"content"`
	Summary   string                `json:"summary"`
	Audit     NovelAuditReport      `json:"audit"`
	Versions  []NovelChapterVersion `json:"versions"`
	CreatedAt string                `json:"created_at"`
	UpdatedAt string                `json:"updated_at"`
}

type NovelChapterVersion struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
}

type NovelAuditReport struct {
	TotalScore     int              `json:"total_score"`
	AIFlavorScore  int              `json:"ai_flavor_score"`
	CharacterScore int              `json:"character_score"`
	LogicScore     int              `json:"logic_score"`
	StyleScore     int              `json:"style_score"`
	Issues         []NovelAuditItem `json:"issues"`
	RevisionAdvice string           `json:"revision_advice"`
}

type NovelAuditItem struct {
	Severity   string `json:"severity"`
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Suggestion string `json:"suggestion"`
}

type NovelMemory struct {
	ChapterSummaries []NovelInfoCard `json:"chapter_summaries"`
	CharacterStates  []NovelInfoCard `json:"character_states"`
	OpenHooks        []NovelInfoCard `json:"open_hooks"`
	Timeline         []NovelInfoCard `json:"timeline"`
}

type novelCreateRequest struct {
	Title          string         `json:"title"`
	Genre          string         `json:"genre"`
	TargetWords    int            `json:"target_words"`
	TargetChapters int            `json:"target_chapters"`
	Materials      NovelMaterials `json:"materials"`
}

type novelGenerateChapterRequest struct {
	OutlineID string `json:"outline_id"`
}

func init() {
	sqlJSONTables["novel_projects"] = sqlJSONTable{
		Table:      "novel_projects",
		PrimaryKey: "id",
		Columns: []sqlJSONColumn{
			{Column: "id", Field: "id"},
			{Column: "title", Field: "title"},
			{Column: "genre", Field: "genre"},
			{Column: "target_words", Field: "target_words"},
			{Column: "target_chapters", Field: "target_chapters"},
			{Column: "status", Field: "status"},
			{Column: "current_stage", Field: "current_stage"},
			{Column: "created_by", Field: "created_by"},
			{Column: "created_at", Field: "created_at"},
			{Column: "updated_at", Field: "updated_at"},
			{Column: "materials", Field: "materials", JSON: true},
			{Column: "extracted", Field: "extracted", JSON: true},
			{Column: "outline", Field: "outline", JSON: true},
			{Column: "style_profile", Field: "style_profile", JSON: true},
			{Column: "chapters", Field: "chapters", JSON: true},
			{Column: "memory", Field: "memory", JSON: true},
		},
	}
}

func EnsureNovelWriterTables() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS novel_projects (
			id VARCHAR(64) PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			genre VARCHAR(120) NULL,
			target_words INT NULL,
			target_chapters INT NULL,
			status VARCHAR(40) NULL,
			current_stage VARCHAR(80) NULL,
			created_by VARCHAR(120) NULL,
			created_at VARCHAR(40) NULL,
			updated_at VARCHAR(40) NULL,
			materials JSON NULL,
			extracted JSON NULL,
			outline JSON NULL,
			style_profile JSON NULL,
			chapters JSON NULL,
			memory JSON NULL,
			raw_json JSON NOT NULL,
			migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func RegisterNovelWriterRoutes(api *gin.RouterGroup) {
	novels := api.Group("/novel-writer")
	novels.Use(func(c *gin.Context) {
		_, err := CurrentUserFromRequest(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
			c.Abort()
			return
		}
		c.Next()
	})

	novels.GET("/projects", ListNovelProjectsHandler)
	novels.POST("/projects", CreateNovelProjectHandler)
	novels.GET("/projects/:id", GetNovelProjectHandler)
	novels.PUT("/projects/:id", UpdateNovelProjectHandler)
	novels.DELETE("/projects/:id", DeleteNovelProjectHandler)
	novels.POST("/projects/:id/delete", DeleteNovelProjectHandler)
	novels.POST("/projects/:id/extract", ExtractNovelInfoHandler)
	novels.POST("/projects/:id/outline", PlanNovelOutlineHandler)
	novels.POST("/projects/:id/style", AnalyzeNovelStyleHandler)
	novels.POST("/projects/:id/chapters/generate", GenerateNovelChapterHandler)
	novels.POST("/projects/:id/chapters/:chapterId/audit", AuditNovelChapterHandler)
	novels.POST("/projects/:id/chapters/:chapterId/revise", ReviseNovelChapterHandler)
	novels.POST("/projects/:id/chapters/:chapterId/approve", ApproveNovelChapterHandler)
}

func ListNovelProjectsHandler(c *gin.Context) {
	projects, err := listNovelProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, projects)
}

func CreateNovelProjectHandler(c *gin.Context) {
	user, _ := CurrentUserFromRequest(c)
	var req novelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	project := NovelProject{
		ID:             "NOVEL-" + uuid.NewString(),
		Title:          strings.TrimSpace(req.Title),
		Genre:          strings.TrimSpace(req.Genre),
		TargetWords:    req.TargetWords,
		TargetChapters: req.TargetChapters,
		Status:         "draft",
		CurrentStage:   "material_ready",
		CreatedBy:      user.Username,
		CreatedAt:      now,
		UpdatedAt:      now,
		Materials:      req.Materials,
		Chapters:       []NovelChapter{},
	}
	if err := saveNovelProject(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func GetNovelProjectHandler(c *gin.Context) {
	project, err := getNovelProject(c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if sqlErrNoRows(err) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func UpdateNovelProjectHandler(c *gin.Context) {
	project, err := getNovelProject(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var req NovelProject
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	project.Title = firstNonEmpty(strings.TrimSpace(req.Title), project.Title)
	project.Genre = strings.TrimSpace(req.Genre)
	project.TargetWords = req.TargetWords
	project.TargetChapters = req.TargetChapters
	project.Materials = req.Materials
	project.Extracted = req.Extracted
	project.Outline = req.Outline
	project.StyleProfile = req.StyleProfile
	project.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	if err := saveNovelProject(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func DeleteNovelProjectHandler(c *gin.Context) {
	if err := deleteNovelProject(c.Param("id")); err != nil {
		status := http.StatusInternalServerError
		if sqlErrNoRows(err) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func ExtractNovelInfoHandler(c *gin.Context) {
	project, err := getNovelProject(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	system := "你是严谨的小说设定编辑。请只输出 JSON，不要输出 Markdown。"
	user := fmt.Sprintf(`从以下素材中提取小说创作事实库。
要求：
1. 不要虚构素材中没有的信息，缺失内容放入 open_questions。
2. characters/world_rules/conflicts/key_events 都用 name + description。
3. 输出 JSON schema:
{"characters":[{"name":"","description":""}],"world_rules":[{"name":"","description":""}],"conflicts":[{"name":"","description":""}],"key_events":[{"name":"","description":""}],"open_questions":[""]}

素材：
人物：%s
世界观：%s
冲突事件：%s
其他素材：%s`, project.Materials.CharacterRaw, project.Materials.WorldRaw, project.Materials.ConflictRaw, project.Materials.RawText)

	var extracted NovelExtractedInfo
	if err := callNovelAIJSON(system, user, &extracted); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	project.Extracted = extracted
	project.CurrentStage = "info_extracted"
	project.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	if err := saveNovelProject(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func PlanNovelOutlineHandler(c *gin.Context) {
	project, err := getNovelProject(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	chapterCount := project.TargetChapters
	if chapterCount <= 0 {
		chapterCount = 8
	}
	system := "你是商业小说结构规划师。请只输出 JSON，不要输出 Markdown。"
	user := fmt.Sprintf(`基于事实库生成小说大纲和章节规格。
%s

要求：
1. chapters 数量尽量接近 %d。
2. 每章必须有 title、goal、conflict、hook、summary、before_state、after_state、must_happen、tension_curve、key_scenes、new_hooks。
3. acts 是三幕式或卷结构。
4. 不要虚构事实库之外的硬设定；可以在不冲突的前提下补充剧情规划。
5. 输出 JSON schema:
{"logline":"","acts":[{"name":"","description":""}],"chapters":[{"title":"","goal":"","conflict":"","hook":"","summary":"","before_state":{"characters":[{"name":"","state":"","location":""}],"plot_hooks":[""],"plot_advances":[]},"after_state":{"characters":[{"name":"","state":"","location":""}],"plot_hooks":[""],"plot_advances":[""]},"must_happen":[""],"tension_curve":[{"position":0,"value":3,"note":""},{"position":50,"value":8,"note":""},{"position":100,"value":5,"note":""}],"key_scenes":[""],"new_hooks":[""]}]}

小说：%s
题材：%s
当前素材：%s
事实库：%s
文风画像：%s`, novelOutlineSkillGuide, chapterCount, project.Title, project.Genre, mustJSON(project.Materials), mustJSON(project.Extracted), mustJSON(project.StyleProfile))

	var outline NovelOutline
	if err := callNovelAIJSON(system, user, &outline); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range outline.Chapters {
		if outline.Chapters[i].ID == "" {
			outline.Chapters[i].ID = fmt.Sprintf("outline-%02d", i+1)
		}
	}
	project.Outline = outline
	project.CurrentStage = "outline_ready"
	project.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	if err := saveNovelProject(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func AnalyzeNovelStyleHandler(c *gin.Context) {
	project, err := getNovelProject(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	system := "你是文风分析师。你只能提炼抽象风格规则，不能复刻原文句子、设定、人物或情节。请只输出 JSON。"
	referenceText := strings.TrimSpace(project.Materials.ReferenceRaw)
	referenceInstruction := "用户未提供文风参考文本。请结合小说题材、事实库和商业网文可读性，生成一套原创、自然、不带明显 AI 味的默认文风画像。"
	if referenceText != "" {
		referenceInstruction = fmt.Sprintf("参考文本：\n%s", truncateForAI(referenceText, 12000))
	}
	user := fmt.Sprintf(`分析参考文本的整体文风，用于后续原创小说的抽象风格指导。
输出 JSON schema:
{"summary":"","narration":"","sentence":"","dialogue":"","rhythm":"","do_rules":[""],"avoid_rules":[""]}

小说：%s
题材：%s
事实库：%s
%s`, project.Title, project.Genre, mustJSON(project.Extracted), referenceInstruction)

	var profile NovelStyleProfile
	if err := callNovelAIJSON(system, user, &profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	project.StyleProfile = profile
	project.CurrentStage = "style_ready"
	project.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	if err := saveNovelProject(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func GenerateNovelChapterHandler(c *gin.Context) {
	project, err := getNovelProject(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var req novelGenerateChapterRequest
	_ = c.ShouldBindJSON(&req)
	outline := findNovelOutline(project, req.OutlineID)
	if outline.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outline chapter is required"})
		return
	}
	targetWords := novelChapterTargetWords(project)
	previousContext := previousNovelChapterContext(project, outline.ID, 3)
	system := "你是小说写手。请原创生成正文，不要复刻参考文本的句子、人物、世界观或情节。输出 JSON。"
	user := fmt.Sprintf(`请根据上下文生成一个章节草稿。
%s

要求：
1. 正文 content 以中文小说正文形式输出。
2. summary 概括本章发生的关键事实。
3. 风格只参考 style_profile 的抽象规则，不复制参考文本。
4. 本章目标字数约 %d 字；如果上下文不足，也要优先保证完整场景和章节钩子。
5. 必须覆盖章节规格中的 must_happen，并让结尾承接 hook/new_hooks。
6. 输出 JSON schema: {"title":"","content":"","summary":"","after_state":{"characters":[{"name":"","state":"","location":""}],"plot_hooks":[""],"plot_advances":[""]},"new_hooks":[""]}

小说：%s
章节规格：%s
前三章上下文：%s
事实库：%s
文风画像：%s
长期记忆：%s`, novelWritingSkillGuide, targetWords, project.Title, mustJSON(outline), previousContext, mustJSON(project.Extracted), mustJSON(project.StyleProfile), mustJSON(project.Memory))

	var generated struct {
		Title      string            `json:"title"`
		Content    string            `json:"content"`
		Summary    string            `json:"summary"`
		AfterState NovelChapterState `json:"after_state"`
		NewHooks   []string          `json:"new_hooks"`
	}
	if err := callNovelAIJSON(system, user, &generated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	chapter := NovelChapter{
		ID:        "CH-" + uuid.NewString(),
		OutlineID: outline.ID,
		Title:     firstNonEmpty(generated.Title, outline.Title),
		Status:    "draft",
		Content:   generated.Content,
		Summary:   generated.Summary,
		CreatedAt: now,
		UpdatedAt: now,
	}
	chapter.Versions = append(chapter.Versions, NovelChapterVersion{
		ID:        "VER-" + uuid.NewString(),
		Type:      "draft",
		Content:   chapter.Content,
		Reason:    "AI 初稿",
		CreatedAt: now,
	})
	project.Chapters = append(project.Chapters, chapter)
	project.CurrentStage = "chapter_drafting"
	project.UpdatedAt = now
	if err := saveNovelProject(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func AuditNovelChapterHandler(c *gin.Context) {
	project, chapter, index, ok := getNovelProjectChapter(c)
	if !ok {
		return
	}
	system := "你是小说审计员，从 AI 味、人物一致性、剧情漏洞、文风贴合度审计章节。请只输出 JSON。"
	user := fmt.Sprintf(`审计以下章节。
%s

输出 JSON schema:
{"total_score":0,"ai_flavor_score":0,"character_score":0,"logic_score":0,"style_score":0,"issues":[{"severity":"high|medium|low","title":"","detail":"","suggestion":""}],"revision_advice":""}

事实库：%s
文风画像：%s
章节标题：%s
章节正文：%s`, novelAuditSkillGuide, mustJSON(project.Extracted), mustJSON(project.StyleProfile), chapter.Title, truncateForAI(chapter.Content, 16000))

	var report NovelAuditReport
	if err := callNovelAIJSON(system, user, &report); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	chapter.Audit = report
	chapter.Status = "reviewing"
	chapter.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	project.Chapters[index] = chapter
	project.CurrentStage = "chapter_auditing"
	project.UpdatedAt = chapter.UpdatedAt
	if err := saveNovelProject(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func ReviseNovelChapterHandler(c *gin.Context) {
	project, chapter, index, ok := getNovelProjectChapter(c)
	if !ok {
		return
	}
	system := "你是小说修订者。根据审计意见修复问题，保留原章节优点。请只输出 JSON。"
	user := fmt.Sprintf(`根据审计意见修订章节。
输出 JSON schema: {"content":"","summary":""}

审计意见：%s
原正文：%s`, mustJSON(chapter.Audit), truncateForAI(chapter.Content, 16000))

	var revised struct {
		Content string `json:"content"`
		Summary string `json:"summary"`
	}
	if err := callNovelAIJSON(system, user, &revised); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	chapter.Content = revised.Content
	chapter.Summary = firstNonEmpty(revised.Summary, chapter.Summary)
	chapter.Status = "revision_needed"
	chapter.UpdatedAt = now
	chapter.Versions = append(chapter.Versions, NovelChapterVersion{
		ID:        "VER-" + uuid.NewString(),
		Type:      "revision",
		Content:   revised.Content,
		Reason:    "根据审计意见智能修订",
		CreatedAt: now,
	})
	project.Chapters[index] = chapter
	project.CurrentStage = "chapter_revising"
	project.UpdatedAt = now
	if err := saveNovelProject(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func ApproveNovelChapterHandler(c *gin.Context) {
	project, chapter, index, ok := getNovelProjectChapter(c)
	if !ok {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	chapter.Status = "approved"
	chapter.UpdatedAt = now
	project.Chapters[index] = chapter
	project.Memory.ChapterSummaries = upsertNovelInfoCard(project.Memory.ChapterSummaries, chapter.Title, chapter.Summary)
	project.CurrentStage = "chapter_approved"
	project.UpdatedAt = now
	if err := saveNovelProject(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func listNovelProjects() ([]NovelProject, error) {
	if err := EnsureNovelWriterTables(); err != nil {
		return nil, err
	}
	projects, err := sqlListJSON[NovelProject]("novel_projects", "`migrated_at` DESC")
	if err != nil {
		return nil, err
	}
	sort.SliceStable(projects, func(i, j int) bool {
		return projects[i].UpdatedAt > projects[j].UpdatedAt
	})
	return projects, nil
}

func getNovelProject(id string) (NovelProject, error) {
	if err := EnsureNovelWriterTables(); err != nil {
		return NovelProject{}, err
	}
	projects, err := sqlListJSON[NovelProject]("novel_projects", "`migrated_at` DESC")
	if err != nil {
		return NovelProject{}, err
	}
	for _, project := range projects {
		if project.ID == id {
			return project, nil
		}
	}
	return NovelProject{}, sql.ErrNoRows
}

func saveNovelProject(project NovelProject) error {
	if err := EnsureNovelWriterTables(); err != nil {
		return err
	}
	return sqlUpsertJSON("novel_projects", project)
}

func deleteNovelProject(id string) error {
	if err := EnsureNovelWriterTables(); err != nil {
		return err
	}
	if _, err := getNovelProject(id); err != nil {
		return err
	}
	return sqlDeleteJSON("novel_projects", id)
}

func getNovelProjectChapter(c *gin.Context) (NovelProject, NovelChapter, int, bool) {
	project, err := getNovelProject(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return NovelProject{}, NovelChapter{}, -1, false
	}
	chapterID := c.Param("chapterId")
	for index, chapter := range project.Chapters {
		if chapter.ID == chapterID {
			return project, chapter, index, true
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "chapter not found"})
	return NovelProject{}, NovelChapter{}, -1, false
}

func findNovelOutline(project NovelProject, outlineID string) NovelChapterOutline {
	if outlineID != "" {
		for _, item := range project.Outline.Chapters {
			if item.ID == outlineID {
				return item
			}
		}
	}
	for _, item := range project.Outline.Chapters {
		if !hasChapterForOutline(project.Chapters, item.ID) {
			return item
		}
	}
	if len(project.Outline.Chapters) > 0 {
		return project.Outline.Chapters[0]
	}
	return NovelChapterOutline{}
}

func hasChapterForOutline(chapters []NovelChapter, outlineID string) bool {
	for _, chapter := range chapters {
		if chapter.OutlineID == outlineID {
			return true
		}
	}
	return false
}

func novelChapterTargetWords(project NovelProject) int {
	if project.TargetWords > 0 && project.TargetChapters > 0 {
		average := project.TargetWords / project.TargetChapters
		if average < 1200 {
			return 1200
		}
		if average > 5000 {
			return 5000
		}
		return average
	}
	return 3000
}

func previousNovelChapterContext(project NovelProject, outlineID string, limit int) string {
	if limit <= 0 {
		limit = 3
	}
	outlineOrder := map[string]int{}
	for index, outline := range project.Outline.Chapters {
		outlineOrder[outline.ID] = index
	}
	currentOrder, hasCurrentOrder := outlineOrder[outlineID]

	type chapterContext struct {
		Order   int    `json:"order"`
		Title   string `json:"title"`
		Status  string `json:"status"`
		Summary string `json:"summary"`
	}

	items := make([]chapterContext, 0, len(project.Chapters))
	for index, chapter := range project.Chapters {
		order, ok := outlineOrder[chapter.OutlineID]
		if !ok {
			order = index
		}
		if hasCurrentOrder && order >= currentOrder {
			continue
		}
		items = append(items, chapterContext{
			Order:   order,
			Title:   chapter.Title,
			Status:  chapter.Status,
			Summary: firstNonEmpty(strings.TrimSpace(chapter.Summary), truncateForAI(chapter.Content, 600)),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Order > items[j].Order
	})
	if len(items) > limit {
		items = items[:limit]
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Order < items[j].Order
	})
	return mustJSON(items)
}

func callNovelAIJSON(systemPrompt string, userPrompt string, target any) error {
	provider, err := selectNovelProvider()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	clientConfig := openai.DefaultConfig(provider.APIKey)
	clientConfig.BaseURL = provider.BaseURL
	client := openai.NewClientWithConfig(clientConfig)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: provider.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.7,
	})
	if err != nil {
		return err
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("novel AI provider returned no choices")
	}
	return decodeNovelJSON(resp.Choices[0].Message.Content, target)
}

func selectNovelProvider() (feishumodel.AIProviderConfig, error) {
	config := feishumodel.GlobalAIConfig
	if config == nil {
		config = feishumodel.LoadAIConfig()
	}
	for _, capability := range []string{"novel_writing", "casual_chat_pro", "casual_chat", "chat"} {
		providers := config.EffectiveProvidersByCapability(capability)
		if len(providers) > 0 {
			return providers[0], nil
		}
	}
	return feishumodel.AIProviderConfig{}, fmt.Errorf("novel writing AI provider is not configured")
}

func decodeNovelJSON(content string, target any) error {
	cleaned := strings.TrimSpace(content)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	if err := json.Unmarshal([]byte(cleaned), target); err == nil {
		return nil
	}
	re := regexp.MustCompile(`(?s)\{.*\}`)
	match := re.FindString(cleaned)
	if match == "" {
		return fmt.Errorf("AI output is not valid JSON")
	}
	return json.Unmarshal([]byte(match), target)
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func upsertNovelInfoCard(items []NovelInfoCard, name string, description string) []NovelInfoCard {
	for index, item := range items {
		if item.Name == name {
			items[index].Description = description
			return items
		}
	}
	return append(items, NovelInfoCard{Name: name, Description: description})
}
