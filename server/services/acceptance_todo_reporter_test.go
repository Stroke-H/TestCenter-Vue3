package services

import "testing"

func TestAcceptanceTodoReporterEnabled(t *testing.T) {
	for _, value := range []string{"minghong", " MINGHONG "} {
		if !acceptanceTodoReporterEnabled(value) {
			t.Fatalf("expected enabled: %s", value)
		}
	}
	for _, value := range []string{"", "Stroke", "rongchang.xing", "jinhuayang"} {
		if acceptanceTodoReporterEnabled(value) {
			t.Fatalf("unexpected enabled: %s", value)
		}
		if err := QueueAcceptanceTodoReminder(AcceptanceReport{Reporter: value}); err != nil {
			t.Fatal(err)
		}
	}
}
