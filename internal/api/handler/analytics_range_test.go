package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// GET /analytics/campaigns/:id takes an optional period: both days or
// neither, each YYYY-MM-DD, from not after to (issue #702).
func TestOptionalDayRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		query    string
		wantNil  bool
		wantErr  bool
		from, to string
	}{
		{query: "", wantNil: true},
		{query: "from=2026-09-01&to=2026-09-27", from: "2026-09-01", to: "2026-09-27"},
		{query: "from=2026-09-27&to=2026-09-27", from: "2026-09-27", to: "2026-09-27"},
		{query: "from=2026-09-01", wantErr: true},
		{query: "to=2026-09-27", wantErr: true},
		{query: "from=2026-09-28&to=2026-09-27", wantErr: true},
		{query: "from=09/01/2026&to=2026-09-27", wantErr: true},
		{query: "from=2026-09-01&to=2026-09-27T10:00:00Z", wantErr: true},
	} {
		t.Run(tc.query, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/analytics/campaigns/x?"+tc.query, nil)
			period, xerr := optionalDayRange(c)
			if tc.wantErr {
				if xerr == nil {
					t.Fatalf("period = %+v, want a 400", period)
				}
				return
			}
			if xerr != nil {
				t.Fatalf("unexpected error: %v", xerr)
			}
			if tc.wantNil {
				if period != nil {
					t.Fatalf("period = %+v, want nil (all time)", period)
				}
				return
			}
			if got := period.From.Format(time.DateOnly); got != tc.from {
				t.Errorf("from = %s, want %s", got, tc.from)
			}
			if got := period.To.Format(time.DateOnly); got != tc.to {
				t.Errorf("to = %s, want %s", got, tc.to)
			}
		})
	}
}
