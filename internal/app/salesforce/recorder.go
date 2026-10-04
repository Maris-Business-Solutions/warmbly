package salesforce

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/app/cipher"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// Recorder turns platform events into outbox rows. It runs wherever events are
// raised (backend and consumer) and only writes; the drain runs elsewhere.
// It is wired ahead of the webhook throttle, because a dropped event is a gap
// in a customer's CRM history.
type Recorder struct {
	repo   repository.SalesforceRepository
	cipher cipher.CipherService

	mu    sync.Mutex
	cache map[uuid.UUID]recorderEntry
}

type recorderEntry struct {
	at    time.Time
	conns []recorderConn
}

type recorderConn struct {
	id       uuid.UUID
	settings Settings
}

// recorderTTL bounds how stale the per-org connection list can be. Settings
// saved on this process are seen at once; another process sees them within it.
const recorderTTL = time.Minute

// NewRecorder builds the event sink.
func NewRecorder(repo repository.SalesforceRepository, c cipher.CipherService) *Recorder {
	return &Recorder{repo: repo, cipher: c, cache: map[uuid.UUID]recorderEntry{}}
}

func (r *Recorder) forget(orgID uuid.UUID) {
	r.mu.Lock()
	delete(r.cache, orgID)
	r.mu.Unlock()
}

func (r *Recorder) connections(ctx context.Context, orgID uuid.UUID) []recorderConn {
	r.mu.Lock()
	e, ok := r.cache[orgID]
	r.mu.Unlock()
	if ok && time.Since(e.at) < recorderTTL {
		return e.conns
	}
	refs, err := r.repo.ActiveConnectionsForOrg(ctx, orgID)
	if err != nil {
		return nil
	}
	conns := make([]recorderConn, 0, len(refs))
	for _, ref := range refs {
		st := ParseSettings(ref.ConfigCapabilities)
		if st.Enabled {
			conns = append(conns, recorderConn{id: ref.ID, settings: st})
		}
	}
	r.mu.Lock()
	r.cache[orgID] = recorderEntry{at: time.Now(), conns: conns}
	r.mu.Unlock()
	return conns
}

// kindFor maps a platform event to an activity kind.
func kindFor(t models.WebhookEventType) string {
	switch t {
	case models.WebhookEventCampaignEmailSent:
		return KindSent
	case models.WebhookEventCampaignEmailOpened:
		return KindOpened
	case models.WebhookEventCampaignEmailClicked:
		return KindClicked
	case models.WebhookEventCampaignReplyReceived:
		return KindReplied
	case models.WebhookEventCampaignEmailBounced:
		return KindBounced
	case models.WebhookEventCampaignUnsubscribed, models.WebhookEventDeliverabilityComplaint:
		return KindUnsubscribed
	case models.WebhookEventMeetingBooked:
		return KindMeetingBooked
	}
	return ""
}

// wants reports whether a connection has anything to do with an event: a Task
// to log or a field to write back.
func wants(st Settings, kind string) bool {
	if st.Activity.Logs(kind) {
		return true
	}
	w := st.Writeback
	switch kind {
	case KindSent:
		return w.LeadStatusOnSent != ""
	case KindReplied:
		for _, v := range w.LeadStatusOnReply {
			if strings.TrimSpace(v) != "" {
				return true
			}
		}
	case KindMeetingBooked:
		return w.LeadStatusOnMeeting != ""
	case KindUnsubscribed:
		return st.Inbound.OptOut == "both" || st.Inbound.OptOut == "to_salesforce"
	}
	return false
}

// Record is the webhook dispatch sink. Best-effort and quick: an event with no
// Salesforce interest costs one cached lookup.
func (r *Recorder) Record(ctx context.Context, orgID uuid.UUID, eventType models.WebhookEventType, data any) {
	kind := kindFor(eventType)
	if kind == "" || orgID == uuid.Nil {
		return
	}
	m, ok := data.(map[string]any)
	if !ok {
		return
	}
	if b, _ := m["test"].(bool); b {
		return
	}
	// The caller's context may end with its request; the insert must not.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	conns := r.connections(wctx, orgID)
	if len(conns) == 0 {
		return
	}
	email := strings.ToLower(str(m, "contact_email", "invitee_email", "recipient", "email"))
	if email == "" {
		return
	}
	var contactID *uuid.UUID
	if id, err := uuid.Parse(str(m, "contact_id")); err == nil {
		contactID = &id
	}
	payload, content, dedupe := r.shape(kind, eventType, m, email)
	sealed := ""
	if content != nil {
		raw, _ := json.Marshal(content)
		if s, err := r.seal(wctx, orgID, string(raw)); err == nil {
			sealed = s
		}
	}
	for _, c := range conns {
		if !wants(c.settings, kind) {
			continue
		}
		a := &models.SalesforceActivity{
			OrganizationID:   orgID,
			ConnectionID:     c.id,
			ContactID:        contactID,
			ContactEmail:     email,
			Kind:             kind,
			DedupeKey:        dedupe,
			Payload:          payload,
			ContentEncrypted: sealed,
			OccurredAt:       time.Now().UTC(),
		}
		if err := r.repo.EnqueueActivity(wctx, a); err != nil {
			log.Warn().Err(err).Str("kind", kind).Msg("salesforce: could not queue activity")
		}
	}
}

// activityContent is the text half of an activity, sealed at rest.
type activityContent struct {
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

// shape picks the ids worth keeping, the text to seal, and the key that makes
// a repeat of the same event a no-op.
func (r *Recorder) shape(kind string, t models.WebhookEventType, m map[string]any, email string) (map[string]any, *activityContent, string) {
	p := map[string]any{}
	keep := func(keys ...string) {
		for _, k := range keys {
			if v := str(m, k); v != "" {
				p[strings.TrimPrefix(k, "_")] = v
			}
		}
	}
	keep("campaign_id", "sequence_id", "from_email")
	step := str(m, "campaign_id") + ":" + str(m, "sequence_id")
	day := time.Now().UTC().Format("2006-01-02")
	switch kind {
	case KindSent:
		keep("_task_id")
		c := &activityContent{Subject: str(m, "_subject"), Body: truncate(str(m, "_body_text"), descriptionCap)}
		if id := str(m, "_task_id"); id != "" {
			return p, c, "sent:" + id
		}
		return p, c, "sent:" + email + ":" + step
	case KindOpened:
		return p, nil, "opened:" + email + ":" + step
	case KindClicked:
		keep("url", "link_label")
		return p, nil, "clicked:" + email + ":" + step + ":" + str(m, "url")
	case KindReplied:
		keep("intent", "thread_id", "email_account_id", "sender_email_account_id")
		body := truncate(str(m, "_body_text"), descriptionCap)
		if body == "" {
			body = str(m, "snippet")
		}
		c := &activityContent{Subject: str(m, "subject"), Body: body}
		if id := str(m, "_message_id"); id != "" {
			return p, c, "replied:" + id
		}
		return p, c, "replied:" + email + ":" + str(m, "thread_id") + ":" + str(m, "subject") + ":" + day
	case KindBounced:
		keep("reason", "provider")
		return p, nil, "bounced:" + email + ":" + str(m, "campaign_id")
	case KindUnsubscribed:
		keep("source")
		if t == models.WebhookEventDeliverabilityComplaint {
			p["source"] = "complaint"
		}
		return p, nil, "unsubscribed:" + email
	case KindMeetingBooked:
		keep("event_name", "scheduled_for", "join_url", "booking_id", "source")
		return p, nil, "meeting:" + str(m, "booking_id", "scheduled_for") + ":" + email
	}
	return p, nil, kind + ":" + email + ":" + day
}

func (r *Recorder) seal(ctx context.Context, orgID uuid.UUID, plain string) (string, error) {
	if r.cipher == nil {
		return "", fmt.Errorf("cipher unavailable")
	}
	c, err := r.cipher.Cipher(ctx, orgID)
	if err != nil {
		return "", err
	}
	return c.Encrypt(ctx, plain)
}

// str reads the first non-empty value among keys as a string, whatever type
// the event carried it as.
func str(m map[string]any, keys ...string) string {
	for _, k := range keys {
		switch v := m[k].(type) {
		case nil:
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		case *string:
			if v != nil && strings.TrimSpace(*v) != "" {
				return strings.TrimSpace(*v)
			}
		case uuid.UUID:
			if v != uuid.Nil {
				return v.String()
			}
		case *uuid.UUID:
			if v != nil && *v != uuid.Nil {
				return v.String()
			}
		case time.Time:
			if !v.IsZero() {
				return v.UTC().Format(time.RFC3339)
			}
		case *time.Time:
			if v != nil && !v.IsZero() {
				return v.UTC().Format(time.RFC3339)
			}
		case fmt.Stringer:
			if s := strings.TrimSpace(v.String()); s != "" {
				return s
			}
		default:
			if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}
