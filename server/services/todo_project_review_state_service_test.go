package services

import "testing"

func TestAttachTodoProjectReviewStates(t *testing.T) {
	items := []MyTodoItem{
		{ID: "1", ProjectCode: "A1177"},
		{ID: "2", ProjectCode: "a1177"},
		{ID: "3", ProjectCode: "A1244"},
	}
	attachTodoProjectReviewStates(items, map[string]bool{"a1177": true})
	if !items[0].PreviousVersionNotApproved || !items[1].PreviousVersionNotApproved {
		t.Fatalf("case-insensitive project state was not attached: %#v", items)
	}
	if items[2].PreviousVersionNotApproved {
		t.Fatalf("unrelated project unexpectedly inherited the state: %#v", items[2])
	}
}

func TestNormalizeTodoProjectStateKey(t *testing.T) {
	if got := normalizeTodoProjectStateKey("  A1177  "); got != "a1177" {
		t.Fatalf("normalized project key = %q, want a1177", got)
	}
}
