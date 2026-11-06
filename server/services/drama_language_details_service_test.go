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

	"github.com/gin-gonic/gin"
)

func TestLanguageDetailsPublicSnapshotAndEscaping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	runDir := filepath.Join(testRunStorageRoot(root), "run-test")
	if err := os.MkdirAll(filepath.Join(runDir, "artifacts"), 0700); err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(TestRunArchive{RunID: "run-test", TestType: "drama", StartedAt: "2026/10/27 09:00:00", Duration: "00:12:00"})
	if err := os.WriteFile(filepath.Join(runDir, "metadata.json"), metadata, 0600); err != nil {
		t.Fatal(err)
	}
	failures := []dramaFailureSummary{{DramaID: "111111111111111111111111", Title: "Example", CNTitle: "示例", Errors: []string{
		"• [简介语种] 识别语种=en，与 lang=pt 不一致。description=<script>alert(1)</script> full description；内容指纹=abc",
		"• [命名规范] NOT_A_LANGUAGE_ROW",
	}}}
	data, _ := json.Marshal(map[string]interface{}{"failures": failures})
	if err := os.WriteFile(filepath.Join(runDir, "artifacts", "drama_failure_summary.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/api/test-runs/:runId/language-errors", func(c *gin.Context) { serveDramaLanguageDetails(c, root) })
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/test-runs/run-test/language-errors?view=original", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("public request returned %d: %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, expected := range []string{"语种异常明细", "full description", "配置 lang：pt", "识别语种：en", "共 1 条语种异常"} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	if strings.Contains(body, "<script>alert(1)</script>") || strings.Contains(body, "NOT_A_LANGUAGE_ROW") {
		t.Fatal("untrusted markup or unrelated error leaked")
	}
	for path, want := range map[string]int{"missing": http.StatusNotFound, "bad..id": http.StatusBadRequest} {
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/test-runs/"+path+"/language-errors", nil))
		if r.Code != want {
			t.Errorf("%s: %d", path, r.Code)
		}
	}
}

func TestLanguageCardCompactAndLinked(t *testing.T) {
	line := "• [简介语种] 识别语种=en，与 lang=pt 不一致。description=SECRET_FULL_DESCRIPTION；内容指纹=abc"
	failure := dramaFailureSummary{DramaID: "111111111111111111111111", Title: "Example", Errors: []string{line, "• [命名规范] Example issue"}}
	reportURL := "http://example.test:8080/api/test-runs/run-test/artifacts/report"
	card := buildScheduledTaskFeishuCard(ScheduledTask{}, "通过", "00:12:00", reportURL, "summary", nil, nil, nil, []dramaFailureSummary{failure})
	encoded, _ := json.Marshal(card)
	body := string(encoded)
	for _, expected := range []string{"语种异常（1 部）", "查看异常明细", "/api/test-runs/run-test/language-errors", "识别语种=en，与 lang=pt 不一致。", "其他异常（1 部）"} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	if strings.Contains(body, "SECRET_FULL_DESCRIPTION") || strings.Contains(body, "内容指纹") {
		t.Fatal("full description leaked into card")
	}
	if failure.Errors[0] != line {
		t.Fatal("source failure mutated before ignore matching")
	}
	rows := languageDetailRows([]dramaFailureSummary{failure})
	if len(rows) != 1 || rows[0].Expected != "pt" || rows[0].Detected != "en" || rows[0].Description != "SECRET_FULL_DESCRIPTION" {
		t.Fatalf("bad detail: %+v", rows)
	}
}

func TestLargeLanguageCardsStayUnderMessageBudget(t *testing.T) {
	failures := make([]dramaFailureSummary, 170)
	for i := range failures {
		failures[i] = dramaFailureSummary{DramaID: fmt.Sprintf("%024d", i), IntID: fmt.Sprint(i), Title: "示例剧集", CNTitle: "示例中文剧名", Errors: []string{
			"• [简介语种] 识别语种=en，与 lang=pt 不一致。description=" + strings.Repeat("很长的简介", 300) + "；内容指纹=abc",
		}}
	}
	card := buildScheduledTaskFeishuCard(ScheduledTask{}, "通过", "00:12:00", "http://example.test/api/test-runs/run-test/artifacts/report", "summary", nil, nil, nil, failures)
	pages := splitScheduledFeishuCards(card)
	if len(pages) != 1 {
		t.Fatal("large language report should be summarized into one card")
	}
	rows := 0
	for _, page := range pages {
		if size := scheduledCardEnvelopeSize(page); size > 30*1024 {
			t.Fatalf("message too large: %d", size)
		}
		for _, element := range page["elements"].([]map[string]any) {
			encoded, _ := json.Marshal(element)
			if strings.Contains(string(encoded), "[简介语种]") {
				rows++
			}
		}
	}
	if rows != 0 {
		t.Fatalf("large language detail should be linked, not embedded: %d", rows)
	}
	encoded, _ := json.Marshal(card)
	if !strings.Contains(string(encoded), "语种异常（170 部）") || !strings.Contains(string(encoded), "/errors?category=rules") {
		t.Fatal("missing count or dynamic details link")
	}
}
