package salesforce

import "testing"

func TestNormalizeID(t *testing.T) {
	cases := map[string]string{
		"001A0000006Vm9r":    "001A0000006Vm9rIAC",
		"003000000000001":    "003000000000001AAA",
		"001A0000006Vm9rIAC": "001A0000006Vm9rIAC",
		"":                   "",
	}
	for in, want := range cases {
		if got := NormalizeID(in); got != want {
			t.Errorf("NormalizeID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuoteEscapesBackslashBeforeQuote(t *testing.T) {
	if got := Quote(`o'neil\`); got != `'o\'neil\\'` {
		t.Fatalf("Quote = %s", got)
	}
}

func TestParseLimitInfo(t *testing.T) {
	u, ok := parseLimitInfo("api-usage=18/15000")
	if !ok || u.Used != 18 || u.Max != 15000 {
		t.Fatalf("got %+v %v", u, ok)
	}
	if _, ok := parseLimitInfo("per-app-api-usage=1/2(appName=x)"); ok {
		t.Fatal("a per-app reading is not the org budget")
	}
}

func TestIdentityIDs(t *testing.T) {
	org, user := IdentityIDs("https://login.salesforce.com/id/00Dxx0000001gPLEAY/005xx000001SwiUAAS")
	if org != "00Dxx0000001gPLEAY" || user != "005xx000001SwiUAAS" {
		t.Fatalf("got %s %s", org, user)
	}
}

func TestRecordStringFollowsRelationships(t *testing.T) {
	r := Record{"Owner": map[string]any{"Name": "Ada"}, "Amount": 1200.5, "IsWon": true}
	if r.String("Owner.Name") != "Ada" || r.String("Amount") != "1200.5" || r.String("IsWon") != "true" {
		t.Fatalf("unexpected %v", r)
	}
	if r.String("Missing.Name") != "" {
		t.Fatal("a missing relationship reads empty")
	}
}
