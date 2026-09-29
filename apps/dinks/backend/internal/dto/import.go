package dto

// The import file is the export file plus provenance. GET /api/export emits
// {periods, symptoms, exported_at}; the same document is accepted verbatim by
// POST /api/import, so an export is always a valid import (ids are ignored —
// the server assigns them). `format` and `version` let a future revision of the
// file be recognised instead of silently misread, so they are optional on the
// way in but always written by tooling that produces import files.
//
// See apps/dinks/IMPORT.md for the full specification.
const (
	// ImportFormat is the value of the file's "format" field.
	ImportFormat = "dinks-import"
	// ImportVersion is the revision this build writes and accepts.
	ImportVersion = 1
)

// ImportPayload is the body of POST /api/import. The record arrays reuse the
// same input DTOs the single-record create endpoints use, so a hand-written
// file, an export, and a third-party conversion are all validated identically.
type ImportPayload struct {
	Format     string         `json:"format,omitempty"`
	Version    int            `json:"version,omitempty"`
	Source     string         `json:"source,omitempty"`
	ExportedAt string         `json:"exported_at,omitempty"`
	Periods    []PeriodInput  `json:"periods"`
	Symptoms   []SymptomInput `json:"symptoms"`
}

// ImportResult reports what the import actually did, so a client can tell a
// fully applied file from one that was partly a no-op because the records were
// already present. Skipped records are counted, never silently dropped.
type ImportResult struct {
	Mode            string `json:"mode"`
	PeriodsImported int    `json:"periods_imported"`
	PeriodsSkipped  int    `json:"periods_skipped"`
	SymptomImported int    `json:"symptoms_imported"`
	SymptomSkipped  int    `json:"symptoms_skipped"`
}
