package integration

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestSalesforceInstanceURL(t *testing.T) {
	ok := map[string]string{
		"https://acme.my.salesforce.com":                      "https://acme.my.salesforce.com",
		"https://acme.my.salesforce.com/":                     "https://acme.my.salesforce.com",
		"https://ACME--dev.sandbox.my.salesforce.com:443":     "https://acme--dev.sandbox.my.salesforce.com",
		"https://na85.salesforce.com":                         "https://na85.salesforce.com",
		"https://acme.my.salesforce.mil":                      "https://acme.my.salesforce.mil",
		"https://acme-dev-ed.develop.my.salesforce.com":       "https://acme-dev-ed.develop.my.salesforce.com",
		"  https://acme.lightning.force.com  ":                "https://acme.lightning.force.com",
		"https://acme.my.sfcrmproducts.cn":                    "https://acme.my.sfcrmproducts.cn",
		"https://acme.cloudforce.com":                         "https://acme.cloudforce.com",
		"https://acme.my.salesforce.com/services/data/v62.0/": "",
	}
	for in, want := range ok {
		got, err := SalesforceInstanceURL(in)
		if want == "" {
			if err == nil {
				t.Errorf("%q: accepted as %q", in, got)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{
		"", "http://acme.my.salesforce.com", "https://evil.com", "https://salesforce.com.evil.com",
		"https://evilsalesforce.com", "https://user:pw@acme.my.salesforce.com", "https://acme.my.salesforce.com:8443",
		"https://127.0.0.1", "https://acme.my.salesforce.com?x=1", "https://acme.my.salesforce.com#f",
		"https://.salesforce.com", "https://a..salesforce.com", "https://acme_x.salesforce.com",
	} {
		if got, err := SalesforceInstanceURL(in); err == nil {
			t.Errorf("%q: accepted as %q", in, got)
		}
	}
}

func TestPublicMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
		ok   bool
	}{
		{"plain refusal", errors.New("connection not found"), "connection not found", true},
		{"wrapped refusal", fmt.Errorf("url: %w", errors.New("url scheme must be https")), "url: url scheme must be https", true},
		{"sentinel keeps its own text", fmt.Errorf("%w: %v", ErrPushReauth, errors.New("oauth2: refresh failed")), ErrPushReauth.Error(), true},
		{"lower layer", fmt.Errorf("decrypt config: %w", lowerLayer(errors.New("invalid ciphertext"))), "", false},
		{"driver sentinel", pgx.ErrNoRows, "", false},
		{"foreign type", &customErr{}, "", false},
		{"nil", nil, "", false},
	}
	for _, tc := range cases {
		got, ok := PublicMessage(tc.err)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: got %q, %v; want %q, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

type customErr struct{}

func (*customErr) Error() string { return "pq: relation does not exist" }
