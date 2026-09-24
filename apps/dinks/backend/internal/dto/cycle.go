package dto

type Period struct {
	ID        int64   `json:"id"`
	StartedOn string  `json:"started_on"`
	EndedOn   *string `json:"ended_on,omitempty"`
	Flow      string  `json:"flow"`
	Notes     string  `json:"notes,omitempty"`
}
type Symptom struct {
	ID         int64  `json:"id"`
	RecordedOn string `json:"recorded_on"`
	Kind       string `json:"kind"`
	Severity   int    `json:"severity"`
	Notes      string `json:"notes,omitempty"`
}
type Dashboard struct {
	Periods    []Period  `json:"periods"`
	Symptoms   []Symptom `json:"symptoms"`
	NextPeriod *string   `json:"next_period,omitempty"`
	Reminder   string    `json:"reminder,omitempty"`
}
type PeriodInput struct {
	StartedOn string `json:"started_on"`
	EndedOn   string `json:"ended_on"`
	Flow      string `json:"flow"`
	Notes     string `json:"notes"`
}
type SymptomInput struct {
	RecordedOn string `json:"recorded_on"`
	Kind       string `json:"kind"`
	Severity   int    `json:"severity"`
	Notes      string `json:"notes"`
}
type Problem struct {
	Error string `json:"error"`
}
// StatsQuery is the safe, allowlisted "custom query" a frontend dashboard sends —
// see service.AllowedMetrics / service.AllowedGroupBy for what's accepted.
type StatsQuery struct {
	Metrics []string `json:"metrics"`
	GroupBy string   `json:"group_by"`
	From    string   `json:"from"`
	To      string   `json:"to"`
}
type StatsResult struct {
	Group  string         `json:"group"`
	Values map[string]any `json:"values"`
}
type StatsQueryResponse struct {
	Results []StatsResult `json:"results"`
}
type RegisterInput struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type Me struct {
	DisplayName string `json:"display_name"`
}
type Invite struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
}
type RedeemInput struct {
	Code string `json:"code"`
}
type Partner struct {
	Subject     string `json:"subject"`
	DisplayName string `json:"display_name"`
	LinkedAt    string `json:"linked_at"`
}
type PartnerStatus struct {
	Subject     string  `json:"subject"`
	DisplayName string  `json:"display_name"`
	OnPeriod    bool    `json:"on_period"`
	NextPeriod  *string `json:"next_period,omitempty"`
	Reminder    string  `json:"reminder,omitempty"`
}
