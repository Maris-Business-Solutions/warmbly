package releases

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

type memSettings struct{ release *models.FleetReleaseState }

func (m *memSettings) GetRelease(context.Context) (*models.FleetReleaseState, error) {
	return m.release, nil
}

func (m *memSettings) SetRelease(_ context.Context, s *models.FleetReleaseState) error {
	m.release = s
	return nil
}

func (m *memSettings) GetJoinTokenHash(context.Context) (string, error) { return "", nil }
func (m *memSettings) SetJoinTokenHash(context.Context, string) error   { return nil }

type githubStub struct{}

func (githubStub) RoundTrip(*http.Request) (*http.Response, error) {
	body := `[{"tag_name":"v2.0.0","published_at":"2026-09-29T12:00:00Z"}]`
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
}

func TestCheckGitHubHoldsTheFleetWhenTheSchemaGateRefuses(t *testing.T) {
	for _, tc := range []struct {
		name string
		gate func(context.Context, string) error
		want string
	}{
		{"refused", func(context.Context, string) error { return errors.New("incompatible") }, "v1.0.0"},
		{"accepted", func(context.Context, string) error { return nil }, "v2.0.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := &memSettings{release: &models.FleetReleaseState{Channel: models.FleetChannelStable, Tag: "v1.0.0"}}
			svc := New(Config{
				Enabled:    true,
				GithubRepo: "warmbly/warmbly",
				HTTPClient: &http.Client{Transport: githubStub{}},
				SchemaGate: tc.gate,
			}, settings)
			if _, err := svc.CheckGitHub(context.Background()); err != nil {
				t.Fatal(err)
			}
			if settings.release.Tag != tc.want {
				t.Fatalf("fleet target %s, want %s", settings.release.Tag, tc.want)
			}
		})
	}
}
