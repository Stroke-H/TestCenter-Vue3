package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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
		{
			DramaID: "6a61c109e124f7d6d95cbeca",
			Errors:  []string{"• [分组规则] cn_name 包含 -DS 时，App Group 必须且只能为 TT-Minis 分销组"},
		},
	}

	dramaFailures, groupFailures := splitScheduledDramaFailures(failures)
	if len(dramaFailures) != 1 {
		t.Fatalf("expected 1 drama failure, got %d", len(dramaFailures))
	}
	if len(groupFailures) != 3 {
		t.Fatalf("expected 3 group failures, got %d", len(groupFailures))
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
	if len(groupFailures[2].Errors) != 1 || !strings.Contains(groupFailures[2].Errors[0], "[分组规则]") {
		t.Fatalf("expected DS rule error in group failures: %#v", groupFailures[2].Errors)
	}
}

func TestSummarizeScheduledDramaAuditFailures(t *testing.T) {
	failures := []dramaFailureSummary{
		{
			DramaID: "drama-1",
			Errors: []string{
				"• [出错] 章节异常",
				"• [解锁类型] unlock_type=ad，但 App Group 命中 ShortsWave",
			},
		},
		{
			DramaID: "drama-2",
			Errors:  []string{"• [转换中] 章节尚未完成转换"},
		},
		{
			DramaID: "drama-3",
			Errors:  []string{"• [分组规则] cn_name 包含 -DS"},
		},
	}

	if got := summarizeScheduledDramaAuditFailures(failures); got != "剧集错误 2、分组错误 2" {
		t.Fatalf("unexpected audit summary: %s", got)
	}
	if got := summarizeScheduledDramaAuditFailures(nil); got != "无异常" {
		t.Fatalf("unexpected empty audit summary: %s", got)
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

func TestRescheduleScheduledTaskAfterRunMovesDailyTaskToFuture(t *testing.T) {
	tasks := []ScheduledTask{{
		ID:           "ST-daily",
		ScheduleType: "Daily",
		Status:       "running",
		NextRunAt:    "2026-06-27T09:00:09+08:00",
	}}
	now := time.Date(2026, 6, 30, 18, 0, 0, 0, time.FixedZone("CST", 8*60*60))

	rescheduleScheduledTaskAfterRun(&tasks, 0, now)

	if len(tasks) != 1 {
		t.Fatalf("expected daily task to remain, got %d", len(tasks))
	}
	if tasks[0].Status != "active" {
		t.Fatalf("expected daily task to become active, got %s", tasks[0].Status)
	}
	if tasks[0].NextRunAt != "2026-07-01T09:00:09+08:00" {
		t.Fatalf("expected next run to move to next future day, got %s", tasks[0].NextRunAt)
	}
	if tasks[0].NextRun != "2026-07-01 09:00:09" {
		t.Fatalf("expected display next run to be updated, got %s", tasks[0].NextRun)
	}
}

func TestRescheduleScheduledTaskAfterRunRemovesOnceTask(t *testing.T) {
	tasks := []ScheduledTask{{
		ID:           "ST-once",
		ScheduleType: "Once",
		Status:       "running",
		NextRunAt:    "2026-06-30T09:00:00+08:00",
	}}

	rescheduleScheduledTaskAfterRun(&tasks, 0, time.Date(2026, 6, 30, 18, 0, 0, 0, time.FixedZone("CST", 8*60*60)))

	if len(tasks) != 0 {
		t.Fatalf("expected once task to be removed, got %#v", tasks)
	}
}
