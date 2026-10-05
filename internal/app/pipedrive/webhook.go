package pipedrive

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/models"
)

// Pipedrive signs nothing: a delivery is authenticated by HTTP Basic auth,
// with a password derived per connection from the app's client secret, and a
// delivery only queues a re-read, so at most it makes a pull come sooner.

const hookUser = "warmbly"

// hookObjects are the records whose changes Warmbly mirrors.
var hookObjects = []string{"deal", "person", "activity", "note", "stage", "pipeline"}

// WebhookPath is where Pipedrive delivers a connection's webhooks.
const WebhookPath = "/api/v1/integrations/pipedrive/webhooks/"

func (s *Service) hookURL(connID uuid.UUID) string {
	base := strings.TrimRight(s.d.PublicURL, "/")
	if !strings.HasPrefix(base, "https://") || s.d.ClientSecret == "" {
		return ""
	}
	return base + WebhookPath + connID.String()
}

func (s *Service) hookPassword(connID uuid.UUID) string {
	mac := hmac.New(sha256.New, []byte(s.d.ClientSecret))
	mac.Write([]byte("pipedrive-webhook:" + connID.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhook checks a delivery's Basic auth against the connection's password.
func (s *Service) VerifyWebhook(connID uuid.UUID, user, password string) bool {
	if s.d.ClientSecret == "" || user != hookUser || password == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(s.hookPassword(connID))) == 1
}

func hooksKey(orgID uuid.UUID) string { return "pipedrive:hooks:" + orgID.String() }

// hooksLive reports a workspace whose webhooks were registered recently.
func (s *Service) hooksLive(ctx context.Context, orgID uuid.UUID) bool {
	return s.marked(ctx, hooksKey(orgID))
}

// ensureWebhooks registers the webhooks Warmbly needs that are missing.
func (s *Service) ensureWebhooks(ctx context.Context, o *org) {
	url := s.hookURL(o.ConnID)
	if url == "" {
		_ = s.d.Repo.SetCursor(ctx, o.ID, provider, "webhooks", nil,
			"Instant updates need this instance reachable over HTTPS. Warmbly checks Pipedrive every few minutes instead.")
		return
	}
	existing, err := o.Client.Webhooks(ctx)
	if err != nil {
		s.hooksFailed(ctx, o, err)
		return
	}
	have := map[string]bool{}
	for _, w := range existing {
		if w.SubscriptionURL != url {
			continue
		}
		if !webhookActive(w.IsActive) {
			// Pipedrive switches a hook off after repeated failed deliveries
			// (a rotated client secret changes its password): start it over.
			_ = o.Client.DeleteWebhook(ctx, w.ID)
			continue
		}
		have[w.EventObject] = true
	}
	for _, obj := range hookObjects {
		if have[obj] {
			continue
		}
		if err := o.Client.CreateWebhook(ctx, map[string]any{
			"subscription_url":   url,
			"event_action":       "*",
			"event_object":       obj,
			"name":               "Warmbly " + obj + " sync",
			"http_auth_user":     hookUser,
			"http_auth_password": s.hookPassword(o.ConnID),
			"version":            "2.0",
		}); err != nil {
			s.hooksFailed(ctx, o, err)
			return
		}
	}
	s.mark(ctx, hooksKey(o.ID), metaRefreshEvery+time.Hour)
	_ = s.d.Repo.SetCursor(ctx, o.ID, provider, "webhooks", ptrNow(), "")
}

func (s *Service) hooksFailed(ctx context.Context, o *org, err error) {
	msg := s.userError(ctx, o, err).Message
	if ae, ok := AsAPIError(err); ok && ae.Status == 403 {
		msg = "Pipedrive did not let Warmbly add webhooks for the connected user. Warmbly checks Pipedrive every few minutes instead."
	}
	if s.d.Cache != nil {
		s.d.Cache.Del(ctx, hooksKey(o.ID))
	}
	_ = s.d.Repo.SetCursor(ctx, o.ID, provider, "webhooks", nil, msg)
}

// webhookActive reads is_active, which Pipedrive sends as a bool or 0/1.
func webhookActive(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case nil:
		return true
	}
	return true
}

// Left removes Warmbly's webhooks from the company the workspace stopped using.
func (s *Service) Left(ctx context.Context, orgID, connID uuid.UUID) {
	if s.d.Cache != nil {
		s.d.Cache.Del(ctx, hooksKey(orgID))
	}
	url := s.hookURL(connID)
	if url == "" {
		return
	}
	conn, err := s.d.Tokens.GetConnection(ctx, orgID, connID)
	if err != nil || conn == nil {
		return
	}
	o := s.orgFor(ctx, orgID, conn, nil)
	hooks, err := o.Client.Webhooks(ctx)
	if err != nil {
		return
	}
	for _, w := range hooks {
		if w.SubscriptionURL == url {
			if err := o.Client.DeleteWebhook(ctx, w.ID); err != nil {
				log.Info().Err(err).Str("org_id", orgID.String()).Msg("pipedrive: could not remove a webhook")
			}
		}
	}
}

// webhookEvent reads both payload versions: 2.0 ({meta: {action, entity,
// entity_id}}) and 1.0 ({meta: {action, object, id}}).
type webhookEvent struct {
	Meta struct {
		Action   string          `json:"action"`
		Entity   string          `json:"entity"`
		EntityID json.RawMessage `json:"entity_id"`
		Object   string          `json:"object"`
		ID       json.RawMessage `json:"id"`
	} `json:"meta"`
}

func rawID(raw json.RawMessage) string {
	v := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if extID(v) == 0 {
		return ""
	}
	return v
}

// HandleWebhook turns a verified delivery into a refresh job. ok is false when
// no workspace runs its CRM on the connection any more, so Pipedrive can
// retire the webhook.
func (s *Service) HandleWebhook(ctx context.Context, connID uuid.UUID, body []byte) (bool, error) {
	orgID, err := s.d.Repo.OrgForConnection(ctx, provider, connID)
	if err != nil {
		return true, err
	}
	if orgID == uuid.Nil {
		return false, nil
	}
	var ev webhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return true, fmt.Errorf("decode: %w", err)
	}
	entity := firstNonEmpty(ev.Meta.Entity, ev.Meta.Object)
	ext := rawID(ev.Meta.EntityID)
	if ext == "" {
		ext = rawID(ev.Meta.ID)
	}
	objectType := entity
	switch entity {
	case "deal", "person", "activity", "note":
		if ext == "" {
			return true, nil
		}
	case "stage", "pipeline":
		objectType, ext = "pipelines", ""
	default:
		return true, nil
	}
	job := &models.CRMSyncJob{
		OrganizationID: orgID, Provider: provider, Kind: models.CRMJobRefreshObject,
		DedupeKey: "refresh:" + objectType + ":" + ext,
		Subject:   strings.TrimSpace("Pipedrive " + objectType + " " + ext),
		Payload:   map[string]any{"object_type": objectType, "external_id": ext},
		// A short delay folds a burst of edits to one record into one read.
		NextAttemptAt: time.Now().Add(5 * time.Second),
	}
	if err := s.d.Repo.EnqueueJob(ctx, job); err != nil {
		return true, err
	}
	return true, nil
}
