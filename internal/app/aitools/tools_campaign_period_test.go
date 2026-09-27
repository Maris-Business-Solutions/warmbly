package aitools

import (
	"errors"
	"testing"
)

func TestParseDayRangeArgs(t *testing.T) {
	if p, err := parseDayRangeArgs("", ""); err != nil || p != nil {
		t.Fatalf("empty = %+v, %v; want all time", p, err)
	}
	p, err := parseDayRangeArgs("2026-09-01", "2026-09-07")
	if err != nil || p == nil || p.From.Day() != 1 || p.To.Day() != 7 {
		t.Fatalf("range = %+v, %v; want Sep 1 to Sep 7", p, err)
	}
	for _, bad := range [][2]string{{"2026-09-01", ""}, {"", "2026-09-07"}, {"2026-09-08", "2026-09-07"}, {"yesterday", "2026-09-07"}} {
		if _, err := parseDayRangeArgs(bad[0], bad[1]); !errors.Is(err, ErrInvalidArgs) {
			t.Errorf("%v: err = %v, want ErrInvalidArgs", bad, err)
		}
	}
}
