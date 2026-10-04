package webhook

import "testing"

func TestWithoutPrivateKeys(t *testing.T) {
	in := map[string]any{"contact_email": "a@b.co", "_body_text": "secret", "_user_id": "u"}
	out, ok := withoutPrivateKeys(in).(map[string]any)
	if !ok {
		t.Fatal("map payload must stay a map")
	}
	if _, leaked := out["_body_text"]; leaked || out["contact_email"] != "a@b.co" || len(out) != 1 {
		t.Fatalf("private keys must not reach webhook endpoints: %v", out)
	}
	if _, kept := in["_body_text"]; !kept {
		t.Fatal("the caller's map must not be modified")
	}
	if got := withoutPrivateKeys("plain"); got != "plain" {
		t.Fatalf("non-map payloads pass through, got %v", got)
	}
}
