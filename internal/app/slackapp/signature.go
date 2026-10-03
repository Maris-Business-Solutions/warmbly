package slackapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// signatureWindow is how far a request timestamp may sit from now.
const signatureWindow = 5 * time.Minute

var (
	ErrNoSigningSecret = errors.New("slack signing secret not configured")
	ErrBadSignature    = errors.New("slack signature invalid")
)

// VerifySignature checks Slack's v0 request signature: HMAC-SHA256 of
// "v0:{timestamp}:{body}" under the signing secret, compared in constant time,
// with the timestamp inside signatureWindow of now.
func VerifySignature(secret, timestamp, signature string, body []byte, now time.Time) error {
	if secret == "" {
		return ErrNoSigningSecret
	}
	timestamp = strings.TrimSpace(timestamp)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrBadSignature
	}
	skew := now.Sub(time.Unix(ts, 0))
	if skew < -signatureWindow || skew > signatureWindow {
		return ErrBadSignature
	}
	got, ok := strings.CutPrefix(strings.TrimSpace(signature), "v0=")
	if !ok {
		return ErrBadSignature
	}
	gotBytes, err := hex.DecodeString(got)
	if err != nil {
		return ErrBadSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":"))
	mac.Write(body)
	if !hmac.Equal(gotBytes, mac.Sum(nil)) {
		return ErrBadSignature
	}
	return nil
}
