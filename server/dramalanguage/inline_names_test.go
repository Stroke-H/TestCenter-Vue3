package dramalanguage

import "testing"

// 2026-11-02 snapshot: three drama records share this Japanese synopsis.
// Latin names repeated inside Japanese sentences are not bilingual prose.
const japaneseInlineNames = "おとなしく忍耐強いBonnieは、長年Marcoを深く愛していた。しかし、二人の赤ちゃんの性別発表パーティーで、Marcoの元カノEmmaが隠し子のElenaを連れて乗り込んできた。その後の口論の中でBonnieは不幸にも流産してしまい、それなのにMarcoはEmmaを信じたため、Bonnieの心は完全に打ち砕かれた。離婚後、心を閉ざしたBonnieは崖から飛び降りて命を絶った。最初、MarcoはBonnieがただ怒って狂っただけだと思っていたが、真実が次々と明らかになるにつれ、彼は驚愕する——かつて彼をうつ病から救い出し、祖母に腎臓を提供し、最初から最後まで本当に彼を愛していたのは、ずっとBonnieだったのだ。そしてElenaは彼の実の子では決してなかった。姉の復讐のため、Bonnieの双子の妹JennieはBonnieと全く同じ姿に扮してMarcoを苦しめ、Bonnieが生前味わった苦痛をすべて彼に体験させた。Emmaの悪行が完全に暴かれるにつれ、Marcoは底なしの後悔と精神崩壊に陥った。最終的に彼は銃を自らに向けて命を絶ち、かつての愛し合った二人は永遠に引き裂かれ、共に命を落とすという凄絶な結末を迎えた。"

func TestJapaneseInlineNamesAreNotMixedProse(t *testing.T) {
	if r, matched := scriptGuard(japaneseInlineNames); matched {
		t.Fatalf("inline names must not trigger script guard: %+v", r)
	}
	d := Detect(japaneseInlineNames)
	if got := Compare(japaneseInlineNames, "ja", d); got.Status != "passed" {
		t.Fatalf("Japanese synopsis with inline names should pass: %+v", got)
	}
	if got := Compare(japaneseInlineNames, "pt", d); got.Status != "mismatch" {
		t.Fatalf("name filtering must not whitelist configured lang: %+v", got)
	}
}

func TestInlineNameEvidenceIsNarrow(t *testing.T) {
	for text, want := range map[string]int{
		"彼女Bonnieは彼Marcoを愛する":            11,
		"彼女Bonnie Smithは彼を愛する":           0,
		"彼女English prose is not a name彼": 0,
		"彼女lowercaseは彼を愛する":              0,
		"Bonnieは彼を愛する":                   0,
		"彼女ABCは彼を愛する":                    0,
	} {
		if got := inlineNameLetters(text); got != want {
			t.Errorf("%q: got %d, want %d", text, got, want)
		}
	}
	english := "After returning to her hometown, a young woman discovers a hidden family secret. She must protect her daughter and uncover the truth before it is too late."
	text := japaneseInlineNames + " " + english + " " + english
	if got := Compare(text, "ja", Detect(text)); got.Status != "mixed" {
		t.Fatalf("real English paragraphs must still trigger mixed review: %+v", got)
	}
}

func TestURLsAreNotLanguageEvidence(t *testing.T) {
	text := "愛する娘を守るため、彼女は困難に立ち向かい、家族が隠していた真実を探し始めます。"
	url := " https://example.com/TheYoungWomanReturnsToHerHometownAndDiscoversTheFamilySecretAndProtectsHerDaughterBeforeItIsTooLate?token=EnglishWordsInURL"
	if got := Compare(text+url, "ja", Detect(text+url)); got.Status != "passed" {
		t.Fatalf("URL characters must not overwhelm Japanese prose: %+v", got)
	}
	if got := Compare(url, "en", Detect(url)); got.Status != "uncertain" {
		t.Fatalf("a URL alone is not an English description: %+v", got)
	}
	if got := Detect("https://example.com/path" + text); got.Language != "ja" || !got.Reliable {
		t.Fatalf("URL removal swallowed adjacent Japanese prose: %+v", got)
	}
	if DescriptionHash(text+url) == DescriptionHash(text) {
		t.Fatal("original content, not cleaned evidence, must remain the cache/failure fingerprint")
	}
}

func TestConfirmedProductionLanguageMismatchesStayReported(t *testing.T) {
	for _, sample := range []struct{ text, lang, detected string }{
		{"The unthinkable occurs at her ex-husband's grandfather's birthday bash, where Wendy Buck is publicly ridiculed and disrespected. All present disdain her status, deeming her deserving of abandonment. Amidst this contempt, Jack Carter stands tall, offering a grand betrothal gift and proposing to Wendy Buck in front of everyone.", "es", "en"},
		{"江伊一是江家被抱错的真千金，18年后回到江家，所有人都以为她会和江家假千金江舒艺雌竞，却没想到，她回家第一件事情就是抱紧了江舒艺的大腿。相处中，江舒艺被江伊一的真情感动，两姐妹在联合虐渣，并收获甜蜜爱情。", "ja", "zh"},
		{"안소아가 약에 당한 추녀로 빙의해 고지명을 구하고 관계를 맺어 세 아이를 낳았다. 6년 후 고지명을 다시 만나 그가 아내를 찾는 길을 열고, 안소아는 고지명의 병을 치료하며 고지명 삼촌의 음모를 폭로했다. 결국 고지명은 고백하고 안소아가 받아들이며 행복하게 살아갔다.", "ar", "ko"},
		{"Zoé se réveille dans le corps de la sixième princesse, considérée comme un porte-malheur. Pour protéger sa mère Sylvie et accomplir les derniers vœux de son prédécesseur, elle utilise ses pouvoirs mystiques pour déjouer les complots. Peu à peu, elle gagne l'affection du roi Jean, autrefois impitoyable.", "en", "fr"},
	} {
		got := Compare(sample.text, sample.lang, Detect(sample.text))
		if got.Status != "mismatch" || got.Detected != sample.detected {
			t.Fatalf("do not suppress genuine cross-language evidence: %+v", got)
		}
	}
}
