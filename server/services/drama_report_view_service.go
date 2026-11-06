package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

var errDramaReportInvalidID = errors.New("invalid report ID")
var dramaReportIssueIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var dramaReportCategories = []struct{ Key, Label string }{
	{"playback", "播放异常"}, {"group", "分组异常"}, {"classification", "分类异常"},
	{"language", "语种异常"}, {"copyright", "版权异常"}, {"free", "免费集数异常"}, {"other", "其他异常"},
}

func dramaReportCategory(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "•"))
	switch {
	case strings.HasPrefix(line, "[简介语种]"):
		return "language"
	case strings.HasPrefix(line, "[版权规则]"):
		return "copyright"
	case isScheduledDramaGroupError(line):
		return "group"
	case isScheduledDramaClassificationError(line):
		return "classification"
	case strings.HasPrefix(line, "[免费集数]"):
		return "free"
	case isScheduledDramaOtherError(line):
		return "other"
	default:
		return "playback"
	}
}

func isDramaRuleCategory(category string) bool {
	return category == "language" || category == "copyright" || category == "group" || category == "classification"
}

func validDramaReportCategory(category string) bool {
	if category == "all" || category == "rules" || category == "review" {
		return true
	}
	for _, c := range dramaReportCategories {
		if category == c.Key {
			return true
		}
	}
	return false
}

func dramaErrorsDetailsURL(reportURL string) string {
	parsed, err := url.Parse(reportURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "test-runs" || parts[3] != "artifacts" || parts[4] != "report" || !languageRunIDPattern.MatchString(parts[2]) {
		return ""
	}
	parsed.Path = "/api/test-runs/" + parts[2] + "/errors"
	parsed.RawPath, parsed.Fragment = "", ""
	parsed.RawQuery = "category=rules"
	return parsed.String()
}

type dramaReportRecord struct {
	ID          string
	Failure     dramaFailureSummary
	Categories  []string
	Search      string
	Description string
	CanMark     bool
}

type dramaReportSummary struct {
	RunID       string                `json:"runId"`
	StartedAt   string                `json:"startedAt"`
	FinishedAt  string                `json:"finishedAt"`
	Duration    string                `json:"duration"`
	Status      string                `json:"status"`
	Total       int                   `json:"total"`
	RuleTotal   int                   `json:"ruleTotal"`
	Categories  map[string]int        `json:"categories"`
	Coverage    []dramaReportCoverage `json:"coverage"`
	Metrics     DramaRunMetrics       `json:"metrics"`
	RuleScope   json.RawMessage       `json:"ruleScope,omitempty"`
	ReviewCount int                   `json:"reviewCount"`
}

type dramaReportCoverage struct {
	Phase     string `json:"phase"`
	Expected  int    `json:"expected"`
	Completed int    `json:"completed"`
	Missing   int    `json:"missing"`
	Complete  bool   `json:"complete"`
}

type dramaReportSnapshot struct {
	Summary dramaReportSummary
	Rows    []dramaReportRecord
	ByID    map[string]int
}

type dramaReportCacheEntry struct {
	Fingerprint string
	Snapshot    *dramaReportSnapshot
	Bytes       int64
	LastUsed    time.Time
}

// Bounded, source-stamped cache. Nothing is written to an immutable archive.
var dramaReportCache = struct {
	sync.Mutex
	Entries map[string]dramaReportCacheEntry
}{Entries: map[string]dramaReportCacheEntry{}}

func loadDramaReportSnapshot(root, runID string) (*dramaReportSnapshot, error) {
	if !languageRunIDPattern.MatchString(runID) {
		return nil, errDramaReportInvalidID
	}
	dir := filepath.Join(testRunStorageRoot(root), runID)
	files := []string{"metadata.json", "artifacts/drama_failure_summary.json", "artifacts/drama_retry_result.json", "artifacts/drama_retry_scope_changes.json", "artifacts/drama_initial_coverage.json", "artifacts/drama_retry_coverage.json", "artifacts/drama_initial_language_review.json", "artifacts/drama_retry_language_review.json", "artifacts/report.html"}
	var stamp strings.Builder
	var bytes int64
	for _, name := range files {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			if name == "metadata.json" || !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		fmt.Fprintf(&stamp, "%s:%d:%d;", name, info.Size(), info.ModTime().UnixNano())
		bytes += info.Size()
	}
	fingerprint := stamp.String()
	dramaReportCache.Lock()
	defer dramaReportCache.Unlock()
	if entry, ok := dramaReportCache.Entries[dir]; ok && entry.Fingerprint == fingerprint {
		entry.LastUsed = time.Now()
		dramaReportCache.Entries[dir] = entry
		return entry.Snapshot, nil
	}
	snapshot, err := buildDramaReportSnapshot(dir)
	if err != nil {
		return nil, err
	}
	// Keep no more than six reports / 32 MiB of source data cached. Large
	// individual archives can still be viewed, but are not retained globally.
	delete(dramaReportCache.Entries, dir)
	if bytes <= 32*1024*1024 {
		for {
			var total int64
			oldestKey := ""
			var oldest time.Time
			for key, entry := range dramaReportCache.Entries {
				total += entry.Bytes
				if oldestKey == "" || entry.LastUsed.Before(oldest) {
					oldestKey, oldest = key, entry.LastUsed
				}
			}
			if len(dramaReportCache.Entries) < 6 && total+bytes <= 32*1024*1024 || oldestKey == "" {
				break
			}
			delete(dramaReportCache.Entries, oldestKey)
		}
		dramaReportCache.Entries[dir] = dramaReportCacheEntry{fingerprint, snapshot, bytes, time.Now()}
	}
	return snapshot, nil
}

func buildDramaReportSnapshot(dir string) (*dramaReportSnapshot, error) {
	content, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return nil, err
	}
	var archive TestRunArchive
	if err := json.Unmarshal(content, &archive); err != nil {
		return nil, err
	}
	if archive.TestType != "drama" {
		return nil, os.ErrNotExist
	}
	summary := dramaReportSummary{RunID: archive.RunID, StartedAt: archive.StartedAt, FinishedAt: archive.FinishedAt, Duration: archive.Duration, Status: archive.Status, Metrics: archive.Metrics, Categories: map[string]int{}, Coverage: []dramaReportCoverage{}}
	structured := false
	for _, file := range []struct{ Name, Field string }{{"drama_failure_summary.json", "failures"}, {"drama_retry_result.json", "structuredFailures"}} {
		b, readErr := os.ReadFile(filepath.Join(dir, "artifacts", file.Name))
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			return nil, readErr
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(b, &payload); err != nil {
			return nil, err
		}
		if raw, ok := payload[file.Field]; ok {
			var rows []dramaFailureSummary
			if err := json.Unmarshal(raw, &rows); err != nil {
				return nil, err
			}
			structured = true
		}
		if file.Name == "drama_failure_summary.json" {
			summary.RuleScope = payload["ruleScope"]
		}
	}
	reportHTML := ""
	if !structured {
		b, err := os.ReadFile(filepath.Join(dir, "artifacts", "report.html"))
		if err != nil {
			return nil, err
		}
		reportHTML = string(b)
	}
	failures := loadArchivedDramaFailures(dir, reportHTML)
	sort.Slice(failures, func(i, j int) bool {
		a, e1 := strconv.Atoi(failures[i].IntID)
		b, e2 := strconv.Atoi(failures[j].IntID)
		if e1 == nil && e2 == nil && a != b {
			return a < b
		}
		return failures[i].DramaID < failures[j].DramaID
	})
	snapshot := &dramaReportSnapshot{Summary: summary, Rows: []dramaReportRecord{}, ByID: map[string]int{}}
	for _, failure := range failures {
		categories, seen, rule := []string{}, map[string]bool{}, false
		for _, line := range failure.Errors {
			category := dramaReportCategory(line)
			if !seen[category] {
				categories, seen[category] = append(categories, category), true
				snapshot.Summary.Categories[category]++
			}
			rule = rule || isDramaRuleCategory(category)
		}
		if len(categories) == 0 {
			continue
		}
		if rule {
			snapshot.Summary.RuleTotal++
		}
		row := dramaReportRecord{ID: dramaIssueSignature(failure.DramaID, failure.Errors), Failure: failure, Categories: categories, CanMark: true}
		row.Search = strings.ToLower(strings.Join([]string{failure.DramaID, failure.IntID, failure.Title, failure.CNTitle, strings.Join(failure.Errors, " ")}, " "))
		snapshot.ByID[row.ID] = len(snapshot.Rows)
		snapshot.Rows = append(snapshot.Rows, row)
	}
	snapshot.Summary.Total = len(snapshot.Rows)
	for _, phase := range []string{"initial", "retry"} {
		b, err := os.ReadFile(filepath.Join(dir, "artifacts", "drama_"+phase+"_coverage.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var coverage dramaCoverageFile
		if err := json.Unmarshal(b, &coverage); err != nil {
			return nil, err
		}
		snapshot.Summary.Coverage = append(snapshot.Summary.Coverage, dramaReportCoverage{phase, len(coverage.ExpectedIDs), len(coverage.CompletedIDs), len(coverage.MissingIDs), coverage.Complete})
	}
	if err := appendDramaReviewRecords(snapshot, dir); err != nil {
		return nil, err
	}
	return snapshot, nil
}

type dramaLanguageReviewRecord struct {
	DramaID     string `json:"dramaId"`
	IntID       string `json:"intId"`
	Title       string `json:"title"`
	CNTitle     string `json:"cnTitle"`
	Lang        string `json:"lang"`
	Detected    string `json:"detected"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	Description string `json:"description"`
}

var dramaReviewTablePattern = regexp.MustCompile(`(?s)<details><summary>待人工复核明细.*?<tbody>(.*?)</tbody>`)
var dramaReviewRowPattern = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)
var dramaReviewCellPattern = regexp.MustCompile(`(?s)<td>(.*?)</td>`)

func appendDramaReviewRecords(snapshot *dramaReportSnapshot, dir string) error {
	reviews := map[string]dramaLanguageReviewRecord{}
	found := false
	for _, phase := range []string{"initial", "retry"} {
		b, err := os.ReadFile(filepath.Join(dir, "artifacts", "drama_"+phase+"_language_review.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		var payload struct {
			Rows       []dramaLanguageReviewRecord `json:"rows"`
			CheckedIDs []string                    `json:"checkedIds"`
		}
		if err := json.Unmarshal(b, &payload); err != nil {
			return err
		}
		found = true
		for _, id := range payload.CheckedIDs {
			delete(reviews, id)
		}
		for _, row := range payload.Rows {
			reviews[row.DramaID] = row
		}
	}
	// Older reports already contain their review evidence. Extract it server-side
	// once, rather than sending the full legacy HTML to every browser.
	if !found {
		if b, err := os.ReadFile(filepath.Join(dir, "artifacts", "report.html")); err == nil {
			if table := dramaReviewTablePattern.FindStringSubmatch(string(b)); len(table) > 1 {
				for _, tr := range dramaReviewRowPattern.FindAllStringSubmatch(table[1], -1) {
					cells := dramaReviewCellPattern.FindAllStringSubmatch(tr[1], -1)
					if len(cells) != 5 {
						continue
					}
					identity := splitNonEmptyLines(htmlToPlainText(strings.ReplaceAll(cells[0][1], "<br>", "\n")))
					if len(identity) < 2 {
						continue
					}
					row := dramaLanguageReviewRecord{DramaID: identity[len(identity)-1], CNTitle: identity[0], Status: htmlToPlainText(cells[1][1]), Reason: htmlToPlainText(cells[3][1])}
					languages := strings.SplitN(htmlToPlainText(cells[2][1]), "/", 2)
					row.Lang = strings.TrimSpace(languages[0])
					if len(languages) > 1 {
						row.Detected = strings.TrimSpace(languages[1])
					}
					row.Description = strings.TrimSpace(strings.TrimPrefix(htmlToPlainText(cells[4][1]), "查看简介"))
					reviews[row.DramaID] = row
				}
			}
		}
	}
	removed := map[string]bool{}
	if b, err := os.ReadFile(filepath.Join(dir, "artifacts", "drama_retry_scope_changes.json")); err == nil {
		var changes struct {
			RemovedIDs []string `json:"removedIds"`
		}
		if err := json.Unmarshal(b, &changes); err != nil {
			return err
		}
		for _, id := range changes.RemovedIDs {
			removed[id] = true
		}
	}
	ids := make([]string, 0, len(reviews))
	for id := range reviews {
		if !removed[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := reviews[id]
		line := fmt.Sprintf("[简介语种待复核] %s；lang=%s，识别=%s；%s", r.Status, r.Lang, r.Detected, r.Reason)
		failure := dramaFailureSummary{DramaID: id, IntID: r.IntID, Title: r.Title, CNTitle: r.CNTitle, Errors: []string{line}}
		digest := sha256.Sum256([]byte("review\n" + id + "\n" + line))
		row := dramaReportRecord{ID: hex.EncodeToString(digest[:]), Failure: failure, Categories: []string{"review"}, Description: r.Description}
		row.Search = strings.ToLower(id + " " + r.IntID + " " + r.Title + " " + r.CNTitle + " " + line + " " + r.Description)
		snapshot.ByID[row.ID] = len(snapshot.Rows)
		snapshot.Rows = append(snapshot.Rows, row)
	}
	snapshot.Summary.ReviewCount = len(ids)
	return nil
}

func dramaReportError(c *gin.Context, err error) {
	if errors.Is(err, errDramaReportInvalidID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "报告编号无效"})
	} else if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在或不是剧集报告"})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "报告数据读取失败，请稍后重试"})
	}
}

func GetDramaReportDataHandler(c *gin.Context)  { serveDramaReportData(c, projectRootDir()) }
func GetDramaReportIssueHandler(c *gin.Context) { serveDramaReportIssue(c, projectRootDir()) }

func serveDramaReportData(c *gin.Context, root string) {
	category := c.DefaultQuery("category", "all")
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 1000000 {
		c.JSON(400, gin.H{"error": "页码无效"})
		return
	}
	size, err := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if err != nil || size < 1 || size > 100 {
		c.JSON(400, gin.H{"error": "每页数量应为1至100"})
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	if !validDramaReportCategory(category) || utf8.RuneCountInString(query) > 200 {
		c.JSON(400, gin.H{"error": "筛选条件无效"})
		return
	}
	snapshot, err := loadDramaReportSnapshot(root, c.Param("runId"))
	if err != nil {
		dramaReportError(c, err)
		return
	}
	matched := []int{}
	for i, row := range snapshot.Rows {
		selected := category == "all" && row.CanMark
		for _, key := range row.Categories {
			selected = selected || category == key || category == "rules" && isDramaRuleCategory(key)
		}
		if selected && (query == "" || strings.Contains(row.Search, strings.ToLower(query))) {
			matched = append(matched, i)
		}
	}
	pages := (len(matched) + size - 1) / size
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
	}
	start, end := (page-1)*size, page*size
	if end > len(matched) {
		end = len(matched)
	}
	store, storeErr := loadDramaIssueDispositionStore()
	items := []map[string]any{}
	for _, index := range matched[start:end] {
		row := snapshot.Rows[index]
		preview := []string{}
		for _, line := range row.Failure.Errors {
			if category != "all" && category != "review" && !(category == "rules" && isDramaRuleCategory(dramaReportCategory(line))) && category != dramaReportCategory(line) {
				continue
			}
			compact := compactDramaLanguageFailure(dramaFailureSummary{Errors: []string{line}}).Errors[0]
			if len(preview) < 3 {
				preview = append(preview, truncateDramaReportText(compact, 180))
			}
		}
		action := ""
		if storeErr == nil {
			action = store.Items[row.ID].Action
		}
		items = append(items, map[string]any{"id": row.ID, "dramaId": row.Failure.DramaID, "intId": row.Failure.IntID, "title": row.Failure.Title, "cnTitle": row.Failure.CNTitle, "categories": row.Categories, "preview": preview, "errorCount": len(row.Failure.Errors), "action": action, "canMark": row.CanMark, "dramaURL": buildDramaManagementURL(row.Failure.DramaID)})
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(200, gin.H{"summary": snapshot.Summary, "items": items, "total": len(matched), "page": page, "pageSize": size, "pages": pages, "markingAvailable": storeErr == nil})
}

func truncateDramaReportText(text string, size int) string {
	runes := []rune(text)
	if len(runes) > size {
		return string(runes[:size]) + "…"
	}
	return text
}

func serveDramaReportIssue(c *gin.Context, root string) {
	if !dramaReportIssueIDPattern.MatchString(c.Param("issueId")) {
		c.JSON(400, gin.H{"error": "异常编号无效"})
		return
	}
	snapshot, err := loadDramaReportSnapshot(root, c.Param("runId"))
	if err != nil {
		dramaReportError(c, err)
		return
	}
	index, ok := snapshot.ByID[c.Param("issueId")]
	if !ok {
		c.JSON(404, gin.H{"error": "异常不属于该报告"})
		return
	}
	row := snapshot.Rows[index]
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(200, gin.H{"id": row.ID, "dramaId": row.Failure.DramaID, "intId": row.Failure.IntID, "title": row.Failure.Title, "cnTitle": row.Failure.CNTitle, "errors": row.Failure.Errors, "description": row.Description, "canMark": row.CanMark, "dramaURL": buildDramaManagementURL(row.Failure.DramaID)})
}
