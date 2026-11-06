package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentFiveNoLongerSkipsLanguagePreparation(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "server", "data"), 0700)
	path := filepath.Join(root, "drama_info.json")
	meta := map[string]interface{}{"cn_name": "示例(内容五部媒资库)", "description_available": false, "lang_available": false}
	payload := map[string]interface{}{"dramaList": []string{"one"}, "dramaMeta": map[string]interface{}{"one": meta}}
	write := func() {
		b, _ := json.Marshal(payload)
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if err := prepareDramaDescriptionLanguages(context.Background(), root, nil); err == nil {
		t.Fatal("content-five metadata failure must not be skipped")
	}
	meta["description_available"], meta["lang_available"] = true, true
	meta["description"], meta["lang"] = "/", "en"
	write()
	if err := prepareDramaDescriptionLanguages(context.Background(), root, func(string) {}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), `skipped_policy`) || !strings.Contains(string(b), `"status":"skipped"`) {
		t.Fatal("expected normal empty-description policy, no content-five exemption")
	}
}

func TestCopyrightReportCategoryAndAudit(t *testing.T) {
	f := dramaFailureSummary{DramaID: "111111111111111111111111", Errors: []string{"• [版权规则] 截止时间已过期", "• [免费集数] 应为2"}}
	p, g, c, other := splitScheduledDramaFailures([]dramaFailureSummary{f})
	if len(p)+len(g)+len(c) != 0 || len(other) != 1 {
		t.Fatal("wrong initial category")
	}
	card := buildScheduledTaskFeishuCard(ScheduledTask{}, "通过", "1m", "", "", p, g, c, other)
	b, _ := json.Marshal(card)
	for _, want := range []string{"版权异常（1 部）", "免费集数异常（1 部）", "orange"} {
		if !strings.Contains(string(b), want) {
			t.Fatal("missing " + want)
		}
	}
	if !strings.Contains(summarizeScheduledDramaAuditFailures([]dramaFailureSummary{f}), "版权错误 1") {
		t.Fatal("missing audit")
	}
}

func TestFreeChaptersReportCategoryAndAudit(t *testing.T) {
	f := dramaFailureSummary{DramaID: "111111111111111111111111", Errors: []string{"• [免费集数] free_chapters=3，应为2。", "• [简介语种] 识别语种=en，与 lang=pt 不一致。", "• [命名规范] 示例"}}
	p, g, c, other := splitScheduledDramaFailures([]dramaFailureSummary{f})
	if len(p)+len(g)+len(c) != 0 || len(other) != 1 {
		t.Fatal("wrong initial grouping")
	}
	card := buildScheduledTaskFeishuCard(ScheduledTask{}, "通过", "1m", "", "", p, g, c, other)
	b, _ := json.Marshal(card)
	for _, want := range []string{"免费集数异常（1 部）", "语种异常（1 部）", "其他异常（1 部）"} {
		if !strings.Contains(string(b), want) {
			t.Fatal("missing category " + want)
		}
	}
	if !strings.Contains(summarizeScheduledDramaAuditFailures([]dramaFailureSummary{f}), "免费集数错误 1") {
		t.Fatal("missing audit category")
	}
}

func TestFreeChaptersCardsStayWithinBudget(t *testing.T) {
	failures := make([]dramaFailureSummary, 170)
	for i := range failures {
		failures[i] = dramaFailureSummary{DramaID: fmt.Sprintf("%024d", i), Title: "示例剧集", Errors: []string{"• [免费集数] unlock_type=ad 要求free_chapters=2，实际=3。"}}
	}
	pages := splitScheduledFeishuCards(buildScheduledTaskFeishuCard(ScheduledTask{}, "通过", "1m", "", "summary", nil, nil, nil, failures))
	rows := 0
	for _, page := range pages {
		if scheduledCardEnvelopeSize(page) > 30*1024 {
			t.Fatal("free chapter card exceeds message budget")
		}
		for _, el := range page["elements"].([]map[string]any) {
			b, _ := json.Marshal(el)
			if strings.Contains(string(b), "[免费集数]") {
				rows++
			}
		}
	}
	if rows != len(failures) {
		t.Fatalf("lost rows: %d", rows)
	}
}

func TestDramaCollectionRuleArtifactsAreArchived(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	for _, name := range []string{"drama_initial_rules.json", "drama_retry_rules.json"} {
		content := []byte(`{"ready":true,"failures":[]}`)
		if err := os.WriteFile(filepath.Join(source, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	copyDramaRunDiagnosticArtifacts(source, destination)
	for _, name := range []string{"drama_initial_rules.json", "drama_retry_rules.json"} {
		b, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil || !json.Valid(b) {
			t.Fatalf("missing or invalid collection rules %s: %v", name, err)
		}
	}
}
