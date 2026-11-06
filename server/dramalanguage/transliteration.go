package dramalanguage

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var transliterationWords = regexp.MustCompile(`[\p{L}\p{M}ʿʾ'’\-]+`)
var arabicTransliterationLetters = regexp.MustCompile(`[ʿʾḥḍṣṭẓḤḌṢṬẒ]`)
var arabicTransliterationArticles = regexp.MustCompile(`^(?:al|ash|as|ar|at|ad|az|an|ath|adh)-[a-z']{2,}`)

// These signals must occur together. Arabic names, generic accented letters,
// or the configured lang value alone must never decide a text's language.
var arabicTransliterationFunctionWords = map[string]bool{
	"fi": true, "fii": true, "wa": true, "fa": true, "li": true, "bi": true,
	"min": true, "ila": true, "alaa": true, "ala": true, "an": true,
	"anna": true, "inna": true, "alladhi": true, "alladhina": true,
	"allati": true, "ladhi": true, "huwa": true, "hiya": true,
	"kana": true, "kanat": true, "lam": true, "lan": true, "lakin": true,
	"lakinaha": true, "lakinahu": true, "hatta": true, "thumma": true,
	"baad": true, "bada": true, "bayna": true, "indama": true,
}

func foldTransliteration(value string) string {
	var folded strings.Builder
	for _, char := range norm.NFD.String(strings.ToLower(value)) {
		if unicode.Is(unicode.Mn, char) {
			continue
		}
		if char == 'ʿ' || char == 'ʾ' || char == '’' {
			char = '\''
		}
		folded.WriteRune(char)
	}
	return folded.String()
}

// Lingua's script filter cannot recognize Arabic written in Latin letters.
// Strong Arabic-specific evidence is classified locally; weaker plain-ASCII
// transliteration is marked uncertain rather than forcing a Latin language.
// Neither this function nor its cache key receives the expected lang value.
func detectArabicTransliteration(text string) (Detection, bool) {
	text = norm.NFC.String(text)
	latin, letters := 0, 0
	for _, char := range text {
		if unicode.IsLetter(char) {
			letters++
			if unicode.Is(unicode.Latin, char) {
				latin++
			}
		}
	}
	if letters < 20 || float64(latin)/float64(letters) < 0.85 {
		return Detection{}, false
	}
	words := transliterationWords.FindAllString(text, -1)
	if len(words) < 10 {
		return Detection{}, false
	}
	articles, markedWords, features := 0, 0, 0
	functionWords := map[string]bool{}
	markedForms := map[string]bool{}
	for _, word := range words {
		folded := foldTransliteration(word)
		marked := arabicTransliterationLetters.MatchString(word)
		article := arabicTransliterationArticles.MatchString(folded)
		if article {
			articles++
		}
		if marked {
			markedWords++
			markedForms[folded] = true
		}
		function := false
		for _, part := range strings.Split(folded, "-") {
			if arabicTransliterationFunctionWords[part] {
				functionWords[part] = true
				function = true
			}
		}
		if marked || article || function {
			features++
		}
	}
	density := float64(features) / float64(len(words))
	if articles >= 3 && markedWords >= 3 && len(markedForms) >= 3 && len(functionWords) >= 3 && density >= 0.35 {
		return Detection{Language: "ar", Reliable: false, Category: "transliterated", Method: "arabic_transliteration_rules",
			Reason: "疑似阿拉伯语拉丁转写：命中专用字符、冠词和功能词；经验规则未经独立校准，待人工复核"}, true
	}
	if articles >= 5 && len(functionWords) >= 4 && density >= 0.45 {
		return Detection{Language: "ar", Reliable: false, Method: "arabic_transliteration_candidate",
			Reason: "疑似阿拉伯语拉丁转写，证据不足，暂记无法确定"}, true
	}
	return Detection{}, false
}
