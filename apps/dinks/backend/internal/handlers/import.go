package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"dinks/internal/dto"
	"dinks/internal/httpx"
	"dinks/internal/logging"
	"dinks/internal/middleware"
	"dinks/internal/model"
	"dinks/internal/repository"
)

// Import modes. Merge adds whatever the caller does not already have, which
// makes re-importing the same file a no-op; replace wipes the caller's records
// and writes the file wholesale, which is how a user loads a history that
// already lives in another app.
const (
	importModeMerge   = "merge"
	importModeReplace = "replace"
)

const (
	// importBodyBytes bounds an import request. A few thousand check-ins
	// serialise to a few hundred KiB, far past the 64 KiB ordinary cap, so this
	// route raises it — bounded all the same, so a hostile or runaway file
	// cannot exhaust memory.
	importBodyBytes = 4 << 20

	// maxImportRecords caps the record count of one file independently of the
	// byte cap, so a highly compressible body cannot expand into an unbounded
	// number of writes.
	maxImportRecords = 50000
)

// Import applies a whole dinks import file to the caller's account.
//
// The file is validated in full before anything is written, so a malformed
// record leaves the account untouched instead of half-migrated. Records already
// present are counted as skipped rather than duplicated, which is what makes
// the endpoint safe to retry after a partial network failure.
func (h *Handler) Import(w http.ResponseWriter, r *http.Request) {
	var in dto.ImportPayload
	if !httpx.DecodeLimit(w, r, &in, importBodyBytes) {
		return
	}
	if err := checkImportEnvelope(in); err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, err.Error(), err)
		return
	}
	mode, err := importMode(r)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, err.Error(), err)
		return
	}

	sub := middleware.Subject(r.Context())
	periods, symptoms, err := buildImportRecords(sub, in)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, err.Error(), err)
		return
	}

	skippedPeriods := len(in.Periods) - len(periods)
	skippedSymptoms := len(in.Symptoms) - len(symptoms)
	if mode == importModeMerge {
		periods, symptoms, err = h.dropAlreadyHeld(r, sub, periods, symptoms)
		if err != nil {
			httpx.Problem(r, w, http.StatusInternalServerError, "unable to import", err)
			return
		}
		skippedPeriods = len(in.Periods) - len(periods)
		skippedSymptoms = len(in.Symptoms) - len(symptoms)
	}

	if err := h.write(r, sub, mode, periods, symptoms); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to import", err)
		return
	}
	logging.Log.Info("data imported", "subject", sub, "mode", mode, "source", in.Source,
		"periods", len(periods), "symptoms", len(symptoms),
		"periods_skipped", skippedPeriods, "symptoms_skipped", skippedSymptoms)
	httpx.JSON(w, http.StatusOK, dto.ImportResult{
		Mode:            mode,
		PeriodsImported: len(periods),
		PeriodsSkipped:  skippedPeriods,
		SymptomImported: len(symptoms),
		SymptomSkipped:  skippedSymptoms,
	})
}

// write applies the batch under the chosen mode. Both paths are transactional
// inside the repository, so the caller's records are never left half-swapped.
func (h *Handler) write(r *http.Request, subject, mode string, periods []model.Period, symptoms []model.Symptom) error {
	if len(periods) == 0 && len(symptoms) == 0 {
		return nil
	}
	if mode == importModeReplace {
		return h.repo.ReplaceRecords(r.Context(), subject, periods, symptoms)
	}
	return h.repo.AppendRecords(r.Context(), periods, symptoms)
}

// checkImportEnvelope rejects a file this build cannot read confidently. The
// identifying fields are optional so a hand-written or third-party file is
// accepted, but a file that does name a format must name the right one, and a
// newer version is refused rather than partially understood.
func checkImportEnvelope(in dto.ImportPayload) error {
	if in.Format != "" && in.Format != dto.ImportFormat {
		return fmt.Errorf("unsupported format %q, expected %q", in.Format, dto.ImportFormat)
	}
	if in.Version < 0 || in.Version > dto.ImportVersion {
		return fmt.Errorf("unsupported version %d, this build reads up to %d", in.Version, dto.ImportVersion)
	}
	if total := len(in.Periods) + len(in.Symptoms); total > maxImportRecords {
		return fmt.Errorf("file holds %d records, the limit is %d", total, maxImportRecords)
	}
	return nil
}

func importMode(r *http.Request) (string, error) {
	switch m := strings.TrimSpace(r.URL.Query().Get("mode")); m {
	case "", importModeMerge:
		return importModeMerge, nil
	case importModeReplace:
		return importModeReplace, nil
	default:
		return "", fmt.Errorf("mode must be %q or %q", importModeMerge, importModeReplace)
	}
}

// buildImportRecords validates every record in the file up front and converts
// it to the persistence model. Nothing is written here, so the first bad record
// aborts the whole file with a message naming it.
func buildImportRecords(subject string, in dto.ImportPayload) ([]model.Period, []model.Symptom, error) {
	periods := make([]model.Period, 0, len(in.Periods))
	seenPeriods := make(map[string]bool, len(in.Periods))
	for i, p := range in.Periods {
		m, err := periodModel(subject, p)
		if err != nil {
			return nil, nil, fmt.Errorf("periods[%d]: %w", i, err)
		}
		key := m.StartedOn.Format(dateLayout)
		if seenPeriods[key] {
			return nil, nil, fmt.Errorf("periods[%d]: the file holds two periods starting %s", i, key)
		}
		seenPeriods[key] = true
		periods = append(periods, m)
	}

	symptoms := make([]model.Symptom, 0, len(in.Symptoms))
	seenCheckins := make(map[string]bool, len(in.Symptoms))
	for i, s := range in.Symptoms {
		m, err := symptomModel(s)
		if err != nil {
			return nil, nil, fmt.Errorf("symptoms[%d]: %w", i, err)
		}
		// symptomModel validates a single check-in and does not know who owns
		// it — the create endpoint stamps the caller in. Without this the
		// records would be written with no subject and be invisible to every
		// read path, so it is set here rather than assumed.
		m.Subject = subject
		key := repository.SymptomKey(m.RecordedOn.Format(dateLayout), m.Kind)
		if seenCheckins[key] {
			return nil, nil, fmt.Errorf("symptoms[%d]: the file holds two check-ins of kind %q on %s",
				i, m.Kind, m.RecordedOn.Format(dateLayout))
		}
		seenCheckins[key] = true
		symptoms = append(symptoms, m)
	}
	return periods, symptoms, nil
}

// dropAlreadyHeld removes from the batch every record the caller already has.
// Both the file and the account are deduplicated, so the outcome is the same
// however many times the same file is sent.
func (h *Handler) dropAlreadyHeld(r *http.Request, subject string, periods []model.Period, symptoms []model.Symptom) ([]model.Period, []model.Symptom, error) {
	heldPeriods, err := h.repo.PeriodStarts(r.Context(), subject)
	if err != nil {
		return nil, nil, err
	}
	held := make(map[string]bool, len(heldPeriods))
	for _, d := range heldPeriods {
		held[d] = true
	}
	keptPeriods := periods[:0]
	for _, p := range periods {
		if key := p.StartedOn.Format(dateLayout); !held[key] {
			held[key] = true
			keptPeriods = append(keptPeriods, p)
		}
	}

	heldKeys, err := h.repo.SymptomKeys(r.Context(), subject)
	if err != nil {
		return nil, nil, err
	}
	heldCheckins := make(map[string]bool, len(heldKeys))
	for _, k := range heldKeys {
		heldCheckins[k] = true
	}
	keptSymptoms := symptoms[:0]
	for _, s := range symptoms {
		if key := repository.SymptomKey(s.RecordedOn.Format(dateLayout), s.Kind); !heldCheckins[key] {
			heldCheckins[key] = true
			keptSymptoms = append(keptSymptoms, s)
		}
	}
	return keptPeriods, keptSymptoms, nil
}
