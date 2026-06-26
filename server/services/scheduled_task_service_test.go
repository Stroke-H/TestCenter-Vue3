package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSplitScheduledDramaFailures(t *testing.T) {
	failures := []dramaFailureSummary{
		{
			DramaID: "69e1e7e163f7234def41a948",
			IntID:   "15505",
			Title:   "Universitaria del Lobo",
			CNTitle: "绝嗣狼王的命定小娇妻",
			Errors: []string{
				"• [出错] 720p/540p 均无法提供健康章节Index: 1",
				"• [解锁类型] unlock_type=ad，但 App Group 命中 ShortsWave",
			},
		},
		{
			DramaID: "6985803d8671d0853028b57b",
			Errors:  []string{"• [解锁类型] unlock_type=coin，但 cn_name 或 App Group 命中 IAA"},
		},
	}

	dramaFailures, groupFailures := splitScheduledDramaFailures(failures)
	if len(dramaFailures) != 1 {
		t.Fatalf("expected 1 drama failure, got %d", len(dramaFailures))
	}
	if len(groupFailures) != 2 {
		t.Fatalf("expected 2 group failures, got %d", len(groupFailures))
	}
	if len(dramaFailures[0].Errors) != 1 || strings.Contains(dramaFailures[0].Errors[0], "[解锁类型]") {
		t.Fatalf("unexpected drama failure errors: %#v", dramaFailures[0].Errors)
	}
	if len(groupFailures[0].Errors) != 1 || !strings.Contains(groupFailures[0].Errors[0], "[解锁类型]") {
		t.Fatalf("unexpected group failure errors: %#v", groupFailures[0].Errors)
	}
	if groupFailures[0].Title != failures[0].Title || groupFailures[0].CNTitle != failures[0].CNTitle {
		t.Fatalf("expected drama metadata to be preserved: %#v", groupFailures[0])
	}
}

func TestBuildScheduledTaskFeishuCardSeparatesGroupFailures(t *testing.T) {
	task := ScheduledTask{Name: "剧集播放接口测试", TestProject: "ShortsWave", TestEnv: "prod"}
	dramaFailures := []dramaFailureSummary{{
		DramaID: "69e1e7e163f7234def41a948",
		Errors:  []string{"• [出错] 章节异常"},
	}}
	groupFailures := []dramaFailureSummary{{
		DramaID: "6985803d8671d0853028b57b",
		Errors:  []string{"• [解锁类型] App Group 异常"},
	}}

	card := buildScheduledTaskFeishuCard(task, "失败", "1m", "", "测试总结", dramaFailures, groupFailures)
	content, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}
	text := string(content)
	dramaIndex := strings.Index(text, "异常剧集明细")
	groupIndex := strings.Index(text, "分组异常")
	if dramaIndex < 0 || groupIndex < 0 {
		t.Fatalf("expected both report sections, got %s", text)
	}
	if groupIndex <= dramaIndex {
		t.Fatalf("expected group section after drama section, got %s", text)
	}
}

func TestParseDramaFailuresFromReportHTML(t *testing.T) {
	html := `<details><summary><b>Love Potions/CnName:妻子的爱情咒语<br>ID:6a2120fa1914a76d05ba4053(18239)</b></summary><div>• <span class="drama-error-keyword">[解锁类型]</span> unlock_type=ad，但 App Group 命中 ShortsWave<br>• <span class="drama-error-keyword">[出错]</span> 720p异常</div></details>`

	failures := parseDramaFailuresFromReportHTML(html)
	if len(failures) != 1 {
		t.Fatalf("expected 1 parsed failure, got %d", len(failures))
	}
	failure := failures[0]
	if failure.DramaID != "6a2120fa1914a76d05ba4053" || failure.IntID != "18239" {
		t.Fatalf("unexpected parsed ids: %#v", failure)
	}
	if failure.Title != "Love Potions" || failure.CNTitle != "妻子的爱情咒语" {
		t.Fatalf("unexpected parsed titles: %#v", failure)
	}
	dramaFailures, groupFailures := splitScheduledDramaFailures(failures)
	if len(dramaFailures) != 1 || len(groupFailures) != 1 {
		t.Fatalf("expected parsed errors to split into drama/group, got drama=%d group=%d", len(dramaFailures), len(groupFailures))
	}
}

func TestDramaArchiveDoneStatusIsSuccess(t *testing.T) {
	if !isDramaArchiveSuccessStatus("done") {
		t.Fatal("expected done archive status to be treated as success")
	}
	if isDramaArchiveSuccessStatus("failed") {
		t.Fatal("expected failed archive status to be treated as failure")
	}
}
