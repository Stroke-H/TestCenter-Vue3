package services

import (
	"context"
	"encoding/json"
	"testing"
)

func ledgerChange(feature, audience, value string) extractedProjectConfig {
	return extractedProjectConfig{Key: feature, Feature: feature, Audience: audience, Value: value, Action: "upsert", Content: audience + feature + value, Evidence: "原文", Category: "插屏广告"}
}
func ledgerReport(id, version string) AcceptanceReport {
	return AcceptanceReport{ID: id, Version: version, CreatedAt: "2026-09-08T10:00:00+08:00"}
}

func TestProjectTreeVersionComparison(t *testing.T) {
	if result, ok := compareProjectTreeVersions("1.0 (99)", "1.0.1 (1)"); !ok || result != -1 {
		t.Fatal("build number must not override release version")
	}
	for _, c := range []struct {
		a, b string
		want int
	}{{"1.10", "1.9", 1}, {"v1.0", "1.0.0", 0}, {"1.0.2", "1.0.11", -1}, {"1.0.0 (28)", "1.0.0 (27)", 1}} {
		got, ok := compareProjectTreeVersions(c.a, c.b)
		if !ok || got != c.want {
			t.Fatalf("%s/%s => %d %v", c.a, c.b, got, ok)
		}
	}
	if _, ok := compareProjectTreeVersions("iOS beta", "1.0"); ok {
		t.Fatal("must not guess unknown version order")
	}
}

func TestProjectTreeInheritReviseAndRemove(t *testing.T) {
	r := ProjectConfigRecord{}
	c := ledgerChange("插屏间隔", "自然量", "90秒")
	c.PreviousValue = "30秒"
	applyProjectTreeChanges(&r, ledgerReport("a", "1.0"), "h1", []extractedProjectConfig{c, ledgerChange("预播功能", "", "")})
	if r.Items[0].PreviousValue != "30秒" {
		t.Fatal("source previous value lost")
	}
	snapshot, _ := json.Marshal(r.Versions[0])
	applyProjectTreeChanges(&r, ledgerReport("b", "1.0"), "h2", []extractedProjectConfig{ledgerChange("插屏间隔", "自然量", "60秒")})
	if len(r.Items) != 2 || r.Items[0].Value != "60秒" || r.Items[0].PreviousValue != "90秒" {
		t.Fatalf("same version incremental merge: %+v", r.Items)
	}
	applyProjectTreeChanges(&r, ledgerReport("c", "1.1"), "h3", nil)
	if len(r.Items) != 2 || r.Items[1].Removed {
		t.Fatal("omission must inherit")
	}
	deletion := ledgerChange("预播功能", "", "")
	deletion.Action = "remove"
	applyProjectTreeChanges(&r, ledgerReport("d", "1.2"), "h4", []extractedProjectConfig{deletion})
	if !r.Items[1].Removed || r.Versions[0].Items[1].Removed {
		t.Fatal("removal must preserve history")
	}
	after, _ := json.Marshal(r.Versions[0])
	if string(snapshot) != string(after) {
		t.Fatal("old snapshot mutated")
	}
}

func TestProjectTreeScopeAndSimilarity(t *testing.T) {
	r := ProjectConfigRecord{}
	applyProjectTreeChanges(&r, ledgerReport("a", "1.0"), "h", []extractedProjectConfig{ledgerChange("插屏间隔", "自然量", "90秒"), ledgerChange("插屏间隔", "归因", "30秒")})
	r.Items[0].Kind = "manual"
	r.Items[0].Color = "green"
	applyProjectTreeChanges(&r, ledgerReport("b", "1.1"), "h2", []extractedProjectConfig{ledgerChange("插屏CD", "自然量用户", "60秒")})
	if len(r.Items) != 2 || r.Items[0].Value != "60秒" || r.Items[1].Value != "30秒" || len(r.Items[0].History) != 1 {
		t.Fatal("manual or audience partial update failed")
	}
	if projectTreeNameSimilarity("abcde", "abcdx") > 0.8 {
		t.Fatal("80 percent is not over 80 percent")
	}
	if projectTreeNameSimilarity("返回插屏", "返回插屏精细化运营20") > 0.8 {
		t.Fatal("should preserve distinct feature")
	}
	a, b := ledgerChange("插屏间隔", "自然量", "20秒"), ledgerChange("插屏间隔", "自然量", "30秒")
	a.Variant = "A"
	b.Variant = "B"
	applyProjectTreeChanges(&r, ledgerReport("c", "1.2"), "h3", []extractedProjectConfig{a, b})
	if len(r.Items) != 4 {
		t.Fatal("A/B scopes merged")
	}
}

func TestProjectTreeUnknownValueDoesNotErase(t *testing.T) {
	r := ProjectConfigRecord{}
	applyProjectTreeChanges(&r, ledgerReport("a", "1.0"), "h", []extractedProjectConfig{ledgerChange("插屏间隔", "自然量", "90秒")})
	applyProjectTreeChanges(&r, ledgerReport("b", "1.1"), "h2", []extractedProjectConfig{ledgerChange("插屏间隔", "自然量", "")})
	if r.Items[0].Value != "90秒" {
		t.Fatal("unspecified value erased")
	}
}

func TestProjectTreeSourceEvidenceAndActionValidation(t *testing.T) {
	cases := []struct {
		source string
		c      extractedProjectConfig
		want   int
	}{
		{"新增前N集免广告", extractedProjectConfig{Feature: "前N集免广告", Action: "upsert", Evidence: "新增前N集免广告"}, 1},
		{"新增预播", extractedProjectConfig{Feature: "预播", Action: "remove", Evidence: "新增预播"}, 0},
		{"删除预播功能", extractedProjectConfig{Feature: "预播", Action: "remove", Evidence: "删除预播功能"}, 1},
		{"返回活动2.0", extractedProjectConfig{Feature: "回退版本", Action: "upsert", Evidence: "回退2.0"}, 0},
		{"网赚新包", extractedProjectConfig{Feature: "网赚新包", Action: "upsert", Evidence: "网赚新包"}, 0},
	}
	for _, c := range cases {
		if len(validateProjectTreeExtraction([]extractedProjectConfig{c.c}, c.source)) != c.want {
			t.Fatalf("unexpected validation: %+v", c)
		}
	}
}

func TestProjectTreeLowerVersionAndIdempotency(t *testing.T) {
	report := ledgerReport("r", "1.0")
	r := ProjectConfigRecord{Schema: 2, CurrentVersion: "2.0", Processed: map[string]string{}}
	n, err := reconcileProjectTreeRecord(context.Background(), &r, []AcceptanceReport{report}, false)
	if err != nil || n != 0 || len(r.Warnings) != 1 || r.CurrentVersion != "2.0" {
		t.Fatal("lower version not guarded")
	}
	report.Version = "2.0"
	r.Processed[report.ID] = projectTreeReportHash(report)
	_, err = reconcileProjectTreeRecord(context.Background(), &r, []AcceptanceReport{report}, false)
	if err != nil || len(r.Versions) != 0 {
		t.Fatal("unchanged report reprocessed")
	}
	legacy := ProjectConfigRecord{Items: []ProjectConfigMemoItem{{Content: "历史便签", Kind: "ai"}}}
	_, err = reconcileProjectTreeRecord(context.Background(), &legacy, []AcceptanceReport{report}, false)
	if err != nil || legacy.Schema != 0 || len(legacy.Items) != 1 || len(legacy.Warnings) != 1 {
		t.Fatal("legacy requires preview")
	}
}

func TestProjectTreeParseSameFeatureDifferentGroups(t *testing.T) {
	items, err := parseExtractedProjectConfigs(`[{"key":"间隔","content":"自然90","audience":"自然量"},{"key":"间隔","content":"归因30","audience":"归因"}]`)
	if err != nil || len(items) != 2 {
		t.Fatalf("scoped items deduplicated: %v %+v", err, items)
	}
}
