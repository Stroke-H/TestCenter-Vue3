package services

import "testing"

func TestDescriptionLanguageErrorsGoToOtherFailures(t *testing.T) {
	failure := dramaFailureSummary{DramaID: "111111111111111111111111", Errors: []string{
		"• [简介语种] 识别语种=en，与 lang=ja 不一致。description=example；内容指纹=abc",
	}}
	playback, groups, categories, other := splitScheduledDramaFailures([]dramaFailureSummary{failure})
	if len(playback) != 0 || len(groups) != 0 || len(categories) != 0 || len(other) != 1 {
		t.Fatalf("unexpected grouping: playback=%d groups=%d categories=%d other=%d", len(playback), len(groups), len(categories), len(other))
	}
}
