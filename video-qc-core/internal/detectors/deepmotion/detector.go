package deepmotion

import (
	"context"
	"fmt"
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
	return "deepmotion"
}

func (Detector) Stage() detector.Stage {
	return detector.StageFrame
}

func (Detector) Enabled(profile config.Profile) bool {
	return profile.Stages.DeepMotion
}

func (Detector) Run(ctx context.Context, qc detector.Context) ([]schema.Event, error) {
	stats, rawLog, err := ffmpeg.SignalStats(ctx, qc.Profile.Tools.FFmpeg, qc.Input)
	if err != nil {
		return []schema.Event{{
			ID:        "deep_motion_stats_error",
			Type:      "deep_motion_stats_error",
			Category:  "motion",
			Severity:  schema.SeverityMedium,
			Message:   err.Error(),
			Method:    "signalstats yd_if",
			Evidence:  schema.Evidence{RawLog: rawLog},
			CreatedAt: time.Now(),
		}}, nil
	}
	return detectMotionJumps(stats, qc.Video.FPS), nil
}

func detectMotionJumps(stats []ffmpeg.FrameSignalStat, fps float64) []schema.Event {
	events := make([]schema.Event, 0)
	for index := 1; index < len(stats); index++ {
		prev := stats[index-1]
		curr := stats[index]
		if curr.YDif < 35 || curr.YDif < prev.YDif*4 {
			continue
		}
		durationMs := int64(0)
		if fps > 0 {
			durationMs = int64((1 / fps) * 1000)
		}
		events = append(events, schema.Event{
			ID:          fmt.Sprintf("motion_jump_%03d", len(events)+1),
			Type:        "motion_jump",
			Category:    "motion",
			Severity:    schema.SeverityMedium,
			StartTimeMs: int64(prev.PTS * 1000),
			EndTimeMs:   int64(curr.PTS * 1000),
			DurationMs:  durationMs,
			Message:     fmt.Sprintf("Motion discontinuity YDIF %.3f -> %.3f", prev.YDif, curr.YDif),
			Confidence:  0.68,
			Method:      "signalstats yd_if",
			Evidence: schema.Evidence{
				FrameStart: prev.Frame,
				FrameEnd:   curr.Frame,
				Metrics: map[string]float64{
					"previousYDif": prev.YDif,
					"currentYDif":  curr.YDif,
				},
			},
			CreatedAt: time.Now(),
		})
	}
	return events
}
