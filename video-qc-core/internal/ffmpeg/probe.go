package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"video-qc-core/pkg/schema"
)

type ProbeResult struct {
	Streams []ProbeStream `json:"streams"`
	Format  ProbeFormat   `json:"format"`
}

type ProbeStream struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	AvgFrameRate string `json:"avg_frame_rate"`
	RFrameRate   string `json:"r_frame_rate"`
	StartTime    string `json:"start_time"`
	Duration     string `json:"duration"`
}

type ProbeFormat struct {
	Duration string `json:"duration"`
}

func Probe(ctx context.Context, ffprobePath string, input string) (ProbeResult, schema.VideoInfo, error) {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}

	cmd := exec.CommandContext(ctx, ffprobePath, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return ProbeResult{}, schema.VideoInfo{}, fmt.Errorf("ffprobe failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var result ProbeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return ProbeResult{}, schema.VideoInfo{}, fmt.Errorf("parse ffprobe json: %w", err)
	}

	video := schema.VideoInfo{Input: input}
	video.Duration, _ = strconv.ParseFloat(result.Format.Duration, 64)
	for _, stream := range result.Streams {
		if stream.CodecType != "video" {
			continue
		}
		video.Codec = stream.CodecName
		if stream.Width > 0 && stream.Height > 0 {
			video.Resolution = fmt.Sprintf("%dx%d", stream.Width, stream.Height)
		}
		video.FPS = parseFrameRate(stream.AvgFrameRate)
		if video.FPS == 0 {
			video.FPS = parseFrameRate(stream.RFrameRate)
		}
		break
	}

	return result, video, nil
}

func parseFrameRate(value string) float64 {
	if value == "" || value == "0/0" {
		return 0
	}
	parts := strings.Split(value, "/")
	if len(parts) == 1 {
		fps, _ := strconv.ParseFloat(value, 64)
		return fps
	}
	numerator, _ := strconv.ParseFloat(parts[0], 64)
	denominator, _ := strconv.ParseFloat(parts[1], 64)
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

func ValidateProbe(result ProbeResult, video schema.VideoInfo) []string {
	var problems []string
	if len(result.Streams) == 0 {
		problems = append(problems, "no streams found")
	}
	if video.Duration <= 0 {
		problems = append(problems, "duration is missing or invalid")
	}
	if video.Codec == "" {
		problems = append(problems, "video codec is missing")
	}
	if video.Resolution == "" {
		problems = append(problems, "video resolution is missing")
	}
	return problems
}
