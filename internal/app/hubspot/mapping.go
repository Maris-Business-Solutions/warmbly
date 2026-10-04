package hubspot

import (
	"context"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// The "Warmbly" property group on contacts: what HubSpot users filter lists,
// build reports and trigger workflows on.
const (
	propGroup         = "warmbly"
	propStatus        = "warmbly_status"
	propLastCampaign  = "warmbly_last_campaign"
	propLastContacted = "warmbly_last_contacted_at"
	propLastReplied   = "warmbly_last_replied_at"
	propReplyIntent   = "warmbly_reply_intent"
	propLastOpened    = "warmbly_last_opened_at"
	propLastClicked   = "warmbly_last_clicked_at"
	propUnsubscribed  = "warmbly_unsubscribed"
	propLink          = "warmbly_link"
)

// Values of warmbly_status.
const (
	statusContacted    = "contacted"
	statusReplied      = "replied"
	statusInterested   = "interested"
	statusNotInterest  = "not_interested"
	statusBounced      = "bounced"
	statusUnsubscribed = "unsubscribed"
	statusMeeting      = "meeting_booked"
)

var warmblyProperties = []PropertyDef{
	{Name: propStatus, Label: "Warmbly status", Type: "enumeration", FieldType: "select", GroupName: propGroup,
		Description: "Where this contact stands in Warmbly outreach.",
		Options: []PropertyOption{
			{Label: "Contacted", Value: statusContacted, DisplayOrder: 0},
			{Label: "Replied", Value: statusReplied, DisplayOrder: 1},
			{Label: "Interested", Value: statusInterested, DisplayOrder: 2},
			{Label: "Not interested", Value: statusNotInterest, DisplayOrder: 3},
			{Label: "Meeting booked", Value: statusMeeting, DisplayOrder: 4},
			{Label: "Bounced", Value: statusBounced, DisplayOrder: 5},
			{Label: "Unsubscribed", Value: statusUnsubscribed, DisplayOrder: 6},
		}},
	{Name: propLastCampaign, Label: "Warmbly last campaign", Type: "string", FieldType: "text", GroupName: propGroup},
	{Name: propLastContacted, Label: "Warmbly last contacted", Type: "datetime", FieldType: "date", GroupName: propGroup},
	{Name: propLastReplied, Label: "Warmbly last replied", Type: "datetime", FieldType: "date", GroupName: propGroup},
	{Name: propReplyIntent, Label: "Warmbly reply intent", Type: "string", FieldType: "text", GroupName: propGroup,
		Description: "How Warmbly classified the contact's latest reply."},
	{Name: propLastOpened, Label: "Warmbly last opened", Type: "datetime", FieldType: "date", GroupName: propGroup},
	{Name: propLastClicked, Label: "Warmbly last clicked", Type: "datetime", FieldType: "date", GroupName: propGroup},
	{Name: propUnsubscribed, Label: "Warmbly unsubscribed", Type: "bool", FieldType: "booleancheckbox", GroupName: propGroup,
		Options: []PropertyOption{{Label: "Yes", Value: "true", DisplayOrder: 0}, {Label: "No", Value: "false", DisplayOrder: 1}}},
	{Name: propLink, Label: "Open in Warmbly", Type: "string", FieldType: "text", GroupName: propGroup},
}

func (s *Service) ensureProperties(ctx context.Context, o *org) error {
	if s.d.Cache != nil {
		key := "hubspot:propsready:" + o.Portal
		if ok, _ := s.d.Cache.Exists(ctx, key).Result(); ok > 0 {
			return nil
		}
		defer s.d.Cache.Set(ctx, key, 1, 6*time.Hour)
	}
	return o.Client.EnsureContactProperties(ctx, propGroup, "Warmbly", warmblyProperties)
}

func hsTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// parseHSTime reads a datetime property (ISO 8601 or epoch milliseconds).
func parseHSTime(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
		t := time.UnixMilli(ms).UTC()
		return &t
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

// ---------- tasks ----------

type taskTypeOption struct{ value, label string }

// taskTypeOptions are HubSpot's task types as Warmbly shows them.
var taskTypeOptions = []taskTypeOption{
	{"TODO", "To-do"}, {"CALL", "Call"}, {"EMAIL", "Email"}, {"LINKED_IN", "LinkedIn"},
}

var taskTypeColors = map[string]string{"TODO": "#64748b", "CALL": "#8b5cf6", "EMAIL": "#0ea5e9", "LINKED_IN": "#0a66c2"}

// taskTypeNamespace keeps the synthetic task type ids stable across calls.
var taskTypeNamespace = uuid.MustParse("6f1d7b2e-6d0c-4c1e-9a52-6d1f0c1b8a01")

// TaskTypes is the fixed HubSpot task type set, in the shape the pickers read.
func (s *Service) TaskTypes(ctx context.Context, orgID uuid.UUID) []models.CRMTaskType {
	out := make([]models.CRMTaskType, 0, len(taskTypeOptions))
	for i, t := range taskTypeOptions {
		out = append(out, models.CRMTaskType{
			ID:             uuid.NewSHA1(taskTypeNamespace, []byte(t.value)),
			OrganizationID: orgID,
			Name:           t.label,
			Color:          taskTypeColors[t.value],
			Position:       i,
		})
	}
	return out
}

func hsTaskType(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == "call" || strings.Contains(n, "phone"):
		return "CALL"
	case n == "email" || n == "e-mail":
		return "EMAIL"
	case strings.Contains(n, "linkedin"):
		return "LINKED_IN"
	default:
		return "TODO"
	}
}

func localTaskType(v string) string {
	switch strings.ToUpper(v) {
	case "CALL":
		return "Call"
	case "EMAIL":
		return "Email"
	case "LINKED_IN", "LINKED_IN_CONNECT", "LINKED_IN_MESSAGE":
		return "LinkedIn"
	default:
		return "To-do"
	}
}

func hsTaskStatus(s models.CRMTaskStatus) string {
	switch s {
	case models.CRMTaskStatusCompleted:
		return "COMPLETED"
	case models.CRMTaskStatusInProgress:
		return "IN_PROGRESS"
	case models.CRMTaskStatusCancelled:
		return "DEFERRED"
	default:
		return "NOT_STARTED"
	}
}

func localTaskStatus(v string) models.CRMTaskStatus {
	switch strings.ToUpper(v) {
	case "COMPLETED":
		return models.CRMTaskStatusCompleted
	case "IN_PROGRESS", "WAITING":
		return models.CRMTaskStatusInProgress
	case "DEFERRED":
		return models.CRMTaskStatusCancelled
	default:
		return models.CRMTaskStatusPending
	}
}

func hsTaskPriority(p models.CRMTaskPriority) string {
	switch p {
	case models.CRMTaskPriorityLow:
		return "LOW"
	case models.CRMTaskPriorityHigh, models.CRMTaskPriorityUrgent:
		return "HIGH"
	default:
		return "MEDIUM"
	}
}

func localTaskPriority(v string) models.CRMTaskPriority {
	switch strings.ToUpper(v) {
	case "HIGH":
		return models.CRMTaskPriorityHigh
	case "LOW", "NONE":
		return models.CRMTaskPriorityLow
	default:
		return models.CRMTaskPriorityMedium
	}
}

var taskProps = []string{"hs_task_subject", "hs_task_body", "hs_timestamp", "hs_task_status", "hs_task_priority",
	"hs_task_type", "hubspot_owner_id", "hs_task_completion_date", "hs_createdate", "hs_lastmodifieddate"}

// ---------- deals ----------

var dealProps = []string{"dealname", "amount", "dealstage", "pipeline", "closedate", "hubspot_owner_id",
	"deal_currency_code", "createdate", "hs_lastmodifieddate", "closed_lost_reason", "hs_is_closed_won", "hs_is_closed"}

// stageMeta is what a mirrored stage remembers of HubSpot's metadata.
type stageMeta struct {
	Closed      bool
	Won         bool
	Probability *float64
}

func stageMetaFrom(st Stage) stageMeta {
	m := stageMeta{Closed: strings.EqualFold(st.Metadata["isClosed"], "true")}
	if p, err := strconv.ParseFloat(st.Metadata["probability"], 64); err == nil {
		m.Probability = &p
		m.Won = m.Closed && p >= 1
	}
	return m
}

func (m stageMeta) toMap() map[string]any {
	out := map[string]any{"closed": m.Closed, "won": m.Won}
	if m.Probability != nil {
		out["probability"] = *m.Probability
	}
	return out
}

func stageMetaFromMap(meta map[string]any) stageMeta {
	m := stageMeta{}
	m.Closed, _ = meta["closed"].(bool)
	m.Won, _ = meta["won"].(bool)
	if p, ok := meta["probability"].(float64); ok {
		m.Probability = &p
	}
	return m
}

func (m stageMeta) status() models.DealStatus {
	switch {
	case m.Closed && m.Won:
		return models.DealStatusWon
	case m.Closed:
		return models.DealStatusLost
	default:
		return models.DealStatusOpen
	}
}

// stagePalette colours mirrored stages by position; HubSpot stages carry none.
var stagePalette = []string{"#0ea5e9", "#6366f1", "#8b5cf6", "#f59e0b", "#14b8a6", "#ec4899", "#64748b", "#84cc16"}

func stageColor(i int, m stageMeta) string {
	switch {
	case m.Closed && m.Won:
		return "#10b981"
	case m.Closed:
		return "#ef4444"
	default:
		return stagePalette[i%len(stagePalette)]
	}
}

// ---------- text ----------

var (
	reBreak = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</div>|</li>`)
	reTag   = regexp.MustCompile(`<[^>]*>`)
	reBlank = regexp.MustCompile(`\n{3,}`)
)

// htmlToText flattens HubSpot rich text (notes, task bodies) for Warmbly.
func htmlToText(s string) string {
	s = reBreak.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(reBlank.ReplaceAllString(s, "\n\n"))
}

// textToHTML renders Warmbly plain text for HubSpot's rich text fields.
func textToHTML(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = html.EscapeString(l)
	}
	return strings.Join(lines, "<br>")
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func emailDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}
	d := strings.ToLower(strings.TrimSpace(email[at+1:]))
	if freeMailDomains[d] {
		return ""
	}
	return d
}

// freeMailDomains never become companies.
var freeMailDomains = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "yahoo.com": true, "outlook.com": true, "hotmail.com": true,
	"live.com": true, "icloud.com": true, "me.com": true, "aol.com": true, "proton.me": true, "protonmail.com": true,
	"gmx.com": true, "gmx.de": true, "mail.com": true, "yandex.com": true, "zoho.com": true, "msn.com": true,
}
