package ffmpeg

import (
	"testing"

	"video-qc-core/pkg/schema"
)

func TestValidateProbe(t *testing.T) {
	result := ProbeResult{
		Streams: []ProbeStream{{
			CodecType:    "video",
			CodecName:    "h264",
			Width:        1920,
			Height:       1080,
			AvgFrameRate: "25/1",
		}},
		Format: ProbeFormat{Duration: "12.5"},
	}

	video := schema.VideoInfo{
		Input:      "sample.mp4",
		Duration:   12.5,
		FPS:        parseFrameRate(result.Streams[0].AvgFrameRate),
		Resolution: "1920x1080",
		Codec:      "h264",
	}
	if video.FPS != 25 {
		t.Fatalf("expected fps 25, got %f", video.FPS)
	}
	if video.Resolution != "1920x1080" {
		t.Fatalf("expected resolution, got %s", video.Resolution)
	}
	if problems := ValidateProbe(result, video); len(problems) != 0 {
		t.Fatalf("expected no problems, got %v", problems)
	}
}

func TestValidateProbeReportsMissingVideo(t *testing.T) {
	problems := ValidateProbe(ProbeResult{}, schema.VideoInfo{Input: "bad.mp4"})
	if len(problems) == 0 {
		t.Fatalf("expected validation problems")
	}
}
