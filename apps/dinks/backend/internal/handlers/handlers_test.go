package handlers

import (
	"fmt"
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

func TestPreferencesDefaultsToAReminderSevenDaysOut(t *testing.T) {
	var p model.Preferences
	if !p.ReminderOn() {
		t.Error("ReminderOn = false for an unset preference, want true")
	}
	if p.LeadDays() != 7 {
		t.Errorf("LeadDays = %d, want the long-standing default 7", p.LeadDays())
	}
}

func TestPreferencesLetAMemberSilenceTheReminder(t *testing.T) {
	off := false
	p := model.Preferences{ReminderEnabled: &off}
	if p.ReminderOn() {
		t.Error("ReminderOn = true after explicitly disabling it")
	}
}

func TestPreferencesResolveADefaultFlowSoThePickerIsNeverEmpty(t *testing.T) {
	// An unset preference used to be reported as "", which left the flow picker
	// with nothing preselected and started new periods on an arbitrary level.
	var unset model.Preferences
	if got := unset.DefaultFlowLevel(); got != model.FlowMedium {
		t.Errorf("DefaultFlowLevel() = %q for an unset preference, want %q", got, model.FlowMedium)
	}
	if got := preferencesDTO(unset).DefaultFlow; got != model.FlowMedium {
		t.Errorf("preferencesDTO default_flow = %q, want %q", got, model.FlowMedium)
	}

	chosen := model.Preferences{DefaultFlow: model.FlowLight}
	if got := chosen.DefaultFlowLevel(); got != model.FlowLight {
		t.Errorf("DefaultFlowLevel() = %q, want the member's choice %q", got, model.FlowLight)
	}
}

func TestPeriodModelRejectsAnUnknownFlowLevel(t *testing.T) {
	if _, err := periodModel("s", dto.PeriodInput{StartedOn: "2026-01-01", Flow: "gushing"}); err == nil {
		t.Error("periodModel accepted a flow level that does not exist")
	}
	if _, err := periodModel("s", dto.PeriodInput{
		StartedOn: "2026-01-01",
		Flow:      model.FlowMedium,
		Days:      []dto.FlowDay{{Date: "2026-01-02", Flow: "gushing"}},
	}); err == nil {
		t.Error("periodModel accepted a per-day flow level that does not exist")
	}
	// An unset flow is not an error: it means the member recorded nothing.
	if _, err := periodModel("s", dto.PeriodInput{StartedOn: "2026-01-01"}); err != nil {
		t.Errorf("periodModel rejected an unset flow: %v", err)
	}
}

func TestParseTrackersDerivesAKeyFromTheLabel(t *testing.T) {
	trackers, err := parseTrackers([]dto.Tracker{{Label: "Body weight", Unit: "kg", Emoji: "⚖️"}})
	if err != nil {
		t.Fatalf("parseTrackers: %v", err)
	}
	if len(trackers) != 1 || trackers[0].Key != "body_weight" {
		t.Errorf("trackers = %+v, want a single tracker keyed body_weight", trackers)
	}
	// The emoji is stored whole: "⚖️" is two codepoints, and truncating to one
	// would drop the variation selector and change how it renders.
	if trackers[0].Emoji != "⚖️" {
		t.Errorf("emoji = %q, want it preserved intact", trackers[0].Emoji)
	}
}

func TestParseTrackersRejectsAnUnusableKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []dto.Tracker
	}{
		{"two trackers sharing a key", []dto.Tracker{{Key: "w", Label: "A"}, {Key: "w", Label: "B"}}},
		{"an illegal key", []dto.Tracker{{Key: "has spaces", Label: "A"}}},
		{"a key that cannot be derived", []dto.Tracker{{Label: "!!!"}}},
		{"an empty label", []dto.Tracker{{Label: "  "}}},
		{"more than the cap", manyTrackers(maxTrackers + 1)},
	} {
		if _, err := parseTrackers(tc.in); err == nil {
			t.Errorf("%s: err = nil, want a rejection", tc.name)
		}
	}
}

func manyTrackers(n int) []dto.Tracker {
	out := make([]dto.Tracker, 0, n)
	for i := range n {
		out = append(out, dto.Tracker{Key: fmt.Sprintf("k%d", i), Label: fmt.Sprintf("K%d", i)})
	}
	return out
}

func TestFlowDayModelsRejectsADayOutsideThePeriod(t *testing.T) {
	start := mustDate(t, "2026-01-05")
	end := mustDate(t, "2026-01-08")
	_, err := flowDayModels(start, &end, []dto.FlowDay{{Date: "2026-01-09", Flow: "light"}})
	if err == nil {
		t.Fatal("err = nil, want a rejection for a day past the period end")
	}
	if _, err := flowDayModels(start, &end, []dto.FlowDay{{Date: "2026-01-04", Flow: "light"}}); err == nil {
		t.Fatal("err = nil, want a rejection for a day before the period start")
	}
}

func TestFlowDayModelsSortsAndAcceptsAnOpenPeriod(t *testing.T) {
	start := mustDate(t, "2026-01-05")
	days, err := flowDayModels(start, nil, []dto.FlowDay{
		{Date: "2026-01-09", Flow: "light"},
		{Date: "2026-01-06", Flow: "heavy"},
	})
	if err != nil {
		t.Fatalf("flowDayModels on an open period: %v", err)
	}
	if len(days) != 2 || !days[0].Date.Equal(mustDate(t, "2026-01-06")) {
		t.Errorf("days = %+v, want them sorted oldest first", days)
	}
}

func TestPeriodFlowOnFallsBackToTheSummary(t *testing.T) {
	p := model.Period{
		Flow: "heavy",
		Days: []model.FlowDay{{Date: mustDate(t, "2026-01-06"), Flow: "light"}},
	}
	if got := p.FlowOn(mustDate(t, "2026-01-06")); got != "light" {
		t.Errorf("FlowOn = %q, want the day's own level", got)
	}
	if got := p.FlowOn(mustDate(t, "2026-01-07")); got != "heavy" {
		t.Errorf("FlowOn = %q, want the summary for a day with no entry", got)
	}
}

func TestCycleStatusOngoingPeriod(t *testing.T) {
	ps := []model.Period{
		{StartedOn: mustDate(t, "2026-04-01")},
		{StartedOn: mustDate(t, "2026-03-02")},
	}
	onPeriod, _, _ := cycleStatus(ps, model.Preferences{})
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
	onPeriod, _, _ := cycleStatus(ps, model.Preferences{})
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
