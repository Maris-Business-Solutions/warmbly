package oauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// capRepo is the slice of the repository the consent and token paths touch.
type capRepo struct {
	repository.OAuthRepository
	app      *models.OAuthApplication
	code     *models.OAuthAuthorizationCode
	grant    *models.OAuthAccessGrant
	rotated  bool
	revoked  bool
	featured bool
}

func (r *capRepo) GetApplicationByClientID(context.Context, string) (*models.OAuthApplication, error) {
	return r.app, nil
}

func (r *capRepo) CreateAuthorizationCode(_ context.Context, c *models.OAuthAuthorizationCode) error {
	r.code = c
	return nil
}

func (r *capRepo) IsFeatured(context.Context, uuid.UUID) (bool, error) { return r.featured, nil }

func (r *capRepo) GetGrantByAccessTokenHash(context.Context, string) (*models.OAuthAccessGrant, error) {
	return r.grant, nil
}

func (r *capRepo) GetGrantByRefreshTokenHash(context.Context, string) (*models.OAuthAccessGrant, error) {
	return r.grant, nil
}

func (r *capRepo) RotateGrantTokens(context.Context, uuid.UUID, string, string, string, time.Time, *time.Time) (bool, error) {
	return r.rotated, nil
}

func (r *capRepo) RevokeGrantByPreviousRefresh(context.Context, uuid.UUID, string) (bool, error) {
	return false, nil
}

func (r *capRepo) RevokeGrant(context.Context, uuid.UUID) error {
	r.revoked = true
	return nil
}

const capRedirect = "https://app.example/callback"

func capApp(scopes uint64) *models.OAuthApplication {
	return &models.OAuthApplication{
		ID: uuid.New(), Name: "Acme", ClientID: "wmcid_test", Scopes: scopes, Status: models.OAuthAppActive,
		IsPublic: true, RedirectURIs: []string{capRedirect}, WebsiteURL: "javascript:alert(1)",
	}
}

func capRequest(scope string) AuthorizeRequest {
	return AuthorizeRequest{ResponseType: "code", ClientID: "wmcid_test", RedirectURI: capRedirect, Scope: scope,
		CodeChallenge: "challenge", CodeChallengeMethod: "S256"}
}

func viewerCap() uint64 {
	return models.APIPermissionsForMember(models.GetRolePermissions(models.RoleViewer), false)
}

func TestConsentGrantsOnlyWhatTheMembersRoleCovers(t *testing.T) {
	repo := &capRepo{app: capApp(models.AllAPIPermissionsMask)}
	s := NewService(repo, nil)
	req := capRequest("read_contacts write_contacts send_campaigns api_keys")

	info, err := s.AuthorizeDetails(context.Background(), viewerCap(), req)
	if err != nil {
		t.Fatalf("AuthorizeDetails: %v", err)
	}
	if len(info.Scopes) != 1 || info.Scopes[0] != "read_contacts" {
		t.Errorf("scopes = %v, want [read_contacts]", info.Scopes)
	}
	if len(info.WithheldScopes) != 3 {
		t.Errorf("withheld = %v, want write_contacts, send_campaigns and api_keys", info.WithheldScopes)
	}
	if info.WebsiteURL != "" {
		t.Errorf("website %q was not re-validated", info.WebsiteURL)
	}

	if _, err := s.IssueAuthorizationCode(context.Background(), uuid.New(), uuid.New(), viewerCap(), req); err != nil {
		t.Fatalf("IssueAuthorizationCode: %v", err)
	}
	if repo.code.Scopes != models.APIPermReadContacts {
		t.Errorf("code scopes = %b, want only read_contacts", repo.code.Scopes)
	}
}

func TestConsentNeverGrantsAPIKeysEvenToTheOwner(t *testing.T) {
	repo := &capRepo{app: capApp(models.AllAPIPermissionsMask)}
	s := NewService(repo, nil)
	if _, err := s.IssueAuthorizationCode(context.Background(), uuid.New(), uuid.New(), models.AllAPIPermissionsMask, capRequest("")); err != nil {
		t.Fatalf("IssueAuthorizationCode: %v", err)
	}
	if repo.code.Scopes&models.APIPermAPIKeys != 0 {
		t.Error("an app was granted api_keys")
	}
}

func TestConsentRefusesWhenNothingIsGrantable(t *testing.T) {
	repo := &capRepo{app: capApp(models.AllAPIPermissionsMask)}
	s := NewService(repo, nil)
	_, err := s.IssueAuthorizationCode(context.Background(), uuid.New(), uuid.New(), viewerCap(), capRequest("write_campaigns"))
	var oe *OAuthError
	if !errors.As(err, &oe) || oe.Code != "access_denied" {
		t.Fatalf("err = %v, want access_denied", err)
	}
	if repo.code != nil {
		t.Error("a code was stored")
	}
}

func TestConsentMarksOnlyFeaturedRegisteredAppsVerified(t *testing.T) {
	repo := &capRepo{app: capApp(models.APIPermReadContacts), featured: true}
	s := NewService(repo, nil)
	info, err := s.AuthorizeDetails(context.Background(), viewerCap(), capRequest(""))
	if err != nil || !info.Verified {
		t.Fatalf("featured app: verified = %v, err = %v", info != nil && info.Verified, err)
	}
	repo.app.DynamicallyRegistered = true
	info, err = s.AuthorizeDetails(context.Background(), viewerCap(), capRequest(""))
	if err != nil || info.Verified {
		t.Fatalf("self-registered app: verified = %v, err = %v", info != nil && info.Verified, err)
	}
}

func TestAccessTokenFollowsTheMembersCurrentRole(t *testing.T) {
	viewer := &models.OrganizationMember{Role: "member", Permissions: models.GetRolePermissions(models.RoleViewer)}
	repo := &capRepo{grant: &models.OAuthAccessGrant{
		ID: uuid.New(), Scopes: models.APIPermReadContacts | models.APIPermWriteContacts,
		AccessExpiresAt: time.Now().Add(time.Hour), Holder: viewer,
	}}
	s := NewService(repo, nil)
	claims, err := s.ValidateAccessToken(context.Background(), models.OAuthAccessTokenPrefix+"x")
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.Scopes != models.APIPermReadContacts {
		t.Errorf("scopes = %b, want only read_contacts", claims.Scopes)
	}

	repo.grant.Holder = &models.OrganizationMember{Role: string(models.RoleOwner)}
	claims, err = s.ValidateAccessToken(context.Background(), models.OAuthAccessTokenPrefix+"x")
	if err != nil || claims.Scopes != repo.grant.Scopes {
		t.Fatalf("owner scopes = %b, err = %v", claims.Scopes, err)
	}
}

func TestRefreshTokenPresentedTwiceEndsTheGrant(t *testing.T) {
	app := capApp(models.APIPermReadContacts)
	repo := &capRepo{app: app, grant: &models.OAuthAccessGrant{
		ID: uuid.New(), ApplicationID: app.ID, Scopes: models.APIPermReadContacts,
		Holder: &models.OrganizationMember{Role: string(models.RoleOwner)},
	}}
	s := NewService(repo, nil)
	_, err := s.RefreshToken(context.Background(), app.ClientID, "", "wmrt_x")
	var oe *OAuthError
	if !errors.As(err, &oe) || oe.Code != "invalid_grant" {
		t.Fatalf("err = %v, want invalid_grant", err)
	}
	if !repo.revoked {
		t.Error("the grant was not revoked")
	}

	repo.rotated, repo.revoked = true, false
	resp, err := s.RefreshToken(context.Background(), app.ClientID, "", "wmrt_x")
	if err != nil || resp.Scope != "read_contacts" || repo.revoked {
		t.Fatalf("rotation: resp = %+v, err = %v, revoked = %v", resp, err, repo.revoked)
	}
}
