package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// oauthCaller stands in for setOAuthCaller with a token granting scopes for member.
func oauthCaller(orgID uuid.UUID, member *models.OrganizationMember, scopes uint64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(AuthTypeKey, AuthTypeOAuth)
		c.Set(APIKeyPermissionsKey, scopes)
		c.Set(UserIDKey, member.UserID.String())
		c.Set(OrganizationIDKey, orgID)
		c.Set(SessionMemberKey, member)
		c.Next()
	}
}

func TestOAuthTokenActsWithinItsMembersRole(t *testing.T) {
	orgID := uuid.New()
	viewer := &models.OrganizationMember{OrganizationID: orgID, UserID: uuid.New(), Role: "member", Permissions: models.GetRolePermissions(models.RoleViewer)}
	owner := &models.OrganizationMember{OrganizationID: orgID, UserID: uuid.New(), Role: string(models.RoleOwner)}
	h := &Handler{OrganizationService: &membershipOrgs{}}

	tests := []struct {
		name   string
		member *models.OrganizationMember
		gate   gin.HandlerFunc
		want   int
	}{
		{"scope and role both cover the route", viewer, h.RequireAccess(models.PermViewContacts, models.APIPermReadContacts), http.StatusOK},
		{"scope without the role is refused", viewer, h.RequireAccess(models.PermManageSettings, models.APIPermReadContacts), http.StatusForbidden},
		{"any-of gate needs one of the member's permissions", viewer, h.RequireAnyAccess(models.APIPermReadContacts, models.PermManageSettings, models.PermUseIntegrations), http.StatusForbidden},
		{"the owner holds every permission", owner, h.RequireAccess(models.PermManageSettings, models.APIPermReadContacts), http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(RequestIDMiddleware())
			r.GET("/x", oauthCaller(orgID, tt.member, models.APIPermReadContacts), tt.gate, func(c *gin.Context) { c.Status(http.StatusOK) })
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil))
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

func TestRefuseOAuthKeepsAppTokensOffCredentialRoutes(t *testing.T) {
	for _, tt := range []struct {
		authType string
		want     int
	}{
		{AuthTypeOAuth, http.StatusForbidden},
		{AuthTypeAPIKey, http.StatusOK},
		{AuthTypeJWT, http.StatusOK},
	} {
		r := gin.New()
		r.Use(RequestIDMiddleware())
		r.POST("/api-keys", func(c *gin.Context) { c.Set(AuthTypeKey, tt.authType); c.Next() }, RefuseOAuth(), func(c *gin.Context) { c.Status(http.StatusOK) })
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api-keys", nil))
		if rec.Code != tt.want {
			t.Errorf("%s: status = %d, want %d", tt.authType, rec.Code, tt.want)
		}
	}
}
