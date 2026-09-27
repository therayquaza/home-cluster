package handlers

import (
	"net/http"

	"dinks/internal/dto"
	"dinks/internal/httpx"
	"dinks/internal/middleware"
	"dinks/internal/model"
)

// stats accepts only allowlisted metrics (not raw user queries) so
// frontend-customizable dashboards cannot execute arbitrary database operations.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	sub := middleware.Subject(r.Context())
	ps, err := h.repo.Periods(r.Context(), sub)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load statistics", err)
		return
	}
	ss, err := h.repo.Symptoms(r.Context(), sub)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load statistics", err)
		return
	}

	lengths := plausibleCycleLengths(ps)
	payload := map[string]any{
		"period_count":       len(ps),
		"checkin_count":      len(ss),
		"average_cycle_days": averageCycleDays(lengths),
		"symptom_counts":     symptomCounts(ss),
		"cycles_sampled":     len(lengths),
	}
	if metric := r.URL.Query().Get("metric"); metric != "" {
		if _, ok := payload[metric]; !ok {
			httpx.Problem(r, w, http.StatusBadRequest, "unsupported metric", nil)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"metric": metric, "value": payload[metric]})
		return
	}
	httpx.JSON(w, http.StatusOK, payload)
}

// statsQuery is the frontend-customizable "advanced stats" endpoint: users compose
// a query from allowlisted metrics/group_by (service.AllowedMetrics/AllowedGroupBy)
// instead of sending a raw database query.
func (h *Handler) StatsQuery(w http.ResponseWriter, r *http.Request) {
	var in dto.StatsQuery
	if !httpx.Decode(w, r, &in) {
		return
	}
	sub := middleware.Subject(r.Context())
	ps, err := h.repo.Periods(r.Context(), sub)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load statistics", err)
		return
	}
	ss, err := h.repo.Symptoms(r.Context(), sub)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load statistics", err)
		return
	}
	out, err := h.statsService.Query(ps, ss, in)
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, err.Error(), err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// plausibleCycle is the inclusive day range treated as a real cycle when
// averaging. Anything outside it is a data-entry artefact, not a measurement.
const (
	minPlausibleCycle = 15
	maxPlausibleCycle = 60
)

// plausibleCycleLengths returns the day gaps between consecutive periods that
// fall in the plausible range, and is the sample size behind the average.
func plausibleCycleLengths(ps []model.Period) []int {
	lengths := []int{}
	for i := 0; i < len(ps)-1; i++ {
		d := int(ps[i].StartedOn.Sub(ps[i+1].StartedOn).Hours() / 24)
		if d >= minPlausibleCycle && d <= maxPlausibleCycle {
			lengths = append(lengths, d)
		}
	}
	return lengths
}

// averageCycleDays returns the mean of the plausible cycle lengths, or 0 when
// there are no samples.
func averageCycleDays(lengths []int) int {
	if len(lengths) == 0 {
		return 0
	}
	total := 0
	for _, d := range lengths {
		total += d
	}
	return total / len(lengths)
}

func symptomCounts(ss []model.Symptom) map[string]int {
	bySymptom := map[string]int{}
	for _, s := range ss {
		bySymptom[s.Kind]++
	}
	return bySymptom
}
