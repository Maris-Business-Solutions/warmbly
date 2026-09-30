package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

func sign(key, msg string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func headers(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestVerifyCalendlySignature(t *testing.T) {
	const key = "calendly-signing-key"
	body := []byte(`{"event":"invitee.created"}`)
	now := time.Unix(1_700_000_000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	good := "t=" + ts + ",v1=" + sign(key, ts+"."+string(body))

	cases := []struct {
		name   string
		header string
		body   []byte
		at     time.Time
		want   bool
	}{
		{"valid", good, body, now, true},
		{"tampered body", good, []byte(`{"event":"invitee.canceled"}`), now, false},
		{"wrong key", "t=" + ts + ",v1=" + sign("other-key", ts+"."+string(body)), body, now, false},
		{"stale timestamp", good, body, now.Add(10 * time.Minute), false},
		{"missing header", "", body, now, false},
		{"body-only signature", "t=" + ts + ",v1=" + sign(key, string(body)), body, now, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := VerifyInboundSignature(models.IntegrationCalendly, key,
				headers(map[string]string{"Calendly-Webhook-Signature": tc.header}), tc.body, tc.at)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestVerifyCalComSignature(t *testing.T) {
	const key = "calcom-secret"
	body := []byte(`{"triggerEvent":"BOOKING_CREATED"}`)
	now := time.Now()

	if !VerifyInboundSignature(models.IntegrationCalCom, key,
		headers(map[string]string{"X-Cal-Signature-256": sign(key, string(body))}), body, now) {
		t.Fatal("valid signature refused")
	}
	if VerifyInboundSignature(models.IntegrationCalCom, key,
		headers(map[string]string{"X-Cal-Signature-256": sign("wrong", string(body))}), body, now) {
		t.Fatal("signature under another key accepted")
	}
	if VerifyInboundSignature(models.IntegrationCalCom, key, headers(nil), body, now) {
		t.Fatal("missing signature accepted")
	}
}
