package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func coverageFixture(t *testing.T, root, dir, phase string, expected, done []string, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
		t.Fatal(err)
	}
	put := func(name string, value any) {
		t.Helper()
		if err := writeDramaJSON(filepath.Join(root, dir, name), value); err != nil {
			t.Fatal(err)
		}
	}
	put("drama_"+phase+"_completion.json", dramaCompletionFile{phase, expected, done})
	failure := dramaFailureSummary{DramaID: id, Errors: []string{"[免费集数] 不符合规则"}}
	if phase == "initial" {
		put("drama_failure_summary.json", map[string]any{"failures": []dramaFailureSummary{failure}, "ruleScope": map[string]any{"version": "v2"}})
		put("drama_retry_candidates.json", dramaRetryCandidateFile{[]dramaRetryCandidate{{DramaID: id, Kind: "business_rules"}}})
	} else {
		put("drama_retry_result.json", map[string]any{"retryIds": expected, "persistentFailures": []dramaRetryFailure{{Name: id, Fails: 1}}, "structuredFailures": []dramaFailureSummary{failure}, "ruleScope": map[string]any{"version": "v2"}})
	}
	name := "drama_check_report.html"
	if phase == "retry" {
		name = "drama_retry_report.html"
	}
	if err := os.WriteFile(filepath.Join(root, dir, name), []byte("<html><body>first batch</body></html>"), 0644); err != nil {
		t.Fatal(err)
	}
}

func coverageEnv(env []string, key string) string {
	for _, value := range env {
		if strings.HasPrefix(value, key+"=") {
			return strings.TrimPrefix(value, key+"=")
		}
	}
	return ""
}

func TestDramaCoverageContinuesOnlyMissingAndMergesBothPhases(t *testing.T) {
	for _, phase := range []string{"initial", "retry"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			expected := []string{"a", "b", "c"}
			coverageFixture(t, root, "report", phase, expected, []string{"a"}, "a")
			calls := 0
			err := finishDramaPhaseCoverage(context.Background(), root, "report", []string{"DRAMA_REPORT_DIR=stale", "DRAMA_CASE_IDS_FILE=stale", "KEEP=yes"}, phase, expected, nil, func(env []string) error {
				calls++
				var ids []string
				b, err := os.ReadFile(coverageEnv(env, "DRAMA_CASE_IDS_FILE"))
				if err != nil {
					return err
				}
				if err = json.Unmarshal(b, &ids); err != nil {
					return err
				}
				if strings.Join(ids, ",") != "b,c" {
					t.Fatalf("reran completed ID or lost missing IDs: %v", ids)
				}
				if coverageEnv(env, "KEEP") != "yes" || coverageEnv(env, "DRAMA_RETRY_PHASE") != map[bool]string{true: "1", false: "0"}[phase == "retry"] {
					t.Fatal(env)
				}
				coverageFixture(t, root, coverageEnv(env, "DRAMA_REPORT_DIR"), phase, ids, ids, "b")
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatal(calls)
			}
			var audit dramaCoverageFile
			b, _ := os.ReadFile(filepath.Join(root, "report", "drama_"+phase+"_coverage.json"))
			if err = json.Unmarshal(b, &audit); err != nil {
				t.Fatal(err)
			}
			if !audit.Complete || len(audit.CompletedIDs) != 3 || len(audit.MissingIDs) != 0 || len(audit.Rounds) != 2 {
				t.Fatalf("%+v", audit)
			}
			name := "drama_failure_summary.json"
			if phase == "retry" {
				name = "drama_retry_result.json"
			}
			b, _ = os.ReadFile(filepath.Join(root, "report", name))
			if !strings.Contains(string(b), `"a"`) || !strings.Contains(string(b), `"b"`) || !strings.Contains(string(b), `"v2"`) {
				t.Fatal(string(b))
			}
		})
	}
}

func TestDramaCoverageRejectsMissingStaleAndForeignInventories(t *testing.T) {
	for _, kind := range []string{"missing", "wrong-phase", "wrong-input", "foreign-completion"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			coverageFixture(t, root, "report", "initial", []string{"a"}, []string{"a"}, "a")
			p := filepath.Join(root, "report", "drama_initial_completion.json")
			c := dramaCompletionFile{"initial", []string{"a"}, []string{"a"}}
			switch kind {
			case "missing":
				_ = os.Remove(p)
			case "wrong-phase":
				c.Phase = "retry"
			case "wrong-input":
				c.ExpectedIDs = []string{"b"}
			case "foreign-completion":
				c.CompletedIDs = []string{"x"}
			}
			if kind != "missing" {
				_ = writeDramaJSON(p, c)
			}
			err := finishDramaPhaseCoverage(context.Background(), root, "report", nil, "initial", []string{"a"}, nil, func([]string) error { t.Fatal("must not execute"); return nil })
			if err == nil {
				t.Fatal("invalid inventory passed")
			}
		})
	}
}

func TestDramaCoverageNoProgressAndBoundedContinuationFailClosed(t *testing.T) {
	for _, noProgress := range []bool{true, false} {
		t.Run(map[bool]string{true: "no-progress", false: "round-limit"}[noProgress], func(t *testing.T) {
			root := t.TempDir()
			ids := []string{"a", "b", "c", "d"}
			done := []string{"a"}
			if noProgress {
				done = nil
			}
			coverageFixture(t, root, "report", "initial", ids, done, "a")
			calls := 0
			err := finishDramaPhaseCoverage(context.Background(), root, "report", nil, "initial", ids, nil, func(env []string) error {
				calls++
				var pending []string
				b, _ := os.ReadFile(coverageEnv(env, "DRAMA_CASE_IDS_FILE"))
				_ = json.Unmarshal(b, &pending)
				completed := pending[:1]
				if noProgress {
					completed = nil
				}
				coverageFixture(t, root, coverageEnv(env, "DRAMA_REPORT_DIR"), "initial", pending, completed, pending[0])
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), "检测不完整") {
				t.Fatal(err)
			}
			if noProgress && calls != 1 || !noProgress && calls != dramaCoverageMaxRounds-1 {
				t.Fatal(calls)
			}
			var audit dramaCoverageFile
			b, _ := os.ReadFile(filepath.Join(root, "report", "drama_initial_coverage.json"))
			_ = json.Unmarshal(b, &audit)
			if audit.Complete || len(audit.MissingIDs) == 0 || audit.Error == "" {
				t.Fatalf("%+v", audit)
			}
		})
	}
}

func TestDramaCoverageAlreadyCompleteDoesNotRerun(t *testing.T) {
	root := t.TempDir()
	coverageFixture(t, root, "report", "initial", []string{"a"}, []string{"a", "a"}, "a")
	if err := finishDramaPhaseCoverage(context.Background(), root, "report", nil, "initial", []string{"a"}, nil, func([]string) error { t.Fatal("unexpected rerun"); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestDramaPhaseEnvReplacesInsteadOfDuplicating(t *testing.T) {
	env := dramaPhaseEnv([]string{"DRAMA_REPORT_DIR=one", "DRAMA_REPORT_DIR=two", "KEEP=yes"}, map[string]string{"DRAMA_REPORT_DIR": "three"})
	if len(env) != 2 || coverageEnv(env, "DRAMA_REPORT_DIR") != "three" {
		t.Fatal(env)
	}
}
