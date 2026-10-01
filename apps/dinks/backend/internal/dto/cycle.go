package dto

// FlowDay is one day's flow inside a period.
type FlowDay struct {
	Date string `json:"date"`
	Flow string `json:"flow"`
}

type Period struct {
	ID        int64     `json:"id"`
	StartedOn string    `json:"started_on"`
	EndedOn   *string   `json:"ended_on,omitempty"`
	Flow      string    `json:"flow"`
	Days      []FlowDay `json:"days,omitempty"`
	Notes     string    `json:"notes,omitempty"`
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
	StartedOn string    `json:"started_on"`
	EndedOn   string    `json:"ended_on"`
	Flow      string    `json:"flow"`
	Days      []FlowDay `json:"days"`
	Notes     string    `json:"notes"`
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

// Tracker is a user-defined indicator definition.
type Tracker struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Emoji string `json:"emoji"`
	Unit  string `json:"unit,omitempty"`
}

// Preferences is the settings bag returned by GET /api/preferences. The
// *_set flags let a PATCH distinguish "leave this alone" from "set it to the
// zero value" — without them a client could never turn a reminder back on.
type Preferences struct {
	DefaultFlow      string    `json:"default_flow"`
	ReminderEnabled  bool      `json:"reminder_enabled"`
	ReminderLeadDays int       `json:"reminder_lead_days"`
	Trackers         []Tracker `json:"trackers"`
}

// PreferencesInput is a partial update: only *_set fields are applied.
type PreferencesInput struct {
	DefaultFlow      *string    `json:"default_flow,omitempty"`
	ReminderEnabled  *bool      `json:"reminder_enabled,omitempty"`
	ReminderLeadDays *int       `json:"reminder_lead_days,omitempty"`
	Trackers         *[]Tracker `json:"trackers,omitempty"`
}
type Invite struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
}
type RedeemInput struct {
	Code string `json:"code"`
}

// ShareField is a field an owner may expose to a partner.
type ShareField string

// A Partner is a linked partner, with the fields the owner currently shares.
type Partner struct {
	Subject     string       `json:"subject"`
	DisplayName string       `json:"display_name"`
	LinkedAt    string       `json:"linked_at"`
	Share       []ShareField `json:"share"`
}

// ShareInput sets the fields a partner may see. Share is a pointer so that
// omitting it (a client bug) is rejected rather than read as "share nothing" —
// revoking access must be an explicit empty array.
type ShareInput struct {
	Share *[]ShareField `json:"share"`
}
type PartnerStatus struct {
	Subject     string `json:"subject"`
	DisplayName string `json:"display_name"`
	// OnPeriod is a pointer so an owner who has not shared it produces no
	// `on_period` key at all. A plain false would be indistinguishable from a
	// real "not on a period", which is the kind of quiet lie a status view must
	// never tell.
	OnPeriod   *bool   `json:"on_period,omitempty"`
	NextPeriod *string `json:"next_period,omitempty"`
	Reminder   string  `json:"reminder,omitempty"`
}

// PartnerDay is one day of a shared cycle. Fields the owner has not shared are
// omitted or empty, never zero-valued, so the UI can distinguish "nothing
// happened" from "you may not see this".
type PartnerDay struct {
	Date          string   `json:"date"`
	OnPeriod      bool     `json:"on_period"`
	Flow          string   `json:"flow,omitempty"`
	Symptoms      []string `json:"symptoms,omitempty"`
	Sex           []string `json:"sex,omitempty"`
	Libido        []string `json:"libido,omitempty"`
	Notes         string   `json:"notes,omitempty"`
	PeriodStarted bool     `json:"period_started,omitempty"`
	PeriodEnded   bool     `json:"period_ended,omitempty"`
}

// PartnerView is everything one owner shares with the caller: the status, the
// fields actually shared, and the most recent shared days.
type PartnerView struct {
	PartnerStatus
	Shared []string     `json:"shared"`
	Days   []PartnerDay `json:"days"`
}
