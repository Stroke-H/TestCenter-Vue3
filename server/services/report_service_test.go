package services

import (
	"strings"
	"testing"
)

func TestFormatAcceptanceReportTextUsesBugStatusMapping(t *testing.T) {
	report := AcceptanceReport{
		BugFixStatus:        "当前版本缺陷",
		BugSubmissionStatus: "历史遗留缺陷",
	}

	text := FormatAcceptanceReportText(report)
	if !strings.Contains(text, "正式版本缺陷修复验证情况：\n历史遗留缺陷") {
		t.Fatalf("historical bug fixes were not mapped to the formal-version section:\n%s", text)
	}
	if !strings.Contains(text, "本次预提审版本缺陷提交情况：\n当前版本缺陷") {
		t.Fatalf("current-version bugs were not mapped to the pre-review section:\n%s", text)
	}
}

func TestAcceptanceReportEmptyBugStatusesUseNone(t *testing.T) {
	report := AcceptanceReport{}

	text := FormatAcceptanceReportText(report)
	if !strings.Contains(text, "正式版本缺陷修复验证情况：\n无") {
		t.Fatalf("formal-version section did not use the empty fallback:\n%s", text)
	}
	if !strings.Contains(text, "本次预提审版本缺陷提交情况：\n无") {
		t.Fatalf("pre-review section did not use the empty fallback:\n%s", text)
	}

	card := buildAcceptanceReportFeishuCard(report, "tester")
	elements := card["elements"].([]map[string]any)
	formalSection := elements[2]["text"].(map[string]any)["content"].(string)
	currentSection := elements[4]["text"].(map[string]any)["content"].(string)
	if formalSection != "**正式版本缺陷修复验证情况**\n无" {
		t.Fatalf("unexpected formal-version card section: %q", formalSection)
	}
	if currentSection != "**本次预提审版本缺陷提交情况**\n无" {
		t.Fatalf("unexpected pre-review card section: %q", currentSection)
	}
}
