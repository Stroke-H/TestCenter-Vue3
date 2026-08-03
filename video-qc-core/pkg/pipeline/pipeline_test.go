package pipeline

import (
	"context"
	"testing"
	"time"

	"video-qc-core/pkg/config"
	"video-qc-core/pkg/detector"
	"video-qc-core/pkg/schema"
)

type fakeDetector struct{}

func (fakeDetector) Name() string                { return "fake" }
func (fakeDetector) Stage() detector.Stage       { return detector.StageFile }
func (fakeDetector) Enabled(config.Profile) bool { return true }
func (fakeDetector) Run(context.Context, detector.Context) ([]schema.Event, error) {
	return []schema.Event{{
		ID:        "evt_fake",
		Type:      "probe_ok",
		Category:  "file",
		Severity:  schema.SeverityInfo,
		Message:   "probe skeleton ok",
		CreatedAt: time.Now(),
	}}, nil
}

func TestRunnerRun(t *testing.T) {
	profile := config.Profile{Name: "test"}
	report, err := NewRunner(fakeDetector{}).Run(context.Background(), Options{
		TaskID:  "task-test",
		Input:   "sample.mp4",
		Profile: profile,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.Status != schema.StatusSuccess {
		t.Fatalf("expected success, got %s", report.Status)
	}
	if len(report.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(report.Events))
	}
}

func TestReconcileEventsDropsOverlappingSimilarityFreeze(t *testing.T) {
	events := reconcileEvents([]schema.Event{
		{Type: "freeze", StartTimeMs: 1000, EndTimeMs: 2200, DurationMs: 1200},
		{Type: "freeze_similarity", StartTimeMs: 1040, EndTimeMs: 2200, DurationMs: 1160},
	})
	if len(events) != 1 {
		t.Fatalf("expected 1 reconciled event, got %d", len(events))
	}
	if events[0].Type != "freeze" {
		t.Fatalf("expected freeze event, got %s", events[0].Type)
	}
}
