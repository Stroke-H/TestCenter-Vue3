package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"testcenter-server/dramalanguage"
)

func TestDramaDescriptionLanguageCacheAndFreshLang(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "server", "data"), 0700); err != nil {
		t.Fatal(err)
	}
	text := "After returning to her hometown, a young woman discovers a family secret. She must protect her daughter and uncover the truth before it is too late."
	payload := map[string]interface{}{"auth": map[string]string{"x_token": "unchanged"}, "dramaList": []string{"one", "empty"}, "dramaMeta": map[string]interface{}{
		"one":   map[string]interface{}{"description": text, "lang": "en", "description_available": true, "lang_available": true},
		"empty": map[string]interface{}{"description": "/", "lang": "", "description_available": true, "lang_available": true},
	}}
	path := filepath.Join(root, "drama_info.json")
	write := func() {
		body, _ := json.Marshal(payload)
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	read := func() map[string]interface{} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var data map[string]interface{}
		if err = json.Unmarshal(body, &data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	write()
	// Simulate an old, wrong prediction. Changing the model/rule version must
	// invalidate it even though the description text and hash are unchanged.
	oldCache, _ := json.Marshal(map[string]interface{}{
		"version": "lingua-go-1.4.0-14langs-v1",
		"items":   map[string]dramalanguage.Detection{dramalanguage.DescriptionHash(text): {Language: "id", Reliable: true, Confidence: 0.99}},
	})
	if err := os.WriteFile(filepath.Join(root, "server", "data", "drama_description_language_cache.json"), oldCache, 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareDramaDescriptionLanguages(context.Background(), root, func(string) {}); err != nil {
		t.Fatal(err)
	}
	result := read()
	check := result["dramaMeta"].(map[string]interface{})["one"].(map[string]interface{})["description_language_check"].(map[string]interface{})
	if check["status"] != "passed" {
		t.Fatalf("first check: %v", check)
	}
	if result["descriptionLanguageValidation"].(map[string]interface{})["cache_hits"] != float64(0) {
		t.Fatal("old model cache was not invalidated")
	}
	// Reuse the same description prediction, but compare the NEW lang value.
	payload["dramaMeta"].(map[string]interface{})["one"].(map[string]interface{})["lang"] = "ja"
	write()
	if err := prepareDramaDescriptionLanguages(context.Background(), root, func(string) {}); err != nil {
		t.Fatal(err)
	}
	result = read()
	check = result["dramaMeta"].(map[string]interface{})["one"].(map[string]interface{})["description_language_check"].(map[string]interface{})
	if check["status"] != "mismatch" {
		t.Fatalf("fresh lang not checked: %v", check)
	}
	if result["descriptionLanguageValidation"].(map[string]interface{})["cache_hits"] != float64(2) {
		t.Fatal("cache was not used")
	}
	if result["auth"].(map[string]interface{})["x_token"] != "unchanged" {
		t.Fatal("unrelated fields changed")
	}
	payload["dramaMeta"].(map[string]interface{})["one"].(map[string]interface{})["description_available"] = false
	write()
	if err := prepareDramaDescriptionLanguages(context.Background(), root, nil); err == nil {
		t.Fatal("fetch failure must not silently skip detection")
	}
}
