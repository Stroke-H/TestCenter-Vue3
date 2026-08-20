package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDramaIssueSignatureRequiresExactDramaAndDetail(t *testing.T) {
	const dramaID = "6a3dd62f4f8256c9c3fc551b"
	iaa := []string{"• [分组规则] 分组错了，分到了 IAA 组"}
	ipa := []string{"• [分组规则] 分组错了，分到了 IPA 组"}

	base := dramaIssueSignature(dramaID, iaa)
	if base == "" {
		t.Fatal("expected a non-empty signature")
	}
	if base == dramaIssueSignature(dramaID, ipa) {
		t.Fatal("different group detail must not share the same signature")
	}
	if base == dramaIssueSignature("6a3dd62f4f8256c9c3fc551c", iaa) {
		t.Fatal("different drama must not share the same signature")
	}
	if base != dramaIssueSignature(strings.ToUpper(dramaID), []string{" [分组规则]   分组错了，分到了 IAA 组 "}) {
		t.Fatal("presentation-only case and whitespace differences should normalize")
	}
}

func TestFilterTemporarilyIgnoredDramaFailuresOnlyFiltersExactMatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drama_issue_dispositions.json")
	t.Setenv("DRAMA_ISSUE_DISPOSITIONS_FILE", path)

	exact := dramaFailureSummary{
		DramaID: "6a3dd62f4f8256c9c3fc551b",
		Errors:  []string{"• [分组规则] 分组错了，分到了 IAA 组"},
	}
	changedDetail := dramaFailureSummary{
		DramaID: exact.DramaID,
		Errors:  []string{"• [分组规则] 分组错了，分到了 IPA 组"},
	}
	dataIssue := dramaFailureSummary{
		DramaID: "6a3dd62f4f8256c9c3fc551c",
		Errors:  []string{"• [命名规范] IAA 大小写不规范"},
	}

	ignoredSignature := dramaIssueSignature(exact.DramaID, exact.Errors)
	if err := saveDramaIssueDisposition(dramaIssueDisposition{
		Signature: ignoredSignature,
		DramaID:   exact.DramaID,
		Errors:    normalizeDramaIssueErrors(exact.Errors),
		Action:    dramaIssueActionTemporarilyIgnored,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("save temporary ignore: %v", err)
	}
	dataSignature := dramaIssueSignature(dataIssue.DramaID, dataIssue.Errors)
	if err := saveDramaIssueDisposition(dramaIssueDisposition{
		Signature: dataSignature,
		DramaID:   dataIssue.DramaID,
		Errors:    normalizeDramaIssueErrors(dataIssue.Errors),
		Action:    dramaIssueActionDataIssue,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("save data issue: %v", err)
	}

	visible, ignored := filterTemporarilyIgnoredDramaFailures([]dramaFailureSummary{exact, changedDetail, dataIssue})
	if ignored != 1 {
		t.Fatalf("expected exactly one ignored failure, got %d", ignored)
	}
	if len(visible) != 2 {
		t.Fatalf("expected changed detail and data issue to remain visible, got %#v", visible)
	}
	if visible[0].Errors[0] != changedDetail.Errors[0] || visible[1].Errors[0] != dataIssue.Errors[0] {
		t.Fatalf("unexpected visible failures: %#v", visible)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persisted disposition file: %v", err)
	}
}

func TestTemporaryIgnoreMatchesWholeErrorSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drama_issue_dispositions.json")
	t.Setenv("DRAMA_ISSUE_DISPOSITIONS_FILE", path)

	original := dramaFailureSummary{
		DramaID: "6a3dd62f4f8256c9c3fc551b",
		Errors: []string{
			"• [分组规则] 分组错了，分到了 IAA 组",
			"• [解锁类型] unlock_type=coin",
		},
	}
	if err := saveDramaIssueDisposition(dramaIssueDisposition{
		Signature: dramaIssueSignature(original.DramaID, original.Errors),
		DramaID:   original.DramaID,
		Errors:    normalizeDramaIssueErrors(original.Errors),
		Action:    dramaIssueActionTemporarilyIgnored,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("save temporary ignore: %v", err)
	}

	changedSet := original
	changedSet.Errors = append([]string{}, original.Errors...)
	changedSet.Errors[0] = "• [分组规则] 分组错了，分到了 IPA 组"
	visible, ignored := filterTemporarilyIgnoredDramaFailures([]dramaFailureSummary{changedSet})
	if ignored != 0 || len(visible) != 1 {
		t.Fatalf("a changed detail in the row must make the whole row reportable: ignored=%d visible=%#v", ignored, visible)
	}
}

func TestApplyDramaIssueOperatingControls(t *testing.T) {
	html := `<html><head></head><body><section class="drama-other-checks"><table><thead><tr><th>Check Name</th><th>Failures</th></tr></thead><tbody><tr><td><details><summary><b>Drama<br>ID:6a3dd62f4f8256c9c3fc551b(1)</b></summary><div>• [分组规则] 分到了 IAA 组</div></details></td><td>1</td></tr></tbody></table></section></body></html>`

	result := applyDramaIssueOperatingControls(html, "RUN-123")
	for _, expected := range []string{"OPERATING", "数据问题", "暂时忽略", "RUN-123", "issue-dispositions"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("expected injected report to contain %q", expected)
		}
	}
	if got := strings.Count(applyDramaIssueOperatingControls(result, "RUN-123"), "testcenter-drama-issue-operating-controls"); got != 1 {
		t.Fatalf("expected controls to be injected once, got %d", got)
	}
}
