package poollink

import "testing"

func TestReturnURLStaysOnTheRegisteredInstance(t *testing.T) {
	const inst = "https://warmbly.acme.io"
	cases := []struct {
		instance, ret string
		want          bool
	}{
		{inst, "https://warmbly.acme.io/cloud-oauth/done", true},
		{inst, "https://WARMBLY.acme.io/cloud-oauth/done?x=1", true},
		{inst, "https://evil.example/cloud-oauth/done", false},
		{inst, "http://warmbly.acme.io/cloud-oauth/done", false},
		{inst, "https://warmbly.acme.io.evil.example/", false},
		{inst, "https://user@warmbly.acme.io/", false},
		{inst, "/cloud-oauth/done", false},
		{inst, "javascript:alert(1)", false},
		{"", "https://anything.example/", false},
		{"ftp://warmbly.acme.io", "ftp://warmbly.acme.io/", false},
	}
	for _, c := range cases {
		if got := returnURLAllowed(c.ret, c.instance); got != c.want {
			t.Errorf("returnURLAllowed(%q, %q) = %v, want %v", c.ret, c.instance, got, c.want)
		}
	}
}

func TestBrokerBindingMustMatch(t *testing.T) {
	if bindingMatches("", "") {
		t.Fatal("an unbound state matched a browser without the cookie")
	}
	if bindingMatches("abc", "abd") || bindingMatches("abc", "") {
		t.Fatal("a different browser matched")
	}
	if !bindingMatches("abc", "abc") {
		t.Fatal("the bound browser did not match")
	}
}
