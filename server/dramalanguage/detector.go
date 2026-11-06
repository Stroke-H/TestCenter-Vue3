// Package dramalanguage performs local-only description language checks.
package dramalanguage

import (
	"crypto/sha256"
	"fmt"
	"html"
	"regexp"
	"strings"
	"sync"
	"unicode"

	lingua "github.com/pemistahl/lingua-go"
)

// Change this when language coverage, normalization or thresholds change.
const CacheVersion = "lingua-go-1.4.0-v4-inline-names-url-evidence"

type Detection struct {
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
	Margin     float64 `json:"margin"`
	Reliable   bool    `json:"reliable"`
	Method     string  `json:"method,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	Category   string  `json:"category,omitempty"`
}

type Result struct {
	Status          string  `json:"status"`
	Expected        string  `json:"expected"`
	Detected        string  `json:"detected"`
	Confidence      float64 `json:"confidence"`
	Reason          string  `json:"reason,omitempty"`
	DescriptionHash string  `json:"description_hash"`
	Method          string  `json:"method,omitempty"`
}

var detectorOnce sync.Once
var detector lingua.LanguageDetector
var htmlTag = regexp.MustCompile(`<[^>]*>`)

// URL paths, tokens and domain names are not natural-language evidence. Limit
// the match to URL ASCII characters so adjacent CJK prose is never swallowed.
var evidenceURL = regexp.MustCompile(`(?i)(?:https?://|www\.)[a-z0-9/?:@!$&*+;=%,_.~#()\-]+`)

func localDetector() lingua.LanguageDetector {
	detectorOnce.Do(func() {
		detector = lingua.NewLanguageDetectorBuilder().FromLanguages(
			lingua.English, lingua.Japanese, lingua.Spanish, lingua.Indonesian,
			lingua.French, lingua.Korean, lingua.Thai, lingua.Russian, lingua.Malay,
			lingua.Chinese, lingua.Arabic, lingua.Portuguese, lingua.German, lingua.Turkish,
			// Rejection candidates, not additional supported business lang values.
			lingua.Italian, lingua.Vietnamese, lingua.Dutch, lingua.Polish,
		).Build()
	})
	return detector
}

func NormalizeLanguage(value string) string {
	code := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
	base := strings.SplitN(code, "-", 2)[0]
	if base == "in" {
		base = "id"
	}
	if base == "tw" {
		base = "zh"
	}
	switch base {
	case "en", "ja", "es", "id", "fr", "ko", "th", "ru", "ms", "zh", "ar", "pt", "de", "tr":
		return base
	default:
		return ""
	}
}

func NormalizeDescription(value string) string {
	return strings.Join(strings.Fields(html.UnescapeString(htmlTag.ReplaceAllString(value, " "))), " ")
}

func DescriptionHash(description string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(NormalizeDescription(description))))
}

// Cache only the text-based prediction, never the expected lang value.
func Detect(description string) Detection {
	text := evidenceURL.ReplaceAllString(NormalizeDescription(description), " ")
	letters := 0
	for _, char := range text {
		if unicode.IsLetter(char) {
			letters++
		}
	}
	if letters < 20 {
		return Detection{Category: "unknown", Reason: "有效字母少于20个，待人工复核"}
	}
	if result, matched := scriptGuard(text); matched {
		return result
	}
	if result, matched := detectArabicTransliteration(text); matched {
		return result
	}
	result := detectPlain(text)
	if NormalizeLanguage(result.Language) == "" && result.Language != "" {
		result.Reliable, result.Category = false, "unsupported"
		result.Reason = "疑似范围外语言（" + result.Language + "），不强制归入支持的14种语言，待人工复核"
		return result
	}
	if mixedSentences(text) {
		result.Reliable, result.Category = false, "mixed"
		result.Reason = "多个较长语句识别出不同语种，疑似混合文本，待人工复核"
	}
	return result
}

func detectPlain(text string) Detection {
	values := localDetector().ComputeLanguageConfidenceValues(text)
	if len(values) == 0 {
		return Detection{}
	}
	first := values[0]
	margin := first.Value()
	if len(values) > 1 {
		margin -= values[1].Value()
	}
	return Detection{
		Language:   strings.ToLower(first.Language().IsoCode639_1().String()),
		Confidence: first.Value(), Margin: margin,
		Reliable: first.Value() >= 0.75 && margin >= 0.25,
		Method:   "lingua",
		Category: "language",
	}
}

func Compare(description, lang string, detection Detection) Result {
	text := NormalizeDescription(description)
	result := Result{Expected: NormalizeLanguage(lang), Detected: detection.Language,
		Confidence: detection.Confidence, DescriptionHash: DescriptionHash(description), Method: detection.Method, Reason: detection.Reason}
	if text == "" || text == "/" {
		result.Status, result.Reason = "skipped", "简介为空或只有一个 /"
	} else if result.Expected == "" {
		result.Status, result.Reason = "invalid_lang", "lang 为空或不在支持的语言范围内"
	} else if detection.Category == "mixed" || detection.Category == "unsupported" {
		result.Status = detection.Category
	} else if !detection.Reliable {
		result.Status = "uncertain"
		if result.Reason == "" {
			result.Reason = "文本较短、混合语种或识别置信度不足"
		}
	} else if detection.Language == result.Expected {
		result.Status = "passed"
	} else if relatedLanguages(detection.Language, result.Expected) {
		result.Status, result.Reason = "uncertain", "相近语种（id/ms 或 es/pt）发生冲突，本地分数不等于正确概率，待人工复核"
	} else {
		result.Status = "mismatch"
		result.Reason = "本地识别疑似不一致；刷新元数据仅确认数据未变，不代表独立语言复核，需人工确认"
	}
	return result
}
