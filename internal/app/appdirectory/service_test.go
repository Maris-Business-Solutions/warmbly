package appdirectory

import (
	"strings"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

func validWrite() models.AppListingWrite {
	return models.AppListingWrite{
		Slug:       "acme-sync",
		Tagline:    "Sync replies into Acme",
		Category:   "crm",
		InstallURL: "https://acme.example/install",
	}
}

func TestNormalizeAcceptsValidListing(t *testing.T) {
	w := validWrite()
	w.Slug = "  Acme-Sync "
	w.Tagline = "  Sync   replies\tinto Acme "
	w.Description = "Line one\r\nLine two"
	l, xerr := normalize(w)
	if xerr != nil {
		t.Fatalf("normalize: %v", xerr)
	}
	if l.Slug != "acme-sync" || l.Tagline != "Sync replies into Acme" || l.Description != "Line one\nLine two" {
		t.Fatalf("unexpected normalized listing: %+v", l)
	}
}

func TestNormalizeRefuses(t *testing.T) {
	cases := map[string]func(*models.AppListingWrite){
		"short slug":        func(w *models.AppListingWrite) { w.Slug = "ab" },
		"double dash":       func(w *models.AppListingWrite) { w.Slug = "acme--sync" },
		"edge dash":         func(w *models.AppListingWrite) { w.Slug = "-acme" },
		"reserved builtin":  func(w *models.AppListingWrite) { w.Slug = "hubspot" },
		"reserved dashed":   func(w *models.AppListingWrite) { w.Slug = "cal-com" },
		"reserved product":  func(w *models.AppListingWrite) { w.Slug = "warmbly" },
		"empty tagline":     func(w *models.AppListingWrite) { w.Tagline = "   " },
		"long tagline":      func(w *models.AppListingWrite) { w.Tagline = strings.Repeat("a", 121) },
		"bidi tagline":      func(w *models.AppListingWrite) { w.Tagline = "Safe ‮gnp.exe" },
		"control in desc":   func(w *models.AppListingWrite) { w.Description = "ok\x07" },
		"long description":  func(w *models.AppListingWrite) { w.Description = strings.Repeat("a", 2001) },
		"unknown category":  func(w *models.AppListingWrite) { w.Category = "games" },
		"missing install":   func(w *models.AppListingWrite) { w.InstallURL = "" },
		"http install":      func(w *models.AppListingWrite) { w.InstallURL = "http://acme.example" },
		"credentials":       func(w *models.AppListingWrite) { w.InstallURL = "https://user:pw@acme.example" },
		"javascript scheme": func(w *models.AppListingWrite) { w.SupportURL = "javascript:alert(1)" },
		"hostless":          func(w *models.AppListingWrite) { w.PrivacyURL = "https:///privacy" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			w := validWrite()
			mutate(&w)
			if _, xerr := normalize(w); xerr == nil {
				t.Fatalf("expected a refusal")
			} else if xerr.ResponseCode() != ErrCodeInvalidListing {
				t.Fatalf("code = %q, want %q", xerr.ResponseCode(), ErrCodeInvalidListing)
			}
		})
	}
}

func TestSameContentIgnoresStatus(t *testing.T) {
	a, _ := normalize(validWrite())
	b, _ := normalize(validWrite())
	a.Status = models.AppListingFeatured
	if !sameContent(a, b) {
		t.Fatal("identical content should compare equal regardless of status")
	}
	b.InstallURL = "https://other.example/install"
	if sameContent(a, b) {
		t.Fatal("a changed install URL must count as a change")
	}
}
