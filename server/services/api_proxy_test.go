package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newProxyTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/proxy", ProxyHandler)
	return router
}

func TestProxyHandlerSeparatesHeadersAndBody(t *testing.T) {
	var receivedHeader string
	var receivedNumberHeader string
	var receivedBody map[string]any

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom")
		receivedNumberHeader = r.Header.Get("X-Number")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		if err := json.Unmarshal(body, &receivedBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	payload := `{
		"method":"POST",
		"url":"` + upstream.URL + `",
		"headers":{"X-Custom":"header-value","X-Number":42},
		"body":{"payload":"body-value","X-Custom":"body-value"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/proxy", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	newProxyTestRouter().ServeHTTP(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if receivedHeader != "header-value" {
		t.Fatalf("expected custom header, got %q", receivedHeader)
	}
	if receivedNumberHeader != "42" {
		t.Fatalf("expected numeric header 42, got %q", receivedNumberHeader)
	}
	if receivedBody["payload"] != "body-value" {
		t.Fatalf("expected request body payload, got %#v", receivedBody)
	}
	if receivedBody["X-Custom"] != "body-value" {
		t.Fatalf("request body was unexpectedly replaced by headers: %#v", receivedBody)
	}
}

func TestProxyHandlerKeepsLegacyDataAsHeaders(t *testing.T) {
	var receivedHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Legacy-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	payload := `{"method":"GET","url":"` + upstream.URL + `","data":{"X-Legacy-Token":"legacy-token"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/proxy", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	newProxyTestRouter().ServeHTTP(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if receivedHeader != "legacy-token" {
		t.Fatalf("expected legacy data to remain available as headers, got %q", receivedHeader)
	}
}

func TestProxyHandlerEnvelopeIncludesHeadersCookiesAndBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Access-Key", "response-key")
		http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "cookie-value", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"session_token":"body-token"}}`))
	}))
	defer upstream.Close()

	payload := `{"method":"POST","url":"` + upstream.URL + `","body":{},"response_mode":"envelope"}`
	req := httptest.NewRequest(http.MethodPost, "/api/proxy", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	newProxyTestRouter().ServeHTTP(response, req)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", response.Code, response.Body.String())
	}
	var envelope ProxyResponseEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Status != http.StatusCreated {
		t.Fatalf("expected upstream status 201, got %d", envelope.Status)
	}
	if values := envelope.Headers["X-Access-Key"]; len(values) != 1 || values[0] != "response-key" {
		t.Fatalf("expected response header in envelope, got %#v", envelope.Headers)
	}
	if envelope.Cookies["session_id"] != "cookie-value" {
		t.Fatalf("expected response cookie in envelope, got %#v", envelope.Cookies)
	}
	body, ok := envelope.Body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected JSON body object, got %#v", envelope.Body)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok || data["session_token"] != "body-token" {
		t.Fatalf("expected response body in envelope, got %#v", envelope.Body)
	}
}
