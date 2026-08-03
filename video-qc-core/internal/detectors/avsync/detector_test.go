package avsync

import (
	"testing"

	"video-qc-core/internal/ffmpeg"
	"video-qc-core/pkg/schema"
)

func TestFirstVideoAudioStreams(t *testing.T) {
	video, audio, ok := firstVideoAudioStreams([]ffmpeg.ProbeStream{
		{CodecType: "video", StartTime: "0"},
		{CodecType: "audio", StartTime: "0.3"},
	})
	if !ok || video.CodecType != "video" || audio.CodecType != "audio" {
		t.Fatalf("unexpected streams: %+v %+v %v", video, audio, ok)
	}
}

func TestSeverityForOffset(t *testing.T) {
	if severityForOffset(1200, 0) != schema.SeverityHigh {
		t.Fatalf("expected high severity")
	}
	if severityForOffset(100, 600) != schema.SeverityMedium {
		t.Fatalf("expected medium severity")
	}
}
