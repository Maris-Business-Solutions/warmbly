package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

func TestProviderSupportsAction(t *testing.T) {
	for _, tc := range []struct {
		provider models.IntegrationProvider
		action   models.IntegrationAction
		want     bool
	}{
		{models.IntegrationSlack, models.IntegrationActionSlackNotify, true},
		{models.IntegrationHubSpot, models.IntegrationActionHubSpotUpsert, true},
		{models.IntegrationZapier, models.IntegrationActionGenericWebhookPing, true},
		{models.IntegrationHubSpot, models.IntegrationActionSlackNotify, false},
		{models.IntegrationSlack, models.IntegrationActionGenericWebhookPing, false},
		{models.IntegrationCalendly, models.IntegrationActionDiscordNotify, false},
		{models.IntegrationProvider("unknown"), models.IntegrationActionSlackNotify, false},
	} {
		if got := models.ProviderSupportsAction(tc.provider, tc.action); got != tc.want {
			t.Errorf("ProviderSupportsAction(%s, %s) = %v, want %v", tc.provider, tc.action, got, tc.want)
		}
	}
}

// execAction refuses before it opens a connection's secrets for another provider's action.
func TestExecActionRefusesAnotherProvidersAction(t *testing.T) {
	target := repository.DispatchTarget{
		Subscription: models.IntegrationEventSubscription{Action: models.IntegrationActionSlackNotify},
		Secrets:      repository.ConnectionSecrets{Conn: models.IntegrationConnection{Provider: models.IntegrationHubSpot}},
	}
	err := (&service{}).execAction(context.Background(), target, map[string]any{})
	if !errors.Is(err, ErrActionProviderMismatch) {
		t.Fatalf("execAction = %v, want ErrActionProviderMismatch", err)
	}
}
