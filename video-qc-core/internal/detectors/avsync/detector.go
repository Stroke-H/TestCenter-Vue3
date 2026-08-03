package avsync

import (
	"context"
	"fmt"
	"math"
	"strconv"
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
	return "avsync"
}

func (Detector) Stage() detector.Stage {
	return detector.StageAudio
}

func (Detector) Enabled(profile config.Profile) bool {
	return profile.Stages.AVSync
}

func (Detector) Run(ctx context.Context, qc detector.Context) ([]schema.Event, error) {
	probe, _, err := ffmpeg.Probe(ctx, qc.Profile.Tools.FFProbe, qc.Input)
	if err != nil {
		return []schema.Event{{
			ID:        "avsync_probe_error",
			Type:      "avsync_probe_error",
			Category:  "audio",
			Severity:  schema.SeverityMedium,
			Message:   err.Error(),
			Method:    "ffprobe stream timing",
			CreatedAt: time.Now(),
		}}, nil
	}

	video, audio, ok := firstVideoAudioStreams(probe.Streams)
	if !ok {
		return nil, nil
	}
	videoStart := parseFloat(video.StartTime)
	audioStart := parseFloat(audio.StartTime)
	videoDuration := parseFloat(video.Duration)
	audioDuration := parseFloat(audio.Duration)
	startOffsetMs := (audioStart - videoStart) * 1000
	durationOffsetMs := (audioDuration - videoDuration) * 1000

	events := make([]schema.Event, 0)
	if math.Abs(startOffsetMs) >= 250 || math.Abs(durationOffsetMs) >= 500 {
		events = append(events, schema.Event{
			ID:          "av_sync_001",
			Type:        "av_sync_offset",
			Category:    "audio",
			Severity:    severityForOffset(startOffsetMs, durationOffsetMs),
			StartTimeMs: 0,
			EndTimeMs:   int64(qc.Video.Duration * 1000),
			DurationMs:  int64(qc.Video.Duration * 1000),
			Message:     fmt.Sprintf("AV sync offset start=%.0fms duration=%.0fms", startOffsetMs, durationOffsetMs),
			Confidence:  0.78,
			Method:      "ffprobe stream timing",
			Evidence: schema.Evidence{
				Metrics: map[string]float64{
					"audioStartMs":     audioStart * 1000,
					"videoStartMs":     videoStart * 1000,
					"startOffsetMs":    startOffsetMs,
					"audioDurationMs":  audioDuration * 1000,
					"videoDurationMs":  videoDuration * 1000,
					"durationOffsetMs": durationOffsetMs,
				},
			},
			CreatedAt: time.Now(),
		})
	}
	return events, nil
}

func firstVideoAudioStreams(streams []ffmpeg.ProbeStream) (ffmpeg.ProbeStream, ffmpeg.ProbeStream, bool) {
	var video ffmpeg.ProbeStream
	var audio ffmpeg.ProbeStream
	for _, stream := range streams {
		if stream.CodecType == "video" && video.CodecType == "" {
			video = stream
		}
		if stream.CodecType == "audio" && audio.CodecType == "" {
			audio = stream
		}
	}
	return video, audio, video.CodecType != "" && audio.CodecType != ""
}

func parseFloat(value string) float64 {
	result, _ := strconv.ParseFloat(value, 64)
	return result
}

func severityForOffset(startOffsetMs float64, durationOffsetMs float64) schema.Severity {
	offset := math.Max(math.Abs(startOffsetMs), math.Abs(durationOffsetMs))
	if offset >= 1000 {
		return schema.SeverityHigh
	}
	if offset >= 500 {
		return schema.SeverityMedium
	}
	return schema.SeverityLow
}
