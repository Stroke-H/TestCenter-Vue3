package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Profile struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Stages      Stages     `json:"stages"`
	Thresholds  Thresholds `json:"thresholds"`
	Limits      Limits     `json:"limits"`
	Tools       Tools      `json:"tools"`
}

type Stages struct {
	FileCheck      bool `json:"fileCheck"`
	DecodeCheck    bool `json:"decodeCheck"`
	BlackScreen    bool `json:"blackScreen"`
	FrameDrop      bool `json:"frameDrop"`
	Freeze         bool `json:"freeze"`
	VisualArtifact bool `json:"visualArtifact"`
	AVSync         bool `json:"avSync"`
	DeepMotion     bool `json:"deepMotion"`
}

type Thresholds struct {
	BlackMeanY          float64 `json:"blackMeanY"`
	BlackPixelRatio     float64 `json:"blackPixelRatio"`
	BlackEdgeRatio      float64 `json:"blackEdgeRatio"`
	BlackMinDurationMs  int64   `json:"blackMinDurationMs"`
	FreezeSSIM          float64 `json:"freezeSsim"`
	FreezeMinDurationMs int64   `json:"freezeMinDurationMs"`
	PTSGapRatio         float64 `json:"ptsGapRatio"`
	WhiteMeanY          float64 `json:"whiteMeanY"`
	WhitePixelRatio     float64 `json:"whitePixelRatio"`
}

type Limits struct {
	MaxWorkers         int `json:"maxWorkers"`
	TaskTimeoutSeconds int `json:"taskTimeoutSeconds"`
}

type Tools struct {
	FFProbe string `json:"ffprobe"`
	FFmpeg  string `json:"ffmpeg"`
}

func LoadProfile(path string) (Profile, error) {
	if profile, ok, err := loadBuiltinProfile(path); ok || err != nil {
		return profile, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}

	return parseProfile(data)
}

func parseProfile(data []byte) (Profile, error) {
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	if profile.Name == "" {
		return Profile{}, fmt.Errorf("profile name is required")
	}
	if profile.Tools.FFProbe == "" {
		profile.Tools.FFProbe = "ffprobe"
	}
	if profile.Tools.FFmpeg == "" {
		profile.Tools.FFmpeg = "ffmpeg"
	}
	return profile, nil
}

func loadBuiltinProfile(ref string) (Profile, bool, error) {
	name := strings.ToLower(strings.TrimSpace(ref))
	if name == "" {
		name = "default"
	}
	if strings.ContainsAny(name, `/\`) {
		base := filepath.Base(name)
		ext := filepath.Ext(base)
		if ext == ".json" {
			name = strings.TrimSuffix(base, ext)
		} else {
			return Profile{}, false, nil
		}
	}

	raw, ok := builtinProfiles[name]
	if !ok {
		return Profile{}, false, nil
	}
	profile, err := parseProfile([]byte(raw))
	return profile, true, err
}
