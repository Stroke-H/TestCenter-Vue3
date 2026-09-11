package services

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeDefectVersions(t *testing.T) {
	got, err := normalizeDefectVersions([]string{" 2.0.0 ", "2.0.0", "v3-beta"})
	if err != nil || !reflect.DeepEqual(got, []string{"2.0.0", "v3-beta"}) {
		t.Fatalf("unexpected normalized versions: %v %v", got, err)
	}
	for _, values := range [][]string{{" "}, {strings.Repeat("a", 129)}, {"a\nb"}, make([]string, 501)} {
		if _, err := normalizeDefectVersions(values); err == nil {
			t.Fatalf("invalid versions accepted: %v", values)
		}
	}
	got, err = normalizeDefectVersions([]string{})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatal("empty configuration must remain an empty array")
	}
}

func TestDecodeDefectVersionStages(t *testing.T) {
	legacy, err := decodeDefectVersionConfig(`["1.0","2.0"]`)
	if err != nil || legacy.Online != "" || legacy.Testing != "" || len(legacy.Versions) != 2 {
		t.Fatalf("legacy values must be preserved without guessing stages: %+v %v", legacy, err)
	}
	config, err := decodeDefectVersionConfig(`{"online":"1.0","testing":"2.0","versions":["1.0","2.0"]}`)
	if err != nil || config.Online != "1.0" || config.Testing != "2.0" || len(config.Versions) != 2 {
		t.Fatalf("stage configuration not decoded: %+v %v", config, err)
	}
	if _, err := decodeDefectVersionConfig(`invalid`); err == nil {
		t.Fatal("invalid configuration must fail closed")
	}
}

func TestDefectVersionValidation(t *testing.T) {
	for _, test := range []struct {
		name, value, previous string
		configured            bool
		want                  bool
	}{
		{"configured selection", "2.0", "", true, true},
		{"reject other project version", "3.0", "", true, false},
		{"preserve unchanged history", "1.0", "1.0", true, true},
		{"moving project cannot keep unlisted version", "1.0", "", true, false},
		{"optional blank", "", "", true, true},
		{"legacy project free input", "custom", "", false, true},
		{"legacy length bounded", strings.Repeat("x", 129), "", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := defectVersionAllowed(test.value, test.previous, []string{"2.0"}, test.configured); got != test.want {
				t.Fatalf("allowed=%v want=%v", got, test.want)
			}
		})
	}
	if defectVersionAllowed("2.0", "", []string{}, true) {
		t.Fatal("empty configured list must not fall back to unrestricted input")
	}
}
