package ffmpeg

import "testing"

func TestParseBlackDetectLog(t *testing.T) {
	log := `[blackdetect @ 000001] black_start:0 black_end:2.52 black_duration:2.52
[blackdetect @ 000001] black_start:12.1 black_end:13.8 black_duration:1.7`
	segments := ParseBlackDetectLog(log)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0].Start != 0 || segments[0].End != 2.52 || segments[0].Duration != 2.52 {
		t.Fatalf("unexpected first segment: %+v", segments[0])
	}
}
