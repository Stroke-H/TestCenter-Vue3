package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDramaK6DiagnosticsFiltersBusinessLogsAndPreservesSplitFatalLines(t *testing.T) {
	d := &dramaK6Diagnostics{}
	_, _ = d.Write([]byte("level=error msg=业务异常 source=console\n__DRAMA_CASE_DONE__|a\nlevel=error msg=failed to handle the end-"))
	_, _ = d.Write([]byte("of-test summary error=handleSummary() execution timed out after 120 seconds\n"))
	lines := d.finish()
	if len(lines) != 1 || !strings.Contains(lines[0], "timed out") {
		t.Fatal(lines)
	}
	if len(d.pending) != 0 {
		t.Fatal("pending data was not flushed")
	}
}

func TestDramaK6DiagnosticsBounded(t *testing.T) {
	d := &dramaK6Diagnostics{}
	for i := 0; i < 30; i++ {
		_, _ = d.Write([]byte("source=stacktrace " + strings.Repeat("x", 100000) + "\n"))
	}
	lines := d.finish()
	if len(lines) != 12 {
		t.Fatal(len(lines))
	}
	for _, line := range lines {
		if len(line) > 4100 {
			t.Fatal(len(line))
		}
	}
}

func TestDramaK6RealSummaryErrorDoesNotPassOnZeroExit(t *testing.T) {
	if _, err := exec.LookPath("k6"); err != nil {
		t.Skip("k6 not installed")
	}
	root := t.TempDir()
	for _, dir := range []string{"k6-scripts", "report"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	script := `export const options={vus:1,iterations:1}; export default function(){}; export function handleSummary(){throw new Error("offline summary failure");}`
	if err := os.WriteFile(filepath.Join(root, "k6-scripts", "drama_check_flow.js"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := runDramaK6Command(ctx, root, dramaPhaseEnv(os.Environ(), map[string]string{"DRAMA_REPORT_DIR": "report", "DRAMA_RETRY_PHASE": "1"}))
	if err == nil || !isDramaPhaseError(err) || !strings.Contains(err.Error(), "offline summary failure") {
		t.Fatalf("summary error not detected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "report", "drama_k6_diagnostics.json")); err != nil {
		t.Fatal(err)
	}
}

func TestDramaK6RealBusinessConsoleErrorIsNotInfrastructureFailure(t *testing.T) {
	if _, err := exec.LookPath("k6"); err != nil {
		t.Skip("k6 not installed")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "k6-scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	script := `export const options={vus:1,iterations:1}; export default function(){console.error("[业务异常] test-only");}; export function handleSummary(){return {};}`
	if err := os.WriteFile(filepath.Join(root, "k6-scripts", "drama_check_flow.js"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := runDramaK6Command(ctx, root, os.Environ()); err != nil {
		t.Fatal(err)
	}
}

func TestDramaPhaseErrorSurvivesWrapping(t *testing.T) {
	err := fmt.Errorf("retry failed: %w", &dramaPhaseError{fmt.Errorf("completion missing")})
	if !isDramaPhaseError(err) || isDramaPhaseError(fmt.Errorf("temporary prepare failure")) {
		t.Fatal("outer retry classification failed")
	}
}

func TestDramaRetryNeverReadsInitialCompletion(t *testing.T) {
	root := t.TempDir()
	coverageFixture(t, root, "report", "initial", []string{"a"}, []string{"a"}, "a")
	err := finishDramaPhaseCoverage(context.Background(), root, "report", nil, "retry", []string{"a"}, nil, func([]string) error { t.Fatal("must not execute"); return nil })
	if err == nil || !isDramaPhaseError(err) || !strings.Contains(err.Error(), "完成名单缺失") {
		t.Fatal(err)
	}
}
