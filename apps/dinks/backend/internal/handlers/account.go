package handlers

import (
	"net/http"
	"time"

	"dinks/internal/dto"
	"dinks/internal/httpx"
	"dinks/internal/logging"
	"dinks/internal/middleware"
)

// export streams the caller's own records back as a downloadable JSON file, so
// the data can be moved out of the service.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	sub := middleware.Subject(r.Context())
	ps, err := h.repo.Periods(r.Context(), sub)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to export", err)
		return
	}
	ss, err := h.repo.Symptoms(r.Context(), sub)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to export", err)
		return
	}
	periods := make([]dto.Period, 0, len(ps))
	symptoms := make([]dto.Symptom, 0, len(ss))
	for _, p := range ps {
		periods = append(periods, periodDTO(p))
	}
	for _, s := range ss {
		symptoms = append(symptoms, symptomDTO(s))
	}
	w.Header().Set("Content-Disposition", `attachment; filename="dinks-export.json"`)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"periods":     periods,
		"symptoms":    symptoms,
		"exported_at": time.Now().UTC(),
	})
}

// deleteMe erases the caller's account and every record attached to it, then
// ends the session.
func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	sub := middleware.Subject(r.Context())
	if err := h.repo.DeleteMember(r.Context(), sub); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to delete data", err)
		return
	}
	logging.Log.Info("account and all data deleted", "subject", sub)
	_ = h.sessions.Destroy(r.Context())
	w.WriteHeader(http.StatusNoContent)
}
