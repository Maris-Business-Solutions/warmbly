package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/warmbly/warmbly/internal/app/hubspot"
)

func hubspotSigned(t *testing.T, secret, signedURL, path, body string) *http.Request {
	t.Helper()
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(http.MethodPost + signedURL + body + ts))
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("X-HubSpot-Signature-v3", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-HubSpot-Request-Timestamp", ts)
	return req
}

func TestVerifyHubSpotUsesPublicURLOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("BACKEND_PUBLIC_URL", "https://api.example.com")
	h := &Handler{HubSpot: hubspot.New(hubspot.Deps{ClientSecret: "shh"})}
	path := "/api/v1/integrations/hubspot/webhooks"
	body := `[]`

	check := func(req *http.Request) bool {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req
		return h.verifyHubSpot(c, []byte(body))
	}
	if !check(hubspotSigned(t, "shh", "https://api.example.com"+path, path, body)) {
		t.Fatal("a signature over the public URL was refused")
	}
	req := hubspotSigned(t, "shh", "https://attacker.example"+path, path, body)
	req.Host = "attacker.example"
	req.Header.Set("X-Forwarded-Proto", "https")
	if check(req) {
		t.Fatal("a signature over a client-chosen host was accepted")
	}
	v2 := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	v2.Header.Set("X-HubSpot-Signature", "deadbeef")
	if check(v2) {
		t.Fatal("an untimestamped signature was accepted")
	}
}

func TestHubSpotServerRoutesRefuseCardFetches(t *testing.T) {
	cases := map[string]bool{
		"":                                false,
		"portalId=1":                      true,
		"appId=2":                         true,
		"userId=3":                        true,
		"userEmail=a%40b.com":             true,
		"PORTALID=1":                      true,
		"other=1":                         false,
		"portalId=1&userId=3&appId=2&x=1": true,
	}
	for raw, want := range cases {
		q, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := fromHubSpotFetch(q); got != want {
			t.Errorf("fromHubSpotFetch(%q) = %v, want %v", raw, got, want)
		}
	}

	gin.SetMode(gin.TestMode)
	h := &Handler{HubSpot: hubspot.New(hubspot.Deps{ClientSecret: "shh"})}
	for _, route := range []func(*gin.Context){h.HubSpotWebhook, h.HubSpotActionEnroll, h.HubSpotActionCampaigns} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/x?portalId=1&userId=3&userEmail=a%40b.com&appId=2", strings.NewReader(`{}`))
		route(c)
		if got := c.Writer.Status(); got != http.StatusForbidden {
			t.Errorf("a card fetch to a server route answered %d, want 403", got)
		}
	}
}

func TestHubSpotFetchUserNeedsOneOfEach(t *testing.T) {
	cases := map[string]bool{
		"portalId=1&userId=2&userEmail=a%40b.com":                     true,
		"portalId=1&userId=2":                                         false,
		"portalId=1&userEmail=a%40b.com":                              false,
		"userId=2&userEmail=a%40b.com":                                false,
		"portalId=1&userId=2&userEmail=a%40b.com&userEmail=c%40d.com": false,
		"portalId=1&userId=2&userId=9&userEmail=a%40b.com":            false,
		"portalId=1&portalId=9&userId=2&userEmail=a%40b.com":          false,
		"portalId=1&userId=2&userEmail=+":                             false,
	}
	for raw, want := range cases {
		q, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		u, ok := hubspotFetchUser(q)
		if ok != want {
			t.Errorf("hubspotFetchUser(%q) ok = %v, want %v", raw, ok, want)
		}
		if ok && (u.ID != "2" || u.Email != "a@b.com") {
			t.Errorf("hubspotFetchUser(%q) = %+v", raw, u)
		}
	}
}
