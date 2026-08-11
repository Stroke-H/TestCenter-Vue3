package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"video-qc-core/internal/detectors/avsync"
	"video-qc-core/internal/detectors/blackscreen"
	"video-qc-core/internal/detectors/decodecheck"
	"video-qc-core/internal/detectors/deepmotion"
	"video-qc-core/internal/detectors/filecheck"
	"video-qc-core/internal/detectors/framedrop"
	"video-qc-core/internal/detectors/freeze"
	"video-qc-core/internal/detectors/visualartifact"
	"video-qc-core/pkg/config"
	"video-qc-core/pkg/pipeline"
)

var version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "check":
		if err := runCheck(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "version":
		fmt.Println(version)
	default:
		printUsage()
		os.Exit(2)
	}
}

func runCheck(args []string) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	input := flags.String("input", "", "video file path or URL")
	profilePath := flags.String("profile", "default", "profile name (default, fast, deep) or profile JSON path")
	output := flags.String("output", "", "report JSON output path")
	workDir := flags.String("work-dir", "", "task work directory")
	taskID := flags.String("task-id", "", "task id")
	ffmpegPath := flags.String("ffmpeg", "", "override ffmpeg executable path")
	ffprobePath := flags.String("ffprobe", "", "override ffprobe executable path")
	if err := flags.Parse(args); err != nil {
		return err
	}

	profile, err := config.LoadProfile(*profilePath)
	if err != nil {
		return err
	}
	profile.Tools.FFmpeg = resolveBundledTool(profile.Tools.FFmpeg)
	profile.Tools.FFProbe = resolveBundledTool(profile.Tools.FFProbe)
	if *ffmpegPath != "" {
		profile.Tools.FFmpeg = *ffmpegPath
	}
	if *ffprobePath != "" {
		profile.Tools.FFProbe = *ffprobePath
	}
	if *taskID == "" {
		*taskID = fmt.Sprintf("video-qc-%d", time.Now().UnixMilli())
	}

	report, err := pipeline.NewRunner(
		filecheck.New(),
		decodecheck.New(),
		blackscreen.New(),
		framedrop.New(),
		freeze.New(),
		visualartifact.New(),
		avsync.New(),
		deepmotion.New(),
	).Run(context.Background(), pipeline.Options{
		TaskID:  *taskID,
		Input:   *input,
		WorkDir: *workDir,
		Profile: profile,
	})
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	if *output == "" {
		fmt.Println(string(data))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(*output, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

func resolveBundledTool(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	exe, err := os.Executable()
	if err != nil {
		return name
	}
	candidate := filepath.Join(filepath.Dir(exe), name)
	if runtime.GOOS == "windows" && filepath.Ext(candidate) == "" {
		candidate += ".exe"
	}
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return name
}

func printUsage() {
	fmt.Println("video-qc commands:")
	fmt.Println("  check   run a video QC task")
	fmt.Println("  version print CLI version")
}
