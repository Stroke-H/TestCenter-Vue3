package services

import (
	"net/http"
	"sync"
	"testcenter-server/models"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	pwMutex        sync.Mutex
	pwSuiteTable   = "pw_suites"
	pwCaseTable    = "pw_cases"
	pwKeywordTable = "pw_keywords"
)

// --- SQL-backed Helpers ---

func loadPlaywrightRecords[T any](tableName string) ([]T, error) {
	pwMutex.Lock()
	defer pwMutex.Unlock()
	return sqlListJSON[T](tableName, "`migrated_at` ASC")
}

func savePlaywrightRecords[T any](tableName string, items []T) error {
	pwMutex.Lock()
	defer pwMutex.Unlock()
	return sqlReplaceAllJSON(tableName, items)
}

// --- Suite Handlers ---

func ListPlaywrightSuitesHandler(c *gin.Context) {
	suites, err := loadPlaywrightRecords[models.PlaywrightSuite](pwSuiteTable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, suites)
}

func GetPlaywrightSuiteHandler(c *gin.Context) {
	id := c.Param("id")
	suites, err := loadPlaywrightRecords[models.PlaywrightSuite](pwSuiteTable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, s := range suites {
		if s.ID == id {
			c.JSON(http.StatusOK, s)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Suite not found"})
}

func SavePlaywrightSuiteHandler(c *gin.Context) {
	var suite models.PlaywrightSuite
	if err := c.ShouldBindJSON(&suite); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	suites, _ := loadPlaywrightRecords[models.PlaywrightSuite](pwSuiteTable)
	now := time.Now().Format(time.RFC3339)

	found := false
	if suite.ID != "" {
		for i, s := range suites {
			if s.ID == suite.ID {
				suite.CreatedAt = s.CreatedAt
				suite.UpdatedAt = now
				suites[i] = suite
				found = true
				break
			}
		}
	}

	if !found {
		if suite.ID == "" {
			suite.ID = "suite-" + uuid.New().String()[:8]
		}
		suite.CreatedAt = now
		suite.UpdatedAt = now
		suites = append(suites, suite)
	}

	if err := savePlaywrightRecords(pwSuiteTable, suites); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, suite)
}

func DeletePlaywrightSuiteHandler(c *gin.Context) {
	id := c.Param("id")
	suites, _ := loadPlaywrightRecords[models.PlaywrightSuite](pwSuiteTable)

	newSuites := []models.PlaywrightSuite{}
	deleted := false
	for _, s := range suites {
		if s.ID == id {
			deleted = true
			continue
		}
		newSuites = append(newSuites, s)
	}

	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "Suite not found"})
		return
	}

	if err := savePlaywrightRecords(pwSuiteTable, newSuites); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Suite deleted"})
}

// --- Case Handlers ---

func ListPlaywrightCasesHandler(c *gin.Context) {
	cases, err := loadPlaywrightRecords[models.PlaywrightCase](pwCaseTable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	suiteID := c.Query("suite_id")
	if suiteID != "" {
		filtered := []models.PlaywrightCase{}
		for _, cs := range cases {
			if cs.SuiteID == suiteID {
				filtered = append(filtered, cs)
			}
		}
		c.JSON(http.StatusOK, filtered)
		return
	}

	c.JSON(http.StatusOK, cases)
}

func GetPlaywrightCaseHandler(c *gin.Context) {
	id := c.Param("id")
	cases, err := loadPlaywrightRecords[models.PlaywrightCase](pwCaseTable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, cs := range cases {
		if cs.ID == id {
			c.JSON(http.StatusOK, cs)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Case not found"})
}

func SavePlaywrightCaseHandler(c *gin.Context) {
	var cs models.PlaywrightCase
	if err := c.ShouldBindJSON(&cs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	cases, _ := loadPlaywrightRecords[models.PlaywrightCase](pwCaseTable)
	now := time.Now().Format(time.RFC3339)

	found := false
	if cs.ID != "" {
		for i, item := range cases {
			if item.ID == cs.ID {
				cs.CreatedAt = item.CreatedAt
				cs.UpdatedAt = now
				cases[i] = cs
				found = true
				break
			}
		}
	}

	if !found {
		if cs.ID == "" {
			cs.ID = "case-" + uuid.New().String()[:8]
		}
		cs.CreatedAt = now
		cs.UpdatedAt = now
		cases = append(cases, cs)

		// Optionally update suite's CaseIDs if SuiteID is provided
		if cs.SuiteID != "" {
			updateSuiteCaseList(cs.SuiteID, cs.ID)
		}
	}

	if err := savePlaywrightRecords(pwCaseTable, cases); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cs)
}

func updateSuiteCaseList(suiteID, caseID string) {
	suites, _ := loadPlaywrightRecords[models.PlaywrightSuite](pwSuiteTable)
	for i, s := range suites {
		if s.ID == suiteID {
			exists := false
			for _, cid := range s.CaseIDs {
				if cid == caseID {
					exists = true
					break
				}
			}
			if !exists {
				suites[i].CaseIDs = append(suites[i].CaseIDs, caseID)
				_ = savePlaywrightRecords(pwSuiteTable, suites)
			}
			return
		}
	}
}

func DeletePlaywrightCaseHandler(c *gin.Context) {
	id := c.Param("id")
	cases, _ := loadPlaywrightRecords[models.PlaywrightCase](pwCaseTable)

	newCases := []models.PlaywrightCase{}
	deleted := false
	for _, cs := range cases {
		if cs.ID == id {
			deleted = true
			continue
		}
		newCases = append(newCases, cs)
	}

	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "Case not found"})
		return
	}

	if err := savePlaywrightRecords(pwCaseTable, newCases); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Case deleted"})
}

// --- Built-in Keywords ---

func GetBuiltinKeywordsHandler(c *gin.Context) {
	// Static list of mapped Playwright functions
	keywords := []map[string]interface{}{
		{"name": "打开浏览器", "keyword": "Launch", "group": "浏览器", "args": []string{"headless"}},
		{"name": "导航到", "keyword": "Goto", "group": "导航", "args": []string{"url", "waitUntil"}},
		{"name": "点击元素", "keyword": "Click", "group": "元素交互", "args": []string{"selector"}},
		{"name": "输入文本", "keyword": "Fill", "group": "元素交互", "args": []string{"selector", "value"}},
		{"name": "键盘按键", "keyword": "Press", "group": "元素交互", "args": []string{"selector", "key"}},
		{"name": "获取文本", "keyword": "TextContent", "group": "元素交互", "args": []string{"selector"}},
		{"name": "等待元素", "keyword": "WaitForSelector", "group": "元素交互", "args": []string{"selector", "timeout"}},
		{"name": "截图", "keyword": "Screenshot", "group": "工具", "args": []string{"fullPage"}},
		{"name": "暂停", "keyword": "Sleep", "group": "工具", "args": []string{"seconds"}},
		{"name": "断言URL包含", "keyword": "AssertURL", "group": "断言", "args": []string{"expected"}},
		{"name": "断言文本包含", "keyword": "AssertText", "group": "断言", "args": []string{"selector", "expected"}},
		{"name": "关闭浏览器", "keyword": "Close", "group": "浏览器", "args": []string{}},
	}
	c.JSON(http.StatusOK, keywords)
}

// --- User Keywords Handlers ---

func ListUserKeywordsHandler(c *gin.Context) {
	keywords, err := loadPlaywrightRecords[models.UserKeyword](pwKeywordTable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, keywords)
}

func SaveUserKeywordHandler(c *gin.Context) {
	var kw models.UserKeyword
	if err := c.ShouldBindJSON(&kw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	keywords, _ := loadPlaywrightRecords[models.UserKeyword](pwKeywordTable)
	now := time.Now().Format(time.RFC3339)

	found := false
	if kw.ID != "" {
		for i, item := range keywords {
			if item.ID == kw.ID {
				keywords[i] = kw
				found = true
				break
			}
		}
	}

	if !found {
		if kw.ID == "" {
			kw.ID = "kw-" + uuid.New().String()[:8]
		}
		kw.CreatedAt = now
		keywords = append(keywords, kw)
	}

	if err := savePlaywrightRecords(pwKeywordTable, keywords); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, kw)
}

func DeleteUserKeywordHandler(c *gin.Context) {
	id := c.Param("id")
	keywords, _ := loadPlaywrightRecords[models.UserKeyword](pwKeywordTable)

	newKeywords := []models.UserKeyword{}
	deleted := false
	for _, kw := range keywords {
		if kw.ID == id {
			deleted = true
			continue
		}
		newKeywords = append(newKeywords, kw)
	}

	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "User keyword not found"})
		return
	}

	if err := savePlaywrightRecords(pwKeywordTable, newKeywords); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User keyword deleted"})
}

func RenameUserKeywordSuiteHandler(c *gin.Context) {
	oldName := c.Param("name")
	if oldName == "未归类套件" {
		oldName = ""
	}
	var req struct {
		NewName string `json:"new_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New name is required"})
		return
	}

	keywords, _ := loadPlaywrightRecords[models.UserKeyword](pwKeywordTable)
	changed := false
	for i, kw := range keywords {
		if kw.SuiteName == oldName {
			keywords[i].SuiteName = req.NewName
			changed = true
		}
	}

	if !changed {
		c.JSON(http.StatusNotFound, gin.H{"error": "Suite not found"})
		return
	}

	if err := savePlaywrightRecords(pwKeywordTable, keywords); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Suite renamed successfully"})
}

func DeleteUserKeywordSuiteHandler(c *gin.Context) {
	name := c.Param("name")
	if name == "未归类套件" {
		name = ""
	}
	keywords, _ := loadPlaywrightRecords[models.UserKeyword](pwKeywordTable)

	newKeywords := []models.UserKeyword{}
	deleted := false
	for _, kw := range keywords {
		if kw.SuiteName == name {
			deleted = true
			continue
		}
		newKeywords = append(newKeywords, kw)
	}

	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "Suite not found"})
		return
	}

	if err := savePlaywrightRecords(pwKeywordTable, newKeywords); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Suite deleted successfully"})
}
