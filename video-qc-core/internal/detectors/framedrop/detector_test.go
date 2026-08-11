package framedrop

import (
	"testing"

	"video-qc-core/internal/ffmpeg"
)

func TestExpectedFrameDeltaUsesFPS(t *testing.T) {
	delta := expectedFrameDelta(nil, 25)
	if delta != 0.04 {
		t.Fatalf("expected 0.04, got %f", delta)
	}
}

func TestExpectedFrameDeltaFallsBackToMedian(t *testing.T) {
	pts := []ffmpeg.FramePTS{
		{Index: 0, PTS: 0},
		{Index: 1, PTS: 0.04},
		{Index: 2, PTS: 0.08},
		{Index: 3, PTS: 0.20},
	}
	delta := expectedFrameDelta(pts, 0)
	if delta != 0.04 {
		t.Fatalf("expected median delta 0.04, got %f", delta)
	}
}

func TestDetectFrameDropEvents(t *testing.T) {
	pts := []ffmpeg.FramePTS{
		{Index: 0, PTS: 0},
		{Index: 1, PTS: 0.04},
		{Index: 2, PTS: 0.20},
	}

	events := detectFrameDropEvents(pts, 0.04, 1.8)
	if len(events) != 1 {
		t.Fatalf("expected 1 frame drop event, got %d", len(events))
	}
	if events[0].Type != "frame_drop" {
		t.Fatalf("unexpected event type: %s", events[0].Type)
	}
	if events[0].Evidence.Metrics["dropCount"] != 3 {
		t.Fatalf("unexpected drop count: %f", events[0].Evidence.Metrics["dropCount"])
	}
}
