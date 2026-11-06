package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestDramaFeishuRuleThresholdAndPlaybackPreserved(t *testing.T) {
	for _, count := range []int{0, 29, 30, 31, 2657} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			rows := make([]dramaFailureSummary, 0, count+3)
			for i := 0; i < count; i++ {
				tag := []string{"[分组规则]", "[分类规则]", "[简介语种]", "[版权规则]"}[i%4]
				rows = append(rows, dramaFailureSummary{DramaID: fmt.Sprintf("%024x", i+1), Title: "RULE_DETAIL_TITLE", Errors: []string{tag + " RULE_DETAIL_MARKER"}})
			}
			rows = append(rows, dramaFailureSummary{DramaID: strings.Repeat("a", 24), Errors: []string{"[出错] PLAYBACK_KEEP_ALL_DETAILS", "[直接跳集] PLAYBACK_SECOND_ERROR"}}, dramaFailureSummary{DramaID: strings.Repeat("b", 24), Errors: []string{"[免费集数] FREE_KEEP_DETAILS"}}, dramaFailureSummary{DramaID: strings.Repeat("c", 24), Errors: []string{"[命名规范] OTHER_KEEP_DETAILS"}})
			play, groups, classification, other := splitScheduledDramaFailures(rows)
			card := buildScheduledTaskFeishuCard(ScheduledTask{}, "通过", "1m", "http://example.test/api/test-runs/run-test/artifacts/report", "summary", play, groups, classification, other)
			b, _ := json.Marshal(card)
			text := string(b)
			for _, want := range []string{"PLAYBACK_KEEP_ALL_DETAILS", "PLAYBACK_SECOND_ERROR", "FREE_KEEP_DETAILS", "OTHER_KEEP_DETAILS"} {
				if !strings.Contains(text, want) {
					t.Fatal("lost unaffected detail: " + want)
				}
			}
			compact := count > 30
			if strings.Contains(text, "规则异常摘要") != compact {
				t.Fatalf("wrong threshold for %d", count)
			}
			if strings.Contains(text, "RULE_DETAIL_MARKER") != (count > 0 && !compact) {
				t.Fatal("wrong detail visibility")
			}
			if compact && !strings.Contains(text, "/errors?category=rules") {
				t.Fatal("missing complete dynamic error link")
			}
			if compact && scheduledCardEnvelopeSize(card) > 12000 {
				t.Fatal("summary still contains too much data")
			}
			if strings.Contains(text, "RULE_DETAIL_TITLE") && compact {
				t.Fatal("per-drama title leaked into summary")
			}
			for i := 0; i < count; i++ {
				if !strings.Contains(rows[i].Errors[0], "RULE_DETAIL_MARKER") {
					t.Fatal("source findings mutated")
				}
			}
		})
	}
}

func TestDramaFeishuRuleCountIsSumOfCategoryEntries(t *testing.T) {
	rows := make([]dramaFailureSummary, 8)
	for i := range rows {
		rows[i] = dramaFailureSummary{DramaID: fmt.Sprintf("%024x", i+1), Errors: []string{"[分组规则] G", "[分类规则] C", "[简介语种] L", "[版权规则] R"}}
	}
	_, g, c, other := splitScheduledDramaFailures(rows)
	l, other := splitDramaLanguageFailures(other)
	r, _ := splitDramaCopyrightFailures(other)
	if dramaRuleCardCount(g, c, l, r) != 32 {
		t.Fatal("cross-category entries should count separately")
	}
	b, _ := json.Marshal(buildScheduledTaskFeishuCard(ScheduledTask{}, "通过", "1m", "http://example.test/api/test-runs/run-test/artifacts/report", "summary", nil, g, c, append(l, r...)))
	if !strings.Contains(string(b), "共 32 条分类明细") {
		t.Fatal("combined threshold was not applied")
	}
}

func TestDramaFeishuPlaybackOnlyDoesNotTriggerRuleSummary(t *testing.T) {
	play := make([]dramaFailureSummary, 40)
	for i := range play {
		play[i] = dramaFailureSummary{DramaID: fmt.Sprintf("%024x", i+1), Errors: []string{fmt.Sprintf("[出错] PLAYBACK_%02d", i)}}
	}
	card := buildScheduledTaskFeishuCard(ScheduledTask{}, "通过", "1m", "http://example.test/api/test-runs/run-test/artifacts/report", "summary", play, nil, nil, nil)
	b, _ := json.Marshal(card)
	if strings.Contains(string(b), "规则异常摘要") {
		t.Fatal("playback must not be collapsed")
	}
	for i := 0; i < 40; i++ {
		if !strings.Contains(string(b), fmt.Sprintf("PLAYBACK_%02d", i)) {
			t.Fatal("playback detail lost")
		}
	}
}

func TestDramaFeishuOversizedPlaybackIsSplitWithoutLosingText(t *testing.T) {
	content := strings.Repeat("播放错误<&>😀\\\"\n", 7000) + "[查看剧集信息](https://example.test/drama?id=123)"
	card := map[string]any{"header": map[string]any{"title": map[string]any{"content": "播放检查"}}, "elements": []map[string]any{{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": content}}}}
	pages := splitScheduledFeishuCards(card)
	if len(pages) < 2 {
		t.Fatal("long playback needs multiple cards")
	}
	var joined strings.Builder
	for _, page := range pages {
		if scheduledCardEnvelopeSize(page) > 24*1024 {
			t.Fatal("message over budget")
		}
		for _, e := range page["elements"].([]map[string]any) {
			if text, ok := e["text"].(map[string]any); ok {
				joined.WriteString(text["content"].(string))
			}
		}
	}
	if joined.String() != content {
		t.Fatal("playback was truncated, reordered or mutated")
	}
}
