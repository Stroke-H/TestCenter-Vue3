package service

import (
	"encoding/json"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"strings"
	"testcenter-server/feishu/model"
)

// ParseMessageContent converts raw Feishu message content into a string and standard model
func ParseMessageContent(msgType string, rawContent string) (string, error) {
	var contentMap map[string]interface{}
	if err := json.Unmarshal([]byte(rawContent), &contentMap); err != nil {
		return rawContent, nil // Fallback to raw if unmarshal fails
	}

	switch msgType {
	case "text":
		if text, ok := contentMap["text"].(string); ok {
			return text, nil
		}
	case "post":
		// Post (Rich Text) is complex, for phase 1 we extract plain text parts
		return extractPlainTextFromPost(contentMap), nil
	case "image":
		if key, ok := contentMap["image_key"].(string); ok {
			return "[Image: " + key + "]", nil
		}
	case "file":
		if key, ok := contentMap["file_key"].(string); ok {
			return "[File: " + key + "]", nil
		}
	}

	return rawContent, nil
}

// extractPlainTextFromPost recursively extracts text from rich text structure
func extractPlainTextFromPost(post map[string]interface{}) string {
	// A post content is usually: {"title":"xxx", "content":[[{"tag":"text", "text":"xxx"}, ...]]}
	var sb strings.Builder
	if title, ok := post["title"].(string); ok && title != "" {
		sb.WriteString(title + "\n")
	}

	content, ok := post["content"].([]interface{})
	if !ok {
		return ""
	}

	for _, line := range content {
		elements, ok := line.([]interface{})
		if !ok {
			continue
		}
		for _, el := range elements {
			element, ok := el.(map[string]interface{})
			if !ok {
				continue
			}
			tag, _ := element["tag"].(string)
			if tag == "text" {
				if t, ok := element["text"].(string); ok {
					sb.WriteString(t)
				}
			} else if tag == "at" {
				if label, ok := element["label"].(string); ok {
					sb.WriteString(label)
				}
			}
		}
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

// ExtractMentions extracts users mentioned in the message
func ExtractMentions(mentions []*larkim.MentionEvent) []model.Mention {
	var result []model.Mention
	for _, m := range mentions {
		if m != nil && m.Id != nil {
			result = append(result, model.Mention{
				UserID: *m.Id.OpenId,
				Name:   *m.Name,
				Key:    *m.Key,
			})
		}
	}
	return result
}
