package hubspot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

func sign(secret, method, url, body, ts string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(method + url + body + ts))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	s := New(Deps{ClientSecret: "shh"})
	now := time.Now()
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	url := "https://api.example.com/api/v1/integrations/hubspot/webhooks"
	body := `[{"objectId":1}]`
	good := sign("shh", "POST", url, body, ts)

	if err := s.VerifySignature("POST", url, []byte(body), good, ts, now); err != nil {
		t.Fatalf("valid signature refused: %v", err)
	}
	if err := s.VerifySignature("POST", url, []byte(body+" "), good, ts, now); err == nil {
		t.Fatal("tampered body accepted")
	}
	if err := s.VerifySignature("POST", url, []byte(body), sign("other", "POST", url, body, ts), ts, now); err == nil {
		t.Fatal("signature from another secret accepted")
	}
	if err := s.VerifySignature("POST", url, []byte(body), good, ts, now.Add(10*time.Minute)); err == nil {
		t.Fatal("stale delivery accepted")
	}
	if err := New(Deps{}).VerifySignature("POST", url, []byte(body), good, ts, now); err == nil {
		t.Fatal("accepted with no secret configured")
	}
}

func TestAdvanceNeverSpins(t *testing.T) {
	c := time.UnixMilli(1000)
	same := time.UnixMilli(1000)
	got := advance(&c, &same, true)
	if !got.After(c) {
		t.Fatalf("a full page at the cursor must step past it, got %v", got)
	}
	if got := advance(&c, &same, false); !got.Equal(same) {
		t.Fatalf("a short page keeps the last time, got %v", got)
	}
	later := time.UnixMilli(5000)
	if got := advance(&c, &later, true); !got.Equal(later) {
		t.Fatalf("cursor should move to the last modified time, got %v", got)
	}
	if got := advance(nil, nil, true); got != nil {
		t.Fatalf("no results keep no cursor, got %v", got)
	}
}

func TestStageStatus(t *testing.T) {
	cases := []struct {
		meta map[string]string
		want models.DealStatus
	}{
		{map[string]string{"isClosed": "true", "probability": "1.0"}, models.DealStatusWon},
		{map[string]string{"isClosed": "true", "probability": "0.0"}, models.DealStatusLost},
		{map[string]string{"isClosed": "false", "probability": "0.4"}, models.DealStatusOpen},
		{map[string]string{}, models.DealStatusOpen},
	}
	for _, c := range cases {
		m := stageMetaFrom(Stage{Metadata: c.meta})
		if got := stageMetaFromMap(m.toMap()).status(); got != c.want {
			t.Errorf("%v: got %s, want %s", c.meta, got, c.want)
		}
	}
}

func TestTaskMappingRoundTrips(t *testing.T) {
	for _, st := range []models.CRMTaskStatus{models.CRMTaskStatusPending, models.CRMTaskStatusInProgress, models.CRMTaskStatusCompleted, models.CRMTaskStatusCancelled} {
		if got := localTaskStatus(hsTaskStatus(st)); got != st {
			t.Errorf("status %s came back as %s", st, got)
		}
	}
	for _, name := range []string{"To-do", "Call", "Email", "LinkedIn"} {
		if got := localTaskType(hsTaskType(name)); got != name {
			t.Errorf("type %s came back as %s", name, got)
		}
	}
}

func TestHTMLText(t *testing.T) {
	if got := htmlToText("<p>Hi&nbsp;<b>Ana</b></p><p>Line two<br>three</p>"); got != "Hi Ana\nLine two\nthree" {
		t.Fatalf("htmlToText = %q", got)
	}
	if got := textToHTML("a < b\nc"); got != "a &lt; b<br>c" {
		t.Fatalf("textToHTML = %q", got)
	}
}

func TestPushProperties(t *testing.T) {
	cfg := models.DefaultCRMProviderConfig()
	cfg.FieldMap["custom:title"] = "jobtitle"
	cfg.FieldDirection["custom:title"] = models.CRMFieldPull
	o := &org{Config: cfg}
	c := &models.Contact{FirstName: "Ana", Company: "Acme", CustomFields: map[string]string{"title": "CTO"}}
	got := pushProperties(o, c, false)
	if got["firstname"] != "Ana" || got["company"] != "Acme" {
		t.Fatalf("mapped fields missing: %v", got)
	}
	if _, ok := got["jobtitle"]; ok {
		t.Fatal("a pull-only field must not be written to HubSpot on update")
	}
	if pushProperties(o, c, true)["jobtitle"] != "CTO" {
		t.Fatal("a new HubSpot contact gets every mapped value")
	}
	if len(pushOnlyProperties(o, c)) != 0 {
		t.Fatal("default fields are two-way, so none are push-only")
	}
}

func TestEmailDomainSkipsFreeMail(t *testing.T) {
	if emailDomain("ana@gmail.com") != "" {
		t.Fatal("free mail must not become a company")
	}
	if emailDomain("ana@Acme.io") != "acme.io" {
		t.Fatal("work domain expected")
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := models.DefaultCRMProviderConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("defaults must validate: %v", err)
	}
	bad := models.DefaultCRMProviderConfig()
	bad.FieldMap["nickname"] = "firstname"
	if bad.Validate() == nil {
		t.Fatal("unknown Warmbly field accepted")
	}
	bad = models.DefaultCRMProviderConfig()
	bad.FieldMap["company"] = "company; DROP"
	if bad.Validate() == nil {
		t.Fatal("invalid property name accepted")
	}
	bad = models.DefaultCRMProviderConfig()
	bad.PositiveReply.CreateDeal = true
	if bad.Validate() == nil {
		t.Fatal("deal creation without a stage accepted")
	}
}

func TestWebhookKinds(t *testing.T) {
	cases := []struct {
		ev       webhookEvent
		obj, act string
	}{
		{webhookEvent{SubscriptionType: "contact.propertyChange"}, "contact", "propertyChange"},
		{webhookEvent{SubscriptionType: "deal.deletion"}, "deal", "deletion"},
		{webhookEvent{SubscriptionType: "object.creation", ObjectTypeID: "0-3"}, "deal", "creation"},
		{webhookEvent{SubscriptionType: "object.propertyChange", ObjectTypeID: "0-2"}, "", "propertyChange"},
	}
	for _, c := range cases {
		obj, act := c.ev.kind()
		if obj != c.obj || act != c.act {
			t.Errorf("%+v: got %s/%s, want %s/%s", c.ev, obj, act, c.obj, c.act)
		}
	}
}
