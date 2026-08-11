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

func Decode(ctx context.Context, ffmpegPath string, input string) (string, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, "-v", "error", "-i", input, "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	err := cmd.Run()
	log := strings.TrimSpace(stderr.String())
	if err != nil {
		return log, fmt.Errorf("ffmpeg decode failed: %w", err)
	}
	if log != "" {
		return log, fmt.Errorf("ffmpeg decode reported errors")
	}
	return "", nil
}

type BlackSegment struct {
	Start    float64
	End      float64
	Duration float64
}

func BlackDetect(ctx context.Context, ffmpegPath string, input string, minDurationMs int64) ([]BlackSegment, string, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	minDurationSeconds := float64(minDurationMs) / 1000
	if minDurationSeconds <= 0 {
		minDurationSeconds = 0.5
	}

	filter := fmt.Sprintf("blackdetect=d=%.3f:pix_th=0.10", minDurationSeconds)
	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-nostats", "-i", input, "-vf", filter, "-an", "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	err := cmd.Run()
	log := strings.TrimSpace(stderr.String())
	if err != nil {
		return nil, log, fmt.Errorf("ffmpeg blackdetect failed: %w", err)
	}
	return ParseBlackDetectLog(log), log, nil
}

func ParseBlackDetectLog(log string) []BlackSegment {
	pattern := regexp.MustCompile(`black_start:([0-9.]+)\s+black_end:([0-9.]+)\s+black_duration:([0-9.]+)`)
	matches := pattern.FindAllStringSubmatch(log, -1)
	segments := make([]BlackSegment, 0, len(matches))
	for _, match := range matches {
		start, _ := strconv.ParseFloat(match[1], 64)
		end, _ := strconv.ParseFloat(match[2], 64)
		duration, _ := strconv.ParseFloat(match[3], 64)
		segments = append(segments, BlackSegment{
			Start:    start,
			End:      end,
			Duration: duration,
		})
	}
	return segments
}
