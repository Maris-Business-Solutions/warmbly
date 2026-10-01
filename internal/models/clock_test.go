package models

import "testing"

func TestClockMinutes(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"09:00", 9 * 60},
		{"09:00:00", 9 * 60},
		// What pgx hands back for a `time` column. Rejecting this silently
		// disabled every campaign sending window and every warmup window.
		{"09:00:00.000000", 9 * 60},
		{"17:30:00.000000", 17*60 + 30},
		{"23:59:59.999999", 23*60 + 59},
		{"00:00:00.000000", 0},
		{" 08:15 ", 8*60 + 15},
		{"", -1},
		{"nonsense", -1},
		{"25:00", -1},
		{"12:60", -1},
		{"12", -1},
		{"-1:00", -1},
	}
	for _, tt := range tests {
		// A distinctive fallback, so "parsed as zero" and "fell back" are
		// distinguishable.
		if got := ClockMinutes(tt.in, -1); got != tt.want {
			t.Errorf("ClockMinutes(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestClockHHMM(t *testing.T) {
	tests := map[string]string{
		"08:00:00.000000": "08:00",
		"17:56:00":        "17:56",
		"9:05":            "09:05",
		"":                "",
		"25:00":           "",
		"noon":            "",
	}
	for in, want := range tests {
		if got := ClockHHMM(in); got != want {
			t.Errorf("ClockHHMM(%q) = %q, want %q", in, got, want)
		}
	}
}
