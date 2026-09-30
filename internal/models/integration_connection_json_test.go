package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIntegrationConnectionJSONOmitsSigningSecret(t *testing.T) {
	c := IntegrationConnection{
		Provider:           "zapier",
		ConfigCapabilities: json.RawMessage(`{"signing_secret":"whsec_live","scheduling_url":"https://cal.example/x"}`),
	}
	for _, v := range []any{c, &c, []IntegrationConnection{c}} {
		out, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), "whsec_live") {
			t.Fatalf("signing secret serialized: %s", out)
		}
		if !strings.Contains(string(out), "scheduling_url") {
			t.Fatalf("other capabilities dropped: %s", out)
		}
	}
}
