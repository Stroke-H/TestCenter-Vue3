package ffmpeg

import "testing"

func TestParseFramePTSLog(t *testing.T) {
	log := "0.000000\n0.040000\nN/A\n0.080000\n"

	pts := ParseFramePTSLog(log)
	if len(pts) != 3 {
		t.Fatalf("expected 3 pts entries, got %d", len(pts))
	}
	if pts[1].Index != 1 || pts[1].PTS != 0.04 {
		t.Fatalf("unexpected second pts entry: %+v", pts[1])
	}
}
