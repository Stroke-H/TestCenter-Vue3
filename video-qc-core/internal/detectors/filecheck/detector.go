package filecheck

import (
	"context"
	"fmt"
	"strings"
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
	return "filecheck"
}

func (Detector) Stage() detector.Stage {
	return detector.StageFile
}

func (Detector) Enabled(profile config.Profile) bool {
	return profile.Stages.FileCheck
}

func (Detector) Run(ctx context.Context, qc detector.Context) ([]schema.Event, error) {
	result, video, err := ffmpeg.Probe(ctx, qc.Profile.Tools.FFProbe, qc.Input)
	if err != nil {
		return []schema.Event{{
			ID:        "file_probe_error",
			Type:      "file_probe_error",
			Category:  "file",
			Severity:  schema.SeverityCritical,
			Message:   err.Error(),
			Method:    "ffprobe",
			CreatedAt: time.Now(),
		}}, nil
	}

	qc.Attributes["videoInfo"] = video
	problems := ffmpeg.ValidateProbe(result, video)
	if len(problems) == 0 {
		return nil, nil
	}

	return []schema.Event{{
		ID:        "file_probe_invalid",
		Type:      "file_probe_error",
		Category:  "file",
		Severity:  schema.SeverityCritical,
		Message:   fmt.Sprintf("ffprobe metadata invalid: %s", strings.Join(problems, "; ")),
		Method:    "ffprobe",
		CreatedAt: time.Now(),
	}}, nil
}
