package pipeline

import (
	"context"
	"fmt"
	"time"

	"video-qc-core/pkg/config"
	"video-qc-core/pkg/detector"
	"video-qc-core/pkg/schema"
)

type Runner struct {
	detectors []detector.Detector
}

type Options struct {
	TaskID  string
	Input   string
	WorkDir string
	Profile config.Profile
}

func NewRunner(detectors ...detector.Detector) *Runner {
	return &Runner{detectors: detectors}
}

func (runner *Runner) Run(ctx context.Context, options Options) (schema.Report, error) {
	if options.TaskID == "" {
		return schema.Report{}, fmt.Errorf("task id is required")
	}
	if options.Input == "" {
		return schema.Report{}, fmt.Errorf("input is required")
	}

	report := schema.Report{
		TaskID:    options.TaskID,
		Status:    schema.StatusQueued,
		Profile:   options.Profile.Name,
		StartedAt: time.Now(),
		Video: schema.VideoInfo{
			Input: options.Input,
		},
		Events: []schema.Event{},
	}

	qcContext := detector.Context{
		TaskID:     options.TaskID,
		Input:      options.Input,
		WorkDir:    options.WorkDir,
		Profile:    options.Profile,
		Video:      report.Video,
		Attributes: map[string]any{},
	}

	for _, status := range []schema.Status{
		schema.StatusProbing,
		schema.StatusDecoding,
		schema.StatusAnalyzing,
		schema.StatusReporting,
	} {
		if !CanTransition(report.Status, status) {
			return report, fmt.Errorf("invalid status transition: %s -> %s", report.Status, status)
		}
		report.Status = status
		report.Events = append(report.Events, runner.runStage(ctx, status, qcContext)...)
		if video, ok := qcContext.Attributes["videoInfo"].(schema.VideoInfo); ok {
			report.Video = video
			qcContext.Video = video
		}
	}

	report.FinishedAt = time.Now()
	report.Events = reconcileEvents(report.Events)
	report.RecalculateSummary()
	if report.Summary.Critical > 0 || report.Summary.High > 0 {
		report.Status = schema.StatusFailed
	} else {
		report.Status = schema.StatusSuccess
	}
	return report, nil
}

func (runner *Runner) runStage(ctx context.Context, status schema.Status, qcContext detector.Context) []schema.Event {
	stages := detectorStages(status)
	if len(stages) == 0 {
		return nil
	}

	var events []schema.Event
	for _, item := range runner.detectors {
		if !containsStage(stages, item.Stage()) || !item.Enabled(qcContext.Profile) {
			continue
		}
		detected, err := item.Run(ctx, qcContext)
		if err != nil {
			events = append(events, schema.Event{
				ID:        fmt.Sprintf("%s_error_%d", item.Name(), len(events)+1),
				Type:      "detector_error",
				Category:  string(item.Stage()),
				Severity:  schema.SeverityMedium,
				Message:   fmt.Sprintf("%s failed: %v", item.Name(), err),
				Method:    item.Name(),
				CreatedAt: time.Now(),
			})
			continue
		}
		events = append(events, detected...)
	}
	return events
}

func detectorStages(status schema.Status) []detector.Stage {
	switch status {
	case schema.StatusProbing:
		return []detector.Stage{detector.StageFile}
	case schema.StatusDecoding:
		return []detector.Stage{detector.StageDecode}
	case schema.StatusAnalyzing:
		return []detector.Stage{detector.StageFrame, detector.StageAudio}
	case schema.StatusReporting:
		return []detector.Stage{detector.StageAggregate}
	default:
		return nil
	}
}

func containsStage(stages []detector.Stage, stage detector.Stage) bool {
	for _, item := range stages {
		if item == stage {
			return true
		}
	}
	return false
}

func reconcileEvents(events []schema.Event) []schema.Event {
	if len(events) == 0 {
		return events
	}
	filtered := make([]schema.Event, 0, len(events))
	for _, event := range events {
		if event.Type == "freeze_similarity" && overlapsEventType(event, events, "freeze", 0.5) {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func overlapsEventType(event schema.Event, events []schema.Event, eventType string, minRatio float64) bool {
	for _, candidate := range events {
		if candidate.Type != eventType {
			continue
		}
		overlap := overlapMs(event.StartTimeMs, event.EndTimeMs, candidate.StartTimeMs, candidate.EndTimeMs)
		if overlap <= 0 || event.DurationMs <= 0 {
			continue
		}
		if float64(overlap)/float64(event.DurationMs) >= minRatio {
			return true
		}
	}
	return false
}

func overlapMs(startA int64, endA int64, startB int64, endB int64) int64 {
	start := startA
	if startB > start {
		start = startB
	}
	end := endA
	if endB < end {
		end = endB
	}
	if end <= start {
		return 0
	}
	return end - start
}
