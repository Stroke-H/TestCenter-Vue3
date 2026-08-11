package freeze

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
	return "freeze"
}

func (Detector) Stage() detector.Stage {
	return detector.StageFrame
}

func (Detector) Enabled(profile config.Profile) bool {
	return profile.Stages.Freeze
}

func (Detector) Run(ctx context.Context, qc detector.Context) ([]schema.Event, error) {
	segments, rawLog, err := ffmpeg.FreezeDetect(ctx, qc.Profile.Tools.FFmpeg, qc.Input, qc.Profile.Thresholds.FreezeMinDurationMs)
	if err != nil {
		return []schema.Event{{
			ID:        "freezedetect_error",
			Type:      "freezedetect_error",
			Category:  "visual",
			Severity:  schema.SeverityMedium,
			Message:   err.Error(),
			Method:    "ffmpeg freezedetect",
			Evidence:  schema.Evidence{RawLog: rawLog},
			CreatedAt: time.Now(),
		}}, nil
	}

	events := make([]schema.Event, 0, len(segments))
	for index, segment := range segments {
		events = append(events, schema.Event{
			ID:          fmt.Sprintf("freeze_%03d", index+1),
			Type:        "freeze",
			Category:    "visual",
			Severity:    severityForDuration(segment.Duration),
			StartTimeMs: int64(segment.Start * 1000),
			EndTimeMs:   int64(segment.End * 1000),
			DurationMs:  int64(segment.Duration * 1000),
			Message:     fmt.Sprintf("Freeze lasts %.3f seconds", segment.Duration),
			Confidence:  confidenceForDuration(segment.Duration),
			Method:      "ffmpeg freezedetect",
			Evidence: schema.Evidence{
				Metrics: map[string]float64{
					"freezeStart":    segment.Start,
					"freezeEnd":      segment.End,
					"freezeDuration": segment.Duration,
				},
			},
			CreatedAt: time.Now(),
		})
	}
	return events, nil
}

func severityForDuration(duration float64) schema.Severity {
	if duration >= 5 {
		return schema.SeverityHigh
	}
	if duration >= 2 {
		return schema.SeverityMedium
	}
	return schema.SeverityLow
}

func confidenceForDuration(duration float64) float64 {
	if duration >= 5 {
		return 0.93
	}
	if duration >= 2 {
		return 0.86
	}
	return 0.78
}
