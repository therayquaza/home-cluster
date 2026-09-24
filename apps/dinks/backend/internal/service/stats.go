// Package service holds business logic that sits above the repository and below
// the HTTP handlers. Stats implements a small, allowlisted query DSL so the
// frontend can build custom stat views without ever sending a raw database query.
package service

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"dinks/internal/dto"
	"dinks/internal/model"
)

// AllowedMetrics is the full set of metrics a StatsQuery may request. Keeping this
// as an allowlist (rather than accepting an arbitrary aggregation expression) is
// what makes the "custom query" safe: users compose from known, precomputed
// metrics instead of executing anything against the database directly.
var AllowedMetrics = map[string]bool{
	"period_count":               true,
	"checkin_count":              true,
	"cycles_sampled":             true,
	"average_cycle_days":         true,
	"cycle_length_stddev":        true,
	"average_period_length_days": true,
	"symptom_counts":             true,
	"average_symptom_severity":   true,
}

// AllowedGroupBy is the full set of dimensions a StatsQuery may bucket by.
var AllowedGroupBy = map[string]bool{
	"":             true,
	"month":        true,
	"flow":         true,
	"symptom_kind": true,
}

const (
	minPlausibleCycleDays = 15
	maxPlausibleCycleDays = 60
)

type Stats struct{}

func NewStats() *Stats { return &Stats{} }

// Query evaluates a StatsQuery against a subject's periods/symptoms. It validates
// metrics and group_by against the allowlists above (defense in depth even though
// the HTTP handler validates too), applies the optional date range, then computes
// each requested metric per group.
func (s *Stats) Query(periods []model.Period, symptoms []model.Symptom, q dto.StatsQuery) (dto.StatsQueryResponse, error) {
	if len(q.Metrics) == 0 {
		return dto.StatsQueryResponse{}, errors.New("at least one metric is required")
	}
	for _, m := range q.Metrics {
		if !AllowedMetrics[m] {
			return dto.StatsQueryResponse{}, fmt.Errorf("unsupported metric %q", m)
		}
	}
	if !AllowedGroupBy[q.GroupBy] {
		return dto.StatsQueryResponse{}, fmt.Errorf("unsupported group_by %q", q.GroupBy)
	}
	var from, to time.Time
	if q.From != "" {
		v, err := time.Parse("2006-01-02", q.From)
		if err != nil {
			return dto.StatsQueryResponse{}, errors.New("from must be YYYY-MM-DD")
		}
		from = v
	}
	if q.To != "" {
		v, err := time.Parse("2006-01-02", q.To)
		if err != nil {
			return dto.StatsQueryResponse{}, errors.New("to must be YYYY-MM-DD")
		}
		to = v
	}
	periods = filterPeriodsByRange(periods, from, to)
	symptoms = filterSymptomsByRange(symptoms, from, to)

	groups := groupKeys(periods, symptoms, q.GroupBy)
	results := make([]dto.StatsResult, 0, len(groups))
	for _, g := range groups {
		gp := filterPeriodsByGroup(periods, q.GroupBy, g)
		gs := filterSymptomsByGroup(symptoms, q.GroupBy, g)
		values := make(map[string]any, len(q.Metrics))
		for _, m := range q.Metrics {
			values[m] = computeMetric(m, gp, gs, q.GroupBy)
		}
		label := g
		if label == "" {
			label = "all"
		}
		results = append(results, dto.StatsResult{Group: label, Values: values})
	}
	return dto.StatsQueryResponse{Results: results}, nil
}

func filterPeriodsByRange(periods []model.Period, from, to time.Time) []model.Period {
	if from.IsZero() && to.IsZero() {
		return periods
	}
	out := make([]model.Period, 0, len(periods))
	for _, p := range periods {
		if !from.IsZero() && p.StartedOn.Before(from) {
			continue
		}
		if !to.IsZero() && p.StartedOn.After(to) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func filterSymptomsByRange(symptoms []model.Symptom, from, to time.Time) []model.Symptom {
	if from.IsZero() && to.IsZero() {
		return symptoms
	}
	out := make([]model.Symptom, 0, len(symptoms))
	for _, s := range symptoms {
		if !from.IsZero() && s.RecordedOn.Before(from) {
			continue
		}
		if !to.IsZero() && s.RecordedOn.After(to) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func groupKeys(periods []model.Period, symptoms []model.Symptom, groupBy string) []string {
	switch groupBy {
	case "month":
		set := map[string]bool{}
		for _, p := range periods {
			set[p.StartedOn.Format("2006-01")] = true
		}
		for _, s := range symptoms {
			set[s.RecordedOn.Format("2006-01")] = true
		}
		return sortedKeys(set)
	case "flow":
		set := map[string]bool{}
		for _, p := range periods {
			set[p.Flow] = true
		}
		return sortedKeys(set)
	case "symptom_kind":
		set := map[string]bool{}
		for _, s := range symptoms {
			set[s.Kind] = true
		}
		return sortedKeys(set)
	default:
		return []string{""}
	}
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func filterPeriodsByGroup(periods []model.Period, groupBy, key string) []model.Period {
	switch groupBy {
	case "month":
		out := make([]model.Period, 0, len(periods))
		for _, p := range periods {
			if p.StartedOn.Format("2006-01") == key {
				out = append(out, p)
			}
		}
		return out
	case "flow":
		out := make([]model.Period, 0, len(periods))
		for _, p := range periods {
			if p.Flow == key {
				out = append(out, p)
			}
		}
		return out
	default:
		return periods
	}
}

func filterSymptomsByGroup(symptoms []model.Symptom, groupBy, key string) []model.Symptom {
	switch groupBy {
	case "month":
		out := make([]model.Symptom, 0, len(symptoms))
		for _, s := range symptoms {
			if s.RecordedOn.Format("2006-01") == key {
				out = append(out, s)
			}
		}
		return out
	case "symptom_kind":
		out := make([]model.Symptom, 0, len(symptoms))
		for _, s := range symptoms {
			if s.Kind == key {
				out = append(out, s)
			}
		}
		return out
	default:
		return symptoms
	}
}

// cycleLengths returns plausible (15-60 day) gaps between consecutive period starts,
// mirroring domain.EstimateNextPeriod's plausibility window.
func cycleLengths(periods []model.Period) []int {
	starts := make([]time.Time, 0, len(periods))
	for _, p := range periods {
		starts = append(starts, p.StartedOn)
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i].After(starts[j]) })
	lengths := []int{}
	for i := 0; i < len(starts)-1; i++ {
		days := int(starts[i].Sub(starts[i+1]).Hours() / 24)
		if days >= minPlausibleCycleDays && days <= maxPlausibleCycleDays {
			lengths = append(lengths, days)
		}
	}
	return lengths
}

func computeMetric(metric string, periods []model.Period, symptoms []model.Symptom, groupBy string) any {
	switch metric {
	case "period_count":
		return len(periods)
	case "checkin_count":
		return len(symptoms)
	case "cycles_sampled":
		return len(cycleLengths(periods))
	case "average_cycle_days":
		lengths := cycleLengths(periods)
		if len(lengths) == 0 {
			return 0
		}
		total := 0
		for _, d := range lengths {
			total += d
		}
		return total / len(lengths)
	case "cycle_length_stddev":
		lengths := cycleLengths(periods)
		return round1(stddev(lengths))
	case "average_period_length_days":
		total, count := 0, 0
		for _, p := range periods {
			if p.EndedOn == nil {
				continue
			}
			days := int(p.EndedOn.Sub(p.StartedOn).Hours()/24) + 1
			if days > 0 {
				total += days
				count++
			}
		}
		if count == 0 {
			return 0
		}
		return round1(float64(total) / float64(count))
	case "symptom_counts":
		if groupBy == "symptom_kind" {
			return len(symptoms)
		}
		counts := map[string]int{}
		for _, s := range symptoms {
			counts[s.Kind]++
		}
		return counts
	case "average_symptom_severity":
		if groupBy == "symptom_kind" {
			return round1(avgSeverity(symptoms))
		}
		byKind := map[string][]int{}
		for _, s := range symptoms {
			byKind[s.Kind] = append(byKind[s.Kind], s.Severity)
		}
		out := map[string]float64{}
		for kind, sev := range byKind {
			total := 0
			for _, v := range sev {
				total += v
			}
			out[kind] = round1(float64(total) / float64(len(sev)))
		}
		return out
	default:
		return nil
	}
}

func avgSeverity(symptoms []model.Symptom) float64 {
	if len(symptoms) == 0 {
		return 0
	}
	total := 0
	for _, s := range symptoms {
		total += s.Severity
	}
	return float64(total) / float64(len(symptoms))
}

func stddev(values []int) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := 0.0
	for _, v := range values {
		mean += float64(v)
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, v := range values {
		variance += math.Pow(float64(v)-mean, 2)
	}
	variance /= float64(len(values))
	return math.Sqrt(variance)
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
