package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseManualTodoDueAt(t *testing.T) {
	loc := acceptanceTodoLocation()
	want := time.Date(2026, time.August, 20, 14, 30, 0, 0, loc)
	for _, value := range []string{"2026-08-20 14:30:00", "2026-08-20 14:30"} {
		got, err := parseManualTodoDueAt(value)
		if err != nil {
			t.Fatalf("parseManualTodoDueAt(%q): %v", value, err)
		}
		if !got.Equal(want) {
			t.Fatalf("parseManualTodoDueAt(%q) = %s, want %s", value, got, want)
		}
	}
	if _, err := parseManualTodoDueAt("tomorrow"); err == nil {
		t.Fatal("invalid time did not return an error")
	}
}

func TestBuildManualTodoReminderCard(t *testing.T) {
	reminder := ManualTodoReminder{
		ID:           "manual-1",
		ProjectCode:  "A1177",
		ProjectName:  "Novashort - TTmins",
		EventContent: "复核过审状态，并验证返回插屏",
		DueAt:        time.Date(2026, time.August, 20, 15, 0, 0, 0, acceptanceTodoLocation()),
	}
	cardJSON, err := json.Marshal(BuildManualTodoReminderCard(reminder))
	if err != nil {
		t.Fatal(err)
	}
	card := string(cardJSON)
	for _, expected := range []string{
		"个人待办提醒", "A1177", "Novashort - TTmins", "复核过审状态，并验证返回插屏",
		"2026-08-20 15:00", "查看我的待办", "/my-todos", "仅发送一次",
	} {
		if !strings.Contains(card, expected) {
			t.Fatalf("manual todo card missing %q: %s", expected, card)
		}
	}
	if strings.Contains(card, acceptanceTodoActionDone) {
		t.Fatalf("manual todo card unexpectedly requires a Feishu callback: %s", card)
	}
}

func TestManualTodoToMyTodo(t *testing.T) {
	manual := ManualTodoReminder{
		ID: "manual-1", OwnerUsername: "minghong", ProjectCode: "A1177",
		ProjectName: "Novashort", EventContent: "检查支付链路",
	}
	item := manualTodoToMyTodo(manual)
	if item.TodoType != myTodoTypeManual || item.ReportID != "" {
		t.Fatalf("unexpected common todo type: %#v", item)
	}
	if len(item.FeatureItems) != 1 || item.FeatureItems[0] != manual.EventContent {
		t.Fatalf("unexpected manual todo content: %#v", item.FeatureItems)
	}
}
