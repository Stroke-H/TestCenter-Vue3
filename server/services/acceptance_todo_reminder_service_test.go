package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestIsTTminsProjectText(t *testing.T) {
	tests := []struct {
		name        string
		projectCode string
		projectName string
		want        bool
	}{
		{name: "name contains marker", projectCode: "A1160", projectName: "MeloShorts - TTmins", want: true},
		{name: "case insensitive", projectCode: "ttMINS-demo", projectName: "Demo", want: true},
		{name: "unrelated project", projectCode: "A750", projectName: "Free Drama", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isTTminsProjectText(test.projectCode, test.projectName); got != test.want {
				t.Fatalf("isTTminsProjectText() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestExtractAcceptanceTodoFeatures(t *testing.T) {
	got := extractAcceptanceTodoFeatures("版本更新测试需求点：\n1. 首页新增活动入口\n- 播放器增加返回插屏；播放器增加返回插屏\n• 验证前两集免费")
	want := []string{"首页新增活动入口", "播放器增加返回插屏", "验证前两集免费"}
	if len(got) != len(want) {
		t.Fatalf("feature count = %d, want %d: %#v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("feature[%d] = %q, want %q", index, got[index], want[index])
		}
	}

	fallback := extractAcceptanceTodoFeatures("暂无")
	if len(fallback) != 1 || !strings.Contains(fallback[0], "未填写") {
		t.Fatalf("unexpected fallback: %#v", fallback)
	}
}

func TestNextAcceptanceTodoReminderTime(t *testing.T) {
	loc := acceptanceTodoLocation()
	now := time.Date(2026, time.August, 19, 23, 45, 0, 0, loc)
	got := nextAcceptanceTodoReminderTime(now)
	want := time.Date(2026, time.August, 20, 10, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("next reminder = %s, want %s", got, want)
	}
}

func TestShouldSendAcceptanceTodoReminderOncePerDay(t *testing.T) {
	loc := acceptanceTodoLocation()
	now := time.Date(2026, time.August, 20, 10, 5, 0, 0, loc)
	dueAt := time.Date(2026, time.August, 20, 10, 0, 0, 0, loc)
	reminder := AcceptanceTodoReminder{Status: acceptanceTodoStatusPending, DueAt: dueAt}
	if !shouldSendAcceptanceTodoReminder(reminder, now) {
		t.Fatal("due pending reminder should send")
	}

	lastToday := time.Date(2026, time.August, 20, 10, 1, 0, 0, loc)
	reminder.LastNotifiedAt = &lastToday
	if shouldSendAcceptanceTodoReminder(reminder, now) {
		t.Fatal("reminder should not send twice on the same day")
	}

	nextDay := time.Date(2026, time.August, 21, 10, 1, 0, 0, loc)
	if !shouldSendAcceptanceTodoReminder(reminder, nextDay) {
		t.Fatal("unfinished reminder should repeat on the next day")
	}

	reminder.Status = acceptanceTodoStatusDone
	if shouldSendAcceptanceTodoReminder(reminder, nextDay) {
		t.Fatal("completed reminder must not send")
	}
}

func TestAcceptanceTodoReminderDailyWindow(t *testing.T) {
	loc := acceptanceTodoLocation()
	due := time.Date(2026, time.September, 2, 10, 0, 0, 0, loc)
	yesterday := time.Date(2026, time.September, 9, 10, 0, 0, 0, loc)
	today := time.Date(2026, time.September, 10, 10, 0, 0, 0, loc)
	for _, test := range []struct {
		name string
		now  time.Time
		last *time.Time
		want bool
	}{
		{"midnight repeat", today.Add(-10 * time.Hour), &yesterday, false},
		{"before ten repeat", today.Add(-time.Second), &yesterday, false},
		{"at ten repeat", today, &yesterday, true},
		{"late recovery", today.Add(6 * time.Hour), &yesterday, true},
		{"same day duplicate", today.Add(time.Hour), &today, false},
		{"overdue first notification before ten", today.Add(-time.Hour), nil, false},
		{"overdue first notification at ten", today, nil, true},
		{"UTC before Shanghai ten", today.Add(-time.Second).UTC(), &yesterday, false},
		{"UTC at Shanghai ten", today.UTC(), &yesterday, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			reminder := AcceptanceTodoReminder{Status: acceptanceTodoStatusPending, DueAt: due, LastNotifiedAt: test.last}
			if got := shouldSendAcceptanceTodoReminder(reminder, test.now); got != test.want {
				t.Fatalf("shouldSendAcceptanceTodoReminder() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestBuildAcceptanceTodoReminderCard(t *testing.T) {
	reminder := AcceptanceTodoReminder{
		ID:           "todo-1",
		ProjectCode:  "A1160",
		ProjectName:  "MeloShorts - TTmins",
		FeatureItems: []string{"首页新增活动入口", "播放器增加返回插屏"},
	}

	pendingJSON, err := json.Marshal(BuildAcceptanceTodoReminderCard(reminder, false))
	if err != nil {
		t.Fatal(err)
	}
	pending := string(pendingJSON)
	for _, expected := range []string{"查看我的待办", "/my-todos", "首页新增活动入口", "播放器增加返回插屏"} {
		if !strings.Contains(pending, expected) {
			t.Fatalf("pending card missing %q: %s", expected, pending)
		}
	}
	if strings.Contains(pending, acceptanceTodoActionDone) {
		t.Fatalf("pending card still requires a Feishu callback: %s", pending)
	}

	completedJSON, err := json.Marshal(BuildAcceptanceTodoReminderCard(reminder, true))
	if err != nil {
		t.Fatal(err)
	}
	completed := string(completedJSON)
	if strings.Contains(completed, "查看我的待办") {
		t.Fatalf("completed card still contains an active todo link: %s", completed)
	}
	if !strings.Contains(completed, "已完成") || !strings.Contains(completed, "后续不再提醒") {
		t.Fatalf("completed card missing completed state: %s", completed)
	}
}

func TestGroupAcceptanceTodoRemindersByReporter(t *testing.T) {
	groups := groupAcceptanceTodoRemindersByReporter([]AcceptanceTodoReminder{
		{ID: "1", Reporter: "minghong"},
		{ID: "2", Reporter: "MingHong"},
		{ID: "3", Reporter: "tester-b"},
	})
	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2", len(groups))
	}
	if len(groups[0]) != 2 || groups[0][0].ID != "1" || groups[0][1].ID != "2" {
		t.Fatalf("unexpected first group: %#v", groups[0])
	}
}

func TestBuildAcceptanceTodoReminderGroupCard(t *testing.T) {
	reminders := []AcceptanceTodoReminder{
		{ID: "todo-1", ProjectCode: "A1177", ProjectName: "Novashort - TTmins", FeatureItems: []string{"验证插屏兜底"}, Status: acceptanceTodoStatusPending},
		{ID: "todo-2", ProjectCode: "A1244", ProjectName: "CocoShorts - TTmins", FeatureItems: []string{"验证开屏云控"}, Status: acceptanceTodoStatusDone},
		{ID: "todo-3", ProjectCode: "A1181", ProjectName: "Dramix - TTmins", FeatureItems: []string{"验证 Deeplink 策略"}, Status: acceptanceTodoStatusPending},
	}
	cardJSON, err := json.Marshal(BuildAcceptanceTodoReminderGroupCard(reminders))
	if err != nil {
		t.Fatal(err)
	}
	card := string(cardJSON)
	for _, expected := range []string{"3 个项目", "A1177", "A1244", "A1181", "剩余 2 项未完成", "查看我的待办", "/my-todos"} {
		if !strings.Contains(card, expected) {
			t.Fatalf("group card missing %q: %s", expected, card)
		}
	}
	if strings.Contains(card, acceptanceTodoActionDone) {
		t.Fatalf("group card still contains callback actions: %s", card)
	}
	if got := strings.Count(card, `"content":"查看我的待办"`); got != 1 {
		t.Fatalf("platform link button count = %d, want 1: %s", got, card)
	}
}

func TestAcceptanceReportCreatedOn(t *testing.T) {
	report := AcceptanceReport{CreatedAt: "2026-08-19T15:42:03+08:00"}
	if !acceptanceReportCreatedOn(report, "2026-08-19") {
		t.Fatal("expected report to match its creation date")
	}
	if acceptanceReportCreatedOn(report, "2026-08-18") {
		t.Fatal("report matched a different date")
	}
}

func TestResolveExactFeishuOpenIDDoesNotUseFuzzyMatching(t *testing.T) {
	// The exact lookup itself is database-backed. This test documents and guards
	// the most important edge case at the pure input boundary: blank operators
	// must never resolve to a fallback recipient.
	if got := resolveExactFeishuOpenID("   "); got != "" {
		t.Fatalf("blank operator resolved to %q", got)
	}
}
