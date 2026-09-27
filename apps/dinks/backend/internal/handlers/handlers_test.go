package handlers

import (
	"net/url"
	"testing"
	"time"

	"dinks/internal/dto"
	"dinks/internal/model"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

func TestPeriodModelAcceptsValidInput(t *testing.T) {
	p, err := periodModel("sub", dto.PeriodInput{StartedOn: "2026-01-05", EndedOn: "2026-01-12", Flow: "light", Notes: "  spaced  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Subject != "sub" {
		t.Errorf("Subject = %q", p.Subject)
	}
	if !p.StartedOn.Equal(mustDate(t, "2026-01-05")) {
		t.Errorf("StartedOn = %v", p.StartedOn)
	}
	if p.EndedOn == nil || !p.EndedOn.Equal(mustDate(t, "2026-01-12")) {
		t.Errorf("EndedOn = %v, want 2026-01-12", p.EndedOn)
	}
	if p.Notes != "spaced" {
		t.Errorf("Notes = %q, want trimmed", p.Notes)
	}
}

func TestPeriodModelDefaultsMissingFlowToUnknown(t *testing.T) {
	p, err := periodModel("sub", dto.PeriodInput{StartedOn: "2026-01-05"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Flow != "unknown" {
		t.Errorf("Flow = %q, want %q", p.Flow, "unknown")
	}
	if p.EndedOn != nil {
		t.Errorf("EndedOn = %v, want nil for an ongoing period", p.EndedOn)
	}
}

func TestPeriodModelRejectsBadInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   dto.PeriodInput
	}{
		{"bad start date", dto.PeriodInput{StartedOn: "05-01-2026"}},
		{"end before start", dto.PeriodInput{StartedOn: "2026-01-12", EndedOn: "2026-01-05"}},
		{"bad end date", dto.PeriodInput{StartedOn: "2026-01-05", EndedOn: "nope"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := periodModel("sub", tc.in); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestSymptomModelBoundsSeverity(t *testing.T) {
	s, err := symptomModel(dto.SymptomInput{RecordedOn: "2026-02-01", Kind: "  cramps  ", Severity: 3, Notes: " mild "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Kind != "cramps" {
		t.Errorf("Kind = %q, want trimmed", s.Kind)
	}
	if s.Notes != "mild" {
		t.Errorf("Notes = %q, want trimmed", s.Notes)
	}
	if s.Severity != 3 {
		t.Errorf("Severity = %d, want 3", s.Severity)
	}
}

func TestSymptomModelRejectsOutOfRangeSeverity(t *testing.T) {
	for _, sev := range []int{0, -1, 6, 100} {
		if _, err := symptomModel(dto.SymptomInput{RecordedOn: "2026-02-01", Kind: "cramps", Severity: sev}); err == nil {
			t.Errorf("severity %d was accepted, want rejected", sev)
		}
	}
}

func TestSymptomModelRejectsMissingFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   dto.SymptomInput
	}{
		{"bad date", dto.SymptomInput{RecordedOn: "01-02-2026", Kind: "cramps", Severity: 2}},
		{"blank kind", dto.SymptomInput{RecordedOn: "2026-02-01", Kind: "   ", Severity: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := symptomModel(tc.in); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestPlausibleCycleLengthsIgnoresImplausibleGaps(t *testing.T) {
	// Newest first, as the repository returns them. 28 and 30 are plausible;
	// 3 and 400 are data-entry artefacts and must be excluded from the average.
	ps := []model.Period{
		{StartedOn: mustDate(t, "2026-04-01")},
		{StartedOn: mustDate(t, "2026-03-02")}, // 30
		{StartedOn: mustDate(t, "2026-02-02")}, // 28
		{StartedOn: mustDate(t, "2026-01-30")}, // 3
		{StartedOn: mustDate(t, "2025-06-05")}, // 208
	}
	got := plausibleCycleLengths(ps)
	if len(got) != 2 {
		t.Fatalf("plausibleCycleLengths = %v, want 2 samples", got)
	}
	if got[0] != 30 || got[1] != 28 {
		t.Errorf("plausibleCycleLengths = %v, want [30 28]", got)
	}
}

func TestPlausibleCycleLengthsNeedsAtLeastTwoPeriods(t *testing.T) {
	if got := plausibleCycleLengths(nil); len(got) != 0 {
		t.Errorf("nil periods = %v, want empty", got)
	}
	single := []model.Period{{StartedOn: mustDate(t, "2026-04-01")}}
	if got := plausibleCycleLengths(single); len(got) != 0 {
		t.Errorf("one period = %v, want no samples", got)
	}
}

func TestAverageCycleDays(t *testing.T) {
	if got := averageCycleDays(nil); got != 0 {
		t.Errorf("average of no samples = %d, want 0", got)
	}
	if got := averageCycleDays([]int{30, 28, 29}); got != 29 {
		t.Errorf("average = %d, want 29", got)
	}
}

func TestSymptomCounts(t *testing.T) {
	got := symptomCounts([]model.Symptom{{Kind: "cramps"}, {Kind: "headache"}, {Kind: "cramps"}})
	if got["cramps"] != 2 {
		t.Errorf("cramps = %d, want 2", got["cramps"])
	}
	if got["headache"] != 1 {
		t.Errorf("headache = %d, want 1", got["headache"])
	}
}

func TestCycleStatusOngoingPeriod(t *testing.T) {
	ps := []model.Period{
		{StartedOn: mustDate(t, "2026-04-01")},
		{StartedOn: mustDate(t, "2026-03-02")},
	}
	onPeriod, _, _ := cycleStatus(ps)
	if !onPeriod {
		t.Error("onPeriod = false, want true for a period with no end date")
	}
}

func TestCycleStatusEndedPeriods(t *testing.T) {
	end := mustDate(t, "2026-03-12")
	ps := []model.Period{
		{StartedOn: mustDate(t, "2026-03-02"), EndedOn: &end},
		{StartedOn: mustDate(t, "2026-02-02"), EndedOn: &end},
	}
	onPeriod, _, _ := cycleStatus(ps)
	if onPeriod {
		t.Error("onPeriod = true, want false when every period has ended")
	}
}

// queryKeys exists so a broken callback can be diagnosed without logging the
// values, which include the authorization code.
func TestQueryKeysListsNamesSortedAndNeverValues(t *testing.T) {
	const secret = "super-secret-code"
	q := url.Values{"code": {secret}, "state": {"abc"}, "session_state": {"xyz"}}

	keys := queryKeys(q)
	want := []string{"code", "session_state", "state"}
	if len(keys) != len(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("keys[%d] = %q, want %q", i, keys[i], want[i])
		}
	}
	for _, k := range keys {
		if k == secret {
			t.Errorf("queryKeys returned a value: %q", k)
		}
	}
}
