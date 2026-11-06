package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeleteAccountLookupReadOnlyFlow(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/api/pwd_login":
			if r.Method != "POST" {
				t.Error("login must POST")
			}
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["app"] != "com.shorts.wave.drama" || body["email"] != "test" || body["password"] != "secret" {
				t.Error("wrong auth payload")
			}
			http.SetCookie(w, &http.Cookie{Name: "x-token", Value: "admin-test", Path: "/"})
		case "/api/management/user/list":
			cookie, err := r.Cookie("x-token")
			if err != nil || cookie.Value != "admin-test" {
				t.Error("missing authentication")
			}
			q := r.URL.Query()
			if q.Get("app") != "com.example" || q.Get("user_id") != "user-1" || q.Get("start") != "2025-01-01" || q.Get("end") != "2026-11-03" || q.Get("page_size") != "20" {
				t.Errorf("wrong query %v", q)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":0,"data":[{"uid":"user-1","app":"com.example","device_id":"device-1","app_version":"1.2.3","advertising_id":"ad-1","session_token":"must-not-export","refresh_token":"must-not-export","coin":999}]}`))
		default:
			t.Error("must never call anonymous login or delete")
			http.Error(w, "forbidden", 403)
		}
	}))
	defer s.Close()
	h, err := lookupDeleteAccountUser(context.Background(), dramaProfile{Email: "test", Password: "secret", LoginURL: s.URL + "/api/pwd_login"}, "com.example", "user-1", time.Date(2026, 11, 2, 17, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if h["device-uuid"] != "device-1" || h["version"] != "1.2.3" || h["advertising-id"] != "ad-1" {
		t.Fatalf("mapping failed: %v", h)
	}
	if len(h) != 4 || calls != 2 {
		t.Fatalf("unexpected fields or requests: %v %d", h, calls)
	}
}

func TestDeleteAccountLookupRejectsUnsafeUser(t *testing.T) {
	for _, raw := range []string{
		`[]`,
		`[{"uid":"other","app":"com.example","device_id":"device"}]`,
		`[{"uid":"user-1","app":"other","device_id":"device"}]`,
		`[{"uid":"user-1","app":"com.example"}]`,
		`[{"uid":"user-1","app":"com.example","device_id":"bad\r\nheader"}]`,
		`[{"uid":"user-1","app":"com.example","device_id":"one"},{"uid":"user-1","app":"com.example","device_id":"two"}]`,
		`{"items":[]}`,
	} {
		if _, err := mapDeleteAccountUser(json.RawMessage(raw), "com.example", "user-1"); err == nil {
			t.Errorf("unsafe match accepted: %s", raw)
		}
	}
}

func TestDeleteAccountLookupNeverFollowsLoginRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("credentials must not follow redirect") }))
	defer target.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer s.Close()
	if _, err := lookupDeleteAccountUser(context.Background(), dramaProfile{Email: "test", Password: "secret", LoginURL: s.URL}, "com.example", "user-1", time.Now()); err == nil {
		t.Fatal("redirect accepted")
	}
}
