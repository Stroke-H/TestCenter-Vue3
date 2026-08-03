package deepmotion

import (
	"testing"

	"video-qc-core/internal/ffmpeg"
)

func TestDetectMotionJumps(t *testing.T) {
	stats := []ffmpeg.FrameSignalStat{
		{Frame: 0, PTS: 0, YDif: 2},
		{Frame: 1, PTS: 0.04, YDif: 50},
	}

	events := detectMotionJumps(stats, 25)
	if len(events) != 1 {
		t.Fatalf("expected 1 motion jump event, got %d", len(events))
	}
	if events[0].Type != "motion_jump" {
		t.Fatalf("unexpected event type: %s", events[0].Type)
	}
}
