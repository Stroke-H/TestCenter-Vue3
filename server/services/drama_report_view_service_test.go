package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func dramaReportFixture(t *testing.T, count int) (string, string, []dramaFailureSummary) {
	t.Helper()
	root, runID := t.TempDir(), "report-pagination-test"
	dir := filepath.Join(testRunStorageRoot(root), runID)
	if err := os.MkdirAll(filepath.Join(dir, "artifacts"), 0700); err != nil {
		t.Fatal(err)
	}
	archive := TestRunArchive{RunID: runID, TestType: "drama", Status: "Passed", StartedAt: "2026/11/06 09:00:25", FinishedAt: "2026/11/06 09:24:01", Duration: "00:23:36"}
	if err := writeJSONFile(filepath.Join(dir, "metadata.json"), archive); err != nil {
		t.Fatal(err)
	}
	rows := make([]dramaFailureSummary, count)
	for i := range rows {
		rows[i] = dramaFailureSummary{DramaID: fmt.Sprintf("%024x", i+1), IntID: fmt.Sprint(i + 1), Title: fmt.Sprintf("Drama %d", i+1), CNTitle: fmt.Sprintf("测试剧集 %d", i+1), Errors: []string{"• [版权规则] 版权已过期，copyright_expire_at=2026-01-01。"}}
	}
	if count > 0 {
		rows[0].Errors = append(rows[0].Errors, "• [分组规则] 分组错了，分到了 IAA 组", "• [简介语种] 识别语种=en，与 lang=pt 不一致。description=SECRET_FULL_DESCRIPTION <script>window.__pwned=true</script>；内容指纹=hash")
	}
	if err := writeJSONFile(filepath.Join(dir, "artifacts", "drama_failure_summary.json"), map[string]any{"failures": rows, "ruleScope": map[string]any{"version": "test"}}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(dir, "artifacts", "drama_initial_coverage.json"), dramaCoverageFile{Phase: "initial", Complete: true, ExpectedIDs: []string{"one", "two"}, CompletedIDs: []string{"one", "two"}, MissingIDs: []string{}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "artifacts", "report.html"), []byte("<html><body>ORIGINAL_HTML_NOT_SENT_TO_BROWSER</body></html>"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DRAMA_ISSUE_DISPOSITIONS_FILE", filepath.Join(root, "marks.json"))
	return root, runID, rows
}

func dramaReportTestRouter(root string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/test-runs/:runId/artifacts/report", func(c *gin.Context) { serveDramaReportPage(c, root, "all", "剧集播放接口测试报告") })
	router.GET("/api/test-runs/:runId/errors", func(c *gin.Context) {
		serveDramaReportPage(c, root, c.DefaultQuery("category", "rules"), "检测异常明细")
	})
	router.GET("/api/test-runs/:runId/language-errors", func(c *gin.Context) { serveDramaLanguageDetails(c, root) })
	router.GET("/api/test-runs/:runId/report-data", func(c *gin.Context) { serveDramaReportData(c, root) })
	router.GET("/api/test-runs/:runId/report-data/:issueId", func(c *gin.Context) { serveDramaReportIssue(c, root) })
	return router
}

func dramaReportRequest(router http.Handler, target string) *httptest.ResponseRecorder {
	r := httptest.NewRecorder()
	router.ServeHTTP(r, httptest.NewRequest(http.MethodGet, target, nil))
	return r
}

type dramaReportTestPage struct {
	Summary dramaReportSummary `json:"summary"`
	Items   []struct {
		ID      string   `json:"id"`
		DramaID string   `json:"dramaId"`
		Preview []string `json:"preview"`
		Action  string   `json:"action"`
		CanMark bool     `json:"canMark"`
	} `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Pages int `json:"pages"`
}

func decodeDramaReportPage(t *testing.T, r *httptest.ResponseRecorder) dramaReportTestPage {
	t.Helper()
	if r.Code != 200 {
		t.Fatalf("status %d: %s", r.Code, r.Body.String())
	}
	var page dramaReportTestPage
	if err := json.Unmarshal(r.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestDramaReportServerPaginationAndLazyDetails(t *testing.T) {
	root, id, rows := dramaReportFixture(t, 3000)
	router := dramaReportTestRouter(root)
	base := "/api/test-runs/" + id
	shell := dramaReportRequest(router, base+"/artifacts/report")
	if shell.Code != 200 || shell.Body.Len() > 28000 {
		t.Fatalf("unbounded report shell: %d bytes status %d", shell.Body.Len(), shell.Code)
	}
	for _, forbidden := range []string{"SECRET_FULL_DESCRIPTION", "ORIGINAL_HTML_NOT_SENT_TO_BROWSER", "测试剧集 2999"} {
		if strings.Contains(shell.Body.String(), forbidden) {
			t.Fatal("preloaded full evidence: " + forbidden)
		}
	}
	response := dramaReportRequest(router, base+"/report-data")
	first := decodeDramaReportPage(t, response)
	if first.Total != 3000 || len(first.Items) != 50 || first.Pages != 60 || first.Summary.Categories["copyright"] != 3000 || first.Summary.Categories["group"] != 1 || first.Summary.RuleTotal != 3000 {
		t.Fatalf("wrong page/counts: %+v", first.Summary)
	}
	if response.Body.Len() > 100000 || strings.Contains(response.Body.String(), "SECRET_FULL_DESCRIPTION") {
		t.Fatal("first page contains full descriptions or all records")
	}
	second := decodeDramaReportPage(t, dramaReportRequest(router, base+"/report-data?page=2"))
	if len(second.Items) != 50 || second.Items[0].DramaID == first.Items[0].DramaID {
		t.Fatal("pagination did not advance")
	}
	detail := dramaReportRequest(router, base+"/report-data/"+first.Items[0].ID)
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), "SECRET_FULL_DESCRIPTION") {
		t.Fatal("detail evidence missing")
	}
	if strings.Contains(detail.Body.String(), "<script>") {
		t.Fatal("unescaped script in JSON")
	}
	filtered := decodeDramaReportPage(t, dramaReportRequest(router, base+"/report-data?category=group&q="+rows[0].DramaID))
	if filtered.Total != 1 || len(filtered.Items) != 1 || len(filtered.Items[0].Preview) != 1 {
		t.Fatal("category/search filter mismatch")
	}
	for _, line := range filtered.Items[0].Preview {
		if !strings.Contains(line, "分组规则") {
			t.Fatal("unrelated error in category preview")
		}
	}
	for _, target := range []string{base + "/report-data?pageSize=101", base + "/report-data?page=0", base + "/report-data?category=bad", base + "/report-data?category=..", base + "/report-data/bad"} {
		if r := dramaReportRequest(router, target); r.Code != 400 {
			t.Fatalf("invalid query %s: %d", target, r.Code)
		}
	}
	unknown := dramaReportRequest(router, base+"/report-data/"+strings.Repeat("f", 64))
	if unknown.Code != 404 {
		t.Fatal("unknown issue accepted")
	}
	language := dramaReportRequest(router, base+"/language-errors")
	if language.Code != 200 || strings.Contains(language.Body.String(), "SECRET_FULL_DESCRIPTION") || !strings.Contains(language.Body.String(), `data-category="language"`) {
		t.Fatal("language page did not use dynamic shell")
	}
}

func TestDramaReportFreshMarksAndExactWholeIssueIdentity(t *testing.T) {
	root, id, rows := dramaReportFixture(t, 2)
	router := dramaReportTestRouter(root)
	base := "/api/test-runs/" + id + "/report-data"
	first := decodeDramaReportPage(t, dramaReportRequest(router, base+"?category=group"))
	signature := first.Items[0].ID
	if signature != dramaIssueSignature(rows[0].DramaID, rows[0].Errors) {
		t.Fatal("filtered view changed the whole-issue signature")
	}
	if err := saveDramaIssueDisposition(dramaIssueDisposition{Signature: signature, DramaID: rows[0].DramaID, Errors: rows[0].Errors, Action: dramaIssueActionTemporarilyIgnored}); err != nil {
		t.Fatal(err)
	}
	second := decodeDramaReportPage(t, dramaReportRequest(router, base+"?category=copyright"))
	if second.Items[0].Action != dramaIssueActionTemporarilyIgnored || second.Total != 2 {
		t.Fatal("cached view lost fresh mark or hid ignored records")
	}
	changed := append([]string{}, rows[0].Errors...)
	changed[1] = "• [分组规则] 分组错了，分到了 IPA 组"
	if dramaIssueSignature(rows[0].DramaID, changed) == signature {
		t.Fatal("different detail matched ignore")
	}
}

func TestDramaReportCacheRefreshAndCorruptArchive(t *testing.T) {
	root, id, rows := dramaReportFixture(t, 2)
	router := dramaReportTestRouter(root)
	base := "/api/test-runs/" + id + "/report-data"
	decodeDramaReportPage(t, dramaReportRequest(router, base))
	file := filepath.Join(testRunStorageRoot(root), id, "artifacts", "drama_failure_summary.json")
	rows[0].CNTitle = "更新后的归档名称"
	rows = append(rows, dramaFailureSummary{DramaID: strings.Repeat("e", 24), Errors: []string{"[出错] 新异常"}})
	if err := writeJSONFile(file, map[string]any{"failures": rows}); err != nil {
		t.Fatal(err)
	}
	if page := decodeDramaReportPage(t, dramaReportRequest(router, base)); page.Total != 3 {
		t.Fatal("stale archive cache")
	}
	if err := os.WriteFile(file, []byte("broken JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	if response := dramaReportRequest(router, base); response.Code != 500 {
		t.Fatalf("corrupt data must not look like zero findings: %d", response.Code)
	}
	if response := dramaReportRequest(router, "/api/test-runs/bad..id/report-data"); response.Code != 400 {
		t.Fatal("unsafe report ID accepted")
	}
}

func TestDramaReportReviewEvidenceRemainsLazyAndReadOnly(t *testing.T) {
	root, id, _ := dramaReportFixture(t, 1)
	dir := filepath.Join(testRunStorageRoot(root), id, "artifacts")
	if err := writeJSONFile(filepath.Join(dir, "drama_initial_language_review.json"), map[string]any{"rows": []dramaLanguageReviewRecord{{DramaID: "uncertain", CNTitle: "待复核", Status: "uncertain", Description: "PRIVATE_REVIEW_EVIDENCE"}, {DramaID: "recovered", Status: "mixed", Description: "OLD_REVIEW"}}}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(dir, "drama_retry_language_review.json"), map[string]any{"checkedIds": []string{"recovered"}, "rows": []dramaLanguageReviewRecord{}}); err != nil {
		t.Fatal(err)
	}
	router := dramaReportTestRouter(root)
	base := "/api/test-runs/" + id + "/report-data"
	all := decodeDramaReportPage(t, dramaReportRequest(router, base))
	if all.Total != 1 || all.Summary.ReviewCount != 1 {
		t.Fatal("review counted as business error or recovered review retained")
	}
	response := dramaReportRequest(router, base+"?category=review")
	review := decodeDramaReportPage(t, response)
	if review.Total != 1 || review.Items[0].CanMark || strings.Contains(response.Body.String(), "PRIVATE_REVIEW_EVIDENCE") {
		t.Fatal("review loaded full evidence or allows marking")
	}
	detail := dramaReportRequest(router, base+"/"+review.Items[0].ID)
	if !strings.Contains(detail.Body.String(), "PRIVATE_REVIEW_EVIDENCE") {
		t.Fatal("review evidence lost")
	}
}

// Opt-in local browser fixture. All data and marking writes stay in t.TempDir.
func TestDramaReportBrowserPreview(t *testing.T) {
	output := os.Getenv("TESTCENTER_REPORT_PREVIEW_DIR")
	if output == "" {
		t.Skip("browser preview only")
	}
	root, id, _ := dramaReportFixture(t, 3000)
	router := dramaReportTestRouter(root)
	router.PUT("/api/test-runs/:runId/issue-dispositions", func(c *gin.Context) {
		var request saveDramaIssueDispositionRequest
		if c.ShouldBindJSON(&request) != nil {
			c.Status(400)
			return
		}
		signature := dramaIssueSignature(request.DramaID, request.Errors)
		if err := saveDramaIssueDisposition(dramaIssueDisposition{Signature: signature, DramaID: request.DramaID, Errors: request.Errors, Action: request.Action}); err != nil {
			c.Status(500)
			return
		}
		c.JSON(200, gin.H{"action": request.Action})
	})
	server := httptest.NewServer(router)
	defer server.Close()
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(output, "preview-server.json"), map[string]string{"url": server.URL + "/api/test-runs/" + id + "/artifacts/report"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(output, "preview-stop")); err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("preview not stopped")
}

func TestDramaReportExistingArchiveReadOnly(t *testing.T) {
	root, runID := os.Getenv("TESTCENTER_REPORT_ARCHIVE_ROOT"), os.Getenv("TESTCENTER_REPORT_ARCHIVE_RUN")
	if root == "" || runID == "" {
		t.Skip("opt-in existing archive verification")
	}
	t.Setenv("DRAMA_ISSUE_DISPOSITIONS_FILE", filepath.Join(t.TempDir(), "marks.json"))
	start := time.Now()
	router := dramaReportTestRouter(root)
	base := "/api/test-runs/" + runID + "/report-data"
	response := dramaReportRequest(router, base)
	first := decodeDramaReportPage(t, response)
	cold := time.Since(start)
	if len(first.Items) > 50 || response.Body.Len() > 100000 {
		t.Fatal("unbounded initial response")
	}
	seen := map[string]bool{}
	for page := 1; page <= first.Pages; page++ {
		data := decodeDramaReportPage(t, dramaReportRequest(router, fmt.Sprintf("%s?page=%d", base, page)))
		for _, row := range data.Items {
			if seen[row.DramaID] {
				t.Fatal("duplicate page record")
			}
			seen[row.DramaID] = true
		}
	}
	if len(seen) != first.Total {
		t.Fatal("pagination omitted archive findings")
	}
	snapshot, err := loadDramaReportSnapshot(root, runID)
	if err != nil {
		t.Fatal(err)
	}
	failures := []dramaFailureSummary{}
	for _, row := range snapshot.Rows {
		if row.CanMark {
			failures = append(failures, row.Failure)
		}
	}
	play, groups, classification, other := splitScheduledDramaFailures(failures)
	card := buildScheduledTaskFeishuCard(ScheduledTask{}, "通过", "1m", "http://example.test/api/test-runs/"+runID+"/artifacts/report", "归档只读校验", play, groups, classification, other)
	b, _ := json.Marshal(card)
	if first.Summary.Categories["copyright"] > 30 && strings.Contains(string(b), "copyright_expire_at=") {
		t.Fatal("copyright detail embedded above threshold")
	}
	t.Logf("archive verified: %d unique dramas, copyright %d, initial payload %d bytes, cold API %s, Feishu pages %d (not sent)", first.Total, first.Summary.Categories["copyright"], response.Body.Len(), cold, len(splitScheduledFeishuCards(card)))
}
