package dramalanguage

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestKnownRelatedLanguageRegressions(t *testing.T) {
	for _, sample := range []struct{ text, lang string }{
		{"Pembunuh wanita kelas atas (atau ratu mafia) yang menyembunyikan identitasnya dan bekerja sebagai petugas kebersihan di kawasan kumuh, mendapati putri angkatnya dipatahkan kedua kakinya oleh para perundung di sekolah bangsawan beserta orang tua mereka yang kaya raya. Diam-diam ia melepas seragam kebersihannya, dan mengambil senjata yang sudah lama berdebu.", "id"},
		{"Gloria, la carnicera, suplanta a su gemela, una heredera moribunda, para enfrentar a Eduardo y su familia. Mentiras lo complican todo. Nada es lo que parece.", "es"},
	} {
		got := Compare(sample.text, sample.lang, Detect(sample.text))
		if got.Status != "passed" && got.Status != "uncertain" {
			t.Fatalf("known same-family ambiguity must not raise an automatic error: %+v", got)
		}
	}
}

// Opt-in, read-only snapshot audit. Never writes production metadata/cache or
// sends requests. Counts compare rule behavior, not measured language accuracy.
func TestReadOnlySnapshotAudit(t *testing.T) {
	path := os.Getenv("DRAMA_LANGUAGE_AUDIT_SNAPSHOT")
	if path == "" {
		t.Skip("set DRAMA_LANGUAGE_AUDIT_SNAPSHOT to audit a local snapshot")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		IDs  []string `json:"dramaList"`
		Meta map[string]struct {
			Description string `json:"description"`
			Lang        string `json:"lang"`
			Previous    Result `json:"description_language_check"`
		} `json:"dramaMeta"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	cache := map[string]Detection{}
	counts, transitions := map[string]int{}, map[string]int{}
	for _, id := range snapshot.IDs {
		meta := snapshot.Meta[id]
		key := DescriptionHash(meta.Description)
		d, found := cache[key]
		if !found {
			d = Detect(meta.Description)
			cache[key] = d
		}
		r := Compare(meta.Description, meta.Lang, d)
		counts[r.Status]++
		transitions[meta.Previous.Status+" -> "+r.Status]++
		if r.Status != meta.Previous.Status || r.Detected != meta.Previous.Detected {
			t.Logf("changed id=%s %s/%s -> %s/%s lang=%s reason=%s", id,
				meta.Previous.Status, meta.Previous.Detected, r.Status, r.Detected, meta.Lang, r.Reason)
		}
	}
	summary, _ := json.Marshal(map[string]interface{}{"records": len(snapshot.IDs), "unique_including_empty": len(cache), "counts": counts, "transitions": transitions, "duration_ms": time.Since(started).Milliseconds()})
	t.Log(string(summary))
}

func TestRelatedLanguageConflictsNeedReview(t *testing.T) {
	for _, pair := range [][2]string{{"id", "ms"}, {"ms", "id"}, {"es", "pt"}, {"pt", "es"}} {
		d := Detection{Language: pair[0], Reliable: true, Confidence: 0.9999999}
		if got := Compare("A substantial description with sufficiently many letters.", pair[1], d); got.Status != "uncertain" || got.Reason == "" {
			t.Fatalf("high model score must not override related-language review: %+v", got)
		}
	}
}

func TestOutOfScopeLanguagesAreNotForcedIntoSupportedSet(t *testing.T) {
	for _, text := range []string{
		"Dopo essere tornata nella sua città natale, una giovane donna scopre un segreto di famiglia. Deve proteggere sua figlia e scoprire la verità prima che sia troppo tardi.",
		"Sau khi trở về quê hương, một cô gái trẻ phát hiện bí mật của gia đình. Cô phải bảo vệ con gái mình và tìm ra sự thật trước khi mọi chuyện trở nên quá muộn.",
	} {
		d := Detect(text)
		if got := Compare(text, "pt", d); got.Status != "unsupported" || d.Reliable {
			t.Fatalf("unsupported text was forced into supported candidates: %+v / %+v", d, got)
		}
	}
}

func TestMixedTextAndNames(t *testing.T) {
	en := "After returning to her hometown, a young woman discovers a hidden family secret. She must protect her daughter and uncover the truth before it is too late."
	zh := "重返故鄉後，一名年輕女子發現了家族隱藏多年的秘密。為了保護心愛的女兒，她決定勇敢面對困境，尋找真相，重新開始自己的人生。"
	es := "Después de regresar a su ciudad natal, una joven descubre un secreto familiar. Debe proteger a su hija y encontrar la verdad antes de que sea demasiado tarde."
	for _, text := range []string{en + zh + zh, en + " " + es} {
		if got := Compare(text, "en", Detect(text)); got.Status != "mixed" {
			t.Fatalf("mixed prose must not pass or fail as one language: %+v", got)
		}
	}
	if got := Compare(en+" 王浩", "en", Detect(en+" 王浩")); got.Status != "passed" {
		t.Fatalf("short foreign name must not force mixed status: %+v", got)
	}
	text := "यह एक लड़की की कहानी है जो अपने परिवार के रहस्यों को जानने और अपनी बेटी की रक्षा करने के लिए घर लौटती है।"
	if got := Compare(text, "en", Detect(text)); got.Status != "unsupported" {
		t.Fatalf("unsupported script not rejected: %+v", got)
	}
}
