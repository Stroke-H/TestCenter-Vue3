package services

import (
	"encoding/json"
	"net/url"
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
				"• [分类规则] cn_name 包含西配，但 marketing_position 未命中配音剧",
				"• [命名规范] App Group 中 IAA 大小写不规范，应使用 IAA",
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

	dramaFailures, groupFailures, classificationFailures, otherFailures := splitScheduledDramaFailures(failures)
	if len(dramaFailures) != 1 {
		t.Fatalf("expected 1 drama failure, got %d", len(dramaFailures))
	}
	if len(groupFailures) != 3 {
		t.Fatalf("expected 3 group failures, got %d", len(groupFailures))
	}
	if len(otherFailures) != 1 {
		t.Fatalf("expected 1 other failure, got %d", len(otherFailures))
	}
	if len(classificationFailures) != 1 {
		t.Fatalf("expected 1 classification failure, got %d", len(classificationFailures))
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
	if len(otherFailures[0].Errors) != 1 || !strings.Contains(otherFailures[0].Errors[0], "[命名规范]") {
		t.Fatalf("expected naming error in other failures: %#v", otherFailures[0].Errors)
	}
	if len(classificationFailures[0].Errors) != 1 || !strings.Contains(classificationFailures[0].Errors[0], "[分类规则]") {
		t.Fatalf("expected classification error in classification failures: %#v", classificationFailures[0].Errors)
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
		{
			DramaID: "drama-4",
			Errors:  []string{"• [命名规范] IAA 大小写不规范"},
		},
		{
			DramaID: "drama-5",
			Errors:  []string{"• [分类规则] 配音剧分类与 cn_name 不一致"},
		},
	}

	if got := summarizeScheduledDramaAuditFailures(failures); got != "剧集错误 2、分组错误 2、分类错误 1、其他错误 1" {
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
	classificationFailures := []dramaFailureSummary{{
		DramaID: "69e1e7e163f7234def41a949",
		Errors:  []string{"• [分类规则] 配音剧分类与 cn_name 不一致"},
	}}
	otherFailures := []dramaFailureSummary{{
		DramaID: "6a61c109e124f7d6d95cbeca",
		Errors:  []string{"• [命名规范] IAA 大小写不规范"},
	}}

	card := buildScheduledTaskFeishuCard(task, "失败", "1m", "", "测试总结", dramaFailures, groupFailures, classificationFailures, otherFailures)
	content, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}
	text := string(content)
	dramaIndex := strings.Index(text, "异常剧集明细")
	groupIndex := strings.Index(text, "分组异常")
	classificationIndex := strings.Index(text, "分类异常")
	otherIndex := strings.Index(text, "其他异常")
	if dramaIndex < 0 || groupIndex < 0 || classificationIndex < 0 || otherIndex < 0 {
		t.Fatalf("expected all report sections, got %s", text)
	}
	if groupIndex <= dramaIndex {
		t.Fatalf("expected group section after drama section, got %s", text)
	}
	if classificationIndex <= groupIndex {
		t.Fatalf("expected classification section after group section, got %s", text)
	}
	if otherIndex <= classificationIndex {
		t.Fatalf("expected other section after classification section, got %s", text)
	}
}

func TestBuildDramaManagementURL(t *testing.T) {
	t.Setenv("DRAMA_MANAGEMENT_LIST_URL", "https://admin.shortswave.com/drama/list")
	const dramaID = "6a2f781334dc26d5fae7c40a"

	link := buildDramaManagementURL(dramaID)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse drama management URL: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "admin.shortswave.com" || parsed.Path != "/drama/list" {
		t.Fatalf("unexpected drama management URL: %s", link)
	}
	var target map[string]string
	if err := json.Unmarshal([]byte(parsed.Query().Get("target")), &target); err != nil {
		t.Fatalf("parse target query: %v", err)
	}
	if target["id"] != dramaID {
		t.Fatalf("expected target id %s, got %#v", dramaID, target)
	}
}

func TestFormatDramaFailureCardBlockAddsLinkToEveryError(t *testing.T) {
	t.Setenv("DRAMA_MANAGEMENT_LIST_URL", "https://admin.shortswave.com/drama/list")
	failure := dramaFailureSummary{
		DramaID: "6a2f781334dc26d5fae7c40a",
		IntID:   "18801",
		Title:   "千金归来,她亲爹是超绝女儿奴(AI葡配)(IAA)",
		CNTitle: "A herdeira volta, pai super fã (PT)",
		Errors: []string{
			"• [分类规则] marketing_position=配音剧，但 cn_name 未包含 X配 标识",
			"• [出错] 章节播放异常",
		},
	}

	block := formatDramaFailureCardBlock(failure)
	if got := strings.Count(block, "[查看剧集信息]("); got != len(failure.Errors) {
		t.Fatalf("expected one drama link per error, got %d in %s", got, block)
	}
	link := buildDramaManagementURL(failure.DramaID)
	if got := strings.Count(block, link); got != len(failure.Errors) {
		t.Fatalf("expected encoded drama URL on every error, got %d in %s", got, block)
	}
}

func TestFormatDramaFailureCardBlockSkipsLinkWithoutDramaID(t *testing.T) {
	block := formatDramaFailureCardBlock(dramaFailureSummary{Errors: []string{"• [出错] 缺少剧集 ID"}})
	if strings.Contains(block, "查看剧集信息") {
		t.Fatalf("expected no management link without drama id, got %s", block)
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
	dramaFailures, groupFailures, classificationFailures, otherFailures := splitScheduledDramaFailures(failures)
	if len(dramaFailures) != 1 || len(groupFailures) != 1 || len(classificationFailures) != 0 || len(otherFailures) != 0 {
		t.Fatalf("expected parsed errors to split into drama/group/classification/other, got drama=%d group=%d classification=%d other=%d", len(dramaFailures), len(groupFailures), len(classificationFailures), len(otherFailures))
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

func TestPrepareAbandonedScheduledTaskForRetryPreservesOverdueRun(t *testing.T) {
	task := ScheduledTask{
		ID:           "ST-abandoned",
		ScheduleType: "Daily",
		Status:       "running",
		NextRun:      "2026-08-08 09:00:09",
		NextRunAt:    "2026-08-08T09:00:09+08:00",
	}
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))

	prepareAbandonedScheduledTaskForRetry(&task, now)

	if task.Status != "active" {
		t.Fatalf("expected abandoned task to become active, got %s", task.Status)
	}
	if task.NextRunAt != "2026-08-08T09:00:09+08:00" {
		t.Fatalf("expected overdue run to be preserved for one catch-up execution, got %s", task.NextRunAt)
	}
	if !strings.Contains(task.LastResult, "自动补跑") {
		t.Fatalf("expected recovery result to explain catch-up, got %s", task.LastResult)
	}
}

func TestPrepareAbandonedScheduledTaskForRetryPausesInvalidSchedule(t *testing.T) {
	task := ScheduledTask{
		ScheduleType: "Daily",
		Status:       "running",
		NextRunAt:    "not-a-time",
	}

	prepareAbandonedScheduledTaskForRetry(&task, time.Now())

	if task.Status != "paused" {
		t.Fatalf("expected invalid abandoned task to be paused, got %s", task.Status)
	}
}

func TestCompactScheduledTaskResultFitsLegacyColumn(t *testing.T) {
	result := "failed after 4 attempt(s)\nAttempt 1: prepare data failed\nAttempt 2: prepare data failed\nAttempt 3: prepare data failed\nAttempt 4: prepare data failed"
	compact := compactScheduledTaskResult(result, scheduledTaskSafeResultRunes)

	if len([]rune(compact)) > scheduledTaskSafeResultRunes {
		t.Fatalf("compact result is too long: %d", len([]rune(compact)))
	}
	if strings.Contains(compact, "\n") {
		t.Fatalf("expected compact result to be single-line, got %q", compact)
	}
}
