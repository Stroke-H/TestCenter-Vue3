package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newPushTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/health", PushTestHealthHandler)
	router.POST("/execute", PushTestExecuteHandler)
	router.POST("/stats", PushTestStatsHandler)
	return router
}

func performPushTestRequest(t *testing.T, router http.Handler, path string, payload string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", resp.Code, resp.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}

func TestPushTestHandlersRunScriptFlow(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			if r.Method != http.MethodGet {
				t.Fatalf("health method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/v1/push":
			if r.Method != http.MethodPost {
				t.Fatalf("push method = %s", r.Method)
			}
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode upstream payload: %v", err)
			}
			if payload["app"] != "com.cocoshort.dramareels" {
				t.Fatalf("unexpected app: %#v", payload["app"])
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"task_id":"task-1"}}`))
		case "/api/v1/push/stats/task-1":
			if r.Method != http.MethodGet {
				t.Fatalf("stats method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"success":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	router := newPushTestRouter()
	health := performPushTestRequest(t, router, "/health", `{"base_url":"`+upstream.URL+`","path":"/health"}`)
	if health["ok"] != true || health["status"] != float64(http.StatusOK) {
		t.Fatalf("unexpected health result: %#v", health)
	}

	push := performPushTestRequest(t, router, "/execute", `{"base_url":"`+upstream.URL+`","path":"/api/v1/push","payload":{"app":"com.cocoshort.dramareels","user_ids":["user-1"]}}`)
	pushBody, ok := push["body"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected push body: %#v", push["body"])
	}
	data, _ := pushBody["data"].(map[string]interface{})
	if data["task_id"] != "task-1" {
		t.Fatalf("unexpected task id: %#v", data["task_id"])
	}

	stats := performPushTestRequest(t, router, "/stats", `{"base_url":"`+upstream.URL+`","path":"/api/v1/push/stats","task_id":"task-1"}`)
	if stats["ok"] != true {
		t.Fatalf("unexpected stats result: %#v", stats)
	}
}

func TestNormalizePushTestTargetRejectsAbsolutePath(t *testing.T) {
	if _, err := normalizePushTestTarget("http://34.61.243.173:8080", "https://example.com/health"); err == nil {
		t.Fatal("expected absolute request path to be rejected")
	}
}
