package dramalanguage

import (
	"golang.org/x/text/unicode/norm"
	"testing"
)

// Regression for drama 6adc37876f0b5c8c13d743ce. This is Arabic in Latin
// transliteration (with an English word), previously classified as Indonesian.
const arabicTransliterationRegression = "Fi wasaṭi al-ʿāṣifati al-maṭariyyati al-yaʾisah, shattered ash-shābba al-qalādata ash-shāhibata ḥawla ʿunuqihā, li-taẓhara khātimu al-istighāthati al-yāqūtiyyu al-aḥmaru al-ladhī yumaṭthilu al-sulṭata al-ʿulyā li-l-ʿāʾilati al-mālikah. Wa-fī laḥẓātin, ḥajabat al-mūrikābāt al-ʿaskariyyatu al-mālikiyyah ʿan al-shams, wa-jathā ʿalā urukbātihim ʿamāliqatu Wāl Strīt al-ladhīna yaḥakumūna shiryāna al-iqtiṣādi al-ʿālamī!"

func TestArabicTransliterationRegression(t *testing.T) {
	for _, text := range []string{arabicTransliterationRegression, "<p>" + arabicTransliterationRegression + "</p>"} {
		got := Detect(text)
		if got.Language != "ar" || got.Reliable || got.Method != "arabic_transliteration_rules" {
			t.Fatalf("transliteration not recognized: %+v", got)
		}
		if Compare(text, "ar", got).Status != "uncertain" {
			t.Fatal("transliteration is a candidate, not a verified pass")
		}
		if Compare(text, "id", got).Status != "uncertain" {
			t.Fatal("transliteration must require review regardless of configured lang")
		}
	}
}

func TestArabicTransliterationNeedsCombinedEvidence(t *testing.T) {
	negative := []string{
		"After returning to her hometown, a young woman discovers a family secret. She must protect her daughter before it is too late.",
		"Setelah kembali ke kampung halamannya, seorang wanita muda menemukan rahasia keluarganya. Dia harus melindungi putrinya dan mengungkap kebenaran.",
		"Selepas pulang ke kampung halaman, seorang wanita muda mengetahui rahsia keluarganya. Dia perlu melindungi anak perempuannya dan mencari kebenaran.",
		"Seorang wanita bertemu Ali al-Hakim, Hasan al-Rashid dan Umar al-Malik untuk mencari kebenaran tentang keluarganya di sebuah kota kecil.",
		"A young woman meets Ḥasan, Ṣālih and Ṭāriq on a journey to discover her forgotten family history and protect her daughter.",
		"To describe the story, the English introduction mentions al-Hakim and al-Rashid, while its actual prose remains entirely in English.",
		"<p> / </p>",
	}
	for _, text := range negative {
		if got, matched := detectArabicTransliteration(NormalizeDescription(text)); matched {
			t.Errorf("false transliteration detection for %q: %+v", text, got)
		}
	}
	text := "A young woman returns to her hometown and discovers a hidden family secret. She must protect her daughter and find the truth before it is too late."
	if result := Compare(text, "ar", Detect(text)); result.Status != "mismatch" {
		t.Fatalf("lang=ar must not whitelist an English description: %+v", result)
	}
}

func TestPlainArabicTransliterationRemainsUncertain(t *testing.T) {
	text := "Fi al-madina wa fi al-bayt kana al-rajul ma'a al-bint, thumma min al-qasr ila al-suq hatta al-layl wa kana al-amal fi al-qalb."
	got := Detect(text)
	if got.Method != "arabic_transliteration_candidate" || got.Reliable {
		t.Fatalf("unmarked transliteration must not force a language: %+v", got)
	}
	result := Compare(text, "ar", got)
	if result.Status != "uncertain" || result.Reason == "" {
		t.Fatalf("missing uncertainty reason: %+v", result)
	}
}

func TestDecomposedArabicTransliteration(t *testing.T) {
	text := norm.NFD.String(arabicTransliterationRegression)
	got := Detect(text)
	if got.Language != "ar" || got.Reliable {
		t.Fatalf("decomposed accented text must also be recognized: %+v", got)
	}
}
