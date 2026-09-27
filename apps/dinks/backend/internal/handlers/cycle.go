package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dinks/internal/dto"
	"dinks/internal/httpx"
	"dinks/internal/middleware"
	"dinks/internal/model"
	"dinks/internal/repository"
)

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	sub := middleware.Subject(r.Context())
	ps, err := h.repo.Periods(r.Context(), sub)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load records", err)
		return
	}
	ss, err := h.repo.Symptoms(r.Context(), sub)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load records", err)
		return
	}
	out := dto.Dashboard{Periods: make([]dto.Period, 0, len(ps)), Symptoms: make([]dto.Symptom, 0, len(ss))}
	for _, p := range ps {
		out.Periods = append(out.Periods, periodDTO(p))
	}
	for _, s := range ss {
		out.Symptoms = append(out.Symptoms, symptomDTO(s))
	}
	_, out.NextPeriod, out.Reminder = cycleStatus(ps)
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) Prediction(w http.ResponseWriter, r *http.Request) {
	ps, err := h.repo.Periods(r.Context(), middleware.Subject(r.Context()))
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load prediction", err)
		return
	}
	_, next, _ := cycleStatus(ps)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"predicted_period_start": next,
		"method":                 "average of recent plausible cycle lengths",
		"disclaimer":             "Not medical advice.",
	})
}

func (h *Handler) CreatePeriod(w http.ResponseWriter, r *http.Request) {
	var in dto.PeriodInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	p, err := periodModel(middleware.Subject(r.Context()), in)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, err.Error(), err)
		return
	}
	if err := h.repo.CreatePeriod(r.Context(), &p); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to save period", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) UpdatePeriod(w http.ResponseWriter, r *http.Request) {
	var in dto.PeriodInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, "invalid period id", err)
		return
	}
	sub := middleware.Subject(r.Context())
	p, err := periodModel(sub, in)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, err.Error(), err)
		return
	}
	switch err := h.repo.UpdatePeriod(r.Context(), sub, id, p); {
	case errors.Is(err, repository.ErrNotFound):
		httpx.Problem(r, w, http.StatusNotFound, "period not found", err)
	case err != nil:
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to update period", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) CreateSymptom(w http.ResponseWriter, r *http.Request) {
	var in dto.SymptomInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	s, err := symptomModel(in)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, err.Error(), err)
		return
	}
	s.Subject = middleware.Subject(r.Context())
	if err := h.repo.CreateSymptom(r.Context(), &s); err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to save symptom", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) UpdateSymptom(w http.ResponseWriter, r *http.Request) {
	var in dto.SymptomInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, "invalid symptom id", err)
		return
	}
	s, err := symptomModel(in)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, err.Error(), err)
		return
	}
	switch err := h.repo.UpdateSymptom(r.Context(), middleware.Subject(r.Context()), id, s); {
	case errors.Is(err, repository.ErrNotFound):
		httpx.Problem(r, w, http.StatusNotFound, "symptom not found", err)
	case err != nil:
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to update symptom", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) DeleteSymptom(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, "invalid symptom id", err)
		return
	}
	switch err := h.repo.DeleteSymptom(r.Context(), middleware.Subject(r.Context()), id); {
	case errors.Is(err, repository.ErrNotFound):
		httpx.Problem(r, w, http.StatusNotFound, "symptom not found", err)
	case err != nil:
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to delete symptom", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// symptomModel validates the input DTO and converts it to the persistence model.
// Severity is bounded to the 1-5 scale the UI offers.
func symptomModel(in dto.SymptomInput) (model.Symptom, error) {
	day, err := time.Parse("2006-01-02", in.RecordedOn)
	if err != nil || strings.TrimSpace(in.Kind) == "" || in.Severity < 1 || in.Severity > 5 {
		return model.Symptom{}, errors.New("recorded_on, kind, and severity (1-5) are required")
	}
	return model.Symptom{
		RecordedOn: day,
		Kind:       strings.TrimSpace(in.Kind),
		Severity:   in.Severity,
		Notes:      strings.TrimSpace(in.Notes),
	}, nil
}
