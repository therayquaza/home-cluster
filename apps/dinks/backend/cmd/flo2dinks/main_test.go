package main

import (
	"encoding/json"
	"testing"
	"time"

	"dinks/internal/dto"
)

const testDate = "2026-09-29"

func mustConvert(t *testing.T, src string) (dto.ImportPayload, *report) {
	t.Helper()
	payload, rep, err := convert([]byte(src), time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	return payload, rep
}

func TestConvertStampsTheDinksEnvelope(t *testing.T) {
	payload, _ := mustConvert(t, `{"operationalData":{"cycles":[],"notes":[]}}`)

	if payload.Format != dto.ImportFormat {
		t.Errorf("Format = %q, want %q", payload.Format, dto.ImportFormat)
	}
	if payload.Version != dto.ImportVersion {
		t.Errorf("Version = %d, want %d", payload.Version, dto.ImportVersion)
	}
	if payload.Source != source {
		t.Errorf("Source = %q, want %q", payload.Source, source)
	}
	if payload.ExportedAt == "" {
		t.Error("ExportedAt is empty")
	}
}

func TestConvertPeriodsCarryTheCalendarDayOnly(t *testing.T) {
	// Flo records the day in the member's own zone, so the time component is
	// an artefact of that zone and must not shift the day.
	payload, _ := mustConvert(t, `{"operationalData":{"cycles":[
		{"id":"a","period_start_date":"2021-05-04 22:00:00.0","period_end_date":"2021-05-08 22:00:00.0","period_intensity":"{}"},
		{"id":"b","period_start_date":"2023-01-01 21:00:00.0","period_end_date":"2023-01-05 21:00:00.0","period_intensity":"{}"}
	]}}`)

	if len(payload.Periods) != 2 {
		t.Fatalf("got %d periods, want 2", len(payload.Periods))
	}
	if got := payload.Periods[0].StartedOn; got != "2021-05-04" {
		t.Errorf("StartedOn = %q, want 2021-05-04", got)
	}
	if got := payload.Periods[0].EndedOn; got != "2021-05-08" {
		t.Errorf("EndedOn = %q, want 2021-05-08", got)
	}
	if got := payload.Periods[1].StartedOn; got != "2023-01-01" {
		t.Errorf("StartedOn = %q, want 2023-01-01", got)
	}
}

func TestConvertSortsPeriodsOldestFirst(t *testing.T) {
	payload, _ := mustConvert(t, `{"operationalData":{"cycles":[
		{"period_start_date":"2024-01-01 00:00:00.0","period_end_date":"2024-01-05 00:00:00.0","period_intensity":"{}"},
		{"period_start_date":"2021-06-01 00:00:00.0","period_end_date":"2021-06-05 00:00:00.0","period_intensity":"{}"},
		{"period_start_date":"2022-11-01 00:00:00.0","period_end_date":"2022-11-05 00:00:00.0","period_intensity":"{}"}
	]}}`)

	want := []string{"2021-06-01", "2022-11-01", "2024-01-01"}
	for i, w := range want {
		if got := payload.Periods[i].StartedOn; got != w {
			t.Errorf("periods[%d] = %q, want %q", i, got, w)
		}
	}
}

func TestConvertKeepsAnOpenPeriodOpen(t *testing.T) {
	payload, _ := mustConvert(t, `{"operationalData":{"cycles":[
		{"period_start_date":"2026-09-25 00:00:00.0","period_end_date":null,"period_intensity":"{}"}
	]}}`)

	if len(payload.Periods) != 1 {
		t.Fatalf("got %d periods, want 1", len(payload.Periods))
	}
	if payload.Periods[0].EndedOn != "" {
		t.Errorf("EndedOn = %q, want empty for a period that has not ended", payload.Periods[0].EndedOn)
	}
}

func TestConvertClampsAFutureEndDateToToday(t *testing.T) {
	payload, rep := mustConvert(t, `{"operationalData":{"cycles":[
		{"period_start_date":"2026-09-25 00:00:00.0","period_end_date":"2026-09-30 00:00:00.0","period_intensity":"{}"}
	]}}`)

	if got := payload.Periods[0].EndedOn; got != testDate {
		t.Errorf("EndedOn = %q, want %q clamped from the future", got, testDate)
	}
	if len(rep.clamped) != 1 {
		t.Errorf("clamped = %v, want one entry", rep.clamped)
	}
}

func TestConvertSkipsUnusableCyclesAndReportsThem(t *testing.T) {
	_, rep := mustConvert(t, `{"operationalData":{"cycles":[
		{"id":"bad-date","period_start_date":"not-a-date","period_end_date":null,"period_intensity":"{}"},
		{"id":"backwards","period_start_date":"2026-01-10 00:00:00.0","period_end_date":"2026-01-05 00:00:00.0","period_intensity":"{}"},
		{"id":"preg","period_start_date":"2026-02-01 00:00:00.0","period_end_date":"2026-02-04 00:00:00.0","period_intensity":"{}","pregnant":true}
	]}}`)

	if len(rep.skipped) != 3 {
		t.Fatalf("skipped = %v, want three entries", rep.skipped)
	}
}

func TestFlowOfTakesThePeakDayAndMapsItToDinksLevels(t *testing.T) {
	for _, tc := range []struct {
		intensity string
		want      string
	}{
		{`{}`, unknownFlow},
		{`not json`, unknownFlow},
		{`{"0": 1}`, "light"},
		{`{"0": 1, "3": 2}`, "medium"},
		{`{"0": 1, "3": 3}`, "heavy"},
		{`{"2": 3, "3": 2}`, "heavy"},
		{`{"1": 9}`, "heavy"},
	} {
		if got := flowOf(tc.intensity); got != tc.want {
			t.Errorf("flowOf(%s) = %q, want %q", tc.intensity, got, tc.want)
		}
	}
}

func TestConvertEmitsPerDayFlowForTheDaysThatDifferFromThePeak(t *testing.T) {
	payload, rep := mustConvert(t, `{"operationalData":{"cycles":[
		{"id":"varying","period_start_date":"2026-03-01 00:00:00.0","period_end_date":"2026-03-04 00:00:00.0","period_intensity":"{\"0\": 3, \"1\": 1, \"2\": 3}"}
	]}}`)

	p := payload.Periods[0]
	// Peak is heavy, so only the light middle day is worth writing; the heavy
	// days fall back to the summary.
	if p.Flow != "heavy" {
		t.Errorf("summary flow = %q, want heavy", p.Flow)
	}
	if len(p.Days) != 1 {
		t.Fatalf("days = %+v, want exactly the one day that differs", p.Days)
	}
	if p.Days[0] != (dto.FlowDay{Date: "2026-03-02", Flow: "light"}) {
		t.Errorf("days[0] = %+v, want 2026-03-02 light", p.Days[0])
	}
	if rep.flowDays != 1 || rep.varyingPeriods != 1 {
		t.Errorf("report = %d day(s) / %d period(s), want 1 / 1", rep.flowDays, rep.varyingPeriods)
	}
}

func TestConvertWritesNoPerDayFlowWhenEveryDayMatchesThePeak(t *testing.T) {
	payload, _ := mustConvert(t, `{"operationalData":{"cycles":[
		{"id":"flat","period_start_date":"2026-03-01 00:00:00.0","period_end_date":"2026-03-03 00:00:00.0","period_intensity":"{\"0\": 2, \"1\": 2}"}
	]}}`)

	if len(payload.Periods[0].Days) != 0 {
		t.Errorf("days = %+v, want none for a period of one level", payload.Periods[0].Days)
	}
}

func TestConvertDropsPerDayFlowPastThePeriodsEnd(t *testing.T) {
	// Flo records intensity for a day the period's own end date does not cover.
	payload, _ := mustConvert(t, `{"operationalData":{"cycles":[
		{"id":"overrun","period_start_date":"2026-03-01 00:00:00.0","period_end_date":"2026-03-02 00:00:00.0","period_intensity":"{\"0\": 3, \"5\": 1}"}
	]}}`)

	for _, d := range payload.Periods[0].Days {
		if d.Date > "2026-03-02" {
			t.Errorf("days contains %s, past the period end 2026-03-02", d.Date)
		}
	}
}

func TestConvertMapsSymptomsOntoDinksLabels(t *testing.T) {
	payload, _ := mustConvert(t, `{"pointEventsManualData":{"point_events_manual_v2":[
		{"local_date":"2021-05-16 22:00:00","category":"Symptom","subcategory":"DrawingPain"},
		{"local_date":"2021-05-17 00:00:00","category":"Symptom","subcategory":"TenderBreasts"},
		{"local_date":"2021-05-18 00:00:00","category":"Symptom","subcategory":"SomeBrandNewThing"}
	]}}`)

	want := []dto.SymptomInput{
		{RecordedOn: "2021-05-16", Kind: "Cramps", Severity: defaultSeverity, Notes: ""},
		{RecordedOn: "2021-05-17", Kind: "Tender breasts", Severity: defaultSeverity, Notes: ""},
		{RecordedOn: "2021-05-18", Kind: "Some Brand New Thing", Severity: defaultSeverity, Notes: ""},
	}
	assertSymptoms(t, payload.Symptoms, want)
}

func TestConvertCollapsesADaysMoodsIntoOneNote(t *testing.T) {
	payload, _ := mustConvert(t, `{"pointEventsManualData":{"point_events_manual_v2":[
		{"local_date":"2021-09-18 00:00:00","category":"Mood","subcategory":"Neutral"},
		{"local_date":"2021-09-18 00:00:00","category":"Mood","subcategory":"Sad"},
		{"local_date":"2021-09-18 00:00:00","category":"Mood","subcategory":"Angry"}
	]}}`)

	want := []dto.SymptomInput{
		{RecordedOn: "2021-09-18", Kind: noteKind, Severity: defaultSeverity, Notes: "Mood: Calm, Sad, Irritable."},
	}
	assertSymptoms(t, payload.Symptoms, want)
}

func TestConvertDropsARepeatedMoodRatherThanRepeatingIt(t *testing.T) {
	payload, _ := mustConvert(t, `{"pointEventsManualData":{"point_events_manual_v2":[
		{"local_date":"2021-09-18 00:00:00","category":"Mood","subcategory":"Sad"},
		{"local_date":"2021-09-18 00:00:00","category":"Mood","subcategory":"Depressed"},
		{"local_date":"2021-09-18 00:00:00","category":"Mood","subcategory":"Sad"}
	]}}`)

	want := []dto.SymptomInput{
		{RecordedOn: "2021-09-18", Kind: noteKind, Severity: defaultSeverity, Notes: "Mood: Sad."},
	}
	assertSymptoms(t, payload.Symptoms, want)
}

func TestConvertMergesMoodAndFreeTextIntoOneNote(t *testing.T) {
	payload, _ := mustConvert(t, `{
		"pointEventsManualData":{"point_events_manual_v2":[
			{"local_date":"2024-10-02 00:00:00","category":"Mood","subcategory":"Angry"}
		]},
		"operationalData":{"notes":[
			{"text":"j'ai très mal ","date":"2024-10-02 00:00:00.0"},
			{"text":"   ","date":"2024-10-03 00:00:00.0"}
		]}}`)

	want := []dto.SymptomInput{
		{RecordedOn: "2024-10-02", Kind: noteKind, Severity: defaultSeverity, Notes: "Mood: Irritable. j'ai très mal"},
	}
	assertSymptoms(t, payload.Symptoms, want)
}

func TestConvertKeepsTextWithNoMoodAsTheNoteAlone(t *testing.T) {
	payload, _ := mustConvert(t, `{"operationalData":{"notes":[
		{"text":"pose du stérilet en cuivre ","date":"2024-07-18 00:00:00.0"}
	]}}`)

	want := []dto.SymptomInput{
		{RecordedOn: "2024-07-18", Kind: noteKind, Severity: defaultSeverity, Notes: "pose du stérilet en cuivre"},
	}
	assertSymptoms(t, payload.Symptoms, want)
}

func TestConvertEmitsASymptomAndANoteForTheSameDay(t *testing.T) {
	payload, _ := mustConvert(t, `{
		"pointEventsManualData":{"point_events_manual_v2":[
			{"local_date":"2023-09-11 00:00:00","category":"Symptom","subcategory":"Acne"},
			{"local_date":"2023-09-11 00:00:00","category":"Mood","subcategory":"Sad"}
		]}}`)

	want := []dto.SymptomInput{
		{RecordedOn: "2023-09-11", Kind: "Acne", Severity: defaultSeverity, Notes: ""},
		{RecordedOn: "2023-09-11", Kind: noteKind, Severity: defaultSeverity, Notes: "Mood: Sad."},
	}
	assertSymptoms(t, payload.Symptoms, want)
}

func TestConvertDropsACategoryDinksCannotHold(t *testing.T) {
	payload, rep := mustConvert(t, `{"pointEventsManualData":{"point_events_manual_v2":[
		{"local_date":"2024-01-01 00:00:00","category":"Sex","subcategory":"SexUnprotected"},
		{"local_date":"2024-01-02 00:00:00","category":"Fluid","subcategory":"Bloody"},
		{"local_date":"2024-01-03 00:00:00","category":"Symptom","subcategory":"Acne"}
	]}}`)

	want := []dto.SymptomInput{
		{RecordedOn: "2024-01-01", Kind: "Sex — unprotected", Severity: defaultSeverity, Notes: ""},
		{RecordedOn: "2024-01-03", Kind: "Acne", Severity: defaultSeverity, Notes: ""},
	}
	assertSymptoms(t, payload.Symptoms, want)
	if rep.dropped["Fluid/Bloody"] != 1 {
		t.Errorf("dropped = %v, want only the fluid event counted", rep.dropped)
	}
}

func TestConvertKeepsSexAndLibidoAsSeparateKinds(t *testing.T) {
	payload, _ := mustConvert(t, `{"pointEventsManualData":{"point_events_manual_v2":[
		{"local_date":"2024-01-01 00:00:00","category":"Sex","subcategory":"SexProtected"},
		{"local_date":"2024-01-01 00:00:00","category":"Sex","subcategory":"Orgasm"},
		{"local_date":"2024-01-01 00:00:00","category":"Sex","subcategory":"HighDrive"},
		{"local_date":"2024-01-01 00:00:00","category":"Sex","subcategory":"NeutralDrive"}
	]}}`)

	want := []dto.SymptomInput{
		{RecordedOn: "2024-01-01", Kind: "Libido — high", Severity: defaultSeverity, Notes: ""},
		{RecordedOn: "2024-01-01", Kind: "Libido — moderate", Severity: defaultSeverity, Notes: ""},
		{RecordedOn: "2024-01-01", Kind: "Orgasm", Severity: defaultSeverity, Notes: ""},
		{RecordedOn: "2024-01-01", Kind: "Sex — protected", Severity: defaultSeverity, Notes: ""},
	}
	assertSymptoms(t, payload.Symptoms, want)
}

// Flo's "SexNone" means no sex that day, which is the absence of an event
// rather than a thing to record. It must not become a check-in.
func TestConvertDropsSexNoneRatherThanRecordingIt(t *testing.T) {
	payload, rep := mustConvert(t, `{"pointEventsManualData":{"point_events_manual_v2":[
		{"local_date":"2024-01-01 00:00:00","category":"Sex","subcategory":"SexNone"}
	]}}`)

	if len(payload.Symptoms) != 0 {
		t.Errorf("symptoms = %+v, want SexNone to be dropped", payload.Symptoms)
	}
	if rep.dropped["Sex/SexNone"] != 1 {
		t.Errorf("dropped = %v, want SexNone reported", rep.dropped)
	}
}

func TestConvertRecordsRepeatedSexOnTheSameDayOnce(t *testing.T) {
	payload, _ := mustConvert(t, `{"pointEventsManualData":{"point_events_manual_v2":[
		{"local_date":"2024-01-01 00:00:00","category":"Sex","subcategory":"SexOral"},
		{"local_date":"2024-01-01 00:00:00","category":"Sex","subcategory":"SexOral"}
	]}}`)

	if len(payload.Symptoms) != 1 {
		t.Errorf("symptoms = %+v, want a single entry for the day", payload.Symptoms)
	}
}

func TestConvertProducesNoDuplicateCheckIns(t *testing.T) {
	payload, _ := mustConvert(t, `{"pointEventsManualData":{"point_events_manual_v2":[
		{"local_date":"2024-01-01 00:00:00","category":"Symptom","subcategory":"Acne"},
		{"local_date":"2024-01-01 00:00:00","category":"Symptom","subcategory":"Acne"}
	]}}`)

	seen := map[string]bool{}
	for _, s := range payload.Symptoms {
		k := s.RecordedOn + "|" + s.Kind
		if seen[k] {
			t.Errorf("duplicate check-in %q; the import would reject this file", k)
		}
		seen[k] = true
	}
}

func TestConvertedFilePassesTheImportValidationRules(t *testing.T) {
	payload, _ := mustConvert(t, `{
		"operationalData":{"cycles":[
			{"period_start_date":"2026-01-05 00:00:00.0","period_end_date":"2026-01-10 00:00:00.0","period_intensity":"{\"1\": 3}"}
		]},
		"pointEventsManualData":{"point_events_manual_v2":[
			{"local_date":"2026-01-07 00:00:00","category":"Symptom","subcategory":"Cramps"}
		]}}`)

	// Re-encode exactly as the file is shipped, then decode it the way the
	// server does, so the test breaks if the wire form ever drifts from the
	// in-memory shape.
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round dto.ImportPayload
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.Periods[0].Flow != "heavy" {
		t.Errorf("Flow = %q, want heavy", round.Periods[0].Flow)
	}
	if round.Symptoms[0].Severity < 1 || round.Symptoms[0].Severity > 5 {
		t.Errorf("Severity = %d, outside the accepted 1-5 scale", round.Symptoms[0].Severity)
	}
}

func TestHumanizeSplitsPascalCase(t *testing.T) {
	for in, want := range map[string]string{
		"Acne":           "Acne",
		"Backache":       "Backache",
		"SomeNewThing":   "Some New Thing",
		"XMLParserError": "XMLParser Error",
		"":               "",
	} {
		if got := humanize(in); got != want {
			t.Errorf("humanize(%q) = %q, want %q", in, got, want)
		}
	}
}

func assertSymptoms(t *testing.T, got, want []dto.SymptomInput) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d check-ins, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("check-ins[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
