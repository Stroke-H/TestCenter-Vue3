package services

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func filterDramaRetryScope(ids, eligible []string) (retained, removed []string) {
	set := map[string]bool{}
	for _, id := range eligible {
		set[id] = true
	}
	retained, removed = []string{}, []string{}
	for _, id := range uniqueNonEmptyStrings(ids) {
		if set[id] {
			retained = append(retained, id)
		} else {
			removed = append(removed, id)
		}
	}
	return
}

// Preserve first-pass evidence, but exclude records that moved outside the
// online allowlist from final notifications. Scope changes remain explicit.
func filterDramaScopeExcludedFailures(failures []dramaFailureSummary, dir string) []dramaFailureSummary {
	b, err := os.ReadFile(filepath.Join(dir, "drama_retry_scope_changes.json"))
	if err != nil {
		return failures
	}
	var changes struct {
		RemovedIDs []string `json:"removedIds"`
	}
	if json.Unmarshal(b, &changes) != nil {
		return failures
	}
	removed := map[string]bool{}
	for _, id := range changes.RemovedIDs {
		removed[id] = true
	}
	result := []dramaFailureSummary{}
	for _, f := range failures {
		if !removed[f.DramaID] {
			result = append(result, f)
		}
	}
	return result
}
