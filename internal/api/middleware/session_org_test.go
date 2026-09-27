package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/app/organization"
	"github.com/warmbly/warmbly/internal/app/token"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

type sessionTokens struct {
	token.TokenService
	session *models.Session
}

func (f sessionTokens) ValidateAccessToken(context.Context, string) (*models.Session, *errx.Error) {
	return f.session, nil
}

type membershipOrgs struct {
	organization.OrganizationService
	members        map[uuid.UUID]*models.OrganizationMember
	permissionHits int
}

func (f *membershipOrgs) GetMembership(_ context.Context, orgID, userID uuid.UUID) (*models.OrganizationMember, *errx.Error) {
	m := f.members[orgID]
	if m == nil || m.UserID != userID {
		return nil, nil
	}
	return m, nil
}

func (f *membershipOrgs) HasPermission(_ context.Context, orgID, userID uuid.UUID, perm models.OrganizationPermission) (bool, *errx.Error) {
	f.permissionHits++
	m, _ := f.GetMembership(context.Background(), orgID, userID)
	return m != nil && m.HasPermission(perm), nil
}

func (f *membershipOrgs) Get(_ context.Context, orgID uuid.UUID) (*models.Organization, *errx.Error) {
	return &models.Organization{ID: orgID}, nil
}

func serveSession(t *testing.T, h *Handler, auth gin.HandlerFunc, gates ...gin.HandlerFunc) (int, *uuid.UUID) {
	t.Helper()
	var seen *uuid.UUID
	r := gin.New()
	r.Use(RequestIDMiddleware())
	chain := append([]gin.HandlerFunc{auth}, gates...)
	chain = append(chain, func(c *gin.Context) {
		seen = GetOrganizationID(c)
		c.Status(http.StatusOK)
	})
	r.GET("/x", chain...)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code, seen
}

func TestSessionOrganizationRequiresMembership(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	session := &models.Session{ID: uuid.New(), UserID: userID, CurrentOrganizationID: &orgID}

	member := &models.OrganizationMember{OrganizationID: orgID, UserID: userID, Permissions: models.PermViewContacts}
	for _, tt := range []struct {
		name     string
		members  map[uuid.UUID]*models.OrganizationMember
		wantCode int
		wantOrg  bool
	}{
		{"member keeps the workspace", map[uuid.UUID]*models.OrganizationMember{orgID: member}, http.StatusOK, true},
		{"former member has none", map[uuid.UUID]*models.OrganizationMember{}, http.StatusBadRequest, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			orgs := &membershipOrgs{members: tt.members}
			h := &Handler{TokenService: sessionTokens{session: session}, OrganizationService: orgs}

			for name, auth := range map[string]gin.HandlerFunc{
				"AuthMiddleware":         h.AuthMiddleware(),
				"CombinedAuthMiddleware": h.CombinedAuthMiddleware(),
			} {
				code, seen := serveSession(t, h, auth, h.RequireOrganization())
				if code != tt.wantCode {
					t.Fatalf("%s: status = %d, want %d", name, code, tt.wantCode)
				}
				if (seen != nil) != tt.wantOrg {
					t.Fatalf("%s: organization in request = %v, want present=%v", name, seen, tt.wantOrg)
				}
			}
		})
	}
}

func TestSessionMemberAnswersPermissionGates(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	session := &models.Session{ID: uuid.New(), UserID: userID, CurrentOrganizationID: &orgID}
	orgs := &membershipOrgs{members: map[uuid.UUID]*models.OrganizationMember{
		orgID: {OrganizationID: orgID, UserID: userID, Permissions: models.PermViewContacts},
	}}
	h := &Handler{TokenService: sessionTokens{session: session}, OrganizationService: orgs}

	if code, _ := serveSession(t, h, h.AuthMiddleware(), h.RequirePermission(models.PermViewContacts)); code != http.StatusOK {
		t.Fatalf("granted permission: status = %d, want 200", code)
	}
	if code, _ := serveSession(t, h, h.AuthMiddleware(), h.RequirePermission(models.PermManageTeam)); code != http.StatusForbidden {
		t.Fatalf("missing permission: status = %d, want 403", code)
	}
	if orgs.permissionHits != 0 {
		t.Fatalf("permission lookups = %d, want 0 (answered from the session's membership)", orgs.permissionHits)
	}
}
