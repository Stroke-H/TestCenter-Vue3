package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// Each phase gets one normal run plus at most two continuations. The enclosing
// job context remains authoritative; no-progress and incomplete runs fail closed.
const dramaCoverageMaxRounds = 3

type dramaPhaseError struct{ Err error }

func (e *dramaPhaseError) Error() string { return e.Err.Error() }
func (e *dramaPhaseError) Unwrap() error { return e.Err }
func isDramaPhaseError(err error) bool   { var target *dramaPhaseError; return errors.As(err, &target) }

type dramaCompletionFile struct {
	Phase        string   `json:"phase"`
	ExpectedIDs  []string `json:"expectedIds"`
	CompletedIDs []string `json:"completedIds"`
}

type dramaCoverageRound struct {
	Directory string `json:"directory"`
	Expected  int    `json:"expected"`
	Completed int    `json:"completed"`
	Missing   int    `json:"missing"`
}

type dramaCoverageFile struct {
	Phase        string               `json:"phase"`
	Complete     bool                 `json:"complete"`
	ExpectedIDs  []string             `json:"expectedIds"`
	CompletedIDs []string             `json:"completedIds"`
	MissingIDs   []string             `json:"missingIds"`
	Rounds       []dramaCoverageRound `json:"rounds"`
	Error        string               `json:"error,omitempty"`
}

func dramaCoverageMissing(expected, completed []string) []string {
	done := make(map[string]bool, len(completed))
	for _, id := range completed {
		done[id] = true
	}
	missing := []string{}
	for _, id := range uniqueNonEmptyStrings(expected) {
		if !done[id] {
			missing = append(missing, id)
		}
	}
	return missing
}

func writeDramaJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// Replace keys rather than appending duplicates: subprocess environment lookup
// differs across platforms, and retry lists can exceed macOS argv/env limits.
func dramaPhaseEnv(env []string, values map[string]string) []string {
	out := make([]string, 0, len(env)+len(values))
	for _, entry := range env {
		key := strings.SplitN(entry, "=", 2)[0]
		if _, replace := values[key]; !replace {
			out = append(out, entry)
		}
	}
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}

// The caller has executed round 1 in reportDir. Validate actual completed IDs,
// run ONLY unfinished IDs, and merge every round's candidates/failures. Missing
// completion artifacts are errors, never evidence that every drama passed.
func finishDramaPhaseCoverage(ctx context.Context, rootDir, reportDir string, env []string, phase string, expected []string, notify func(string), run func([]string) error) (resultErr error) {
	expected = uniqueNonEmptyStrings(expected)
	audit := dramaCoverageFile{Phase: phase, ExpectedIDs: expected, CompletedIDs: []string{}, MissingIDs: expected}
	currentDir, pending := reportDir, expected
	noProgressRounds := 0
	var mergeErr error
	defer func() {
		if resultErr != nil && !isDramaPhaseError(resultErr) {
			resultErr = &dramaPhaseError{resultErr}
		}
		// Every return path retains a diagnostic inventory, including cancellation.
		_ = writeDramaJSON(filepath.Join(rootDir, reportDir, "drama_"+phase+"_coverage.json"), audit)
	}()
	for round := 1; round <= dramaCoverageMaxRounds; round++ {
		if ctx.Err() != nil {
			audit.Error = ctx.Err().Error()
			return ctx.Err()
		}
		var completion dramaCompletionFile
		b, err := os.ReadFile(filepath.Join(rootDir, currentDir, "drama_"+phase+"_completion.json"))
		if err == nil {
			err = json.Unmarshal(b, &completion)
		}
		if err != nil {
			audit.Error = "完成名单缺失或无效: " + err.Error()
			return fmt.Errorf("%s coverage: %s", phase, audit.Error)
		}
		if completion.Phase != phase || len(dramaCoverageMissing(pending, completion.ExpectedIDs)) != 0 || len(dramaCoverageMissing(completion.ExpectedIDs, pending)) != 0 {
			audit.Error = "完成名单的阶段或待测集合与本轮不一致"
			return fmt.Errorf("%s coverage: %s", phase, audit.Error)
		}
		if len(dramaCoverageMissing(completion.CompletedIDs, pending)) != 0 {
			audit.Error = "完成名单包含非本轮剧集"
			return fmt.Errorf("%s coverage: %s", phase, audit.Error)
		}
		if err := validateDramaPhaseArtifacts(rootDir, currentDir, phase); err != nil {
			audit.Error = err.Error()
			return err
		}
		completed := uniqueNonEmptyStrings(completion.CompletedIDs)
		if len(completed) == 0 {
			noProgressRounds++
		} else {
			noProgressRounds = 0
		}
		audit.CompletedIDs = uniqueNonEmptyStrings(append(audit.CompletedIDs, completed...))
		audit.MissingIDs = dramaCoverageMissing(expected, audit.CompletedIDs)
		audit.Rounds = append(audit.Rounds, dramaCoverageRound{currentDir, len(pending), len(completed), len(audit.MissingIDs)})
		if round > 1 {
			mergeErr = mergeDramaCoverageRound(rootDir, reportDir, currentDir, phase)
		}
		if mergeErr != nil {
			audit.Error = mergeErr.Error()
			return mergeErr
		}
		audit.Complete = len(audit.MissingIDs) == 0
		if err := writeDramaJSON(filepath.Join(rootDir, reportDir, "drama_"+phase+"_coverage.json"), audit); err != nil {
			return err
		}
		logIfPresent(notify, fmt.Sprintf("[覆盖校验][%s] 第%d轮：本轮完成%d/%d，累计完成%d/%d，未完成%d", phase, round, len(completed), len(pending), len(audit.CompletedIDs), len(expected), len(audit.MissingIDs)), false)
		if audit.Complete {
			return appendDramaCoverageSection(rootDir, reportDir, audit)
		}
		if noProgressRounds >= 2 || round == dramaCoverageMaxRounds {
			audit.Error = fmt.Sprintf("检测不完整：%d部未完成（无进展或达到%d轮上限）", len(audit.MissingIDs), dramaCoverageMaxRounds)
			_ = appendDramaCoverageSection(rootDir, reportDir, audit)
			return fmt.Errorf("%s coverage: %s", phase, audit.Error)
		}
		pending = append([]string{}, audit.MissingIDs...)
		currentDir = filepath.ToSlash(filepath.Join(reportDir, fmt.Sprintf("%s-continuation-%d", phase, round+1)))
		if err := os.MkdirAll(filepath.Join(rootDir, currentDir), 0755); err != nil {
			audit.Error = err.Error()
			return err
		}
		if err := os.Remove(filepath.Join(rootDir, currentDir, "drama_"+phase+"_completion.json")); err != nil && !os.IsNotExist(err) {
			audit.Error = err.Error()
			return err
		}
		idsPath := filepath.Join(rootDir, currentDir, "case_ids.json")
		if err := writeDramaJSON(idsPath, pending); err != nil {
			audit.Error = err.Error()
			return err
		}
		// k6 open() resolves relative to its script, so pass an absolute path.
		nextEnv := dramaPhaseEnv(env, map[string]string{"DRAMA_REPORT_DIR": currentDir, "DRAMA_CASE_IDS_FILE": idsPath, "DRAMA_RETRY_IDS": "", "DRAMA_RETRY_PHASE": map[bool]string{true: "1", false: "0"}[phase == "retry"]})
		if err := run(nextEnv); err != nil {
			audit.Error = err.Error()
			return fmt.Errorf("%s continuation: %w", phase, err)
		}
	}
	return fmt.Errorf("unreachable coverage state")
}

func validateDramaPhaseArtifacts(rootDir, dir, phase string) error {
	files := map[string][]string{"drama_failure_summary.json": {"failures"}, "drama_retry_candidates.json": {"candidates"}}
	if phase == "retry" {
		files = map[string][]string{"drama_retry_result.json": {"retryIds", "persistentFailures", "structuredFailures"}}
	}
	for name, fields := range files {
		b, err := os.ReadFile(filepath.Join(rootDir, dir, name))
		if err != nil {
			return fmt.Errorf("%s结果文件缺失: %w", phase, err)
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(b, &payload); err != nil {
			return err
		}
		for _, field := range fields {
			var entries []json.RawMessage
			value, ok := payload[field]
			if !ok {
				return fmt.Errorf("%s 缺少 %s", name, field)
			}
			if err := json.Unmarshal(value, &entries); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeDramaCoverageRound(rootDir, parent, child, phase string) error {
	files := []string{"drama_failure_summary.json", "drama_retry_candidates.json"}
	if phase == "retry" {
		files = []string{"drama_retry_result.json"}
	}
	for _, name := range files {
		var base, extra map[string]json.RawMessage
		for dir, dst := range map[string]*map[string]json.RawMessage{parent: &base, child: &extra} {
			b, err := os.ReadFile(filepath.Join(rootDir, dir, name))
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, dst); err != nil {
				return err
			}
		}
		if len(base["ruleScope"]) > 0 && len(extra["ruleScope"]) > 0 {
			var a, b any
			if err := json.Unmarshal(base["ruleScope"], &a); err != nil {
				return err
			}
			if err := json.Unmarshal(extra["ruleScope"], &b); err != nil {
				return err
			}
			left, _ := json.Marshal(a)
			right, _ := json.Marshal(b)
			if string(left) != string(right) {
				return fmt.Errorf("续跑规则配置与首轮不一致，拒绝混合报告")
			}
		}
		// The rounds have disjoint unfinished input sets, but a killed iteration
		// may already have emitted an error/candidate. Deduplicate exact entries.
		for _, key := range []string{"failures", "candidates", "persistentFailures", "structuredFailures", "retryIds"} {
			if _, ok := extra[key]; !ok {
				continue
			}
			var a, b []json.RawMessage
			if err := json.Unmarshal(base[key], &a); err != nil {
				return err
			}
			if err := json.Unmarshal(extra[key], &b); err != nil {
				return err
			}
			seen := map[string]bool{}
			merged := []json.RawMessage{}
			for _, value := range append(a, b...) {
				signature := string(value)
				if !seen[signature] {
					seen[signature] = true
					merged = append(merged, value)
				}
			}
			base[key], _ = json.Marshal(merged)
		}
		if err := writeDramaJSON(filepath.Join(rootDir, parent, name), base); err != nil {
			return err
		}
	}
	// Retain the original request metrics, explicitly labelled as the first
	// batch, and show continuation failures in the same report (not lost in JSON).
	name := "drama_check_report.html"
	if phase == "retry" {
		name = "drama_retry_report.html"
	}
	var payload struct {
		Failures   []dramaFailureSummary `json:"failures"`
		Structured []dramaFailureSummary `json:"structuredFailures"`
	}
	summary := files[0]
	b, err := os.ReadFile(filepath.Join(rootDir, child, summary))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	failures := append(payload.Failures, payload.Structured...)
	var section strings.Builder
	section.WriteString(`<section style="margin:24px;padding:20px;border:1px solid #dbeafe;border-radius:12px"><h3>未完成项续跑结果</h3><p>上方 k6 请求指标为首批执行统计；完整覆盖以完成名单校验为准。</p>`)
	for _, f := range failures {
		section.WriteString("<p>" + html.EscapeString(f.DramaID+" "+f.CNTitle+" "+strings.Join(f.Errors, "；")) + "</p>")
	}
	section.WriteString("</section>")
	return appendDramaHTML(filepath.Join(rootDir, parent, name), section.String())
}

func appendDramaHTML(path, section string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(b)
	if strings.Contains(s, "</body>") {
		s = strings.Replace(s, "</body>", section+"</body>", 1)
	} else {
		s += section
	}
	return os.WriteFile(path, []byte(s), 0644)
}

func appendDramaCoverageSection(rootDir, reportDir string, audit dramaCoverageFile) error {
	name := "drama_check_report.html"
	if audit.Phase == "retry" {
		name = "drama_retry_report.html"
	}
	return appendDramaHTML(filepath.Join(rootDir, reportDir, name), dramaCoverageSection(audit))
}

func dramaCoverageSection(audit dramaCoverageFile) string {
	label := "覆盖完整"
	if !audit.Complete {
		label = "检测不完整"
	}
	section := fmt.Sprintf(`<section style="margin:24px;padding:20px;border:2px solid #94a3b8;border-radius:12px"><h3>%s · %s</h3><p>待测 %d · 实际完成 %d · 未完成 %d · 执行批次 %d。完成不代表业务通过；上方原始指标仅为首批统计。</p><p>%s</p></section>`, html.EscapeString(audit.Phase), label, len(audit.ExpectedIDs), len(audit.CompletedIDs), len(audit.MissingIDs), len(audit.Rounds), html.EscapeString(audit.Error))
	return section
}
