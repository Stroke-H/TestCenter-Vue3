package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type FreezeSegment struct {
	Start    float64
	End      float64
	Duration float64
}

func FreezeDetect(ctx context.Context, ffmpegPath string, input string, minDurationMs int64) ([]FreezeSegment, string, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	minDurationSeconds := float64(minDurationMs) / 1000
	if minDurationSeconds <= 0 {
		minDurationSeconds = 1
	}

	filter := fmt.Sprintf("freezedetect=n=-60dB:d=%.3f", minDurationSeconds)
	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-nostats", "-i", input, "-vf", filter, "-an", "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	err := cmd.Run()
	log := strings.TrimSpace(stderr.String())
	if err != nil {
		return nil, log, fmt.Errorf("ffmpeg freezedetect failed: %w", err)
	}
	return ParseFreezeDetectLog(log), log, nil
}

func ParseFreezeDetectLog(log string) []FreezeSegment {
	startPattern := regexp.MustCompile(`freeze_start:\s*([0-9.]+)`)
	durationPattern := regexp.MustCompile(`freeze_duration:\s*([0-9.]+)`)
	endPattern := regexp.MustCompile(`freeze_end:\s*([0-9.]+)(?:\s+\|\s+(?:lavfi\.freezedetect\.)?freeze_duration:\s*([0-9.]+))?`)

	segments := make([]FreezeSegment, 0)
	var pendingStart float64
	var pendingDuration float64
	hasStart := false
	for _, line := range strings.Split(log, "\n") {
		if match := startPattern.FindStringSubmatch(line); len(match) == 2 {
			pendingStart, _ = strconv.ParseFloat(match[1], 64)
			pendingDuration = 0
			hasStart = true
			continue
		}
		if match := endPattern.FindStringSubmatch(line); len(match) >= 2 && hasStart {
			end, _ := strconv.ParseFloat(match[1], 64)
			duration := pendingDuration
			if len(match) >= 3 && match[2] != "" {
				duration, _ = strconv.ParseFloat(match[2], 64)
			}
			if duration <= 0 {
				duration = end - pendingStart
			}
			segments = append(segments, FreezeSegment{
				Start:    pendingStart,
				End:      end,
				Duration: duration,
			})
			hasStart = false
			continue
		}
		if match := durationPattern.FindStringSubmatch(line); len(match) == 2 && hasStart {
			pendingDuration, _ = strconv.ParseFloat(match[1], 64)
		}
	}
	return segments
}
