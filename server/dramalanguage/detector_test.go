package dramalanguage

import "testing"

func TestSupportedLanguageSamples(t *testing.T) {
	samples := map[string]string{
		"en": "After returning to her hometown, a young woman discovers a family secret. She must protect her daughter and uncover the truth before it is too late.",
		"ja": "故郷に戻った若い女性は、家族に隠された秘密を知りました。愛する娘を守るため、彼女は困難に立ち向かい、失われた真実を探し始めます。",
		"es": "Después de regresar a su ciudad natal, una joven descubre un secreto familiar. Debe proteger a su hija y encontrar la verdad antes de que sea demasiado tarde.",
		"id": "Setelah kembali ke kampung halamannya, seorang wanita muda menemukan rahasia keluarganya. Dia harus melindungi putrinya dan mengungkap kebenaran sebelum semuanya terlambat.",
		"fr": "Après son retour dans sa ville natale, une jeune femme découvre un secret de famille. Elle doit protéger sa fille et découvrir la vérité avant qu'il ne soit trop tard.",
		"ko": "고향으로 돌아온 젊은 여성은 가족의 숨겨진 비밀을 알게 됩니다. 사랑하는 딸을 지키기 위해 어려움에 맞서고 진실을 찾아 나서는 감동적인 이야기입니다.",
		"th": "หญิงสาวเดินทางกลับบ้านเกิดและค้นพบความลับของครอบครัว เธอต้องปกป้องลูกสาวและค้นหาความจริงก่อนที่จะสายเกินไป เรื่องราวความรักและการต่อสู้เพื่อคนที่เธอรักเริ่มต้นขึ้นแล้ว",
		"ru": "Вернувшись в родной город, молодая женщина узнаёт семейную тайну. Она должна защитить свою дочь и раскрыть правду, пока не стало слишком поздно.",
		"ms": "Selepas pulang ke kampung halaman, seorang wanita muda mengetahui rahsia keluarganya. Dia perlu melindungi anak perempuannya dan mencari kebenaran sebelum semuanya terlambat.",
		"zh": "重返故鄉後，一名年輕女子發現了家族隱藏多年的秘密。為了保護心愛的女兒，她決定勇敢面對困境，尋找真相，重新開始自己的人生。",
		"ar": "بعد عودتها إلى مسقط رأسها تكتشف امرأة شابة سراً عائلياً خطيراً. عليها حماية ابنتها وكشف الحقيقة قبل فوات الأوان في رحلة مليئة بالحب والتحديات.",
		"pt": "Depois de voltar à sua cidade natal, uma jovem descobre um segredo de família. Ela precisa proteger sua filha e descobrir a verdade antes que seja tarde demais.",
		"de": "Nach ihrer Rückkehr in die Heimatstadt entdeckt eine junge Frau ein Familiengeheimnis. Sie muss ihre Tochter beschützen und die Wahrheit herausfinden, bevor es zu spät ist.",
		"tr": "Memleketine dönen genç bir kadın ailesinin gizlediği bir sırrı öğrenir. Kızını korumak ve çok geç olmadan gerçeği ortaya çıkarmak için zorlu bir mücadeleye başlar.",
	}
	for language, text := range samples {
		t.Run(language, func(t *testing.T) {
			got := Detect(text)
			if got.Language != language || !got.Reliable {
				t.Fatalf("unexpected detection: %+v", got)
			}
			if Compare(text, language, got).Status != "passed" {
				t.Fatal("matching language must pass")
			}
		})
	}
}

func TestComparisonAndLanguageAliases(t *testing.T) {
	for raw, want := range map[string]string{"EN": "en", "ja-JP": "ja", "zh_TW": "zh", "zh-Hant": "zh", "ar-SA": "ar", "pt-BR": "pt", "in": "id", "": "", "it": ""} {
		if got := NormalizeLanguage(raw); got != want {
			t.Errorf("%q = %q, want %q", raw, got, want)
		}
	}
	detection := Detection{Language: "en", Confidence: 0.99, Margin: 0.98, Reliable: true}
	for _, text := range []string{"", " \n ", " / ", "<p> / </p>"} {
		if got := Compare(text, "", detection); got.Status != "skipped" {
			t.Errorf("%q: %+v", text, got)
		}
	}
	if Compare("An English description", "ja", detection).Status != "mismatch" {
		t.Fatal("mismatch not detected")
	}
	if Compare("An English description", "", detection).Status != "invalid_lang" {
		t.Fatal("empty lang not detected")
	}
	if Compare("short", "en", Detect("short")).Status != "uncertain" {
		t.Fatal("short text must not produce a language error")
	}
	if DescriptionHash("hello") == DescriptionHash("hello changed") {
		t.Fatal("changed description must invalidate cache")
	}
}
