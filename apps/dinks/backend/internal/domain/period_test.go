package domain

import (
	"testing"
	"time"
)

func TestEstimateNextPeriodUsesRecentValidCycles(t *testing.T) {
	periods := []Period{{StartedOn: "2026-03-01"}, {StartedOn: "2026-02-01"}, {StartedOn: "2026-01-04"}}
	got := EstimateNextPeriod(periods)
	if got == nil {
		t.Fatal("got nil")
	}
	if !got.After(time.Now()) {
		t.Fatalf("got %v, want a date after now (overdue estimate must roll forward, not go stale)", got)
	}
	// 2026-03-29 is the raw last-start-plus-average-cycle estimate; since it's now in the
	// past it must have rolled forward by whole 28-day cycles to the next future occurrence.
	raw, _ := time.Parse("2006-01-02", "2026-03-29")
	if got.Sub(raw)%(28*24*time.Hour) != 0 {
		t.Fatalf("got %v, want raw estimate %v plus a whole number of 28-day cycles", got, raw)
	}
}
func TestEstimateNextPeriodRequiresHistory(t *testing.T) {
	if got := EstimateNextPeriod([]Period{{StartedOn: "2026-03-01"}}); got != nil {
		t.Fatal("expected nil")
	}
}
