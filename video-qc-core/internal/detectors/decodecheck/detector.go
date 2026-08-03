package decodecheck

import (
	"context"
	"strings"
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
	return "decodecheck"
}

func (Detector) Stage() detector.Stage {
	return detector.StageDecode
}

func (Detector) Enabled(profile config.Profile) bool {
	return profile.Stages.DecodeCheck
}

func (Detector) Run(ctx context.Context, qc detector.Context) ([]schema.Event, error) {
	rawLog, err := ffmpeg.Decode(ctx, qc.Profile.Tools.FFmpeg, qc.Input)
	if err == nil {
		return nil, nil
	}

	eventType := "decode_error"
	category := "decode"
	if isTimestampError(rawLog) {
		eventType = "timestamp_error"
		category = "timeline"
	}

	return []schema.Event{{
		ID:       eventType,
		Type:     eventType,
		Category: category,
		Severity: schema.SeverityCritical,
		Message:  err.Error(),
		Method:   "ffmpeg -v error -f null",
		Evidence: schema.Evidence{
			RawLog: rawLog,
		},
		CreatedAt: time.Now(),
	}}, nil
}

func isTimestampError(log string) bool {
	lower := strings.ToLower(log)
	return strings.Contains(lower, "negative timestamp") ||
		strings.Contains(lower, "timestamp discontinuity") ||
		strings.Contains(lower, "non monotonically increasing dts") ||
		strings.Contains(lower, "non-monotonous dts")
}
