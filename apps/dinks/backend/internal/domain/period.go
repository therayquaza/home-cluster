package domain

import "time"

// Period is the domain representation returned by the API and persisted by the repository.
type Period struct {
	ID        int64   `json:"id"`
	StartedOn string  `json:"started_on"`
	EndedOn   *string `json:"ended_on,omitempty"`
	Flow      string  `json:"flow"`
	Notes     string  `json:"notes,omitempty"`
}

// EstimateNextPeriod averages up to six plausible completed cycle lengths.
// It deliberately makes no fertility or medical claim.
func EstimateNextPeriod(periods []Period) *time.Time {
	if len(periods) < 2 {
		return nil
	}
	starts := make([]time.Time, 0, len(periods))
	for _, p := range periods {
		if day, err := time.Parse("2006-01-02", p.StartedOn); err == nil {
			starts = append(starts, day)
		}
	}
	if len(starts) < 2 {
		return nil
	}
	total, count := 0, 0
	for i := 0; i < len(starts)-1 && count < 6; i++ {
		days := int(starts[i].Sub(starts[i+1]).Hours() / 24)
		if days >= 15 && days <= 60 {
			total += days
			count++
		}
	}
	if count == 0 {
		return nil
	}
	avg := total / count
	next := starts[0].AddDate(0, 0, avg)
	// A period can be overdue: the last logged start plus the average cycle is already in
	// the past. Roll forward by whole cycles instead of returning a stale (or nil) estimate.
	now := time.Now()
	for !next.After(now) && avg > 0 {
		next = next.AddDate(0, 0, avg)
	}
	return &next
}
