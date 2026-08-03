package blackscreen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	return "blackscreen"
}

func (Detector) Stage() detector.Stage {
	return detector.StageFrame
}

func (Detector) Enabled(profile config.Profile) bool {
	return profile.Stages.BlackScreen
}

func (Detector) Run(ctx context.Context, qc detector.Context) ([]schema.Event, error) {
	segments, rawLog, err := ffmpeg.BlackDetect(ctx, qc.Profile.Tools.FFmpeg, qc.Input, qc.Profile.Thresholds.BlackMinDurationMs)
	if err != nil {
		return []schema.Event{{
			ID:       "blackdetect_error",
			Type:     "blackdetect_error",
			Category: "visual",
			Severity: schema.SeverityMedium,
			Message:  err.Error(),
			Method:   "ffmpeg blackdetect",
			Evidence: schema.Evidence{
				RawLog: rawLog,
			},
			CreatedAt: time.Now(),
		}}, nil
	}
	segments = mergeSegments(segments, 0.12)

	lumaStats, _, _ := ffmpeg.SignalStats(ctx, qc.Profile.Tools.FFmpeg, qc.Input)
	blackFrameStats, _, _ := ffmpeg.BlackFrameStats(ctx, qc.Profile.Tools.FFmpeg, qc.Input)
	edgeStats, _, _ := ffmpeg.EdgeSignalStats(ctx, qc.Profile.Tools.FFmpeg, qc.Input)

	events := make([]schema.Event, 0, len(segments))
	for index, segment := range segments {
		metrics := map[string]float64{
			"blackStart":    segment.Start,
			"blackEnd":      segment.End,
			"blackDuration": segment.Duration,
		}
		confidence := 0.70
		method := "ffmpeg blackdetect"
		if meanY, ok := meanYForSegment(lumaStats, segment); ok {
			metrics["meanY"] = meanY
		}
		if blackPixelRatio, ok := blackPixelRatioForSegment(blackFrameStats, segment); ok {
			metrics["blackPixelRatio"] = blackPixelRatio
		}
		if edgeRatio, ok := edgeRatioForSegment(edgeStats, segment); ok {
			metrics["edgeRatio"] = edgeRatio
		}
		if hasConfirmationMetrics(metrics) {
			method = "blackdetect+luma+blackframe+edge"
			confidence = confidenceForMetrics(qc.Profile, metrics)
		}
		eventType := classifySegment(segment, qc.Video.Duration)
		thumbnail := captureThumbnail(ctx, qc, eventType, index, segment)

		events = append(events, schema.Event{
			ID:          fmt.Sprintf("black_screen_%03d", index+1),
			Type:        eventType,
			Category:    "visual",
			Severity:    severityForDuration(segment.Duration),
			StartTimeMs: int64(segment.Start * 1000),
			EndTimeMs:   int64(segment.End * 1000),
			DurationMs:  int64(segment.Duration * 1000),
			Message:     fmt.Sprintf("Black screen lasts %.3f seconds", segment.Duration),
			Confidence:  confidence,
			Method:      method,
			Evidence: schema.Evidence{
				Thumbnail: thumbnail,
				Metrics:   metrics,
			},
			CreatedAt: time.Now(),
		})
	}
	return events, nil
}

func severityForDuration(duration float64) schema.Severity {
	if duration >= 3 {
		return schema.SeverityHigh
	}
	if duration >= 1 {
		return schema.SeverityMedium
	}
	return schema.SeverityLow
}

func mergeSegments(segments []ffmpeg.BlackSegment, maxGap float64) []ffmpeg.BlackSegment {
	if len(segments) <= 1 {
		return segments
	}
	merged := make([]ffmpeg.BlackSegment, 0, len(segments))
	current := segments[0]
	for _, segment := range segments[1:] {
		if segment.Start-current.End <= maxGap {
			current.End = segment.End
			current.Duration = current.End - current.Start
			continue
		}
		merged = append(merged, current)
		current = segment
	}
	merged = append(merged, current)
	return merged
}

func classifySegment(segment ffmpeg.BlackSegment, videoDuration float64) string {
	if segment.Duration < 1 {
		return "flash_black"
	}
	if segment.Start <= 0.3 {
		return "head_black"
	}
	if videoDuration > 0 && videoDuration-segment.End <= 0.3 {
		return "tail_black"
	}
	return "black_screen"
}

func captureThumbnail(ctx context.Context, qc detector.Context, eventType string, index int, segment ffmpeg.BlackSegment) string {
	if qc.WorkDir == "" {
		return ""
	}
	evidenceDir := filepath.Join(qc.WorkDir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		return ""
	}
	filename := fmt.Sprintf("%s_%03d.jpg", eventType, index+1)
	output := filepath.Join(evidenceDir, filename)
	timestamp := segment.Start + segment.Duration/2
	if _, err := ffmpeg.CaptureFrame(ctx, qc.Profile.Tools.FFmpeg, qc.Input, timestamp, output); err != nil {
		return ""
	}
	return filepath.ToSlash(filepath.Join("evidence", filename))
}

func meanYForSegment(stats []ffmpeg.FrameSignalStat, segment ffmpeg.BlackSegment) (float64, bool) {
	var total float64
	var count int
	for _, stat := range stats {
		if stat.PTS >= segment.Start && stat.PTS < segment.End {
			total += stat.YAvg
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

func blackPixelRatioForSegment(stats []ffmpeg.BlackFrameStat, segment ffmpeg.BlackSegment) (float64, bool) {
	var total float64
	var count int
	for _, stat := range stats {
		if stat.Time >= segment.Start && stat.Time < segment.End {
			total += stat.PBlack / 100
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

func edgeRatioForSegment(stats []ffmpeg.FrameSignalStat, segment ffmpeg.BlackSegment) (float64, bool) {
	var total float64
	var count int
	for _, stat := range stats {
		if stat.PTS >= segment.Start && stat.PTS < segment.End {
			total += stat.YAvg / 255
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

func hasConfirmationMetrics(metrics map[string]float64) bool {
	_, hasMeanY := metrics["meanY"]
	_, hasBlackPixelRatio := metrics["blackPixelRatio"]
	_, hasEdgeRatio := metrics["edgeRatio"]
	return hasMeanY || hasBlackPixelRatio || hasEdgeRatio
}

func confidenceForMetrics(profile config.Profile, metrics map[string]float64) float64 {
	meanY := metrics["meanY"]
	blackPixelRatio := metrics["blackPixelRatio"]
	edgeRatio := metrics["edgeRatio"]
	edgeThreshold := profile.Thresholds.BlackEdgeRatio
	if edgeThreshold <= 0 {
		edgeThreshold = 0.03
	}

	if meanY <= profile.Thresholds.BlackMeanY &&
		blackPixelRatio >= profile.Thresholds.BlackPixelRatio &&
		edgeRatio <= edgeThreshold {
		return 0.95
	}
	if edgeRatio > edgeThreshold*2 {
		return 0.55
	}
	if meanY <= profile.Thresholds.BlackMeanY || blackPixelRatio >= profile.Thresholds.BlackPixelRatio {
		return 0.82
	}
	return 0.70
}
