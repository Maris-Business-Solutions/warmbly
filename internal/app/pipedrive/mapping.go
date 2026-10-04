package pipedrive

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// ---------- time ----------

// parseTime reads Pipedrive's timestamps: RFC 3339 (v2), "2006-01-02 15:04:05"
// in UTC (v1) and plain dates.
func parseTime(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

func parsePtrTime(v *string) *time.Time {
	if v == nil {
		return nil
	}
	return parseTime(*v)
}

func pdDate(t time.Time) string { return t.UTC().Format("2006-01-02") }

func pdClock(t time.Time) string { return t.UTC().Format("15:04") }

func pdTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// dueAt joins an activity's due date and UTC time.
func dueAt(date, clock string) *time.Time {
	d := parseTime(date)
	if d == nil {
		return nil
	}
	if clock = strings.TrimSpace(clock); clock != "" {
		for _, layout := range []string{"15:04:05", "15:04"} {
			if c, err := time.Parse(layout, clock); err == nil {
				t := time.Date(d.Year(), d.Month(), d.Day(), c.Hour(), c.Minute(), c.Second(), 0, time.UTC)
				return &t
			}
		}
	}
	return d
}

// ---------- activity types ----------

// defaultActivityTypes are Pipedrive's built-in types, used until the
// company's own list is read.
var defaultActivityTypes = []ActivityType{
	{Name: "Call", KeyString: "call", ActiveFlag: true},
	{Name: "Meeting", KeyString: "meeting", ActiveFlag: true},
	{Name: "Task", KeyString: "task", ActiveFlag: true},
	{Name: "Deadline", KeyString: "deadline", ActiveFlag: true},
	{Name: "Email", KeyString: "email", ActiveFlag: true},
	{Name: "Lunch", KeyString: "lunch", ActiveFlag: true},
}

var activityColors = map[string]string{
	"call": "#8b5cf6", "meeting": "#f59e0b", "task": "#64748b", "deadline": "#ef4444", "email": "#0ea5e9", "lunch": "#14b8a6",
}

// activityTypeNamespace keeps the synthetic task type ids stable across calls.
var activityTypeNamespace = uuid.MustParse("3b6f2a8e-1c4d-4f7a-9e25-7d0c5b1a9f42")

// activityTypes is the company's activity types, cached for an hour.
func (s *Service) activityTypes(ctx context.Context, o *org) []ActivityType {
	key := "pipedrive:acttypes:" + o.Company
	if s.d.Cache != nil {
		if raw, err := s.d.Cache.Get(ctx, key).Bytes(); err == nil && len(raw) > 0 {
			var out []ActivityType
			if json.Unmarshal(raw, &out) == nil && len(out) > 0 {
				return out
			}
		}
	}
	types, err := o.Client.ActivityTypes(ctx)
	if err != nil || len(types) == 0 {
		return defaultActivityTypes
	}
	active := types[:0]
	for _, t := range types {
		if t.ActiveFlag {
			active = append(active, t)
		}
	}
	sort.SliceStable(active, func(i, j int) bool { return active[i].OrderNr < active[j].OrderNr })
	if s.d.Cache != nil {
		_ = s.d.Cache.SetJSON(ctx, key, active, time.Hour)
	}
	return active
}

// TaskTypes is the company's activity types, in the shape the pickers read.
func (s *Service) TaskTypes(ctx context.Context, orgID uuid.UUID) []models.CRMTaskType {
	types := defaultActivityTypes
	if o, err := s.resolve(ctx, orgID); err == nil && o != nil {
		types = s.activityTypes(ctx, o)
	}
	out := make([]models.CRMTaskType, 0, len(types))
	for i, t := range types {
		color := t.Color
		if color == "" || !strings.HasPrefix(color, "#") {
			color = activityColors[t.KeyString]
		}
		if color == "" {
			color = "#64748b"
		}
		out = append(out, models.CRMTaskType{
			ID:             uuid.NewSHA1(activityTypeNamespace, []byte(t.KeyString)),
			OrganizationID: orgID,
			Name:           t.Name,
			Color:          color,
			Position:       i,
		})
	}
	return out
}

// activityKey picks the Pipedrive activity type for a Warmbly task type name.
func (s *Service) activityKey(ctx context.Context, o *org, name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, t := range s.activityTypes(ctx, o) {
		if strings.ToLower(t.Name) == n || t.KeyString == n {
			return t.KeyString
		}
	}
	switch {
	case strings.Contains(n, "call") || strings.Contains(n, "phone"):
		return "call"
	case strings.Contains(n, "meet"):
		return "meeting"
	case n == "email" || n == "e-mail":
		return "email"
	case strings.Contains(n, "deadline"):
		return "deadline"
	default:
		return "task"
	}
}

// activityName is the Warmbly task type name for a Pipedrive activity type.
func (s *Service) activityName(ctx context.Context, o *org, key string) string {
	for _, t := range s.activityTypes(ctx, o) {
		if t.KeyString == key {
			return t.Name
		}
	}
	if key == "" {
		return "Task"
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

// ---------- the Warmbly person fields ----------

// The Warmbly fields on Pipedrive persons: what people filter on, build
// reports and automations from. Matched by name, created when missing.
const (
	fieldStatus        = "Warmbly status"
	fieldLastCampaign  = "Warmbly last campaign"
	fieldLastContacted = "Warmbly last contacted"
	fieldLastReplied   = "Warmbly last replied"
	fieldReplyIntent   = "Warmbly reply intent"
	fieldLastOpened    = "Warmbly last opened"
	fieldLastClicked   = "Warmbly last clicked"
	fieldLink          = "Open in Warmbly"
)

// Values of the Warmbly status field.
const (
	statusContacted    = "Contacted"
	statusReplied      = "Replied"
	statusInterested   = "Interested"
	statusNotInterest  = "Not interested"
	statusMeeting      = "Meeting booked"
	statusBounced      = "Bounced"
	statusUnsubscribed = "Unsubscribed"
)

// statusKey is where the last written status is remembered on the record.
const statusKey = "warmbly_status"

type fieldDef struct {
	Name    string
	Type    string
	Options []string
}

var warmblyFields = []fieldDef{
	{Name: fieldStatus, Type: "enum", Options: []string{statusContacted, statusReplied, statusInterested, statusNotInterest,
		statusMeeting, statusBounced, statusUnsubscribed}},
	{Name: fieldLastCampaign, Type: "varchar"},
	{Name: fieldLastContacted, Type: "date"},
	{Name: fieldLastReplied, Type: "date"},
	{Name: fieldReplyIntent, Type: "varchar"},
	{Name: fieldLastOpened, Type: "date"},
	{Name: fieldLastClicked, Type: "date"},
	{Name: fieldLink, Type: "varchar"},
}

// warmblyFieldSet maps a Warmbly field name to its Pipedrive field code, and
// the status options to their ids.
type warmblyFieldSet struct {
	Codes   map[string]string `json:"codes"`
	Options map[string]int64  `json:"options"`
}

// ---------- text ----------

var (
	reBreak = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</div>|</li>`)
	reTag   = regexp.MustCompile(`<[^>]*>`)
	reBlank = regexp.MustCompile(`\n{3,}`)
)

// htmlToText flattens Pipedrive rich text (notes, activity notes) for Warmbly.
func htmlToText(s string) string {
	s = reBreak.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(reBlank.ReplaceAllString(s, "\n\n"))
}

// textToHTML renders Warmbly plain text for Pipedrive's rich text fields.
func textToHTML(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = html.EscapeString(l)
	}
	return strings.Join(lines, "<br>")
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// ---------- stages ----------

var stagePalette = []string{"#0ea5e9", "#6366f1", "#8b5cf6", "#f59e0b", "#14b8a6", "#ec4899", "#64748b", "#84cc16"}

func stageMeta(st Stage) map[string]any {
	out := map[string]any{"closed": false, "won": false}
	if st.DealProbability != nil {
		out["probability"] = *st.DealProbability / 100
	}
	return out
}

func probabilityOf(meta map[string]any) *float64 {
	if p, ok := meta["probability"].(float64); ok {
		return &p
	}
	return nil
}

func dealStatus(v string) models.DealStatus {
	switch strings.ToLower(v) {
	case "won":
		return models.DealStatusWon
	case "lost":
		return models.DealStatusLost
	default:
		return models.DealStatusOpen
	}
}

func ownerMeta(owner string) map[string]any {
	if owner == "" {
		return map[string]any{}
	}
	return map[string]any{"owner": owner}
}

func contactIDString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// ---------- custom field values ----------

// fieldValue renders a v2 custom field value for display: option ids become
// their labels, structured values their main part.
func fieldValue(v any, f *Field) string {
	label := func(n float64) string {
		if f != nil {
			for _, op := range f.Options {
				if float64(op.ID) == n {
					return op.Label
				}
			}
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "Yes"
		}
		return "No"
	case float64:
		if f != nil && (f.FieldType == "enum" || f.FieldType == "set") {
			return label(x)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			if n, ok := e.(float64); ok {
				parts = append(parts, label(n))
			} else if s := fieldValue(e, nil); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		if val, ok := x["value"]; ok {
			s := fieldValue(val, f)
			if cur, ok := x["currency"].(string); ok && cur != "" {
				s += " " + cur
			}
			return s
		}
		if name, ok := x["name"].(string); ok {
			return name
		}
		return ""
	default:
		return fmt.Sprint(x)
	}
}
