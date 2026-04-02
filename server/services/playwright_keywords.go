package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// KeywordExecutor defines the function signature for a keyword implementation
type KeywordExecutor func(ctx context.Context, page playwright.Page, args map[string]string) (string, error)

// GetKeywordMap returns the mapping of keyword names to their execution logic
func GetKeywordMap() map[string]KeywordExecutor {
	return map[string]KeywordExecutor{
		"Launch":          ExecuteLaunch,
		"Goto":            ExecuteGoto,
		"Click":           ExecuteClick,
		"Fill":            ExecuteFill,
		"Press":           ExecutePress,
		"TextContent":     ExecuteTextContent,
		"WaitForSelector": ExecuteWaitForSelector,
		"Screenshot":      ExecuteScreenshot,
		"Sleep":           ExecuteSleep,
		"AssertURL":       ExecuteAssertURL,
		"AssertText":      ExecuteAssertText,
		"Close":           ExecuteClose,
	}
}

// --- Keyword Implementations ---

func ExecuteLaunch(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	// Launch is handled specially in the runner to initialize the browser
	return "Browser launched", nil
}

func ExecuteGoto(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	url := args["url"]
	if url == "" {
		return "", fmt.Errorf("URL is required")
	}
	_, err := page.Goto(url)
	return fmt.Sprintf("Navigated to %s", url), err
}

func ExecuteClick(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	selector := args["selector"]
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}
	err := page.Click(selector)
	return fmt.Sprintf("Clicked element %s", selector), err
}

func ExecuteFill(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	selector := args["selector"]
	value := args["value"]
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}
	err := page.Fill(selector, value)
	return fmt.Sprintf("Filled %s with value", selector), err
}

func ExecutePress(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	selector := args["selector"]
	key := args["key"]
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}
	if key == "" {
		return "", fmt.Errorf("key is required")
	}
	err := page.Press(selector, key)
	return fmt.Sprintf("Pressed %s on %s", key, selector), err
}

func ExecuteTextContent(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	selector := args["selector"]
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}
	text, err := page.TextContent(selector)
	return text, err
}

func ExecuteWaitForSelector(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	selector := args["selector"]
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}
	_, err := page.WaitForSelector(selector)
	return fmt.Sprintf("Found element %s", selector), err
}

func ExecuteScreenshot(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	// Base64 screenshot
	img, err := page.Screenshot()
	if err != nil {
		return "", err
	}
	return string(img), nil // This will be sent as a message specially
}

func ExecuteSleep(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	durationStr := args["seconds"]
	var duration int
	fmt.Sscanf(durationStr, "%d", &duration)
	if duration <= 0 {
		duration = 1
	}
	time.Sleep(time.Duration(duration) * time.Second)
	return fmt.Sprintf("Slept for %d seconds", duration), nil
}

func ExecuteAssertURL(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	expected := args["expected"]
	actual := page.URL()
	if strings.Contains(actual, expected) {
		return fmt.Sprintf("URL assertion passed: contains %s", expected), nil
	}
	return "", fmt.Errorf("URL assertion failed: expected %s, got %s", expected, actual)
}

func ExecuteAssertText(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	selector := args["selector"]
	expected := args["expected"]
	actual, err := page.TextContent(selector)
	if err != nil {
		return "", err
	}
	if strings.Contains(actual, expected) {
		return fmt.Sprintf("Text assertion passed for %s: contains %s", selector, expected), nil
	}
	return "", fmt.Errorf("Text assertion failed for %s: expected %s, got %s", selector, expected, actual)
}

func ExecuteClose(ctx context.Context, page playwright.Page, args map[string]string) (string, error) {
	// Close is handled in runner
	return "Browser closed", nil
}
