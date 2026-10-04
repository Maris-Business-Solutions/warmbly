package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type labelRepo struct {
	repository.IntegrationRepository
	byLabel map[string]string // label -> external account id
}

func (r labelRepo) GetConnection(_ context.Context, _ uuid.UUID, _ models.IntegrationProvider, label string) (*models.IntegrationConnection, error) {
	id, ok := r.byLabel[label]
	if !ok {
		return nil, nil
	}
	return &models.IntegrationConnection{Label: label, ExternalAccountID: id}, nil
}

// A second account never lands on a label another account already holds.
func TestAccountLabel(t *testing.T) {
	cases := []struct {
		name    string
		byLabel map[string]string
		account extAccount
		want    string
	}{
		{"first connection", map[string]string{}, extAccount{ID: "T1", Name: "Acme"}, "slack"},
		{"same account reconnects", map[string]string{"slack": "T1"}, extAccount{ID: "T1", Name: "Acme"}, "slack"},
		{"another account", map[string]string{"slack": "T1"}, extAccount{ID: "T2", Name: "Beta"}, "slack (Beta)"},
		{"another account, same name", map[string]string{"slack": "T1", "slack (Acme)": "T2"}, extAccount{ID: "T3", Name: "Acme"}, "slack (Acme) 2"},
		{"that account again", map[string]string{"slack": "T1", "slack (Acme)": "T2", "slack (Acme) 2": "T3"}, extAccount{ID: "T3", Name: "Acme"}, "slack (Acme) 2"},
	}
	for _, tc := range cases {
		s := &service{repo: labelRepo{byLabel: tc.byLabel}}
		if got := s.accountLabel(context.Background(), uuid.New(), models.IntegrationSlack, "slack", tc.account); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
