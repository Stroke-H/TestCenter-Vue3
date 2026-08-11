package blackscreen

import (
	"testing"

	"video-qc-core/internal/ffmpeg"
)

func TestMergeSegments(t *testing.T) {
	segments := []ffmpeg.BlackSegment{
		{Start: 0, End: 0.5, Duration: 0.5},
		{Start: 0.58, End: 1.1, Duration: 0.52},
		{Start: 2, End: 3, Duration: 1},
	}

	merged := mergeSegments(segments, 0.12)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged segments, got %d", len(merged))
	}
	if merged[0].Start != 0 || merged[0].End != 1.1 || merged[0].Duration != 1.1 {
		t.Fatalf("unexpected first merged segment: %+v", merged[0])
	}
}

func TestClassifySegment(t *testing.T) {
	tests := []struct {
		name     string
		segment  ffmpeg.BlackSegment
		duration float64
		want     string
	}{
		{name: "flash", segment: ffmpeg.BlackSegment{Start: 3, End: 3.4, Duration: 0.4}, duration: 10, want: "flash_black"},
		{name: "head", segment: ffmpeg.BlackSegment{Start: 0, End: 1.2, Duration: 1.2}, duration: 10, want: "head_black"},
		{name: "tail", segment: ffmpeg.BlackSegment{Start: 8.5, End: 10, Duration: 1.5}, duration: 10, want: "tail_black"},
		{name: "middle", segment: ffmpeg.BlackSegment{Start: 3, End: 4.2, Duration: 1.2}, duration: 10, want: "black_screen"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := classifySegment(test.segment, test.duration)
			if got != test.want {
				t.Fatalf("expected %s, got %s", test.want, got)
			}
		})
	}
}
