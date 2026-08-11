package ffmpeg

import "testing"

func TestParseSignalStatsLog(t *testing.T) {
	log := `[Parsed_metadata_1 @ 000001] frame:0    pts:0       pts_time:0
[Parsed_metadata_1 @ 000001] lavfi.signalstats.YAVG=3.14
[Parsed_metadata_1 @ 000001] lavfi.signalstats.UAVG=54
[Parsed_metadata_1 @ 000001] lavfi.signalstats.VAVG=34
[Parsed_metadata_1 @ 000001] lavfi.signalstats.YDIF=0.2
[Parsed_metadata_1 @ 000001] lavfi.signalstats.YMIN=1
[Parsed_metadata_1 @ 000001] lavfi.signalstats.YMAX=9
[Parsed_metadata_1 @ 000001] frame:1    pts:512     pts_time:0.04
[Parsed_metadata_1 @ 000001] lavfi.signalstats.YAVG=8.5`

	stats := ParseSignalStatsLog(log)
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}
	if stats[0].Frame != 0 || stats[0].PTS != 0 || stats[0].YAvg != 3.14 {
		t.Fatalf("unexpected first stat: %+v", stats[0])
	}
	if stats[0].UAvg != 54 || stats[0].VAvg != 34 {
		t.Fatalf("unexpected chroma stats: %+v", stats[0])
	}
	if stats[0].YDif != 0.2 {
		t.Fatalf("unexpected frame diff stat: %+v", stats[0])
	}
	if stats[0].YMin != 1 || stats[0].YMax != 9 {
		t.Fatalf("unexpected luma range stats: %+v", stats[0])
	}
	if stats[1].Frame != 1 || stats[1].PTS != 0.04 || stats[1].YAvg != 8.5 {
		t.Fatalf("unexpected second stat: %+v", stats[1])
	}
}

func TestParseBlackFrameStatsLog(t *testing.T) {
	log := `[Parsed_blackframe_0 @ 000001] frame:0 pblack:100 pts:0 t:0.000000 type:I last_keyframe:0
[Parsed_blackframe_0 @ 000001] frame:1 pblack:98 pts:512 t:0.040000 type:P last_keyframe:0`

	stats := ParseBlackFrameStatsLog(log)
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}
	if stats[0].Frame != 0 || stats[0].Time != 0 || stats[0].PBlack != 100 {
		t.Fatalf("unexpected first stat: %+v", stats[0])
	}
	if stats[1].Frame != 1 || stats[1].Time != 0.04 || stats[1].PBlack != 98 {
		t.Fatalf("unexpected second stat: %+v", stats[1])
	}
}
