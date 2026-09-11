package services

import (
	"fmt"
	"testing"
)

func TestNextAcceptanceVersion(t *testing.T) {
	for _, test := range []struct {
		name          string
		history       []string
		current, want string
	}{
		{"default", nil, "v1.0.0", "v1.0.1"},
		{"padding", []string{"1.0.01", "1.0.02", "1.1.01"}, "1.1.01", "1.2.00"},
		{"patch majority", []string{"1.0.01", "1.0.02", "1.0.03", "1.1.01"}, "1.1.01", "1.1.02"},
		{"minor", []string{"2.61.0", "2.62.0", "2.63.0", "2.63.1"}, "2.63.1（build 2809）", "2.64.0"},
		{"ten", []string{"2.60.0", "2.70.0"}, "2.70.0", "2.80.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := []AcceptanceReport{}
			for i, v := range test.history {
				r = append(r, AcceptanceReport{ProjectCode: "A1", Version: v, CreatedAt: fmt.Sprintf("2026-08-%02dT00:00:00Z", i+1)})
			}
			got, _, err := NextAcceptanceVersion("A1", test.current, r)
			if err != nil || got != test.want {
				t.Fatalf("got %q %v want %q", got, err, test.want)
			}
		})
	}
	if _, _, err := NextAcceptanceVersion("A1", "beta", nil); err == nil {
		t.Fatal("unknown format must fail")
	}
}
