package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestProjectConfigResponseValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		finish     openai.FinishReason
		wantOK     bool
	}{
		{"object", `{"configs":[{"key":"预播","content":"新增预播"}]}`, openai.FinishReasonStop, true},
		{"empty list", `{"configs":[]}`, openai.FinishReasonStop, true},
		{"legacy", "```json\n[]\n```", openai.FinishReasonStop, true},
		{"blank", "", openai.FinishReasonStop, false},
		{"reasoning only", "", openai.FinishReasonLength, false},
		{"valid but truncated", `{"configs":[]}`, openai.FinishReasonLength, false},
		{"prose", "正在分析历史报告", openai.FinishReasonStop, false},
		{"wrong wrapper", `{"message":"没有配置"}`, openai.FinishReasonStop, false},
		{"null", `{"configs":null}`, openai.FinishReasonStop, false},
		{"missing item fields", `{"configs":[{}]}`, openai.FinishReasonStop, false},
		{"conflicting items", `{"configs":[{"key":"间隔","content":"30秒"},{"key":"间隔","content":"90秒"}]}`, openai.FinishReasonStop, false},
		{"incomplete", `{"configs":[`, openai.FinishReasonStop, false},
		{"extra text", `{"configs":[]} more`, openai.FinishReasonStop, false},
		{"filtered", `[]`, openai.FinishReasonContentFilter, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: tc.finish, Message: openai.ChatCompletionMessage{Content: tc.body, ReasoningContent: "not configuration data"}}}}
			_, err := projectConfigResponseContent(resp)
			if (err == nil) != tc.wantOK {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
	if _, err := projectConfigResponseContent(openai.ChatCompletionResponse{}); err == nil {
		t.Fatal("empty choices accepted")
	}
}

type projectConfigMockTransport func(*http.Request) (*http.Response, error)

func (f projectConfigMockTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProjectConfigV4WireFormat(t *testing.T) {
	for _, model := range []string{"deepseek-v4-flash", "deepseek-v4-pro", "other-model"} {
		t.Run(model, func(t *testing.T) {
			base := projectConfigMockTransport(func(r *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(r.Body)
				var payload map[string]json.RawMessage
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(model, "deepseek-v4-") {
					if string(payload["thinking"]) != `{"type":"disabled"}` {
						t.Fatal("thinking not disabled")
					}
				} else if _, ok := payload["thinking"]; ok {
					t.Fatal("non-DeepSeek provider changed")
				}
				if !strings.Contains(string(payload["response_format"]), "json_object") {
					t.Fatal("JSON mode lost")
				}
				if r.ContentLength != int64(len(body)) {
					t.Fatal("wrong content length")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"configs\":[]}"}}]}`))}, nil
			})
			config := openai.DefaultConfig("fake-test-key")
			config.BaseURL = "https://example.invalid/v1"
			config.HTTPClient = &http.Client{Transport: projectConfigTransport{base: base}}
			resp, err := openai.NewClientWithConfig(config).CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "JSON test"}}, ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = projectConfigResponseContent(resp); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProjectConfigEvidenceFormatting(t *testing.T) {
	item := extractedProjectConfig{Feature: "插屏间隔", Evidence: "自然量  插屏间隔 90秒 & 保留预播", Action: " UPSERT "}
	valid, issues := inspectProjectTreeExtraction([]extractedProjectConfig{item}, "自然量\n插屏间隔\t90秒 &amp; 保留预播")
	if len(issues) != 0 || len(valid) != 1 || valid[0].Action != "upsert" {
		t.Fatal("format-only differences rejected")
	}
	for _, evidence := range []string{"自然量 插屏间隔 30秒 & 保留预播", "自然量 插屏间隔 9 0秒 & 保留预播", "自然量……90秒", "", "\n\t"} {
		item.Evidence = evidence
		if got := validateProjectTreeExtraction([]extractedProjectConfig{item}, "自然量 插屏间隔 90秒 & 保留预播"); len(got) != 0 {
			t.Fatalf("invalid evidence accepted: %q", evidence)
		}
	}
}

func TestProjectConfigBoundedEvidenceRepair(t *testing.T) {
	good := extractedProjectConfig{Key: "预播", Feature: "预播", Content: "新增预播", Action: "upsert", Evidence: "新增预播"}
	bad := extractedProjectConfig{Key: "插屏", Feature: "插屏间隔", Content: "插屏间隔90秒", Value: "90秒", Action: "upsert", Evidence: "间隔调整为90秒"}
	encode := func(items []extractedProjectConfig) string {
		b, _ := json.Marshal(map[string]interface{}{"configs": items})
		return string(b)
	}
	for _, mode := range []string{"fixed", "still-invalid", "changed-value", "dropped-item", "changed-action"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			call := func(ctx context.Context, prompt, source string) (string, error) {
				calls++
				if calls == 1 {
					return encode([]extractedProjectConfig{good, bad}), nil
				}
				if calls > 2 {
					t.Fatal("unbounded repair")
				}
				var payload struct {
					Items []extractedProjectConfig `json:"items_to_correct"`
				}
				if err := json.Unmarshal([]byte(source), &payload); err != nil {
					t.Fatal(err)
				}
				if len(payload.Items) != 1 || payload.Items[0].Key != bad.Key {
					t.Fatal("valid items unnecessarily rewritten")
				}
				fixed := bad
				fixed.Evidence = "插屏间隔90秒"
				switch mode {
				case "still-invalid":
					fixed.Evidence = "改成90秒"
				case "changed-value":
					fixed.Value = "30秒"
				case "dropped-item":
					return encode(nil), nil
				case "changed-action":
					fixed.Action = "remove"
				}
				return encode([]extractedProjectConfig{fixed}), nil
			}
			items, err := extractProjectTreeValidated(context.Background(), "新增预播；插屏间隔90秒", call)
			if calls != 2 {
				t.Fatalf("calls=%d", calls)
			}
			if mode == "fixed" {
				if err != nil || len(items) != 2 || items[0] != good || items[1].Value != "90秒" {
					t.Fatalf("repair failed: %v", err)
				}
			} else if err == nil || items != nil || !strings.Contains(err.Error(), "第2项") {
				t.Fatalf("unsafe repair accepted or missing diagnostics: %v", err)
			}
		})
	}
}

func TestProjectConfigValidEvidenceNeedsNoRepair(t *testing.T) {
	calls := 0
	_, err := extractProjectTreeValidated(context.Background(), "新增预播", func(context.Context, string, string) (string, error) {
		calls++
		return `{"configs":[{"key":"预播","feature":"预播","content":"新增预播","action":"upsert","evidence":"新增预播"}]}`, nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("unexpected repair: %d %v", calls, err)
	}
}
