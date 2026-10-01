package scheduler

import (
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/bitmask"
)

// A Monday-to-Friday warmup skips the weekend and resumes on Monday.
func TestFindNextValidDayReadsTheDashboardMask(t *testing.T) {
	mask := bitmask.DefaultDays()
	saturday := time.Date(2026, time.October, 3, 9, 0, 0, 0, time.UTC)
	if got := findNextValidDay(saturday, mask, time.UTC); got.Weekday() != time.Monday {
		t.Fatalf("from Saturday got %s, want Monday", got.Weekday())
	}
	friday := time.Date(2026, time.October, 2, 9, 0, 0, 0, time.UTC)
	if got := findNextValidDay(friday, mask, time.UTC); !got.Equal(friday) {
		t.Fatalf("Friday moved to %s; it is a sending day", got.Weekday())
	}
}
