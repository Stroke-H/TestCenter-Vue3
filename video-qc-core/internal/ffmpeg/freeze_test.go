package ffmpeg

import "testing"

func TestParseFreezeDetectLog(t *testing.T) {
	log := `[freezedetect @ 000001] lavfi.freezedetect.freeze_start: 2
[freezedetect @ 000001] lavfi.freezedetect.freeze_duration: 1.24
[freezedetect @ 000001] lavfi.freezedetect.freeze_end: 3.24 | lavfi.freezedetect.freeze_duration: 1.24`

	segments := ParseFreezeDetectLog(log)
	if len(segments) != 1 {
		t.Fatalf("expected 1 freeze segment, got %d", len(segments))
	}
	if segments[0].Start != 2 || segments[0].End != 3.24 || segments[0].Duration != 1.24 {
		t.Fatalf("unexpected freeze segment: %+v", segments[0])
	}
}

func TestParseFreezeDetectLogSplitEndAndDuration(t *testing.T) {
	log := `[freezedetect @ 000001] lavfi.freezedetect.freeze_start: 1
[freezedetect @ 000001] lavfi.freezedetect.freeze_duration: 2.2
[freezedetect @ 000001] lavfi.freezedetect.freeze_end: 3.2`

	segments := ParseFreezeDetectLog(log)
	if len(segments) != 1 {
		t.Fatalf("expected 1 freeze segment, got %d", len(segments))
	}
	if segments[0].Start != 1 || segments[0].End != 3.2 || segments[0].Duration != 2.2 {
		t.Fatalf("unexpected freeze segment: %+v", segments[0])
	}
}
