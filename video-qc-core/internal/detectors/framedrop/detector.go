package framedrop

import (
	"context"
	"fmt"
	"math"
	"time"

	"video-qc-core/internal/ffmpeg"
	"video-qc-core/pkg/config"
	"video-qc-core/pkg/detector"
	"video-qc-core/pkg/schema"
)

type Detector struct{}

func New() Detector {
	return Detector{}
}

func (Detector) Name() string {
	return "framedrop"
}

func (Detector) Stage() detector.Stage {
	return detector.StageFrame
}

func (Detector) Enabled(profile config.Profile) bool {
	return profile.Stages.FrameDrop
}

func (Detector) Run(ctx context.Context, qc detector.Context) ([]schema.Event, error) {
	pts, rawLog, err := ffmpeg.FramePTSList(ctx, qc.Profile.Tools.FFProbe, qc.Input)
	if err != nil {
		return []schema.Event{{
			ID:        "frame_pts_error",
			Type:      "frame_pts_error",
			Category:  "timeline",
			Severity:  schema.SeverityMedium,
			Message:   err.Error(),
			Method:    "ffprobe frame pts",
			Evidence:  schema.Evidence{RawLog: rawLog},
			CreatedAt: time.Now(),
		}}, nil
	}

	expectedDelta := expectedFrameDelta(pts, qc.Video.FPS)
	if expectedDelta <= 0 {
		return nil, nil
	}
	ratio := qc.Profile.Thresholds.PTSGapRatio
	if ratio <= 1 {
		ratio = 1.8
	}

	return detectFrameDropEvents(pts, expectedDelta, ratio), nil
}

func detectFrameDropEvents(pts []ffmpeg.FramePTS, expectedDelta float64, ratio float64) []schema.Event {
	events := make([]schema.Event, 0)
	for index := 1; index < len(pts); index++ {
		delta := pts[index].PTS - pts[index-1].PTS
		if delta <= expectedDelta*ratio {
			continue
		}
		dropCount := int(math.Round(delta/expectedDelta)) - 1
		if dropCount < 1 {
			dropCount = 1
		}
		events = append(events, schema.Event{
			ID:          fmt.Sprintf("frame_drop_%03d", len(events)+1),
			Type:        "frame_drop",
			Category:    "timeline",
			Severity:    severityForDropCount(dropCount),
			StartTimeMs: int64(pts[index-1].PTS * 1000),
			EndTimeMs:   int64(pts[index].PTS * 1000),
			DurationMs:  int64(delta * 1000),
			Message:     fmt.Sprintf("Frame PTS gap %.3f seconds, estimated drop=%d", delta, dropCount),
			Confidence:  confidenceForGap(delta, expectedDelta),
			Method:      "ffprobe frame pts",
			Evidence: schema.Evidence{
				FrameStart: pts[index-1].Index,
				FrameEnd:   pts[index].Index,
				Metrics: map[string]float64{
					"actualDelta":   delta,
					"expectedDelta": expectedDelta,
					"gapRatio":      delta / expectedDelta,
					"dropCount":     float64(dropCount),
				},
			},
			CreatedAt: time.Now(),
		})
	}
	return events
}

func expectedFrameDelta(pts []ffmpeg.FramePTS, fps float64) float64 {
	if fps > 0 {
		return 1 / fps
	}
	if len(pts) < 3 {
		return 0
	}
	deltas := make([]float64, 0, len(pts)-1)
	for index := 1; index < len(pts); index++ {
		delta := pts[index].PTS - pts[index-1].PTS
		if delta > 0 {
			deltas = append(deltas, delta)
		}
	}
	if len(deltas) == 0 {
		return 0
	}
	return median(deltas)
}

func median(values []float64) float64 {
	items := append([]float64(nil), values...)
	for i := 1; i < len(items); i++ {
		value := items[i]
		j := i - 1
		for j >= 0 && items[j] > value {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = value
	}
	middle := len(items) / 2
	if len(items)%2 == 1 {
		return items[middle]
	}
	return (items[middle-1] + items[middle]) / 2
}

func severityForDropCount(dropCount int) schema.Severity {
	if dropCount >= 5 {
		return schema.SeverityHigh
	}
	if dropCount >= 2 {
		return schema.SeverityMedium
	}
	return schema.SeverityLow
}

func confidenceForGap(delta float64, expectedDelta float64) float64 {
	ratio := delta / expectedDelta
	if ratio >= 5 {
		return 0.92
	}
	if ratio >= 3 {
		return 0.85
	}
	return 0.75
}
