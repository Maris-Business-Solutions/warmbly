package models

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestWarmupReportRangeIsBounded(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		count    int
		valid    bool
	}{
		{"2026-01-01", "2026-01-01", 1, true},
		{"2024-01-01", "2024-12-31", 100, true},
		{"2024-01-01", "2025-01-01", 1, false},
		{"2026-01-02", "2026-01-01", 1, false},
		{"", "", 1, false},
		{"2026-01-01", "", 1, false},
		{"invalid", "2026-01-01", 1, false},
		{"2026-01-01", "2026-01-01", 0, false},
		{"2026-01-01", "2026-01-01", 101, false},
	} {
		req := PoolLinkWarmupReportRequest{RemoteIDs: make([]uuid.UUID, tc.count), From: tc.from, To: tc.to}
		from, to, err := req.Range()
		if (err == nil) != tc.valid || (err == nil && (from.Location().String() != "UTC" || to.Before(from))) {
			t.Fatalf("%+v: from=%v to=%v error=%v", tc, from, to, err)
		}
	}
}

func TestMergeWarmupStatsPreservesActiveDaysAcrossTransport(t *testing.T) {
	raw, err := json.Marshal([]WarmupDailyStats{{Date: "2026-10-01", EmailsSent: 9, TargetVolume: 10, Active: true}})
	if err != nil {
		t.Fatal(err)
	}
	var cloud []WarmupDailyStats
	if err := json.Unmarshal(raw, &cloud); err != nil {
		t.Fatal(err)
	}
	out := MergeWarmupStats([]WarmupDailyStats{
		{Date: "2026-10-02", EmailsReceived: 2},
		{Date: "2026-10-01", EmailsSent: 1, EmailsReplied: 1, TargetVolume: 2, Active: true},
	}, cloud)
	if len(out) != 2 || out[0].Date != "2026-10-01" || out[0].EmailsSent != 10 || out[0].TargetVolume != 12 || !out[0].Active || out[1].Active {
		t.Fatalf("merged=%+v", out)
	}
}
