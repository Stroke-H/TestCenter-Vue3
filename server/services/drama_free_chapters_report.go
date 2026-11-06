package services

import "strings"

func splitDramaFreeChapterFailures(failures []dramaFailureSummary) (free, remaining []dramaFailureSummary) {
	return splitDramaTaggedFailures(failures, "[免费集数]")
}

func splitDramaCopyrightFailures(failures []dramaFailureSummary) (copyright, remaining []dramaFailureSummary) {
	return splitDramaTaggedFailures(failures, "[版权规则]")
}

func splitDramaTaggedFailures(failures []dramaFailureSummary, prefix string) (free, remaining []dramaFailureSummary) {
	for _, failure := range failures {
		a, b := failure, failure
		a.Errors, b.Errors = nil, nil
		for _, line := range failure.Errors {
			if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "•")), prefix) {
				a.Errors = append(a.Errors, line)
			} else {
				b.Errors = append(b.Errors, line)
			}
		}
		if len(a.Errors) > 0 {
			free = append(free, a)
		}
		if len(b.Errors) > 0 {
			remaining = append(remaining, b)
		}
	}
	return
}
