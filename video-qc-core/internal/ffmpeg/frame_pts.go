package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type FramePTS struct {
	Index int
	PTS   float64
}

func FramePTSList(ctx context.Context, ffprobePath string, input string) ([]FramePTS, string, error) {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}

	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "frame=best_effort_timestamp_time",
		"-of", "csv=p=0",
		input,
	)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	log := strings.TrimSpace(output.String())
	if err != nil {
		return nil, log, fmt.Errorf("ffprobe frame pts failed: %w", err)
	}
	return ParseFramePTSLog(log), log, nil
}

func ParseFramePTSLog(log string) []FramePTS {
	items := make([]FramePTS, 0)
	for _, line := range strings.Split(log, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "N/A" {
			continue
		}
		fields := strings.Split(line, ",")
		value := strings.TrimSpace(fields[0])
		pts, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		items = append(items, FramePTS{
			Index: len(items),
			PTS:   pts,
		})
	}
	return items
}
