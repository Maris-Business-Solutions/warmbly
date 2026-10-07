package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/poollink"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

type instanceReportService struct {
	poollink.Service
	calls   int
	failure *errx.Error
}

func (s *instanceReportService) AuthenticateInstance(_ context.Context, token, _ string) (*models.PoolLinkInstance, *errx.Error) {
	if token != "instance-test-token" {
		return nil, errx.ErrUnauthorized
	}
	return &models.PoolLinkInstance{ID: uuid.New()}, nil
}
func (s *instanceReportService) WarmupStats(_ context.Context, inst *models.PoolLinkInstance, req models.PoolLinkWarmupReportRequest) ([]models.WarmupDailyStats, *errx.Error) {
	s.calls++
	if inst == nil {
		panic("no authenticated instance")
	}
	if s.failure != nil {
		return nil, s.failure
	}
	if _, _, err := req.Range(); err != nil {
		return nil, errx.New(errx.BadRequest, err.Error())
	}
	return []models.WarmupDailyStats{{Date: req.From, EmailsSent: 10, Active: true}}, nil
}
func (s *instanceReportService) WarmupPlacementData(_ context.Context, _ *models.PoolLinkInstance, req models.PoolLinkWarmupReportRequest) (*models.WarmupPlacementData, *errx.Error) {
	s.calls++
	if s.failure != nil {
		return nil, s.failure
	}
	if _, _, err := req.Range(); err != nil {
		return nil, errx.New(errx.BadRequest, err.Error())
	}
	return &models.WarmupPlacementData{}, nil
}

func TestInstanceReportHandlersRequireInstanceTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, placement := range []bool{false, true} {
		svc := &instanceReportService{}
		h := &Handler{PoolLinkService: svc}
		m := &middleware.Handler{PoolLinkService: svc}
		router := gin.New()
		handler := h.PoolLinkWarmupStats
		if placement {
			handler = h.PoolLinkWarmupPlacement
		}
		router.POST("/report", m.PoolLinkAuthMiddleware(), handler)
		for _, tc := range []struct {
			auth, body string
			status     int
		}{
			{"", "{}", 401},
			{"Bearer ordinary-api-key", "{}", 401},
			{"Bearer revoked-instance-token", "{}", 401},
			{"Bearer instance-test-token", "not JSON", 400},
			{"Bearer instance-test-token", "{}", 400},
			{"Bearer instance-test-token", `{"remote_ids":["` + uuid.NewString() + `"],"from":"2026-10-01","to":"2026-10-02"}`, 200},
		} {
			before := svc.calls
			req := httptest.NewRequest(http.MethodPost, "/report", strings.NewReader(tc.body))
			req.Header.Set("Authorization", tc.auth)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("placement=%v status=%d wanted=%d body=%s", placement, response.Code, tc.status, response.Body.String())
			}
			if tc.status == 401 && svc.calls != before {
				t.Fatal("unauthorized call reached reports")
			}
			if !placement && tc.status == 200 && !strings.Contains(response.Body.String(), `"active":true`) {
				t.Fatal("active day was not transported")
			}
		}
		svc.failure = errx.ErrServiceDown
		req := httptest.NewRequest(http.MethodPost, "/report", strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer instance-test-token")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("failure returned %d", response.Code)
		}
	}
}
