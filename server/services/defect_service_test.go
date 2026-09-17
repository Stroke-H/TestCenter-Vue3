package services

import (
	"context"
	"strings"
	"testing"
)

func TestValidateDefectTransition(t *testing.T) {
	tests := []struct {
		status string
		action string
		want   string
		ok     bool
	}{
		{DefectStatusNew, "confirm", DefectStatusActive, true},
		{DefectStatusNew, "resolve", DefectStatusResolved, true},
		{DefectStatusActive, "resolve", DefectStatusResolved, true},
		{DefectStatusResolved, "close", DefectStatusClosed, true},
		{DefectStatusResolved, "reopen", DefectStatusActive, true},
		{DefectStatusClosed, "reopen", DefectStatusActive, true},
		{DefectStatusClosed, "resolve", "", false},
		{DefectStatusActive, "close", "", false},
	}
	for _, test := range tests {
		got, err := validateDefectTransition(test.status, test.action)
		if test.ok && (err != nil || got != test.want) {
			t.Fatalf("%s/%s: got %q err=%v, want %q", test.status, test.action, got, err, test.want)
		}
		if !test.ok && err == nil {
			t.Fatalf("%s/%s: expected an error", test.status, test.action)
		}
	}
}

func TestDirectStatusPermission(t *testing.T) {
	tests := []struct {
		current, target, permission, projectAction string
	}{
		{DefectStatusNew, DefectStatusActive, defectPermissionProcess, "process"},
		{DefectStatusActive, DefectStatusResolved, defectPermissionProcess, "process"},
		{DefectStatusActive, DefectStatusClosed, defectPermissionVerify, "verify"},
		{DefectStatusClosed, DefectStatusActive, defectPermissionVerify, "reopen"},
		{DefectStatusResolved, DefectStatusNew, defectPermissionVerify, "reopen"},
	}
	for _, test := range tests {
		permission, action := directStatusPermission(test.current, test.target)
		if permission != test.permission || action != test.projectAction {
			t.Fatalf("%s -> %s: got %s/%s, want %s/%s", test.current, test.target,
				permission, action, test.permission, test.projectAction)
		}
	}
}

func TestCreateDefectsRejectsInvalidBatchSize(t *testing.T) {
	if _, err := createDefects(context.Background(), nil, nil); err == nil || !strings.Contains(err.Error(), "至少") {
		t.Fatalf("empty batch should be rejected, got %v", err)
	}
	items := make([]DefectSaveRequest, 51)
	if _, err := createDefects(context.Background(), nil, items); err == nil || !strings.Contains(err.Error(), "50") {
		t.Fatalf("oversized batch should be rejected, got %v", err)
	}
}

func TestAllowedDefectAttachment(t *testing.T) {
	for _, name := range []string{"screen.png", "trace.txt", "request.json", "notes.md", "data.csv", "config.yaml", "report.pdf"} {
		if !allowedDefectAttachment(name) {
			t.Fatalf("expected %q to be accepted", name)
		}
	}
	for _, name := range []string{"payload.exe", "script.sh", "archive.rar", "no-extension"} {
		if allowedDefectAttachment(name) {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestNormalizeDefectSaveRequest(t *testing.T) {
	req := DefectSaveRequest{
		Title: "  登录失败  ", ProjectCode: " A1100 ", Severity: 9, Priority: 0,
		Steps: []DefectStep{{Content: " 打开登录页 "}, {Content: "  "}},
		Tags:  []string{" iOS ", "ios", " 回归 "},
	}
	if err := normalizeDefectSaveRequest(&req); err != nil {
		t.Fatal(err)
	}
	if req.Title != "登录失败" || req.ProjectCode != "A1100" {
		t.Fatalf("unexpected normalization: %#v", req)
	}
	if req.Severity != 3 || req.Priority != 3 {
		t.Fatalf("invalid default levels: severity=%d priority=%d", req.Severity, req.Priority)
	}
	if len(req.Steps) != 1 || req.Steps[0].ID == "" {
		t.Fatalf("unexpected steps: %#v", req.Steps)
	}
	if len(req.Tags) != 2 {
		t.Fatalf("unexpected tags: %#v", req.Tags)
	}
}

func TestNormalizeAcceptanceReportVersion(t *testing.T) {
	tests := map[string]string{
		"1.1.1":             "1.1.1",
		"v1.1.1":            "1.1.1",
		"V2.65.0 (Build 8)": "2.65.0",
		"release-3":         "3",
		"beta":              "",
	}
	for input, want := range tests {
		if got := normalizeAcceptanceReportVersion(input); got != want {
			t.Fatalf("normalizeAcceptanceReportVersion(%q) = %q, want %q", input, got, want)
		}
	}
}
