package handler

import (
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/app/unsublink"
	"github.com/warmbly/warmbly/internal/models"
)

func TestUnsubLanguageFollowsTheBrowser(t *testing.T) {
	cases := map[string]string{
		"":                         "en",
		"*":                        "en",
		"de-DE,de;q=0.9,en;q=0.8":  "de",
		"en-US,en;q=0.9,de;q=0.8":  "en",
		"fr;q=0.5, de;q=0.9":       "de",
		"xx-YY, pt-BR;q=0.7":       "pt",
		"de;q=0, fr;q=0.1":         "fr",
		"de;q=0":                   "en",
		"de;q=abc, it":             "it",
		"zh-CN,zh;q=0.9":           "zh",
		"zh-TW":                    "zh-Hant",
		"zh-Hant-HK":               "zh-Hant",
		"nn-NO":                    "nb",
		"no":                       "nb",
		"tl-PH":                    "fil",
		"sr-Latn-RS":               "sr",
		"AR-eg":                    "ar",
		"es_MX":                    "es",
		"ja-JP;q=0.8, ko-KR;q=0.8": "ja",
	}
	for header, want := range cases {
		if got := unsubLanguage(header); got != want {
			t.Errorf("unsubLanguage(%q) = %q, want %q", header, got, want)
		}
	}
}

// Every tagging language has the whole page, not half of it in English.
func TestUnsubPageCoversEveryTaggingLanguage(t *testing.T) {
	codes := []string{"zh-Hant"}
	for code := range models.MailLanguageNames {
		codes = append(codes, code)
	}
	for _, code := range codes {
		c, ok := unsubCopies[code]
		if !ok {
			t.Errorf("%s: no unsubscribe page", code)
			continue
		}
		v := reflect.ValueOf(c)
		for i := range v.NumField() {
			if f := v.Field(i); f.Kind() == reflect.String && strings.TrimSpace(f.String()) == "" {
				t.Errorf("%s: %s is empty", code, v.Type().Field(i).Name)
			}
		}
		if code != "en" && c.ConfirmTitle == unsubCopies["en"].ConfirmTitle {
			t.Errorf("%s: the confirm page is still in English", code)
		}
	}
}

func TestUnsubPageRendersTheRecipientsLanguage(t *testing.T) {
	signer := unsublink.New("secret", "https://api.example.com")
	h := &Handler{UnsubscribeLinks: signer}
	get := func(lang string) string {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/unsubscribe/"+signer.Token(uuid.New(), uuid.New(), uuid.New(), time.Now()), nil)
		r.Header.Set("Accept-Language", lang)
		newUnsubRouter(h).ServeHTTP(w, r)
		if w.Code != http.StatusOK || w.Header().Get("Vary") != "Accept-Language" {
			t.Fatalf("%s: %d, Vary %q", lang, w.Code, w.Header().Get("Vary"))
		}
		return html.UnescapeString(w.Body.String())
	}
	de := get("de-DE,de;q=0.9")
	for _, want := range []string{`<html lang="de">`, unsubCopies["de"].ConfirmTitle, unsubCopies["de"].ConfirmButton} {
		if !strings.Contains(de, want) {
			t.Errorf("German page lacks %q:\n%s", want, de)
		}
	}
	if ar := get("ar"); !strings.Contains(ar, `<html lang="ar" dir="rtl">`) || !strings.Contains(ar, unsubCopies["ar"].ConfirmTitle) {
		t.Errorf("Arabic page is not right to left:\n%s", ar)
	}
	if en := get("xx"); !strings.Contains(en, `<html lang="en">`) || !strings.Contains(en, "Unsubscribe from these emails?") {
		t.Errorf("an unknown language should fall back to English:\n%s", en)
	}
	// A bad link is refused in the recipient's language too.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/unsubscribe/nope", nil)
	r.Header.Set("Accept-Language", "fr")
	newUnsubRouter(h).ServeHTTP(w, r)
	if body := html.UnescapeString(w.Body.String()); !strings.Contains(body, unsubCopies["fr"].InvalidTitle) {
		t.Errorf("invalid link page not in French:\n%s", body)
	}
}

// The tracking service answers a malformed token and an unreachable backend
// itself, with the backend's wording in every language.
func TestTrackingUnsubPagesAgreeWithTheBackend(t *testing.T) {
	src, err := os.ReadFile("../../../tracking/src/unsubscribe_i18n.rs")
	if err != nil {
		t.Fatalf("read tracking copies: %v", err)
	}
	rust := string(src)
	for code, c := range unsubCopies {
		at := strings.Index(rust, "\""+code+"\",\n")
		if at < 0 {
			t.Errorf("%s: no tracking copy", code)
			continue
		}
		block := rust[at:]
		if end := strings.Index(block, "    ),"); end > 0 {
			block = block[:end]
		}
		for _, s := range []string{c.InvalidTitle, c.ReplyBody, c.RetryTitle} {
			if !strings.Contains(block, "\""+s+"\"") {
				t.Errorf("%s: tracking copy lacks %q", code, s)
			}
		}
		if strings.Contains(block, "rtl: true") != c.RTL {
			t.Errorf("%s: tracking copy disagrees on direction", code)
		}
	}
}
