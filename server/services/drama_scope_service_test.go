package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDramaRetryScopeFiltersMovesAndDuplicates(t *testing.T) {
	retained, removed := filterDramaRetryScope([]string{"a", "b", "a", "offline"}, []string{"a", "new"})
	if len(retained) != 1 || retained[0] != "a" || len(removed) != 2 {
		t.Fatalf("%v / %v", retained, removed)
	}
	retained, removed = filterDramaRetryScope([]string{"a"}, nil)
	if len(retained) != 0 || len(removed) != 1 {
		t.Fatal("all-removed case must not run stale candidates")
	}
}

func TestDramaArchivedScopeExclusionsDoNotResurface(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	items := []dramaFailureSummary{{DramaID: "111111111111111111111111", Errors: []string{"• [直接跳集] missing"}}, {DramaID: "222222222222222222222222", Errors: []string{"• [版权规则] expired"}}}
	if err := writeDramaJSON(filepath.Join(dir, "drama_failure_summary.json"), dramaFailureSummaryFile{Failures: items}); err != nil {
		t.Fatal(err)
	}
	if err := writeDramaJSON(filepath.Join(dir, "drama_retry_scope_changes.json"), map[string]any{"removedIds": []string{items[0].DramaID}}); err != nil {
		t.Fatal(err)
	}
	got := loadArchivedDramaFailures(root, "")
	if len(got) != 1 || got[0].DramaID != items[1].DramaID {
		t.Fatalf("moved record resurfaced: %+v", got)
	}
}

func TestDramaInitialDirectFindingsMergeWithConfirmedFindings(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	id := "111111111111111111111111"
	direct := dramaFailureSummary{DramaID: id, Errors: []string{"• [分组规则] 分组不符", "• [免费集数] 应为2", "• [版权规则] 已过期"}}
	confirmed := dramaFailureSummary{DramaID: id, Errors: []string{"• [分类规则] 印配不对应lang", "• [简介语种] 不匹配"}}
	if err := writeDramaJSON(filepath.Join(dir, "drama_failure_summary.json"), dramaFailureSummaryFile{Failures: []dramaFailureSummary{direct}}); err != nil {
		t.Fatal(err)
	}
	if err := writeDramaJSON(filepath.Join(dir, "drama_retry_result.json"), dramaRetryResultFile{StructuredFailures: []dramaFailureSummary{confirmed}}); err != nil {
		t.Fatal(err)
	}
	got := loadArchivedDramaFailures(root, "")
	if len(got) != 1 || len(got[0].Errors) != 5 {
		t.Fatalf("lost or duplicated direct findings: %+v", got)
	}
}
