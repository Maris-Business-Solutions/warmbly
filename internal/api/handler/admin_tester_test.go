package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/organization"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type testerUsers struct {
	repository.UserRepository
	userID uuid.UUID
	until  time.Time
}

func (r *testerUsers) GetUserByEmail(context.Context, string) (*models.User, error) { return nil, nil }

func (r *testerUsers) CreateExemptUser(_ context.Context, email *mail.Address, _, _ string, _ *uuid.UUID, until time.Time) (*models.User, error) {
	r.until = until
	return &models.User{ID: r.userID, Email: email.Address}, nil
}

type testerWorkspaces struct {
	organization.OrganizationService
	orgID, actor, roleID uuid.UUID
	until                time.Time
	created, joined      bool
}

func (s *testerWorkspaces) CreateTesterWorkspace(_ context.Context, _, actor uuid.UUID, _, _ string, until time.Time) (*models.Organization, *errx.Error) {
	s.created, s.actor, s.until = true, actor, until
	return &models.Organization{ID: s.orgID, Category: models.OrganizationCategoryTest}, nil
}

func (s *testerWorkspaces) AttachTester(_ context.Context, orgID, _, actor, roleID uuid.UUID) (*models.OrganizationMember, *errx.Error) {
	s.joined, s.orgID, s.actor, s.roleID = true, orgID, actor, roleID
	return &models.OrganizationMember{OrganizationID: orgID}, nil
}

func TestAdminCreateTesterOnlyGrantsDedicatedWorkspaces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, existing := range []bool{false, true} {
		actor, orgID, roleID := uuid.New(), uuid.New(), uuid.New()
		users := &testerUsers{userID: uuid.New()}
		orgs := &testerWorkspaces{orgID: orgID}
		h := &Handler{UserRepo: users, OrganizationService: orgs}
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set(middleware.AdminUserIDKey, actor) })
		r.POST("/testers", h.AdminCreateTester)
		req := adminCreateTesterRequest{Email: "reviewer@example.test", Reason: "OAuth review", PasswordDays: 7}
		if existing {
			req.OrgID, req.RoleID = &orgID, &roleID
		}
		raw, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/testers", strings.NewReader(string(raw))))
		if w.Code != http.StatusOK {
			t.Fatalf("create tester existing=%v: HTTP %d", existing, w.Code)
		}
		var response adminCreateTesterResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Joined != existing || orgs.created == existing || orgs.joined != existing || orgs.actor != actor {
			t.Fatalf("wrong provisioning path for existing=%v", existing)
		}
		if existing && orgs.roleID != roleID {
			t.Fatal("existing workspace role changed")
		}
		if !existing && !orgs.until.Equal(users.until) {
			t.Fatal("test entitlement did not use the reviewer password expiry")
		}
		if !response.PasswordExpiresAt.Equal(users.until) || !strings.HasPrefix(response.Password, "Tester-") {
			t.Fatal("reviewer credentials response is incomplete")
		}
	}
}
