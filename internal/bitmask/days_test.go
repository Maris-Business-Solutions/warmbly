package bitmask

import (
	"testing"
	"time"
)

// The dashboard writes Monday as bit 0, so Monday to Friday is never Sunday.
func TestHasWeekdayIsMondayFirst(t *testing.T) {
	weekdays := DefaultDays()
	for wd, want := range map[time.Weekday]bool{
		time.Monday: true, time.Friday: true, time.Saturday: false, time.Sunday: false,
	} {
		if got := HasWeekday(weekdays, wd); got != want {
			t.Errorf("HasWeekday(Mon-Fri, %s) = %v, want %v", wd, got, want)
		}
	}
	if !HasWeekday(DaysToMask([]string{"sunday"}), time.Sunday) {
		t.Error("a Sunday-only mask must include Sunday")
	}
}
