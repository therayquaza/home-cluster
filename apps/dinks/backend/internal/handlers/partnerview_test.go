package handlers

import (
	"testing"
	"time"

	"dinks/internal/model"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptrDay(s string) *time.Time {
	d := day(s)
	return &d
}

func TestALinkWithNoShareExposesOnlyTheStatusFields(t *testing.T) {
	// A link created before shares existed has no share stored. It must read as
	// the narrowest scope, never as "everything".
	link := model.PartnerLink{}
	for _, f := range []model.ShareField{model.ShareOnPeriod, model.ShareNextPeriod, model.ShareReminders} {
		if !link.Allows(f) {
			t.Errorf("Allows(%s) = false, want the default status scope to include it", f)
		}
	}
	for _, f := range []model.ShareField{model.ShareFlow, model.ShareSymptoms, model.ShareNotes, model.ShareSex, model.ShareLibido} {
		if link.Allows(f) {
			t.Errorf("Allows(%s) = true, want records hidden until explicitly shared", f)
		}
	}
}

func TestAnEmptyShareRevokesEverything(t *testing.T) {
	// Sharing nothing is meaningful and must differ from sharing the default.
	link := model.PartnerLink{Share: []model.ShareField{}}
	for _, f := range model.ShareFields {
		if link.Allows(f) {
			t.Errorf("Allows(%s) = true for an explicitly empty share", f)
		}
	}
}

func TestAShareGrantsExactlyWhatItLists(t *testing.T) {
	link := model.PartnerLink{Share: []model.ShareField{model.ShareOnPeriod, model.ShareFlow}}
	if !link.Allows(model.ShareFlow) {
		t.Error("Allows(flow) = false, want it shared")
	}
	if link.Allows(model.ShareSex) {
		t.Error("Allows(sex) = true, want sex withheld until it is listed")
	}
}

func TestSexAndLibidoKindsAreRecognisedAcrossSpellings(t *testing.T) {
	// The importer writes em dashes; a future change might use hyphens. A share
	// that only matched one spelling would quietly share nothing.
	for _, kind := range []string{"Sex — protected", "Sex - oral", "Sex — toys", "Sex — unprotected"} {
		if !isSexKind(kind) {
			t.Errorf("isSexKind(%q) = false, want true", kind)
		}
		if isLibidoKind(kind) {
			t.Errorf("isLibidoKind(%q) = true, want false", kind)
		}
	}
	for _, kind := range []string{"Libido — high", "Libido - low", "Libido — none", "Libido — moderate"} {
		if !isLibidoKind(kind) {
			t.Errorf("isLibidoKind(%q) = false, want true", kind)
		}
	}
}

func TestPlainSymptomsAreNotMistakenForSexOrLibido(t *testing.T) {
	for _, kind := range []string{"Cramps", "Sexy thoughts", "Libidinal energy", "Sexual tension", "Note", "Cramps in lower abdomen"} {
		if isSexKind(kind) || isLibidoKind(kind) {
			t.Errorf("%q classified as sex/libido, want neither", kind)
		}
	}
}

func TestProjectDaysHidesWhatIsNotShared(t *testing.T) {
	ps := []model.Period{{
		StartedOn: day("2026-03-02"),
		EndedOn:   ptrDay("2026-03-06"),
		Flow:      "medium",
		Notes:     "private thoughts",
	}}
	ss := []model.Symptom{
		{RecordedOn: day("2026-03-03"), Kind: "Cramps", Severity: 4, Notes: ""},
		{RecordedOn: day("2026-03-03"), Kind: "Sex — protected", Severity: 1, Notes: ""},
		{RecordedOn: day("2026-03-04"), Kind: "Libido — high", Severity: 1, Notes: ""},
	}

	// Status-only share: the days still show that a period was on, but nothing
	// about flow, symptoms or sex leaks.
	rows := (&Handler{}).projectDays(model.PartnerLink{}, ps, ss)
	var onPeriodDays int
	for _, r := range rows {
		if r.OnPeriod {
			onPeriodDays++
		}
		if r.Flow != "" {
			t.Errorf("day %s has flow %q, want it withheld", r.Date, r.Flow)
		}
		if len(r.Symptoms) > 0 || r.Notes != "" {
			t.Errorf("day %s leaked symptoms/notes: %+v", r.Date, r)
		}
	}
	if onPeriodDays != 5 {
		t.Errorf("on-period days = %d, want all 5 days of the period", onPeriodDays)
	}
}

func TestProjectDaysRevealsOnlyTheSharedFields(t *testing.T) {
	h := &Handler{}
	ps := []model.Period{{StartedOn: day("2026-03-02"), EndedOn: ptrDay("2026-03-03"), Flow: "heavy", Notes: "mine"}}
	ss := []model.Symptom{
		{RecordedOn: day("2026-03-02"), Kind: "Cramps", Severity: 4, Notes: ""},
		{RecordedOn: day("2026-03-02"), Kind: "Sex — protected", Severity: 1, Notes: ""},
	}

	// Flow and symptoms shared; sex and notes still not.
	link := model.PartnerLink{Share: []model.ShareField{model.ShareOnPeriod, model.ShareFlow, model.ShareSymptoms}}
	rows := h.projectDays(link, ps, ss)

	var found bool
	for _, r := range rows {
		if r.Date != "2026-03-02" {
			continue
		}
		found = true
		if r.Flow != "heavy" {
			t.Errorf("flow = %q, want heavy", r.Flow)
		}
		if len(r.Symptoms) != 1 || r.Symptoms[0] != "Cramps" {
			t.Errorf("symptoms = %v, want just Cramps", r.Symptoms)
		}
		if r.Notes != "" {
			t.Errorf("notes = %q, want them withheld", r.Notes)
		}
	}
	if !found {
		t.Fatal("no row for 2026-03-02")
	}
}

func TestProjectDaysNeverListsSexAsASymptom(t *testing.T) {
	h := &Handler{}
	ps := []model.Period{{StartedOn: day("2026-03-02"), EndedOn: ptrDay("2026-03-02"), Flow: "light"}}
	ss := []model.Symptom{
		{RecordedOn: day("2026-03-02"), Kind: "Sex — oral", Severity: 1, Notes: ""},
		{RecordedOn: day("2026-03-02"), Kind: "Cramps", Severity: 2, Notes: ""},
	}
	// Symptoms are shared but sex is not: the sex entry must be dropped entirely
	// rather than surfacing as a symptom named "Sex — oral".
	link := model.PartnerLink{Share: []model.ShareField{model.ShareOnPeriod, model.ShareSymptoms}}
	for _, r := range h.projectDays(link, ps, ss) {
		for _, s := range r.Symptoms {
			if isSexKind(s) || isLibidoKind(s) {
				t.Errorf("day %s exposed %q as a symptom while sex was unshared", r.Date, s)
			}
		}
	}
}

func TestProjectDaysUsesThePerDayFlowNotTheSummary(t *testing.T) {
	h := &Handler{}
	ps := []model.Period{{
		StartedOn: day("2026-03-02"),
		EndedOn:   ptrDay("2026-03-04"),
		Flow:      "medium",
		Days:      []model.FlowDay{{Date: day("2026-03-03"), Flow: "heavy"}},
	}}
	link := model.PartnerLink{Share: []model.ShareField{model.ShareOnPeriod, model.ShareFlow}}

	want := map[string]string{"2026-03-02": "medium", "2026-03-03": "heavy", "2026-03-04": "medium"}
	for _, r := range h.projectDays(link, ps, nil) {
		if r.Flow != want[r.Date] {
			t.Errorf("day %s flow = %q, want %q", r.Date, r.Flow, want[r.Date])
		}
	}
}

func TestProjectDaysOmitsOnPeriodEntirelyWhenNotShared(t *testing.T) {
	h := &Handler{}
	ps := []model.Period{{StartedOn: day("2026-03-02"), EndedOn: ptrDay("2026-03-03"), Flow: "light"}}

	// Sharing only symptoms: the partner learns a symptom was logged that day but
	// not that it was a period day.
	link := model.PartnerLink{Share: []model.ShareField{model.ShareSymptoms}}
	rows := h.projectDays(link, ps, []model.Symptom{{RecordedOn: day("2026-03-02"), Kind: "Headache", Severity: 2, Notes: ""}})
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want only the symptom day", len(rows))
	}
	if rows[0].OnPeriod {
		t.Error("OnPeriod = true although on_period is not shared")
	}
}

func TestSharedFieldNamesReportsTheEffectiveScope(t *testing.T) {
	names := sharedFieldNames(model.PartnerLink{})
	if len(names) != 3 {
		t.Errorf("names = %v, want the three default status fields", names)
	}
	names = sharedFieldNames(model.PartnerLink{Share: []model.ShareField{model.ShareFlow}})
	if len(names) != 1 || names[0] != "flow" {
		t.Errorf("names = %v, want just flow", names)
	}
}

func TestProjectDaysTreatsAnOpenPeriodAsRunningToday(t *testing.T) {
	h := &Handler{}
	// Regression: LastDay returned the start date for a period with no end, so
	// an ongoing period projected as a single day instead of running through
	// today — and every day after the start silently showed "not on a period".
	started := day(time.Now().AddDate(0, 0, -2).Format("2006-01-02"))
	ps := []model.Period{{StartedOn: started, Flow: "medium"}}
	link := model.PartnerLink{Share: []model.ShareField{model.ShareOnPeriod, model.ShareFlow}}

	rows := h.projectDays(link, ps, nil)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want the open period to cover start, start+1 and today", len(rows))
	}
	for _, r := range rows {
		if !r.OnPeriod {
			t.Errorf("day %s OnPeriod = false, want true inside an open period", r.Date)
		}
		if r.Flow == "" {
			t.Errorf("day %s has no flow, want the summary applied", r.Date)
		}
	}
	if today := time.Now().Format("2006-01-02"); rows[2].Date != today {
		t.Errorf("last day = %s, want today %s", rows[2].Date, today)
	}
}

func TestProjectDaysKeepsSexAndLibidoOutOfSymptoms(t *testing.T) {
	h := &Handler{}
	today := time.Now().Format("2006-01-02")
	ps := []model.Period{{StartedOn: day(today), Flow: "light"}}
	ss := []model.Symptom{
		{RecordedOn: day(today), Kind: "Cramps", Severity: 3, Notes: ""},
		{RecordedOn: day(today), Kind: "Sex — oral", Severity: 1, Notes: ""},
		{RecordedOn: day(today), Kind: "Libido — high", Severity: 1, Notes: ""},
		{RecordedOn: day(today), Kind: "note", Severity: 3, Notes: "a good day"},
	}
	link := model.PartnerLink{Share: []model.ShareField{
		model.ShareOnPeriod, model.ShareSymptoms, model.ShareNotes, model.ShareSex, model.ShareLibido,
	}}
	rows := h.projectDays(link, ps, ss)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want one day", len(rows))
	}
	r := rows[0]
	if len(r.Symptoms) != 1 || r.Symptoms[0] != "Cramps" {
		t.Errorf("symptoms = %v, want only Cramps", r.Symptoms)
	}
	if len(r.Sex) != 1 {
		t.Errorf("sex = %v, want the sex kind in its own field", r.Sex)
	}
	if len(r.Libido) != 1 {
		t.Errorf("libido = %v, want the libido kind in its own field", r.Libido)
	}
	// The day's free text is a note, not a symptom, and must not be rendered as
	// "note: a good day".
	if r.Notes != "a good day" {
		t.Errorf("notes = %q, want the free text on its own", r.Notes)
	}
}

func TestProjectDaysKeepsSymptomsWithNotesOutWhenNotesAreUnshared(t *testing.T) {
	h := &Handler{}
	today := time.Now().Format("2006-01-02")
	ps := []model.Period{{StartedOn: day(today), Flow: "light"}}
	ss := []model.Symptom{{RecordedOn: day(today), Kind: "note", Severity: 3, Notes: "private"}}

	// Symptoms shared, notes not: the day's text must not slip through.
	link := model.PartnerLink{Share: []model.ShareField{model.ShareOnPeriod, model.ShareSymptoms}}
	for _, r := range h.projectDays(link, ps, ss) {
		if r.Notes != "" {
			t.Errorf("notes = %q, want them withheld", r.Notes)
		}
		if len(r.Symptoms) > 0 {
			t.Errorf("symptoms = %v, want the note kind excluded from symptoms", r.Symptoms)
		}
	}
}

func TestEverySensitiveFieldIsValidForTheShareUI(t *testing.T) {
	for _, f := range model.SensitiveShareFields {
		if !model.ValidShareField(f) {
			t.Errorf("%s is listed as sensitive but is not a shareable field", f)
		}
	}
	// Sanity: a field that does not exist must be rejected, so a stale client
	// cannot store a scope the projection would not honour.
	if model.ValidShareField("everything") {
		t.Error("ValidShareField(\"everything\") = true, want false")
	}
}
