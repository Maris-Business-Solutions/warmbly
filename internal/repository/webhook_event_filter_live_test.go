package repository

import (
	"context"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/encrypt"
)

// An omitted event filter is the documented "every non-firehose event", which
// the matcher reads as an empty array. It used to bind as NULL and fail the
// NOT NULL column, so POST /webhooks without event_types answered with the raw
// Postgres error (the same defect as issue #343 on forms).
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/warmbly_dev?sslmode=disable \
//	  go test ./internal/repository/ -run LiveWebhookEndpointEmptyEventFilter -v
func TestLiveWebhookEndpointEmptyEventFilter(t *testing.T) {
	_, pool := liveContactDB(t)
	f := newSharedOrgFixture(t, pool)
	enc, err := encrypt.NewEncrypter(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewWebhookRepositorySealed(pool, enc)
	ctx := context.Background()

	endpoint := &models.WebhookEndpoint{
		OrganizationID: f.org,
		URL:            "https://example.com/hooks/issue343",
		Description:    "no filter",
		Enabled:        true,
	}
	if err := repo.CreateEndpoint(ctx, endpoint, "secret", "token"); err != nil {
		t.Fatalf("create endpoint without event types: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM webhook_endpoints WHERE id = $1`, endpoint.ID); err != nil {
			t.Errorf("cleanup endpoint: %v", err)
		}
	})
	if endpoint.EventTypes == nil {
		t.Fatal("the created endpoint still reports a null event filter")
	}

	stored, err := repo.GetEndpoint(ctx, f.org, endpoint.ID)
	if err != nil || stored == nil {
		t.Fatalf("get endpoint: %v", err)
	}
	if len(stored.EventTypes) != 0 {
		t.Fatalf("event types: %#v", stored.EventTypes)
	}

	// The empty filter is what makes the endpoint match a standard event.
	if _, err := pool.Exec(ctx, `UPDATE webhook_endpoints SET verified_at = NOW() WHERE id = $1`, endpoint.ID); err != nil {
		t.Fatalf("verify endpoint: %v", err)
	}
	matched, err := repo.MatchingEndpoints(ctx, f.org, models.WebhookEventCampaignReplyReceived)
	if err != nil {
		t.Fatalf("matching endpoints: %v", err)
	}
	found := false
	for _, m := range matched {
		if m.ID == endpoint.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("an endpoint with no filter did not match a standard event")
	}

	// Clearing the filter on update must not reintroduce the NULL either.
	endpoint.EventTypes = nil
	if err := repo.UpdateEndpoint(ctx, endpoint); err != nil {
		t.Fatalf("update endpoint without event types: %v", err)
	}
}
