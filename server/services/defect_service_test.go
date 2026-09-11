package services

import "testing"

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
