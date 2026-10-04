package salesforce

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/warmbly/warmbly/internal/models"
)

// settingsKey is where the settings live inside a connection's
// config_capabilities. Free-form, evolving, read-then-execute config, so jsonb
// with this struct as the type boundary and Validate on every write.
const settingsKey = "salesforce"

// Object names the sync writes.
const (
	ObjectLead    = "Lead"
	ObjectContact = "Contact"
)

// Settings is one connection's sync configuration.
type Settings struct {
	// Enabled is the master switch for logging, writeback and the pull loop.
	// Imports and the contact panel work regardless.
	Enabled bool `json:"enabled"`

	Matching  MatchingSettings  `json:"matching"`
	Activity  ActivitySettings  `json:"activity"`
	Writeback WritebackSettings `json:"writeback"`
	Inbound   InboundSettings   `json:"inbound"`

	// FieldMap is per-object field sync, in both directions.
	FieldMap []FieldRule `json:"field_map"`

	// DailyAPIBudget caps the calls Warmbly makes per day; 0 means a fifth of
	// the org's daily allocation.
	DailyAPIBudget int `json:"daily_api_budget"`
}

// MatchingSettings decides which record an email is, and what happens when it
// is none.
type MatchingSettings struct {
	// Prefer is which object wins when an email is both: "contact" or "lead".
	Prefer string `json:"prefer"`
	// CreateWhen is when a missing person is created: "never", "reply" (on a
	// reply or a meeting) or "send" (on the first logged activity).
	CreateWhen string `json:"create_when"`
	// CreateAs is "lead" or "contact". A created Contact needs no Account.
	CreateAs string `json:"create_as"`
	// LeadSource stamps created records; empty leaves Salesforce's default.
	LeadSource string `json:"lead_source"`
	// LeadStatus is the status a created Lead starts in; empty is the default.
	LeadStatus string `json:"lead_status"`
	// Owner of created records: "connected_user", "sender" (the Salesforce
	// user with the sending mailbox's address) or "fixed".
	Owner   string `json:"owner"`
	OwnerID string `json:"owner_id,omitempty"`
	// RunAssignmentRules lets the org's lead assignment rules pick the owner.
	RunAssignmentRules bool `json:"run_assignment_rules"`
}

// ActivitySettings decides which campaign events become Tasks.
type ActivitySettings struct {
	Sent          bool `json:"sent"`
	Replied       bool `json:"replied"`
	Opened        bool `json:"opened"`
	Clicked       bool `json:"clicked"`
	Bounced       bool `json:"bounced"`
	Unsubscribed  bool `json:"unsubscribed"`
	MeetingBooked bool `json:"meeting_booked"`
	// IncludeBody writes the email or reply text into the Task description.
	IncludeBody bool `json:"include_body"`
	// AssignTo owns the Task: "record_owner", "sender" or "connected_user".
	AssignTo string `json:"assign_to"`
	// RelateToOpportunity puts a Contact's Tasks on their account's open
	// opportunity, else the account.
	RelateToOpportunity bool `json:"relate_to_opportunity"`
}

// Logs reports whether an activity kind is switched on.
func (a ActivitySettings) Logs(kind string) bool {
	switch kind {
	case KindSent:
		return a.Sent
	case KindReplied:
		return a.Replied
	case KindOpened:
		return a.Opened
	case KindClicked:
		return a.Clicked
	case KindBounced:
		return a.Bounced
	case KindUnsubscribed:
		return a.Unsubscribed
	case KindMeetingBooked:
		return a.MeetingBooked
	}
	return false
}

// WritebackSettings decides which Lead and Contact fields Warmbly changes.
type WritebackSettings struct {
	// LeadStatusOnSent is the status a Lead moves to when first emailed.
	LeadStatusOnSent string `json:"lead_status_on_sent"`
	// LeadStatusOnReply maps a reply intent (positive, negative, neutral,
	// question, out_of_office, any) to a Lead status.
	LeadStatusOnReply map[string]string `json:"lead_status_on_reply"`
	// LeadStatusOnMeeting is the status a booked meeting moves a Lead to.
	LeadStatusOnMeeting string `json:"lead_status_on_meeting"`
	// NeverMoveBackwards keeps a Lead that is further along the status
	// picklist where it is.
	NeverMoveBackwards bool `json:"never_move_backwards"`
}

// InboundSettings decides what a change in Salesforce does in Warmbly.
type InboundSettings struct {
	// OptOut keeps do-not-email in step: "both", "to_salesforce",
	// "from_salesforce" or "off".
	OptOut string `json:"opt_out"`
	// PauseOnConverted holds a Lead's outreach once it is converted.
	PauseOnConverted bool `json:"pause_on_converted"`
	// PauseOnStatuses holds a Lead's outreach once it reaches one of these.
	PauseOnStatuses []string `json:"pause_on_statuses"`
	// PauseOnOpenOpportunity holds a Contact whose account has an open
	// opportunity: the deal is already being worked.
	PauseOnOpenOpportunity bool `json:"pause_on_open_opportunity"`
}

// Field directions and conflict policies.
const (
	DirectionPush = "push"
	DirectionPull = "pull"
	DirectionBoth = "both"

	// PolicyOverwrite writes the value whenever it changes.
	PolicyOverwrite = "overwrite"
	// PolicyIfEmpty only fills a field that is blank on the receiving side.
	PolicyIfEmpty = "if_empty"
)

// FieldRule syncs one Warmbly field with one Salesforce field.
type FieldRule struct {
	Object     string `json:"object"`
	Warmbly    string `json:"warmbly"`
	Salesforce string `json:"salesforce"`
	Direction  string `json:"direction"`
	Policy     string `json:"policy"`
}

// Activity kinds the outbox carries; mirror the queue's CHECK.
const (
	KindSent          = "sent"
	KindOpened        = "opened"
	KindClicked       = "clicked"
	KindReplied       = "replied"
	KindBounced       = "bounced"
	KindUnsubscribed  = "unsubscribed"
	KindMeetingBooked = "meeting_booked"
)

// DefaultSettings is what a new connection starts with: sends and replies on
// the timeline, opt-outs in step both ways, nothing created or overwritten.
func DefaultSettings() Settings {
	return Settings{
		Enabled: true,
		Matching: MatchingSettings{
			Prefer:     "contact",
			CreateWhen: "reply",
			CreateAs:   "lead",
			LeadSource: "Warmbly",
			Owner:      "connected_user",
		},
		Activity: ActivitySettings{
			Sent:                true,
			Replied:             true,
			Bounced:             true,
			Unsubscribed:        true,
			MeetingBooked:       true,
			IncludeBody:         true,
			AssignTo:            "record_owner",
			RelateToOpportunity: true,
		},
		Writeback: WritebackSettings{
			LeadStatusOnReply:  map[string]string{},
			NeverMoveBackwards: true,
		},
		Inbound: InboundSettings{
			OptOut:           "both",
			PauseOnConverted: false,
			PauseOnStatuses:  []string{},
		},
		FieldMap: DefaultFieldMap(),
	}
}

// DefaultFieldMap pushes identity fields to new records and fills blanks in
// Warmbly from Salesforce, never overwriting either side.
func DefaultFieldMap() []FieldRule {
	var out []FieldRule
	for _, obj := range []string{ObjectLead, ObjectContact} {
		out = append(out,
			FieldRule{Object: obj, Warmbly: "first_name", Salesforce: "FirstName", Direction: DirectionBoth, Policy: PolicyIfEmpty},
			FieldRule{Object: obj, Warmbly: "last_name", Salesforce: "LastName", Direction: DirectionBoth, Policy: PolicyIfEmpty},
			FieldRule{Object: obj, Warmbly: "phone", Salesforce: "Phone", Direction: DirectionBoth, Policy: PolicyIfEmpty},
		)
	}
	out = append(out, FieldRule{Object: ObjectLead, Warmbly: "company", Salesforce: "Company", Direction: DirectionBoth, Policy: PolicyIfEmpty})
	out = append(out, FieldRule{Object: ObjectContact, Warmbly: "company", Salesforce: "Account.Name", Direction: DirectionPull, Policy: PolicyIfEmpty})
	return out
}

// WarmblyFields is the vocabulary a field rule may name on the Warmbly side.
// custom:<key> reads and writes a contact custom field; engagement fields are
// push-only and derived from the contact's campaign activity.
var WarmblyFields = []models.FieldDef{
	{Key: "first_name", Label: "First name"},
	{Key: "last_name", Label: "Last name"},
	{Key: "company", Label: "Company"},
	{Key: "phone", Label: "Phone"},
	{Key: "engagement.last_campaign", Label: "Last Warmbly campaign"},
	{Key: "engagement.last_sent_at", Label: "Last emailed at"},
	{Key: "engagement.last_reply_at", Label: "Last replied at"},
	{Key: "engagement.reply_intent", Label: "Last reply intent"},
	{Key: "engagement.status", Label: "Outreach status"},
}

func isEngagementField(k string) bool { return strings.HasPrefix(k, "engagement.") }

func validWarmblyField(k string) bool {
	if strings.HasPrefix(k, "custom:") {
		return len(strings.TrimSpace(strings.TrimPrefix(k, "custom:"))) > 0
	}
	for _, f := range WarmblyFields {
		if f.Key == k {
			return true
		}
	}
	return false
}

func validSalesforceField(name string) bool {
	if name == "" || len(name) > 120 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// replyIntents are the keys LeadStatusOnReply may use.
var replyIntents = []string{"any", "positive", "negative", "neutral", "question", "out_of_office"}

// Validate refuses a configuration the sync cannot execute.
func (s *Settings) Validate() error {
	m := s.Matching
	if !oneOf(m.Prefer, "contact", "lead") {
		return fmt.Errorf("matching.prefer must be contact or lead")
	}
	if !oneOf(m.CreateWhen, "never", "reply", "send") {
		return fmt.Errorf("matching.create_when must be never, reply or send")
	}
	if !oneOf(m.CreateAs, "lead", "contact") {
		return fmt.Errorf("matching.create_as must be lead or contact")
	}
	if !oneOf(m.Owner, "connected_user", "sender", "fixed") {
		return fmt.Errorf("matching.owner must be connected_user, sender or fixed")
	}
	if m.Owner == "fixed" && !ValidID(m.OwnerID) {
		return fmt.Errorf("choose the Salesforce user who owns created records")
	}
	if len(m.LeadSource) > 120 || len(m.LeadStatus) > 120 {
		return fmt.Errorf("lead source and status are limited to 120 characters")
	}
	if !oneOf(s.Activity.AssignTo, "record_owner", "sender", "connected_user") {
		return fmt.Errorf("activity.assign_to must be record_owner, sender or connected_user")
	}
	for k, v := range s.Writeback.LeadStatusOnReply {
		if !oneOf(k, replyIntents...) {
			return fmt.Errorf("unknown reply intent %q", k)
		}
		if len(v) > 120 {
			return fmt.Errorf("lead status values are limited to 120 characters")
		}
	}
	if !oneOf(s.Inbound.OptOut, "both", "to_salesforce", "from_salesforce", "off") {
		return fmt.Errorf("inbound.opt_out must be both, to_salesforce, from_salesforce or off")
	}
	if len(s.Inbound.PauseOnStatuses) > 50 {
		return fmt.Errorf("at most 50 pause statuses")
	}
	if s.DailyAPIBudget < 0 {
		return fmt.Errorf("daily_api_budget cannot be negative")
	}
	if len(s.FieldMap) > 100 {
		return fmt.Errorf("at most 100 field rules")
	}
	seen := map[string]bool{}
	for i, r := range s.FieldMap {
		if !oneOf(r.Object, ObjectLead, ObjectContact) {
			return fmt.Errorf("field rule %d: object must be Lead or Contact", i+1)
		}
		if !validWarmblyField(r.Warmbly) {
			return fmt.Errorf("field rule %d: unknown Warmbly field %q", i+1, r.Warmbly)
		}
		if !validSalesforceField(r.Salesforce) {
			return fmt.Errorf("field rule %d: invalid Salesforce field %q", i+1, r.Salesforce)
		}
		if !oneOf(r.Direction, DirectionPush, DirectionPull, DirectionBoth) {
			return fmt.Errorf("field rule %d: direction must be push, pull or both", i+1)
		}
		if !oneOf(r.Policy, PolicyOverwrite, PolicyIfEmpty) {
			return fmt.Errorf("field rule %d: policy must be overwrite or if_empty", i+1)
		}
		if isEngagementField(r.Warmbly) && r.Direction != DirectionPush {
			return fmt.Errorf("field rule %d: engagement fields only push to Salesforce", i+1)
		}
		if strings.Contains(r.Salesforce, ".") && r.Direction != DirectionPull {
			return fmt.Errorf("field rule %d: a related field like %s can only be read", i+1, r.Salesforce)
		}
		key := r.Object + "|" + strings.ToLower(r.Salesforce) + "|" + r.Direction
		if seen[key] {
			return fmt.Errorf("field rule %d: %s.%s is mapped twice", i+1, r.Object, r.Salesforce)
		}
		seen[key] = true
	}
	return nil
}

// ParseSettings reads the settings out of a connection's config_capabilities,
// filling anything unset from the defaults. A connection that never saved any
// predates native sync and stays off until someone turns it on.
func ParseSettings(raw json.RawMessage) Settings {
	legacy := DefaultSettings()
	legacy.Enabled = false
	// The upsert action this replaces created Contacts.
	legacy.Matching.CreateAs = "contact"
	if len(raw) == 0 {
		return legacy
	}
	var wrap map[string]json.RawMessage
	if json.Unmarshal(raw, &wrap) != nil {
		return legacy
	}
	blob, ok := wrap[settingsKey]
	if !ok {
		return legacy
	}
	s := DefaultSettings()
	_ = json.Unmarshal(blob, &s)
	if s.Writeback.LeadStatusOnReply == nil {
		s.Writeback.LeadStatusOnReply = map[string]string{}
	}
	if s.Inbound.PauseOnStatuses == nil {
		s.Inbound.PauseOnStatuses = []string{}
	}
	// Field names reach SOQL, so a rule that would not pass Validate is dropped
	// however it got stored.
	rules := make([]FieldRule, 0, len(s.FieldMap))
	for _, r := range s.FieldMap {
		if oneOf(r.Object, ObjectLead, ObjectContact) && validWarmblyField(r.Warmbly) && validSalesforceField(r.Salesforce) &&
			oneOf(r.Direction, DirectionPush, DirectionPull, DirectionBoth) && oneOf(r.Policy, PolicyOverwrite, PolicyIfEmpty) {
			rules = append(rules, r)
		}
	}
	s.FieldMap = rules
	return s
}

// withLegacyMappings carries a pre-native connection's own Contact field map
// into its rules, replacing the default rule for any field it names.
func (s *Settings) withLegacyMappings(rows []models.IntegrationFieldMapping) {
	var extra []FieldRule
	named := map[string]bool{}
	for _, r := range rows {
		if r.SubscriptionID != nil || !strings.EqualFold(r.ObjectName, "contact") {
			continue
		}
		if r.Transform != "" && r.Transform != string(models.FieldTransformNone) {
			continue
		}
		if !validWarmblyField(r.WarmblyField) || !validSalesforceField(r.ExternalField) || strings.Contains(r.ExternalField, ".") {
			continue
		}
		if strings.EqualFold(r.ExternalField, "Email") || named[strings.ToLower(r.ExternalField)] {
			continue
		}
		named[strings.ToLower(r.ExternalField)] = true
		extra = append(extra, FieldRule{Object: ObjectContact, Warmbly: r.WarmblyField, Salesforce: r.ExternalField, Direction: DirectionPush, Policy: PolicyOverwrite})
	}
	if len(extra) == 0 {
		return
	}
	kept := s.FieldMap[:0]
	for _, r := range s.FieldMap {
		if r.Object == ObjectContact && named[strings.ToLower(r.Salesforce)] {
			continue
		}
		kept = append(kept, r)
	}
	s.FieldMap = append(kept, extra...)
}

// rulesFor returns the object's rules that move data in a direction.
func (s *Settings) rulesFor(object, direction string) []FieldRule {
	var out []FieldRule
	for _, r := range s.FieldMap {
		if r.Object != object {
			continue
		}
		if r.Direction == direction || r.Direction == DirectionBoth {
			out = append(out, r)
		}
	}
	return out
}

// statusForReply picks the Lead status a reply of this intent moves to.
func (w WritebackSettings) statusForReply(intent string) string {
	if v := strings.TrimSpace(w.LeadStatusOnReply[intent]); v != "" {
		return v
	}
	if intent == string(models.ReplyIntentAutomated) || intent == string(models.ReplyIntentOutOfOffice) {
		// An auto-reply is not a conversation; only an explicit mapping moves it.
		return ""
	}
	return strings.TrimSpace(w.LeadStatusOnReply["any"])
}
