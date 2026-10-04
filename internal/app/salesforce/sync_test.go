package salesforce

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

func TestDefaultSettingsAreValid(t *testing.T) {
	s := DefaultSettings()
	if err := s.Validate(); err != nil {
		t.Fatalf("defaults must validate: %v", err)
	}
	if s.Activity.Opened || s.Activity.Clicked {
		t.Fatal("opens and clicks cost an API call each and stay off by default")
	}
}

func TestValidateRefusesWhatTheSyncCannotRun(t *testing.T) {
	cases := map[string]func(*Settings){
		"engagement pulled": func(s *Settings) {
			s.FieldMap = []FieldRule{{Object: ObjectLead, Warmbly: "engagement.status", Salesforce: "X__c", Direction: DirectionPull, Policy: PolicyOverwrite}}
		},
		"related field write": func(s *Settings) {
			s.FieldMap = []FieldRule{{Object: ObjectContact, Warmbly: "company", Salesforce: "Account.Name", Direction: DirectionBoth, Policy: PolicyIfEmpty}}
		},
		"injected field": func(s *Settings) {
			s.FieldMap = []FieldRule{{Object: ObjectLead, Warmbly: "company", Salesforce: "Company FROM User", Direction: DirectionPush, Policy: PolicyIfEmpty}}
		},
		"fixed owner, no id": func(s *Settings) { s.Matching.Owner = "fixed" },
		"unknown intent":     func(s *Settings) { s.Writeback.LeadStatusOnReply = map[string]string{"spam": "Dead"} },
		"duplicate rule": func(s *Settings) {
			r := FieldRule{Object: ObjectLead, Warmbly: "phone", Salesforce: "MobilePhone", Direction: DirectionPush, Policy: PolicyIfEmpty}
			s.FieldMap = []FieldRule{r, r}
		},
	}
	for name, mutate := range cases {
		s := DefaultSettings()
		mutate(&s)
		if s.Validate() == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

func TestParseSettingsKeepsOtherConfigAndFillsDefaults(t *testing.T) {
	raw := json.RawMessage(`{"signing_secret":"x","salesforce":{"enabled":false,"matching":{"prefer":"lead"}}}`)
	s := ParseSettings(raw)
	if s.Enabled || s.Matching.Prefer != "lead" {
		t.Fatalf("stored values must win: %+v", s.Matching)
	}
	if s.Matching.CreateAs != "lead" || s.Activity.AssignTo != "record_owner" {
		t.Fatalf("unset values come from the defaults: %+v", s)
	}
}

func TestAConnectionThatNeverSavedSettingsStaysOff(t *testing.T) {
	if ParseSettings(json.RawMessage(`{"signing_secret":"x"}`)).Enabled {
		t.Fatal("a connection from before native sync must not start logging on its own")
	}
	if !ParseSettings(json.RawMessage(`{"salesforce":{"matching":{"prefer":"lead"}}}`)).Enabled {
		t.Fatal("saved settings without the flag keep the default, on")
	}
}

func TestLegacyConnectionKeepsItsBehaviour(t *testing.T) {
	s := ParseSettings(nil)
	if s.Matching.CreateAs != "contact" {
		t.Fatal("the upsert action this replaces created Contacts")
	}
	s.withLegacyMappings([]models.IntegrationFieldMapping{
		{ObjectName: "contact", WarmblyField: "phone", ExternalField: "Phone"},
		{ObjectName: "contact", WarmblyField: "custom:tier", ExternalField: "Tier__c"},
		{ObjectName: "contact", WarmblyField: "company", ExternalField: "Department", Transform: "uppercase"},
	})
	if err := s.Validate(); err != nil {
		t.Fatalf("carried rules must validate: %v", err)
	}
	var phone, tier int
	for _, r := range s.FieldMap {
		if r.Object == ObjectContact && r.Salesforce == "Phone" {
			phone++
			if r.Policy != PolicyOverwrite {
				t.Fatal("the saved mapping replaces the default rule")
			}
		}
		if r.Salesforce == "Tier__c" {
			tier++
		}
		if r.Salesforce == "Department" {
			t.Fatal("a transformed mapping has no rule equivalent and is not carried")
		}
	}
	if phone != 1 || tier != 1 {
		t.Fatalf("phone=%d tier=%d", phone, tier)
	}
}

func TestWantsRecordsWritebackEvenWhenTheTaskIsOff(t *testing.T) {
	s := DefaultSettings()
	s.Activity.Sent = false
	if wants(s, KindSent) {
		t.Fatal("nothing to do for a send")
	}
	s.Writeback.LeadStatusOnSent = "Working - Contacted"
	if !wants(s, KindSent) {
		t.Fatal("a status writeback needs the event")
	}
	s.Activity.Unsubscribed = false
	s.Inbound.OptOut = "from_salesforce"
	if wants(s, KindUnsubscribed) {
		t.Fatal("opt-out flows only from Salesforce here")
	}
}

func TestStatusForReplySkipsAutoRepliesUnlessMapped(t *testing.T) {
	w := WritebackSettings{LeadStatusOnReply: map[string]string{"any": "Working", "positive": "Qualified"}}
	if got := w.statusForReply("positive"); got != "Qualified" {
		t.Fatalf("got %q", got)
	}
	if got := w.statusForReply("neutral"); got != "Working" {
		t.Fatalf("got %q", got)
	}
	if got := w.statusForReply("out_of_office"); got != "" {
		t.Fatalf("an out-of-office is not a conversation, got %q", got)
	}
	w.LeadStatusOnReply["out_of_office"] = "Nurture"
	if got := w.statusForReply("out_of_office"); got != "Nurture" {
		t.Fatalf("an explicit mapping wins, got %q", got)
	}
}

func TestListViewSOQLKeepsFiltersAndScope(t *testing.T) {
	s := DefaultSettings()
	view := "SELECT Name, Company, toLabel(Status) FROM Lead USING SCOPE mine WHERE IsConverted = false ORDER BY Name ASC NULLS FIRST, Id ASC NULLS FIRST"
	got, ok := listViewSOQL(s, ObjectLead, view)
	if !ok {
		t.Fatal("expected a rewrite")
	}
	if !strings.HasPrefix(got, "SELECT Id, Email,") || !strings.HasSuffix(got, "FROM Lead USING SCOPE mine WHERE IsConverted = false ORDER BY Name ASC NULLS FIRST, Id ASC NULLS FIRST") {
		t.Fatalf("unexpected rewrite: %s", got)
	}
	if _, ok := listViewSOQL(s, ObjectLead, "SELECT Id, (SELECT Id FROM Tasks) FROM Lead"); ok {
		t.Fatal("a subquery in the select list falls back to the id path")
	}
}

func TestPushChangesHonoursPolicy(t *testing.T) {
	st := DefaultSettings()
	st.FieldMap = []FieldRule{
		{Object: ObjectLead, Warmbly: "phone", Salesforce: "Phone", Direction: DirectionBoth, Policy: PolicyIfEmpty},
		{Object: ObjectLead, Warmbly: "company", Salesforce: "Company", Direction: DirectionPush, Policy: PolicyOverwrite},
		{Object: ObjectLead, Warmbly: "custom:tier", Salesforce: "Tier__c", Direction: DirectionPush, Policy: PolicyIfEmpty},
		{Object: ObjectLead, Warmbly: "engagement.last_campaign", Salesforce: "Last_Campaign__c", Direction: DirectionPush, Policy: PolicyOverwrite},
	}
	snap, _ := json.Marshal(map[string]any{"Phone": "+1 555", "Company": "Old", "Tier__c": nil})
	l := models.SalesforceRecordLink{SObject: ObjectLead, Snapshot: snap}
	ct := repository.SalesforceContact{ID: uuid.New(), Phone: "+1 777", Company: "New", CustomFields: map[string]string{"tier": "A"}}
	eng := &repository.SalesforceEngagement{LastCampaign: "Q3 outbound"}
	got := pushChanges(st, l, ct, eng)
	if _, ok := got["Phone"]; ok {
		t.Fatal("a filled field is never overwritten under if_empty")
	}
	if got["Company"] != "New" || got["Tier__c"] != "A" || got["Last_Campaign__c"] != "Q3 outbound" {
		t.Fatalf("unexpected changes: %v", got)
	}
	st.FieldMap = append(st.FieldMap, FieldRule{Object: ObjectLead, Warmbly: "first_name", Salesforce: "Nickname__c", Direction: DirectionPush, Policy: PolicyIfEmpty})
	ct.FirstName = "Ada"
	if _, ok := pushChanges(st, l, ct, eng)["Nickname__c"]; ok {
		t.Fatal("a field never read is unknown and never written blind")
	}
}

func TestMayCreate(t *testing.T) {
	s := DefaultSettings()
	if mayCreate(s, KindSent) || !mayCreate(s, KindReplied) || !mayCreate(s, KindMeetingBooked) {
		t.Fatal("reply mode creates on replies and meetings only")
	}
	s.Matching.CreateWhen = "never"
	if mayCreate(s, KindReplied) {
		t.Fatal("never means never")
	}
}

func TestHTMLToText(t *testing.T) {
	got := htmlToText(`<style>p{}</style><p>Hi Ada,</p><p>Worth a chat?<br>Thanks &amp; regards</p>`)
	if got != "Hi Ada,\nWorth a chat?\nThanks & regards" {
		t.Fatalf("got %q", got)
	}
}

func TestRecorderDedupeKeys(t *testing.T) {
	r := &Recorder{}
	_, _, k := r.shape(KindSent, models.WebhookEventCampaignEmailSent, map[string]any{"_task_id": "t1"}, "a@b.co")
	if k != "sent:t1" {
		t.Fatalf("got %q", k)
	}
	p, c, k := r.shape(KindReplied, models.WebhookEventCampaignReplyReceived, map[string]any{
		"_message_id": "<m@x>", "_body_text": "Sounds good", "subject": "Re: hi", "intent": "positive",
	}, "a@b.co")
	if k != "replied:<m@x>" || c == nil || c.Body != "Sounds good" || p["intent"] != "positive" {
		t.Fatalf("unexpected reply shape: %v %+v %q", p, c, k)
	}
	if _, ok := p["body_text"]; ok {
		t.Fatal("message text is sealed, never kept in the payload")
	}
}

func TestLikeQuoteEscapesWildcards(t *testing.T) {
	if got := likeQuote(`50%_o'k`); got != `'%50\%\_o\'k%'` {
		t.Fatalf("got %s", got)
	}
}

func TestAdvanceNeverStallsInsideOneSecond(t *testing.T) {
	cursor := time.Date(2026, 10, 3, 10, 0, 5, 0, time.UTC)
	same := cursor.Add(400 * time.Millisecond)
	if got := advance(cursor, &same); !got.Equal(cursor.Add(time.Second)) {
		t.Fatalf("a full page inside the cursor's second must step a whole second, got %v", got)
	}
	later := cursor.Add(3 * time.Second)
	if got := advance(cursor, &later); !got.Equal(later) {
		t.Fatalf("got %v", got)
	}
	if got := advance(cursor, nil); !got.Equal(cursor) {
		t.Fatalf("nothing read keeps the cursor, got %v", got)
	}
}

func TestSafeOverridesNeverRetarget(t *testing.T) {
	got := safeOverrides(map[string]any{"Id": "001", "Account.Name": "x", "Tier__c": "A", "bad field": 1})
	if len(got) != 1 || got["Tier__c"] != "A" {
		t.Fatalf("got %v", got)
	}
}

func TestParseSettingsDropsRulesThatCouldNotBeSaved(t *testing.T) {
	raw := json.RawMessage(`{"salesforce":{"field_map":[{"object":"Lead","warmbly":"phone","salesforce":"Phone FROM User","direction":"push","policy":"overwrite"},{"object":"Lead","warmbly":"phone","salesforce":"MobilePhone","direction":"push","policy":"overwrite"}]}}`)
	s := ParseSettings(raw)
	if len(s.FieldMap) != 1 || s.FieldMap[0].Salesforce != "MobilePhone" {
		t.Fatalf("got %+v", s.FieldMap)
	}
}
