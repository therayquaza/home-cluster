package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dinks/internal/dto"
	"dinks/internal/repository"
)

func TestCheckImportEnvelopeAcceptsAnUnlabelledFile(t *testing.T) {
	// The identifying fields are optional so a hand-written or third-party file
	// still works; only a file that names a format is held to it.
	if err := checkImportEnvelope(dto.ImportPayload{}); err != nil {
		t.Errorf("a bare {periods, symptoms} file was rejected: %v", err)
	}
}

func TestCheckImportEnvelopeRejectsAForeignOrNewerFormat(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   dto.ImportPayload
	}{
		{"wrong format", dto.ImportPayload{Format: "some-other-app"}},
		{"future version", dto.ImportPayload{Format: dto.ImportFormat, Version: dto.ImportVersion + 1}},
		{"negative version", dto.ImportPayload{Version: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkImportEnvelope(tc.in); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestCheckImportEnvelopeEnforcesTheRecordCeiling(t *testing.T) {
	in := dto.ImportPayload{Periods: make([]dto.PeriodInput, maxImportRecords+1)}
	if err := checkImportEnvelope(in); err == nil {
		t.Error("an oversized file was accepted")
	}
}

func TestImportModeDefaultsToMergeAndRejectsAnythingElse(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/import", nil)
	if got, err := importMode(req); err != nil || got != importModeMerge {
		t.Errorf("default mode = %q, %v; want %q", got, err, importModeMerge)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/import?mode=replace", nil)
	if got, err := importMode(req); err != nil || got != importModeReplace {
		t.Errorf("mode = %q, %v; want %q", got, err, importModeReplace)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/import?mode=wipe", nil)
	if _, err := importMode(req); err == nil {
		t.Error("an unknown mode was accepted")
	}
}

func TestBuildImportRecordsConvertsTheFile(t *testing.T) {
	periods, symptoms, err := buildImportRecords("sub", dto.ImportPayload{
		Periods:  []dto.PeriodInput{{StartedOn: "2026-01-05", EndedOn: "2026-01-10", Flow: "heavy"}},
		Symptoms: []dto.SymptomInput{{RecordedOn: "2026-01-07", Kind: "Cramps", Severity: 3}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(periods) != 1 || periods[0].Subject != "sub" || periods[0].Flow != "heavy" {
		t.Errorf("periods = %+v", periods)
	}
	if periods[0].EndedOn == nil {
		t.Error("EndedOn = nil, want 2026-01-10")
	}
	if len(symptoms) != 1 || symptoms[0].Kind != "Cramps" {
		t.Errorf("symptoms = %+v", symptoms)
	}
}

func TestBuildImportRecordsNamesTheOffendingRecord(t *testing.T) {
	_, _, err := buildImportRecords("sub", dto.ImportPayload{
		Periods: []dto.PeriodInput{
			{StartedOn: "2026-01-05"},
			{StartedOn: "05-01-2026"},
		},
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "periods[1]") {
		t.Errorf("error = %q, want it to name periods[1]", err)
	}
}

func TestBuildImportRecordsRejectsDuplicatesInsideTheFile(t *testing.T) {
	// A file that duplicates itself is refused rather than silently collapsed,
	// so the person who produced it finds out instead of the person importing it.
	if _, _, err := buildImportRecords("sub", dto.ImportPayload{
		Periods:  []dto.PeriodInput{{StartedOn: "2026-01-05"}, {StartedOn: "2026-01-05"}},
		Symptoms: []dto.SymptomInput{},
	}); err == nil {
		t.Error("two periods starting the same day were accepted")
	}

	if _, _, err := buildImportRecords("sub", dto.ImportPayload{
		Symptoms: []dto.SymptomInput{
			{RecordedOn: "2026-01-07", Kind: "Cramps", Severity: 3},
			{RecordedOn: "2026-01-07", Kind: "Cramps", Severity: 3},
		},
	}); err == nil {
		t.Error("two identical check-ins were accepted")
	}
}

func TestBuildImportRecordsStampsTheCallerOnEveryRecord(t *testing.T) {
	// Regression: symptomModel validates a check-in but does not know its
	// owner, and the create endpoint stamps the caller in separately. An import
	// that skipped that step wrote records with no subject, which every read
	// path filters on — the import reported success and the data was invisible.
	_, symptoms, err := buildImportRecords("subject-abc", dto.ImportPayload{
		Symptoms: []dto.SymptomInput{{RecordedOn: "2026-01-07", Kind: "Cramps", Severity: 3}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if symptoms[0].Subject != "subject-abc" {
		t.Errorf("Subject = %q, want the caller's subject", symptoms[0].Subject)
	}
}

func TestBuildImportRecordsStampsTheCallerOnPeriods(t *testing.T) {
	periods, _, err := buildImportRecords("subject-abc", dto.ImportPayload{
		Periods: []dto.PeriodInput{{StartedOn: "2026-01-05"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if periods[0].Subject != "subject-abc" {
		t.Errorf("Subject = %q, want the caller's subject", periods[0].Subject)
	}
}

func TestBuildImportRecordsKeepsDifferentKindsOnOneDayApart(t *testing.T) {
	_, symptoms, err := buildImportRecords("sub", dto.ImportPayload{
		Symptoms: []dto.SymptomInput{
			{RecordedOn: "2026-01-07", Kind: "Cramps", Severity: 3},
			{RecordedOn: "2026-01-07", Kind: "note", Severity: 3},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symptoms) != 2 {
		t.Errorf("got %d check-ins, want 2", len(symptoms))
	}
}

func TestSymptomKeySeparatesTheDayFromTheKind(t *testing.T) {
	// A day legitimately holds several kinds, so the key has to carry the kind
	// unambiguously — otherwise an import would drop every check-in but the
	// first of the day. The separator matters too: without one, a day/kind
	// pair that happens to concatenate alike would collide.
	keys := map[string]bool{}
	for _, tc := range [][2]string{
		{"2026-01-07", "Cramps"},
		{"2026-01-07", "note"},
		{"2026-01-08", "Cramps"},
		{"2026-01-07", "Cramps1"},
	} {
		k := repository.SymptomKey(tc[0], tc[1])
		if keys[k] {
			t.Errorf("key %q collides for day %q kind %q", k, tc[0], tc[1])
		}
		keys[k] = true
	}
}

func TestImportRejectsAnUnknownModeBeforeTouchingData(t *testing.T) {
	h, rec := importHarness(t)
	h.Import(rec, httptest.NewRequest(http.MethodPost, "/api/import?mode=nuke", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestImportRejectsAnOversizedBody(t *testing.T) {
	h, rec := importHarness(t)
	body := strings.Repeat("x", int(importBodyBytes)+1)
	h.Import(rec, httptest.NewRequest(http.MethodPost, "/api/import", strings.NewReader(body)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a body over the cap", rec.Code)
	}
}

func TestImportRejectsAnUnreadableEnvelope(t *testing.T) {
	h, rec := importHarness(t)
	h.Import(rec, httptest.NewRequest(http.MethodPost, "/api/import",
		strings.NewReader(`{"format":"another-app","periods":[],"symptoms":[]}`)))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "another-app") {
		t.Errorf("body = %q, want it to name the offending format", rec.Body.String())
	}
}

// importHarness builds a handler with no repository behind it. The import
// endpoint touches the database only after the file has been validated, so
// every rejection path is exercised without one.
func importHarness(t *testing.T) (*Handler, *httptest.ResponseRecorder) {
	t.Helper()
	return &Handler{repo: nil}, httptest.NewRecorder()
}
