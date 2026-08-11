package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func CaptureFrame(ctx context.Context, ffmpegPath string, input string, timestamp float64, output string) (string, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if timestamp < 0 {
		timestamp = 0
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-nostats", "-y", "-ss", fmt.Sprintf("%.3f", timestamp), "-i", input, "-frames:v", "1", "-q:v", "2", output)
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	err := cmd.Run()
	log := strings.TrimSpace(stderr.String())
	if err != nil {
		return log, fmt.Errorf("ffmpeg capture frame failed: %w", err)
	}
	return log, nil
}
