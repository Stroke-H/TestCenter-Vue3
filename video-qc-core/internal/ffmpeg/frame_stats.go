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

type FrameSignalStat struct {
	Frame int
	PTS   float64
	YAvg  float64
	UAvg  float64
	VAvg  float64
	YDif  float64
	YMin  float64
	YMax  float64
}

type BlackFrameStat struct {
	Frame  int
	Time   float64
	PBlack float64
}

func SignalStats(ctx context.Context, ffmpegPath string, input string) ([]FrameSignalStat, string, error) {
	return signalStatsWithFilter(ctx, ffmpegPath, input, "signalstats,metadata=print")
}

func EdgeSignalStats(ctx context.Context, ffmpegPath string, input string) ([]FrameSignalStat, string, error) {
	return signalStatsWithFilter(ctx, ffmpegPath, input, "edgedetect=low=0.1:high=0.4,format=gray,signalstats,metadata=print")
}

func signalStatsWithFilter(ctx context.Context, ffmpegPath string, input string, filter string) ([]FrameSignalStat, string, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-nostats", "-i", input, "-vf", filter, "-an", "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	err := cmd.Run()
	log := strings.TrimSpace(stderr.String())
	if err != nil {
		return nil, log, fmt.Errorf("ffmpeg signalstats failed: %w", err)
	}
	return ParseSignalStatsLog(log), log, nil
}

func BlackFrameStats(ctx context.Context, ffmpegPath string, input string) ([]BlackFrameStat, string, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-nostats", "-i", input, "-vf", "blackframe=amount=0:threshold=32", "-an", "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	err := cmd.Run()
	log := strings.TrimSpace(stderr.String())
	if err != nil {
		return nil, log, fmt.Errorf("ffmpeg blackframe failed: %w", err)
	}
	return ParseBlackFrameStatsLog(log), log, nil
}

func ParseSignalStatsLog(log string) []FrameSignalStat {
	headerPattern := regexp.MustCompile(`frame:([0-9]+)\s+pts:[^\s]+\s+pts_time:([0-9.]+)`)
	yAvgPattern := regexp.MustCompile(`lavfi\.signalstats\.YAVG=([0-9.]+)`)
	uAvgPattern := regexp.MustCompile(`lavfi\.signalstats\.UAVG=([0-9.]+)`)
	vAvgPattern := regexp.MustCompile(`lavfi\.signalstats\.VAVG=([0-9.]+)`)
	yDifPattern := regexp.MustCompile(`lavfi\.signalstats\.YDIF=([0-9.]+)`)
	yMinPattern := regexp.MustCompile(`lavfi\.signalstats\.YMIN=([0-9.]+)`)
	yMaxPattern := regexp.MustCompile(`lavfi\.signalstats\.YMAX=([0-9.]+)`)

	stats := make([]FrameSignalStat, 0)
	current := FrameSignalStat{Frame: -1}
	for _, line := range strings.Split(log, "\n") {
		if match := headerPattern.FindStringSubmatch(line); len(match) == 3 {
			if current.Frame >= 0 {
				stats = append(stats, current)
			}
			frame, _ := strconv.Atoi(match[1])
			pts, _ := strconv.ParseFloat(match[2], 64)
			current = FrameSignalStat{Frame: frame, PTS: pts}
			continue
		}
		if current.Frame >= 0 {
			if match := yAvgPattern.FindStringSubmatch(line); len(match) == 2 {
				current.YAvg, _ = strconv.ParseFloat(match[1], 64)
			}
			if match := uAvgPattern.FindStringSubmatch(line); len(match) == 2 {
				current.UAvg, _ = strconv.ParseFloat(match[1], 64)
			}
			if match := vAvgPattern.FindStringSubmatch(line); len(match) == 2 {
				current.VAvg, _ = strconv.ParseFloat(match[1], 64)
			}
			if match := yDifPattern.FindStringSubmatch(line); len(match) == 2 {
				current.YDif, _ = strconv.ParseFloat(match[1], 64)
			}
			if match := yMinPattern.FindStringSubmatch(line); len(match) == 2 {
				current.YMin, _ = strconv.ParseFloat(match[1], 64)
			}
			if match := yMaxPattern.FindStringSubmatch(line); len(match) == 2 {
				current.YMax, _ = strconv.ParseFloat(match[1], 64)
			}
		}
	}
	if current.Frame >= 0 {
		stats = append(stats, current)
	}
	return stats
}

func ParseBlackFrameStatsLog(log string) []BlackFrameStat {
	pattern := regexp.MustCompile(`frame:([0-9]+)\s+pblack:([0-9.]+)\s+pts:[^\s]+\s+t:([0-9.]+)`)
	matches := pattern.FindAllStringSubmatch(log, -1)
	stats := make([]BlackFrameStat, 0, len(matches))
	for _, match := range matches {
		frame, _ := strconv.Atoi(match[1])
		pblack, _ := strconv.ParseFloat(match[2], 64)
		timestamp, _ := strconv.ParseFloat(match[3], 64)
		stats = append(stats, BlackFrameStat{
			Frame:  frame,
			Time:   timestamp,
			PBlack: pblack,
		})
	}
	return stats
}
