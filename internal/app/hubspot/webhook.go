package hubspot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/models"
)

// webhookMaxAge refuses a replayed delivery, as HubSpot's v3 signature advises.
const webhookMaxAge = 5 * time.Minute

// ErrBadSignature is a delivery that is not from HubSpot (or is stale).
var ErrBadSignature = errors.New("invalid HubSpot signature")

// VerifySignature checks X-HubSpot-Signature-v3: base64(HMAC-SHA256(client
// secret, method + full URL + body + timestamp)).
func (s *Service) VerifySignature(method, fullURL string, body []byte, signature, timestamp string, now time.Time) error {
	if s.d.ClientSecret == "" || signature == "" || timestamp == "" {
		return ErrBadSignature
	}
	ms, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrBadSignature
	}
	if age := now.Sub(time.UnixMilli(ms)); age > webhookMaxAge || age < -webhookMaxAge {
		return ErrBadSignature
	}
	mac := hmac.New(sha256.New, []byte(s.d.ClientSecret))
	mac.Write([]byte(method + fullURL + string(body) + timestamp))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return ErrBadSignature
	}
	return nil
}

// VerifySignatureV2 checks the older X-HubSpot-Signature scheme some requests
// still carry: hex(SHA-256(client secret + method + full URL + body)).
func (s *Service) VerifySignatureV2(method, fullURL string, body []byte, signature string) error {
	if s.d.ClientSecret == "" || signature == "" {
		return ErrBadSignature
	}
	sum := sha256.Sum256([]byte(s.d.ClientSecret + method + fullURL + string(body)))
	if !hmac.Equal([]byte(hex.EncodeToString(sum[:])), []byte(strings.ToLower(signature))) {
		return ErrBadSignature
	}
	return nil
}

// webhookEvent is one entry of a HubSpot webhook batch.
type webhookEvent struct {
	PortalID         int64  `json:"portalId"`
	SubscriptionType string `json:"subscriptionType"`
	ObjectTypeID     string `json:"objectTypeId"`
	ObjectID         int64  `json:"objectId"`
	ChangeSource     string `json:"changeSource"`
}

// objectTypeIDs name the generic object.* subscriptions by HubSpot's type id.
var objectTypeIDs = map[string]string{"0-1": "contact", "0-3": "deal", "0-27": "task"}

// kind reads both subscription styles: "contact.propertyChange" and
// "object.propertyChange" with an objectTypeId.
func (ev webhookEvent) kind() (objectType, action string) {
	objectType, action, _ = strings.Cut(ev.SubscriptionType, ".")
	if objectType == "object" {
		objectType = objectTypeIDs[ev.ObjectTypeID]
	}
	return objectType, action
}

// HandleWebhook turns a verified delivery into refresh jobs. The record is
// re-read from HubSpot rather than trusted from the payload, so a delivery
// can only make a pull come sooner.
func (s *Service) HandleWebhook(ctx context.Context, body []byte) error {
	var events []webhookEvent
	if err := json.Unmarshal(body, &events); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if len(events) > 200 {
		events = events[:200]
	}
	orgsByPortal := map[int64][]string{}
	for _, ev := range events {
		if ev.ObjectID == 0 || ev.PortalID == 0 {
			continue
		}
		objectType, action := ev.kind()
		if objectType != "contact" && objectType != "deal" && objectType != "task" {
			continue
		}
		orgs, ok := orgsByPortal[ev.PortalID]
		if !ok {
			ids, err := s.d.Repo.OrgsForAccount(ctx, provider, strconv.FormatInt(ev.PortalID, 10))
			if err != nil {
				return err
			}
			for _, id := range ids {
				orgs = append(orgs, id.String())
			}
			orgsByPortal[ev.PortalID] = orgs
		}
		ext := strconv.FormatInt(ev.ObjectID, 10)
		for _, orgID := range orgs {
			job := &models.CRMSyncJob{
				Provider: provider, Kind: models.CRMJobRefreshObject,
				DedupeKey: "refresh:" + objectType + ":" + ext,
				Subject:   "HubSpot " + objectType + " " + ext,
				Payload:   map[string]any{"object_type": objectType, "external_id": ext, "deleted": action == "deletion"},
				// A short delay folds a burst of property changes into one read.
				NextAttemptAt: time.Now().Add(5 * time.Second),
			}
			if err := job.OrganizationID.UnmarshalText([]byte(orgID)); err != nil {
				continue
			}
			if err := s.d.Repo.EnqueueJob(ctx, job); err != nil {
				log.Warn().Err(err).Msg("hubspot: could not queue a webhook refresh")
			}
		}
	}
	return nil
}
