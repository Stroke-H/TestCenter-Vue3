package schema

import "time"

type Status string

const (
	StatusQueued    Status = "queued"
	StatusProbing   Status = "probing"
	StatusDecoding  Status = "decoding"
	StatusAnalyzing Status = "analyzing"
	StatusReporting Status = "reporting"
	StatusSuccess   Status = "success"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type VideoInfo struct {
	Input      string  `json:"input"`
	Duration   float64 `json:"duration,omitempty"`
	FPS        float64 `json:"fps,omitempty"`
	Resolution string  `json:"resolution,omitempty"`
	Codec      string  `json:"codec,omitempty"`
}

type Summary struct {
	Score    int `json:"score"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

type Evidence struct {
	FrameStart int                `json:"frameStart,omitempty"`
	FrameEnd   int                `json:"frameEnd,omitempty"`
	Thumbnail  string             `json:"thumbnail,omitempty"`
	Metrics    map[string]float64 `json:"metrics,omitempty"`
	RawLog     string             `json:"rawLog,omitempty"`
}

type Event struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Category    string    `json:"category"`
	Severity    Severity  `json:"severity"`
	StartTimeMs int64     `json:"startTimeMs"`
	EndTimeMs   int64     `json:"endTimeMs"`
	DurationMs  int64     `json:"durationMs"`
	Message     string    `json:"message"`
	Confidence  float64   `json:"confidence,omitempty"`
	Method      string    `json:"method,omitempty"`
	Evidence    Evidence  `json:"evidence,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Report struct {
	TaskID     string    `json:"taskId"`
	Status     Status    `json:"status"`
	Profile    string    `json:"profile"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
	Video      VideoInfo `json:"video"`
	Summary    Summary   `json:"summary"`
	Events     []Event   `json:"events"`
	Errors     []string  `json:"errors,omitempty"`
}

func (report *Report) RecalculateSummary() {
	summary := Summary{Score: 100}
	for _, event := range report.Events {
		switch event.Severity {
		case SeverityCritical:
			summary.Critical++
			summary.Score -= 30
		case SeverityHigh:
			summary.High++
			summary.Score -= 15
		case SeverityMedium:
			summary.Medium++
			summary.Score -= 8
		case SeverityLow:
			summary.Low++
			summary.Score -= 3
		default:
			summary.Info++
		}
	}
	if summary.Score < 0 {
		summary.Score = 0
	}
	report.Summary = summary
}
