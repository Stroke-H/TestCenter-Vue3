package dramalanguage

import (
	"strings"
	"unicode"
)

func relatedLanguages(a, b string) bool {
	return (a == "id" || a == "ms") && (b == "id" || b == "ms") ||
		(a == "es" || a == "pt") && (b == "es" || b == "pt")
}

func letterCount(text string) int {
	n := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// A short foreign name must not make an otherwise monolingual text mixed.
// Han and Kana belong to one group when Kana is present (Japanese prose).
func scriptGuard(text string) (Detection, bool) {
	counts := map[string]int{}
	kana := false
	for _, r := range text {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana) {
			kana = true
		}
	}
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		key := "other"
		switch {
		case unicode.Is(unicode.Latin, r):
			key = "latin"
		case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana):
			key = "han"
			if kana {
				key = "japanese"
			}
		case unicode.Is(unicode.Arabic, r):
			key = "arabic"
		case unicode.Is(unicode.Cyrillic, r):
			key = "cyrillic"
		case unicode.Is(unicode.Hangul, r):
			key = "hangul"
		case unicode.Is(unicode.Thai, r):
			key = "thai"
		}
		counts[key]++
	}
	// Repeated inline names can account for >15% of a Japanese synopsis even
	// though they are not foreign-language prose. Exclude only title-case Latin
	// words directly bounded by CJK text, never standalone English sentences.
	names := inlineNameLetters(text)
	counts["latin"] -= names
	total := letterCount(text) - names
	significant := 0
	for _, count := range counts {
		if count >= 20 && float64(count) >= float64(total)*0.15 {
			significant++
		}
	}
	if significant >= 2 {
		return Detection{Category: "mixed", Method: "script_guard", Reason: "存在占比不低于15%且不少于20字母的多种文字体系，疑似混合文本，待人工复核"}, true
	}
	if counts["other"] >= 20 && float64(counts["other"]) >= float64(total)*0.15 {
		return Detection{Category: "unsupported", Method: "script_guard", Reason: "存在不支持的文字体系，待人工复核"}, true
	}
	return Detection{}, false
}

func inlineNameLetters(text string) int {
	runes := []rune(text)
	letters := 0
	for i := 0; i < len(runes); {
		if !unicode.Is(unicode.Latin, runes[i]) {
			i++
			continue
		}
		start := i
		for i < len(runes) && unicode.Is(unicode.Latin, runes[i]) {
			i++
		}
		if start == 0 || i == len(runes) || i-start < 2 || i-start > 24 ||
			!isCJKLetter(runes[start-1]) || !isCJKLetter(runes[i]) || !unicode.IsUpper(runes[start]) {
			continue
		}
		name := true
		for _, r := range runes[start+1 : i] {
			if !unicode.IsLower(r) {
				name = false
				break
			}
		}
		if name {
			letters += i - start
		}
	}
	return letters
}

func isCJKLetter(r rune) bool {
	return unicode.IsLetter(r) && unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// Sentence checks are consistency evidence from the SAME model, not an
// independent verifier. Only substantial, confidently identified parts count.
func mixedSentences(text string) bool {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return strings.ContainsRune(".!?。！？;；\n", r)
	})
	weights := map[string]int{}
	total := letterCount(text)
	for _, part := range parts {
		n := letterCount(part)
		if n < 40 {
			continue
		}
		d := detectPlain(part)
		if !d.Reliable || d.Confidence < 0.9 || d.Margin < 0.5 {
			continue
		}
		code := d.Language
		if code == "id" || code == "ms" {
			code = "id/ms"
		} else if code == "es" || code == "pt" {
			code = "es/pt"
		}
		weights[code] += n
	}
	significant := 0
	for _, n := range weights {
		if n >= 40 && float64(n) >= float64(total)*0.2 {
			significant++
		}
	}
	return significant >= 2
}
