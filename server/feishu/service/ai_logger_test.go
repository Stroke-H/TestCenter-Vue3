package service

import "testing"

func TestExtractScheduledTaskRunID(t *testing.T) {
	detail := "Scheduled task executed | report=http://www.inspdance.com:8080/api/test-runs/ST-ST-123/artifacts/report"
	if got := extractScheduledTaskRunID(detail); got != "ST-ST-123" {
		t.Fatalf("unexpected run id: %s", got)
	}
	if got := extractScheduledTaskRunID("report missing"); got != "" {
		t.Fatalf("expected empty run id, got %s", got)
	}
}
