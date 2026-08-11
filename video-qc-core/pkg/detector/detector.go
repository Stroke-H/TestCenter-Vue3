package detector

import (
	"context"

	"video-qc-core/pkg/config"
	"video-qc-core/pkg/schema"
)

type Stage string

const (
	StageFile      Stage = "file"
	StageDecode    Stage = "decode"
	StageFrame     Stage = "frame"
	StageAudio     Stage = "audio"
	StageAggregate Stage = "aggregate"
)

type Context struct {
	TaskID     string
	Input      string
	WorkDir    string
	Profile    config.Profile
	Video      schema.VideoInfo
	Attributes map[string]any
}

type Detector interface {
	Name() string
	Stage() Stage
	Enabled(profile config.Profile) bool
	Run(ctx context.Context, qc Context) ([]schema.Event, error)
}
