package model

import (
	"time"
)

const (
	CollectionMembers      = "members"
	CollectionPeriods      = "periods"
	CollectionSymptoms     = "symptoms"
	CollectionSessions     = "sessions"
	CollectionInvites      = "invites"
	CollectionPartnerLinks = "partner_links"
)

// Tracker is a user-defined indicator — anything they want to record a number
// for that dinks does not already model (weight, temperature, hours of sleep).
// The definition lives in the member's preferences; the values are check-ins
// whose Kind is "track:<key>", which keeps custom tracking on the same read,
// export and calendar path as every other daily entry.
type Tracker struct {
	Key   string `bson:"key"   json:"key"`
	Label string `bson:"label" json:"label"`
	Emoji string `bson:"emoji" json:"emoji"`
	Unit  string `bson:"unit,omitempty" json:"unit,omitempty"`
}

// Preferences is the per-member settings bag. Every field is optional in the
// stored document so a member created before a field existed reads back with
// the zero value rather than failing to decode.
type Preferences struct {
	DefaultFlow      string    `bson:"default_flow,omitempty"      json:"default_flow,omitempty"`
	ReminderEnabled  *bool     `bson:"reminder_enabled,omitempty"  json:"reminder_enabled,omitempty"`
	ReminderLeadDays int       `bson:"reminder_lead_days,omitempty" json:"reminder_lead_days,omitempty"`
	Trackers         []Tracker `bson:"trackers,omitempty"          json:"trackers,omitempty"`
}

// ReminderOn reports whether the approaching-period reminder is enabled. It
// defaults to on: a member who never set it should still get the reminder, and
// switching it off has to be an explicit choice.
func (p Preferences) ReminderOn() bool { return p.ReminderEnabled == nil || *p.ReminderEnabled }

// LeadDays is how many days ahead the reminder appears, defaulting to the 7 the
// backend has always used.
func (p Preferences) LeadDays() int {
	if p.ReminderLeadDays <= 0 {
		return 7
	}
	return p.ReminderLeadDays
}

// DefaultFlowLevel is the flow preselected when a new period is logged. It
// defaults to medium rather than to nothing: an empty value would leave the
// flow picker with no selection at all, and "medium" is what the app has
// always assumed when logging a period.
// The flow levels the app offers, in increasing order. "unknown" is what an
// import can carry when the source recorded no flow at all, and is displayed
// as "not recorded" rather than offered as a choice.
const (
	FlowUnknown = "unknown"
	FlowNone    = "none"
	FlowLight   = "light"
	FlowMedium  = "medium"
	FlowHeavy   = "heavy"
)

// FlowLevels are the levels a member can pick, in the order the UI shows them.
var FlowLevels = []string{FlowNone, FlowLight, FlowMedium, FlowHeavy}

func (p Preferences) DefaultFlowLevel() string {
	if p.DefaultFlow == "" {
		return FlowMedium
	}
	return p.DefaultFlow
}

type Member struct {
	Subject      string      `bson:"_id"`
	DisplayName  string      `bson:"display_name"`
	Email        string      `bson:"email,omitempty"`
	PasswordHash string      `bson:"password_hash,omitempty"`
	Preferences  Preferences `bson:"preferences,omitempty"`
	CreatedAt    time.Time   `bson:"created_at"`
}

// FlowDay is the flow recorded for one specific day of a period. Flow genuinely
// varies within a single period — it is usually lightest on the first and last
// days — so a single Flow on the period can only ever be a summary.
type FlowDay struct {
	Date time.Time `bson:"date"`
	Flow string    `bson:"flow"`
}

type Period struct {
	ID        int64      `bson:"_id"`
	Subject   string     `bson:"subject"`
	StartedOn time.Time  `bson:"started_on"`
	EndedOn   *time.Time `bson:"ended_on,omitempty"`
	// Flow is the period's overall/typical flow, and the fallback for any day
	// not named in Days. It stays on the record so periods logged before
	// per-day flow existed (and files that never break flow down) keep working
	// unchanged.
	Flow      string    `bson:"flow"`
	Days      []FlowDay `bson:"days,omitempty"`
	Notes     string    `bson:"notes"`
	CreatedAt time.Time `bson:"created_at"`
}

// FlowOn returns the flow for one day of this period, falling back to the
// period's summary Flow when that day has no explicit entry.
func (p Period) FlowOn(day time.Time) string {
	for _, d := range p.Days {
		if d.Date.Equal(day) {
			return d.Flow
		}
	}
	return p.Flow
}

type Symptom struct {
	ID         int64     `bson:"_id"`
	Subject    string    `bson:"subject"`
	RecordedOn time.Time `bson:"recorded_on"`
	Kind       string    `bson:"kind"`
	Severity   int       `bson:"severity"`
	Notes      string    `bson:"notes"`
	CreatedAt  time.Time `bson:"created_at"`
}

// Session matches the schema required by our scs mongo session store (see repository.SessionStore).
type Session struct {
	Token  string    `bson:"_id"`
	Data   []byte    `bson:"data"`
	Expiry time.Time `bson:"expiry"`
}

// Invite is a short-lived code an owner generates so a partner can link to their
// status feed. Reaped by a TTL index on ExpiresAt once redeemed or expired.
type Invite struct {
	Code         string    `bson:"_id"`
	OwnerSubject string    `bson:"owner_subject"`
	ExpiresAt    time.Time `bson:"expires_at"`
}

// ShareField is one piece of information an owner may expose to a partner.
type ShareField string

const (
	ShareOnPeriod   ShareField = "on_period"
	ShareNextPeriod ShareField = "next_period"
	ShareReminders  ShareField = "reminders"
	ShareFlow       ShareField = "flow"
	ShareSymptoms   ShareField = "symptoms"
	ShareLibido     ShareField = "libido"
	ShareSex        ShareField = "sex"
	ShareNotes      ShareField = "notes"
)

// ShareFields is every field a share may cover, in the order the UI presents
// them. Sensitive fields are listed after the neutral ones on purpose.
var ShareFields = []ShareField{
	ShareOnPeriod, ShareNextPeriod, ShareReminders,
	ShareFlow, ShareSymptoms, ShareLibido, ShareSex, ShareNotes,
}

// ValidShareField reports whether f is a field dinks knows how to project.
func ValidShareField(f ShareField) bool {
	for _, known := range ShareFields {
		if f == known {
			return true
		}
	}
	return false
}

// SensitiveShareFields are the fields a share starts empty on: the sensitive
// ones are opt-in, never opt-out, so a link can never leak them by omission.
var SensitiveShareFields = []ShareField{ShareSex, ShareLibido, ShareNotes}

// PartnerLink grants PartnerSubject visibility into OwnerSubject's cycle, limited
// to the fields in Share. A link created before shares existed, or with no
// shares set, is treated as sharing only the status fields — never the records.
type PartnerLink struct {
	ID             int64        `bson:"_id"`
	OwnerSubject   string       `bson:"owner_subject"`
	PartnerSubject string       `bson:"partner_subject"`
	Share          []ShareField `bson:"share,omitempty"`
	CreatedAt      time.Time    `bson:"created_at"`
}

// LastDay is the final day the period covers: its end date, or today when it is
// still open. Iterating a period's days needs this, and an open period must run
// through today rather than collapsing to a single day.
func (p Period) LastDay() time.Time {
	if p.EndedOn == nil {
		return time.Now()
	}
	return *p.EndedOn
}

// Allows reports whether the link exposes f.
//
// A nil Share means the field was never written — a link created before shares
// existed — and reads as the default status-only scope. An empty but non-nil
// Share is an explicit "share nothing", so it denies everything. Confusing the
// two would make it impossible to revoke access, which is the one operation a
// partner link must never get wrong.
func (l PartnerLink) Allows(f ShareField) bool {
	if l.Share == nil {
		return f == ShareOnPeriod || f == ShareNextPeriod || f == ShareReminders
	}
	for _, have := range l.Share {
		if have == f {
			return true
		}
	}
	return false
}

// Counter backs int64 auto-increment ids for Period/Symptom, since Mongo has no serial type.
type Counter struct {
	Name string `bson:"_id"`
	Seq  int64  `bson:"seq"`
}

const CollectionCounters = "counters"
