package aitools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/app/integration"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

type slackStub struct {
	cur     models.SlackSettings
	written *models.SlackSettings
}

func (s *slackStub) CurrentSettings(context.Context, uuid.UUID) (*models.SlackSettings, string, *errx.Error) {
	c := s.cur
	return &c, "Acme", nil
}

func (s *slackStub) UpdateSettings(_ context.Context, _ uuid.UUID, in models.SlackSettings) (*models.SlackSettings, *errx.Error) {
	s.written = &in
	return &in, nil
}

// Changing Slack setup needs Manage settings, as on the Slack panel.
func TestUpdateSlackSettingsNeedsManageSettings(t *testing.T) {
	stub := &slackStub{}
	r := NewRegistry()
	RegisterSlackTools(r, stub, nil)
	member := Invocation{OrgID: uuid.New(), UserID: uuid.New(), OrgPerms: models.PermUseAI}
	if _, err := r.Call(context.Background(), member, "update_slack_settings", json.RawMessage(`{"notification_channel":"C1"}`)); !errors.Is(err, ErrToolForbidden) {
		t.Fatalf("a member without Manage settings got %v, want ErrToolForbidden", err)
	}
	if stub.written != nil {
		t.Fatal("settings were written for a member without Manage settings")
	}
	if _, err := r.Call(context.Background(), member, "get_slack_settings", nil); err != nil {
		t.Fatalf("any member reads the Slack setup, got %v", err)
	}
}

// Only the fields passed change; the rest of the setup is kept.
func TestUpdateSlackSettingsMerges(t *testing.T) {
	stub := &slackStub{cur: models.SlackSettings{
		Channel:      "C1",
		InboxChannel: "C9",
		Routes:       map[models.NotificationCategory]string{models.NotifHealthBounce: "C2", models.NotifBillingAlert: "C3"},
	}}
	r := NewRegistry()
	RegisterSlackTools(r, stub, nil)
	admin := Invocation{OrgID: uuid.New(), UserID: uuid.New(), OrgPerms: models.PermManageSettings}
	args := `{"category_channels":{"inbound_reply":"C7","billing_alert":""},"assistant":"dm_only"}`
	if _, err := r.Call(context.Background(), admin, "update_slack_settings", json.RawMessage(args)); err != nil {
		t.Fatal(err)
	}
	w := stub.written
	if w == nil || w.Channel != "C1" || w.InboxChannel != "C9" || !w.AssistantDMOnly || w.AssistantDisabled {
		t.Fatalf("unrelated settings changed: %+v", w)
	}
	want := map[models.NotificationCategory]string{models.NotifHealthBounce: "C2", models.NotifInboundReply: "C7"}
	if len(w.Routes) != len(want) {
		t.Fatalf("routes %v, want %v", w.Routes, want)
	}
	for k, v := range want {
		if w.Routes[k] != v {
			t.Fatalf("routes %v, want %v", w.Routes, want)
		}
	}
	if stub.cur.Routes[models.NotifBillingAlert] != "C3" {
		t.Fatal("the current settings map was mutated")
	}
}

type integrationsStub struct{ integration.Service }

func (integrationsStub) ListConnections(context.Context, uuid.UUID) ([]models.IntegrationConnection, error) {
	return nil, nil
}

func (integrationsStub) Catalog(context.Context) []models.IntegrationCatalogEntry {
	return []models.IntegrationCatalogEntry{
		{Provider: models.IntegrationSlack, Name: "Slack", Configured: true},
		{Provider: models.IntegrationHubSpot, Name: "HubSpot"},
	}
}

// Integrations are visible with either permission the Integrations page accepts.
func TestListIntegrationsGate(t *testing.T) {
	r := NewRegistry()
	Deps{Automations: integrationsStub{}}.registerIntegrationTools(r)
	ctx := context.Background()
	none := Invocation{OrgID: uuid.New(), OrgPerms: models.PermUseAI}
	if _, err := r.Call(ctx, none, "list_integrations", nil); !errors.Is(err, ErrToolForbidden) {
		t.Fatalf("got %v, want ErrToolForbidden", err)
	}
	for _, p := range []models.OrganizationPermission{models.PermManageSettings, models.PermUseIntegrations} {
		if _, err := r.Call(ctx, Invocation{OrgID: uuid.New(), OrgPerms: p}, "list_integrations", nil); err != nil {
			t.Fatalf("permission %d: %v", p, err)
		}
	}
	if _, err := r.Call(ctx, Invocation{OrgID: uuid.New(), OrgPerms: models.PermUseIntegrations}, "integration_connect_link", json.RawMessage(`{"provider":"slack"}`)); !errors.Is(err, ErrToolForbidden) {
		t.Fatalf("connecting needs Manage settings, got %v", err)
	}
	admin := Invocation{OrgID: uuid.New(), OrgPerms: models.PermManageSettings}
	if _, err := r.Call(ctx, admin, "integration_connect_link", json.RawMessage(`{"provider":"slack"}`)); err != nil {
		t.Fatalf("a set-up app gets a link, got %v", err)
	}
	if _, err := r.Call(ctx, admin, "integration_connect_link", json.RawMessage(`{"provider":"hubspot"}`)); err == nil {
		t.Fatal("an app this instance has no credentials for got a connect link")
	}
}
