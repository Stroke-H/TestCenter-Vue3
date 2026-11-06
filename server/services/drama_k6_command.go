package services

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Capture infrastructure/summary failures only. Business findings and per-case
// progress are deliberately not logged to the scheduled-task terminal.
type dramaK6Diagnostics struct {
	mu      sync.Mutex
	pending []byte
	lines   []string
}

func (d *dramaK6Diagnostics) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	length := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		part := p
		if i >= 0 {
			part = p[:i]
		}
		if remaining := 65536 - len(d.pending); remaining > 0 {
			if len(part) > remaining {
				part = part[:remaining]
			}
			d.pending = append(d.pending, part...)
		}
		if i < 0 {
			break
		}
		d.capture(string(d.pending))
		d.pending = nil
		p = p[i+1:]
	}
	return length, nil
}

func (d *dramaK6Diagnostics) capture(line string) {
	fatal := false
	for _, marker := range []string{"failed to handle the end-of-test summary", "handleSummary() execution timed out", "source=stacktrace", "could not initialize", "error calling handleSummary"} {
		if strings.Contains(line, marker) {
			fatal = true
			break
		}
	}
	if !fatal {
		return
	}
	if len(line) > 4096 {
		line = line[:4096] + "…"
	}
	d.lines = append(d.lines, line)
	if len(d.lines) > 12 {
		d.lines = d.lines[len(d.lines)-12:]
	}
}

func (d *dramaK6Diagnostics) finish() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pending) > 0 {
		d.capture(string(d.pending))
		d.pending = nil
	}
	return append([]string{}, d.lines...)
}

func runDramaK6Command(ctx context.Context, rootDir string, env []string) error {
	cmd := exec.CommandContext(ctx, "k6", "run", filepath.Join("k6-scripts", "drama_check_flow.js"))
	cmd.Dir = rootDir
	cmd.Env = env
	diagnostics := &dramaK6Diagnostics{}
	cmd.Stdout = diagnostics
	cmd.Stderr = diagnostics
	err := cmd.Run()
	lines := diagnostics.finish()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil && len(lines) == 0 {
		return nil
	}
	reason := strings.Join(lines, "\n")
	if err != nil {
		reason = fmt.Sprintf("%v\n%s", err, reason)
	}
	if dir := dramaEnvironmentValue(env, "DRAMA_REPORT_DIR"); dir != "" {
		_ = writeDramaJSON(filepath.Join(rootDir, dir, "drama_k6_diagnostics.json"), map[string]any{"phase": dramaEnvironmentValue(env, "DRAMA_RETRY_PHASE"), "error": reason})
	}
	// k6 can exit zero when handleSummary times out. Never treat that as success,
	// and never let the outer scheduler repeat the entire completed playback run.
	return &dramaPhaseError{fmt.Errorf("k6执行或汇总失败: %s", strings.TrimSpace(reason))}
}

func dramaEnvironmentValue(env []string, key string) string {
	for _, entry := range env {
		if strings.HasPrefix(entry, key+"=") {
			return strings.TrimPrefix(entry, key+"=")
		}
	}
	return ""
}
