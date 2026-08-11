package visualartifact

import (
	"testing"

	"video-qc-core/internal/ffmpeg"
	"video-qc-core/pkg/config"
)

func TestDetectWhiteScreens(t *testing.T) {
	stats := []ffmpeg.FrameSignalStat{
		{Frame: 0, PTS: 0, YAvg: 250},
		{Frame: 1, PTS: 0.5, YAvg: 251},
		{Frame: 2, PTS: 1.0, YAvg: 252},
	}
	profile := config.Profile{Thresholds: config.Thresholds{WhiteMeanY: 245}}

	events := detectWhiteScreens(stats, profile, 25)
	if len(events) != 1 {
		t.Fatalf("expected 1 white screen event, got %d", len(events))
	}
	if events[0].Type != "white_screen" {
		t.Fatalf("unexpected event type: %s", events[0].Type)
	}
}

func TestDetectBrightnessJumps(t *testing.T) {
	stats := []ffmpeg.FrameSignalStat{
		{Frame: 0, PTS: 0, YAvg: 20},
		{Frame: 1, PTS: 0.04, YAvg: 140},
	}

	events := detectBrightnessJumps(stats, 25)
	if len(events) != 1 {
		t.Fatalf("expected 1 brightness jump event, got %d", len(events))
	}
	if events[0].Type != "brightness_jump" {
		t.Fatalf("unexpected event type: %s", events[0].Type)
	}
}

func TestDetectGreenScreens(t *testing.T) {
	stats := []ffmpeg.FrameSignalStat{
		{Frame: 0, PTS: 0, YAvg: 145, UAvg: 54, VAvg: 34},
		{Frame: 1, PTS: 0.5, YAvg: 145, UAvg: 54, VAvg: 34},
		{Frame: 2, PTS: 1.0, YAvg: 145, UAvg: 54, VAvg: 34},
	}

	events := detectGreenScreens(stats, 25)
	if len(events) != 1 {
		t.Fatalf("expected 1 green screen event, got %d", len(events))
	}
	if events[0].Type != "green_screen" {
		t.Fatalf("unexpected event type: %s", events[0].Type)
	}
}

func TestDetectSimilarityFreezes(t *testing.T) {
	stats := []ffmpeg.FrameSignalStat{
		{Frame: 0, PTS: 0, YDif: 5},
		{Frame: 1, PTS: 0.04, YDif: 0.1},
		{Frame: 2, PTS: 0.08, YDif: 0.1},
		{Frame: 3, PTS: 0.12, YDif: 0.1},
	}
	profile := config.Profile{Thresholds: config.Thresholds{FreezeMinDurationMs: 100}}

	events := detectSimilarityFreezes(stats, profile, 25)
	if len(events) != 1 {
		t.Fatalf("expected 1 similarity freeze event, got %d", len(events))
	}
	if events[0].Type != "freeze_similarity" {
		t.Fatalf("unexpected event type: %s", events[0].Type)
	}
}

func TestDetectArtifactSpikes(t *testing.T) {
	stats := []ffmpeg.FrameSignalStat{
		{Frame: 0, PTS: 0, YDif: 1, YMin: 20, YMax: 60},
		{Frame: 1, PTS: 0.04, YDif: 70, YMin: 0, YMax: 255},
		{Frame: 2, PTS: 0.08, YDif: 72, YMin: 0, YMax: 255},
		{Frame: 3, PTS: 0.12, YDif: 74, YMin: 0, YMax: 255},
	}

	events := detectArtifactSpikes(stats, 25)
	if len(events) != 1 {
		t.Fatalf("expected 1 artifact spike event, got %d", len(events))
	}
	if events[0].Type != "artifact_spike" {
		t.Fatalf("unexpected event type: %s", events[0].Type)
	}
}

func TestDetectArtifactSpikesIgnoresSingleFrameTransition(t *testing.T) {
	stats := []ffmpeg.FrameSignalStat{
		{Frame: 0, PTS: 0, YDif: 1, YMin: 20, YMax: 60},
		{Frame: 1, PTS: 0.04, YDif: 90, YMin: 0, YMax: 255},
		{Frame: 2, PTS: 0.08, YDif: 5, YMin: 20, YMax: 60},
	}

	events := detectArtifactSpikes(stats, 25)
	if len(events) != 0 {
		t.Fatalf("expected no artifact spike event, got %d", len(events))
	}
}
