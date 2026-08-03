package visualartifact

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
	return "visualartifact"
}

func (Detector) Stage() detector.Stage {
	return detector.StageFrame
}

func (Detector) Enabled(profile config.Profile) bool {
	return profile.Stages.VisualArtifact
}

func (Detector) Run(ctx context.Context, qc detector.Context) ([]schema.Event, error) {
	stats, rawLog, err := ffmpeg.SignalStats(ctx, qc.Profile.Tools.FFmpeg, qc.Input)
	if err != nil {
		return []schema.Event{{
			ID:        "visual_stats_error",
			Type:      "visual_stats_error",
			Category:  "visual",
			Severity:  schema.SeverityMedium,
			Message:   err.Error(),
			Method:    "ffmpeg signalstats",
			Evidence:  schema.Evidence{RawLog: rawLog},
			CreatedAt: time.Now(),
		}}, nil
	}

	events := detectWhiteScreens(stats, qc.Profile, qc.Video.FPS)
	events = append(events, detectGreenScreens(stats, qc.Video.FPS)...)
	events = append(events, detectSimilarityFreezes(stats, qc.Profile, qc.Video.FPS)...)
	events = append(events, detectBrightnessJumps(stats, qc.Video.FPS)...)
	events = append(events, detectLowDetailBlur(ctx, qc)...)
	events = append(events, detectArtifactSpikes(stats, qc.Video.FPS)...)
	return events, nil
}

func detectWhiteScreens(stats []ffmpeg.FrameSignalStat, profile config.Profile, fps float64) []schema.Event {
	threshold := profile.Thresholds.WhiteMeanY
	if threshold <= 0 {
		threshold = 232
	}
	minDuration := 0.8
	if fps > 0 {
		minDuration = math.Max(0.8, 20/fps)
	}

	var events []schema.Event
	var start *ffmpeg.FrameSignalStat
	var last ffmpeg.FrameSignalStat
	var total float64
	var count int
	for _, stat := range stats {
		if stat.YAvg >= threshold {
			if start == nil {
				item := stat
				start = &item
				total = 0
				count = 0
			}
			last = stat
			total += stat.YAvg
			count++
			continue
		}
		if start != nil {
			events = appendWhiteEvent(events, *start, last, total, count, minDuration)
			start = nil
		}
	}
	if start != nil {
		events = appendWhiteEvent(events, *start, last, total, count, minDuration)
	}
	return events
}

func appendWhiteEvent(events []schema.Event, start ffmpeg.FrameSignalStat, end ffmpeg.FrameSignalStat, total float64, count int, minDuration float64) []schema.Event {
	duration := end.PTS - start.PTS
	if duration < minDuration {
		return events
	}
	meanY := total / float64(count)
	return append(events, schema.Event{
		ID:          fmt.Sprintf("white_screen_%03d", len(events)+1),
		Type:        "white_screen",
		Category:    "visual",
		Severity:    severityForDuration(duration),
		StartTimeMs: int64(start.PTS * 1000),
		EndTimeMs:   int64(end.PTS * 1000),
		DurationMs:  int64(duration * 1000),
		Message:     fmt.Sprintf("White screen lasts %.3f seconds", duration),
		Confidence:  0.82,
		Method:      "signalstats meanY",
		Evidence: schema.Evidence{
			FrameStart: start.Frame,
			FrameEnd:   end.Frame,
			Metrics: map[string]float64{
				"meanY":    meanY,
				"duration": duration,
			},
		},
		CreatedAt: time.Now(),
	})
}

func detectBrightnessJumps(stats []ffmpeg.FrameSignalStat, fps float64) []schema.Event {
	if len(stats) < 2 {
		return nil
	}
	threshold := 90.0
	events := make([]schema.Event, 0)
	for index := 1; index < len(stats); index++ {
		delta := math.Abs(stats[index].YAvg - stats[index-1].YAvg)
		if delta < threshold {
			continue
		}
		durationMs := int64(0)
		if fps > 0 {
			durationMs = int64((1 / fps) * 1000)
		}
		events = append(events, schema.Event{
			ID:          fmt.Sprintf("brightness_jump_%03d", len(events)+1),
			Type:        "brightness_jump",
			Category:    "visual",
			Severity:    schema.SeverityLow,
			StartTimeMs: int64(stats[index-1].PTS * 1000),
			EndTimeMs:   int64(stats[index].PTS * 1000),
			DurationMs:  durationMs,
			Message:     fmt.Sprintf("Brightness jump %.3f", delta),
			Confidence:  0.72,
			Method:      "signalstats meanY delta",
			Evidence: schema.Evidence{
				FrameStart: stats[index-1].Frame,
				FrameEnd:   stats[index].Frame,
				Metrics: map[string]float64{
					"previousMeanY": stats[index-1].YAvg,
					"currentMeanY":  stats[index].YAvg,
					"delta":         delta,
				},
			},
			CreatedAt: time.Now(),
		})
	}
	return events
}

func detectLowDetailBlur(ctx context.Context, qc detector.Context) []schema.Event {
	edgeStats, _, err := ffmpeg.EdgeSignalStats(ctx, qc.Profile.Tools.FFmpeg, qc.Input)
	if err != nil || len(edgeStats) == 0 {
		return nil
	}
	var total float64
	for _, stat := range edgeStats {
		total += stat.YAvg / 255
	}
	edgeRatio := total / float64(len(edgeStats))
	if edgeRatio >= 0.008 {
		return nil
	}
	durationMs := int64(qc.Video.Duration * 1000)
	return []schema.Event{{
		ID:          "blur_low_detail_001",
		Type:        "blur_low_detail",
		Category:    "visual",
		Severity:    schema.SeverityLow,
		StartTimeMs: 0,
		EndTimeMs:   durationMs,
		DurationMs:  durationMs,
		Message:     fmt.Sprintf("Low edge detail ratio %.5f", edgeRatio),
		Confidence:  0.62,
		Method:      "edgedetect signalstats",
		Evidence: schema.Evidence{
			Metrics: map[string]float64{
				"edgeRatio": edgeRatio,
			},
		},
		CreatedAt: time.Now(),
	}}
}

func detectArtifactSpikes(stats []ffmpeg.FrameSignalStat, fps float64) []schema.Event {
	events := make([]schema.Event, 0)
	const minConsecutiveFrames = 3
	var startIndex = -1
	var totalYDif float64

	for index := 1; index < len(stats); index++ {
		stat := stats[index]
		lumaRange := stat.YMax - stat.YMin
		if stat.YDif >= 55 && lumaRange >= 180 {
			if startIndex == -1 {
				startIndex = index
				totalYDif = 0
			}
			totalYDif += stat.YDif
			continue
		}
		events = appendArtifactEvent(events, stats, startIndex, index-1, totalYDif, minConsecutiveFrames, fps)
		startIndex = -1
		totalYDif = 0
	}
	if startIndex != -1 {
		events = appendArtifactEvent(events, stats, startIndex, len(stats)-1, totalYDif, minConsecutiveFrames, fps)
	}
	return events
}

func appendArtifactEvent(events []schema.Event, stats []ffmpeg.FrameSignalStat, startIndex int, endIndex int, totalYDif float64, minConsecutiveFrames int, fps float64) []schema.Event {
	if startIndex < 0 || endIndex < startIndex {
		return events
	}
	count := endIndex - startIndex + 1
	if count < minConsecutiveFrames {
		return events
	}
	start := stats[startIndex]
	end := stats[endIndex]
	duration := end.PTS - start.PTS
	if fps > 0 {
		duration += 1 / fps
	}
	return append(events, schema.Event{
		ID:          fmt.Sprintf("artifact_spike_%03d", len(events)+1),
		Type:        "artifact_spike",
		Category:    "visual",
		Severity:    schema.SeverityLow,
		StartTimeMs: int64(start.PTS * 1000),
		EndTimeMs:   int64((start.PTS + duration) * 1000),
		DurationMs:  int64(duration * 1000),
		Message:     fmt.Sprintf("Possible visual artifact spike for %d frames", count),
		Confidence:  0.6,
		Method:      "signalstats yd_if+luma_range",
		Evidence: schema.Evidence{
			FrameStart: start.Frame,
			FrameEnd:   end.Frame,
			Metrics: map[string]float64{
				"meanYDif":   totalYDif / float64(count),
				"frameCount": float64(count),
				"duration":   duration,
			},
		},
		CreatedAt: time.Now(),
	})
}

func detectGreenScreens(stats []ffmpeg.FrameSignalStat, fps float64) []schema.Event {
	minDuration := 0.8
	if fps > 0 {
		minDuration = math.Max(0.8, 20/fps)
	}

	var events []schema.Event
	var start *ffmpeg.FrameSignalStat
	var last ffmpeg.FrameSignalStat
	var totalY float64
	var totalU float64
	var totalV float64
	var count int
	for _, stat := range stats {
		if isGreenFrame(stat) {
			if start == nil {
				item := stat
				start = &item
				totalY = 0
				totalU = 0
				totalV = 0
				count = 0
			}
			last = stat
			totalY += stat.YAvg
			totalU += stat.UAvg
			totalV += stat.VAvg
			count++
			continue
		}
		if start != nil {
			events = appendGreenEvent(events, *start, last, totalY, totalU, totalV, count, minDuration)
			start = nil
		}
	}
	if start != nil {
		events = appendGreenEvent(events, *start, last, totalY, totalU, totalV, count, minDuration)
	}
	return events
}

func detectSimilarityFreezes(stats []ffmpeg.FrameSignalStat, profile config.Profile, fps float64) []schema.Event {
	if len(stats) == 0 {
		return nil
	}
	minDuration := float64(profile.Thresholds.FreezeMinDurationMs) / 1000
	if minDuration <= 0 {
		minDuration = 1
	}
	threshold := 0.8

	var events []schema.Event
	var start *ffmpeg.FrameSignalStat
	var last ffmpeg.FrameSignalStat
	var total float64
	var count int
	for _, stat := range stats {
		if stat.Frame > 0 && stat.YDif <= threshold {
			if start == nil {
				item := stat
				start = &item
				total = 0
				count = 0
			}
			last = stat
			total += stat.YDif
			count++
			continue
		}
		if start != nil {
			events = appendSimilarityFreezeEvent(events, *start, last, total, count, minDuration, fps)
			start = nil
		}
	}
	if start != nil {
		events = appendSimilarityFreezeEvent(events, *start, last, total, count, minDuration, fps)
	}
	return events
}

func appendSimilarityFreezeEvent(events []schema.Event, start ffmpeg.FrameSignalStat, end ffmpeg.FrameSignalStat, total float64, count int, minDuration float64, fps float64) []schema.Event {
	duration := end.PTS - start.PTS
	if fps > 0 {
		duration += 1 / fps
	}
	if duration < minDuration {
		return events
	}
	meanDiff := total / float64(count)
	return append(events, schema.Event{
		ID:          fmt.Sprintf("freeze_similarity_%03d", len(events)+1),
		Type:        "freeze_similarity",
		Category:    "visual",
		Severity:    severityForDuration(duration),
		StartTimeMs: int64(start.PTS * 1000),
		EndTimeMs:   int64((start.PTS + duration) * 1000),
		DurationMs:  int64(duration * 1000),
		Message:     fmt.Sprintf("Low frame difference lasts %.3f seconds", duration),
		Confidence:  0.74,
		Method:      "signalstats yd_if",
		Evidence: schema.Evidence{
			FrameStart: start.Frame,
			FrameEnd:   end.Frame,
			Metrics: map[string]float64{
				"meanYDif": meanDiff,
				"duration": duration,
			},
		},
		CreatedAt: time.Now(),
	})
}

func appendGreenEvent(events []schema.Event, start ffmpeg.FrameSignalStat, end ffmpeg.FrameSignalStat, totalY float64, totalU float64, totalV float64, count int, minDuration float64) []schema.Event {
	duration := end.PTS - start.PTS
	if duration < minDuration {
		return events
	}
	return append(events, schema.Event{
		ID:          fmt.Sprintf("green_screen_%03d", len(events)+1),
		Type:        "green_screen",
		Category:    "visual",
		Severity:    severityForDuration(duration),
		StartTimeMs: int64(start.PTS * 1000),
		EndTimeMs:   int64(end.PTS * 1000),
		DurationMs:  int64(duration * 1000),
		Message:     fmt.Sprintf("Green screen lasts %.3f seconds", duration),
		Confidence:  0.84,
		Method:      "signalstats yuv",
		Evidence: schema.Evidence{
			FrameStart: start.Frame,
			FrameEnd:   end.Frame,
			Metrics: map[string]float64{
				"meanY":    totalY / float64(count),
				"meanU":    totalU / float64(count),
				"meanV":    totalV / float64(count),
				"duration": duration,
			},
		},
		CreatedAt: time.Now(),
	})
}

func isGreenFrame(stat ffmpeg.FrameSignalStat) bool {
	return stat.YAvg >= 70 &&
		stat.UAvg >= 40 && stat.UAvg <= 130 &&
		stat.VAvg >= 30 && stat.VAvg <= 120 &&
		stat.UAvg+stat.VAvg <= 220
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
