package services

import "testing"

func TestParseExtractedProjectConfigs(t *testing.T) {
	configs, err := parseExtractedProjectConfigs("```json\n[" +
		`{"key":"App Group","content":"App Group 配置为 IAA 组"},` +
		`{"key":"app-group","content":"重复项应被忽略"},` +
		`{"key":"","content":"无效项"}` +
		"]\n```")
	if err != nil {
		t.Fatalf("parse configs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected one normalized config, got %#v", configs)
	}
	if configs[0].Key != "appgroup" {
		t.Fatalf("unexpected normalized key: %s", configs[0].Key)
	}
}

func TestProjectCodesMatchOptionalAPrefix(t *testing.T) {
	for _, pair := range [][2]string{
		{"1106", "A1106"},
		{"a1106", "1106"},
		{" A1106 ", "A1106"},
	} {
		if !projectCodesMatch(pair[0], pair[1]) {
			t.Fatalf("expected project codes to match: %q %q", pair[0], pair[1])
		}
	}
	if projectCodesMatch("A1106", "A1160") {
		t.Fatal("different project codes should not match")
	}
}
