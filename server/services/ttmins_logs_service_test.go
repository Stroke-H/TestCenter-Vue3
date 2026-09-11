package services

import (
	"net/url"
	"testing"
	"time"
)

func TestTTminsLogsTicketIsScopedAndSingleUse(t *testing.T) {
	hub := &ttminsLogsHub{targets: map[string]*ttminsLogTarget{}}
	target := &ttminsLogTarget{ID: "target-1", Ticket: "secret", TicketUntil: time.Now().Add(time.Minute)}
	hub.targets[target.ID] = target
	if hub.consumeTicket("other", "secret") != nil || hub.consumeTicket(target.ID, "wrong") != nil {
		t.Fatal("ticket must be scoped to its target")
	}
	if hub.consumeTicket(target.ID, "secret") != target {
		t.Fatal("valid ticket should be accepted")
	}
	if hub.consumeTicket(target.ID, "secret") != nil {
		t.Fatal("ticket must be single use")
	}
}

func TestTTminsLogsProxyOnlyAllowsActiveTargetOrigin(t *testing.T) {
	hub := &ttminsLogsHub{targets: map[string]*ttminsLogTarget{"one": {URL: "https://example.com/page"}}}
	for raw, want := range map[string]bool{
		"https://example.com/file.css":           true,
		"http://example.com/file.css":            false,
		"https://example.com.evil.test/file.css": false,
		"https://other.test/file.css":            false,
	} {
		parsed, _ := url.Parse(raw)
		if got := hub.proxyOriginAllowed(parsed); got != want {
			t.Fatalf("origin %s allowed=%v want=%v", raw, got, want)
		}
	}
}

func TestTTminsLogsBundledClientManifest(t *testing.T) {
	target, err := ttminsLogsAssets.ReadFile("assets/ttmins_logs/target.js")
	if err != nil || len(target) < 100_000 {
		t.Fatalf("controlled target is missing: %v", err)
	}
	manifest, err := ttminsLogsAssets.ReadFile("assets/ttmins_logs/integrity.json")
	if err != nil || len(manifest) == 0 {
		t.Fatalf("integrity manifest is missing: %v", err)
	}
	if err := validateTTminsLogsClientAssets(); err != nil {
		t.Fatalf("controlled client integrity validation failed: %v", err)
	}
}
