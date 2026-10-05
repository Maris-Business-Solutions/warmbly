package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/admin"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type invitationCleanupRepo struct {
	repository.OrganizationRepository
	called bool
	err    error
}

func (r *invitationCleanupRepo) DeleteExpiredInvitations(context.Context) error {
	r.called = true
	return r.err
}

type invitationCleanupAudit struct {
	admin.AdminService
	actor  uuid.UUID
	action string
}

func (a *invitationCleanupAudit) LogAdminAction(_ context.Context, actor uuid.UUID, action, _ string, _ *uuid.UUID, _ map[string]any, _, _ string) {
	a.actor, a.action = actor, action
}

func TestAdminDeleteExpiredInvitations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name       string
		permission models.AdminPermission
		identity   bool
		missing    bool
		err        error
		status     int
		called     bool
	}{
		{name: "success", permission: models.AdminPermManageOrganizations, identity: true, status: http.StatusOK, called: true},
		{name: "read-only admin", permission: models.AdminPermViewAnalytics, identity: true, status: http.StatusForbidden},
		{name: "missing identity", permission: models.AdminPermManageOrganizations, status: http.StatusUnauthorized},
		{name: "missing repository", permission: models.AdminPermManageOrganizations, identity: true, missing: true, status: http.StatusBadRequest},
		{name: "repository failure", permission: models.AdminPermManageOrganizations, identity: true, err: errors.New("database unavailable"), status: http.StatusInternalServerError, called: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &invitationCleanupRepo{err: tc.err}
			audit := &invitationCleanupAudit{}
			h := &Handler{OrgRepo: repo, AdminService: audit}
			if tc.missing {
				h.OrgRepo = nil
			}
			actor := uuid.New()
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(middleware.AdminPermissionsKey, tc.permission)
				if tc.identity {
					c.Set(middleware.AdminUserIDKey, actor)
				}
			})
			r.DELETE("/admin/instance/invitations/expired", middleware.RequireAdminPermission(models.AdminPermManageOrganizations), h.AdminDeleteExpiredInvitations)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/admin/instance/invitations/expired", nil))
			if w.Code != tc.status || repo.called != tc.called {
				t.Fatalf("status=%d, called=%t, body=%s", w.Code, repo.called, w.Body.String())
			}
			if tc.status == http.StatusOK {
				if audit.actor != actor || audit.action != "delete_expired_invitations" {
					t.Fatalf("cleanup not attributed in audit: %+v", audit)
				}
			} else if audit.action != "" {
				t.Fatal("failed cleanup was logged as successful")
			}
		})
	}
}
