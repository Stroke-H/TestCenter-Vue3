package services

import "testing"

func TestDefectProjectRoleActionMatrix(t *testing.T) {
	wants := map[string]map[string]bool{
		"":          {},
		"unknown":   {},
		"viewer":    {"view": true},
		"developer": {"view": true, "comment": true, "edit": true, "process": true},
		"tester":    {"view": true, "comment": true, "edit": true, "create": true, "verify": true, "reopen": true},
	}
	for role, actions := range wants {
		for _, action := range []string{"view", "comment", "edit", "process", "create", "verify", "reopen", "archive"} {
			t.Run(role+"/"+action, func(t *testing.T) {
				if got := defectRoleAllows(role, action); got != actions[action] {
					t.Fatalf("role=%q action=%q allowed=%v, want %v", role, action, got, actions[action])
				}
			})
		}
	}
	for _, role := range []string{"admin", "lead", "open"} {
		for _, action := range []string{"view", "comment", "edit", "process", "create", "verify", "reopen"} {
			if !defectRoleAllows(role, action) {
				t.Fatalf("role=%q should allow %s at the project layer", role, action)
			}
		}
	}
}

func TestDefectProjectScopeRejectsMissingUser(t *testing.T) {
	clause, args := defectProjectScopeSQL(nil)
	if clause != "1=0" || len(args) != 0 {
		t.Fatalf("missing identity must not expose project records: %q %v", clause, args)
	}
}
