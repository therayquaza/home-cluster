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

type Member struct {
	Subject      string    `bson:"_id"`
	DisplayName  string    `bson:"display_name"`
	Email        string    `bson:"email,omitempty"`
	PasswordHash string    `bson:"password_hash,omitempty"`
	CreatedAt    time.Time `bson:"created_at"`
}

type Period struct {
	ID        int64      `bson:"_id"`
	Subject   string     `bson:"subject"`
	StartedOn time.Time  `bson:"started_on"`
	EndedOn   *time.Time `bson:"ended_on,omitempty"`
	Flow      string     `bson:"flow"`
	Notes     string     `bson:"notes"`
	CreatedAt time.Time  `bson:"created_at"`
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

// PartnerLink grants PartnerSubject status-only visibility (on-period, next-period
// estimate, reminder — never symptoms/notes) into OwnerSubject's cycle.
type PartnerLink struct {
	ID             int64     `bson:"_id"`
	OwnerSubject   string    `bson:"owner_subject"`
	PartnerSubject string    `bson:"partner_subject"`
	CreatedAt      time.Time `bson:"created_at"`
}

// Counter backs int64 auto-increment ids for Period/Symptom, since Mongo has no serial type.
type Counter struct {
	Name string `bson:"_id"`
	Seq  int64  `bson:"seq"`
}

const CollectionCounters = "counters"
