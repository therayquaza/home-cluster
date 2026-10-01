package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"dinks/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("already exists")

// Read caps for the dashboard and export queries. These are the number of
// records a single request will return, not a retention policy — a member who
// has logged daily for several years has far more than a hundred check-ins, and
// truncating silently hides their own history from the calendar and the stats.
// The sort in each query is newest-first, so the cap always keeps the recent
// end of the history.
const (
	maxPeriodsRead  = 2000
	maxSymptomsRead = 20000
)

// dateLayout is the calendar-day format every date in the API uses. Parsing
// and formatting a day through this layout keeps it in the server's zone
// rather than drifting with the process's location settings.
const dateLayout = "2006-01-02"

type Cycle struct {
	client       *mongo.Client
	db           *mongo.Database
	members      *mongo.Collection
	periods      *mongo.Collection
	symptoms     *mongo.Collection
	counters     *mongo.Collection
	sessions     *mongo.Collection
	invites      *mongo.Collection
	partnerLinks *mongo.Collection
}

func New(client *mongo.Client, dbName string) *Cycle {
	db := client.Database(dbName)
	return &Cycle{
		client:       client,
		db:           db,
		members:      db.Collection(model.CollectionMembers),
		periods:      db.Collection(model.CollectionPeriods),
		symptoms:     db.Collection(model.CollectionSymptoms),
		counters:     db.Collection(model.CollectionCounters),
		sessions:     db.Collection(model.CollectionSessions),
		invites:      db.Collection(model.CollectionInvites),
		partnerLinks: db.Collection(model.CollectionPartnerLinks),
	}
}

// Migrate creates the indexes the query patterns above rely on. Mongo has no schema
// migration step (collections are created lazily on first write), so this only sets up indexes.
func (r *Cycle) Migrate(ctx context.Context) error {
	if _, err := r.periods.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "subject", Value: 1}, {Key: "started_on", Value: -1}},
	}); err != nil {
		return err
	}
	if _, err := r.symptoms.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "subject", Value: 1}, {Key: "recorded_on", Value: -1}},
	}); err != nil {
		return err
	}
	if _, err := r.members.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetPartialFilterExpression(
			bson.M{"email": bson.M{"$exists": true, "$gt": ""}}),
	}); err != nil {
		return err
	}
	if _, err := r.sessions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expiry", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}); err != nil {
		return err
	}
	if _, err := r.invites.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}); err != nil {
		return err
	}
	if _, err := r.partnerLinks.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "owner_subject", Value: 1}, {Key: "partner_subject", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}
	_, err := r.partnerLinks.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "partner_subject", Value: 1}},
	})
	return err
}

func (r *Cycle) UpsertMember(ctx context.Context, subject, name string) error {
	_, err := r.members.UpdateByID(ctx, subject, bson.M{
		"$set":         bson.M{"display_name": name},
		"$setOnInsert": bson.M{"created_at": time.Now().UTC()},
	}, options.UpdateOne().SetUpsert(true))
	return err
}

func (r *Cycle) FindMemberByEmail(ctx context.Context, email string) (*model.Member, error) {
	var m model.Member
	err := r.members.FindOne(ctx, bson.M{"email": email}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Cycle) CreateMemberWithPassword(ctx context.Context, subject, email, displayName, passwordHash string) error {
	_, err := r.members.InsertOne(ctx, model.Member{
		Subject: subject, Email: email, DisplayName: displayName,
		PasswordHash: passwordHash, CreatedAt: time.Now().UTC(),
	})
	if mongo.IsDuplicateKeyError(err) {
		return ErrConflict
	}
	return err
}

// UpdatePreferences replaces the member's settings bag. A full replace is fine
// and simpler than a per-field update: the document is small, and a PATCH that
// only ever sends changed fields would have to read-modify-write it anyway.
func (r *Cycle) UpdatePreferences(ctx context.Context, subject string, p model.Preferences) error {
	_, err := r.members.UpdateByID(ctx, subject, bson.M{"$set": bson.M{"preferences": p}})
	return err
}

func (r *Cycle) GetMember(ctx context.Context, subject string) (*model.Member, error) {
	var m model.Member
	err := r.members.FindOne(ctx, bson.M{"_id": subject}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// MembersBySubject looks up several members at once, returning a subject ->
// member map. The partner listings call this instead of GetMember per row: with
// one query per partner the endpoint cost grew linearly with the number of links.
func (r *Cycle) MembersBySubject(ctx context.Context, subjects []string) (map[string]model.Member, error) {
	out := make(map[string]model.Member, len(subjects))
	if len(subjects) == 0 {
		return out, nil
	}
	// De-duplicate: a subject can appear twice when a pair is linked both ways.
	unique := make([]string, 0, len(subjects))
	seen := make(map[string]bool, len(subjects))
	for _, s := range subjects {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		unique = append(unique, s)
	}
	cur, err := r.members.Find(ctx, bson.M{"_id": bson.M{"$in": unique}})
	if err != nil {
		return nil, err
	}
	var members []model.Member
	if err := cur.All(ctx, &members); err != nil {
		return nil, err
	}
	for _, m := range members {
		out[m.Subject] = m
	}
	return out, nil
}

// StatusPeriods returns just the period fields a partner's status view needs, for
// one owner. The partner status endpoint used to load every period of every
// shared owner; EstimateNextPeriod only ever looks at the six most recent starts,
// so this reads a projection capped well below the member's whole history. The
// cap is the same guarantee EstimateNextPeriod gives either way — a shorter
// history than the cap would produce the same estimate.
func (r *Cycle) StatusPeriods(ctx context.Context, subject string) ([]model.Period, error) {
	cur, err := r.periods.Find(ctx, bson.M{"subject": subject},
		options.Find().
			SetProjection(bson.M{"started_on": 1, "ended_on": 1, "flow": 1}).
			SetSort(bson.D{{Key: "started_on", Value: -1}}).
			SetLimit(partnerStatusPeriodReads))
	if err != nil {
		return nil, err
	}
	var docs []model.Period
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// partnerStatusPeriodReads is comfortably above the six plausible cycles
// EstimateNextPeriod averages, so a member with implausible gaps (which the
// estimator skips) still gets an estimate from a full six.
const partnerStatusPeriodReads = 24

// SharedPeriods returns full period records for a partner view, unlike
// StatusPeriods which projects away the notes and per-day flow a partner has no
// right to see unless the owner shared them.
func (r *Cycle) SharedPeriods(ctx context.Context, subject string, limit int) ([]model.Period, error) {
	cur, err := r.periods.Find(ctx, bson.M{"subject": subject},
		options.Find().
			SetSort(bson.D{{Key: "started_on", Value: -1}}).
			SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	var docs []model.Period
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// SharedSymptoms returns check-ins on or after `from` for a partner view, most
// recent first. Symptoms carry free-form notes, so the caller must strip what
// the owner has not shared rather than projecting the field away here — the
// query has to be able to serve owners who did share notes.
func (r *Cycle) SharedSymptoms(ctx context.Context, subject string, from time.Time) ([]model.Symptom, error) {
	cur, err := r.symptoms.Find(ctx, bson.M{"subject": subject, "recorded_on": bson.M{"$gte": from}},
		options.Find().
			SetSort(bson.D{{Key: "recorded_on", Value: -1}}).
			SetLimit(int64(maxSharedDayReads*8)))
	if err != nil {
		return nil, err
	}
	var docs []model.Symptom
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// maxSharedDayReads bounds how much history one partner view may return.
const maxSharedDayReads = 30

// LinkBetween returns the single link connecting these two subjects, or
// ErrNotFound. It is the authorisation check for every partner read: no link
// means no view, regardless of what the caller asks for.
func (r *Cycle) LinkBetween(ctx context.Context, owner, partner string) (model.PartnerLink, error) {
	var link model.PartnerLink
	err := r.partnerLinks.FindOne(ctx,
		bson.M{"owner_subject": owner, "partner_subject": partner}).Decode(&link)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return model.PartnerLink{}, ErrNotFound
	}
	if err != nil {
		return model.PartnerLink{}, err
	}
	return link, nil
}

// UpdateShare replaces the set of fields one partner may see. The share is
// matched on the owner+partner pair rather than on the id the client would have
// to fetch first.
func (r *Cycle) UpdateShare(ctx context.Context, owner, partner string, share []model.ShareField) error {
	_, err := r.partnerLinks.UpdateOne(ctx,
		bson.M{"owner_subject": owner, "partner_subject": partner},
		bson.M{"$set": bson.M{"share": share}})
	return err
}

func (r *Cycle) Periods(ctx context.Context, subject string) ([]model.Period, error) {
	cur, err := r.periods.Find(ctx, bson.M{"subject": subject},
		options.Find().SetSort(bson.D{{Key: "started_on", Value: -1}}).SetLimit(maxPeriodsRead))
	if err != nil {
		return nil, err
	}
	var out []model.Period
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Cycle) Symptoms(ctx context.Context, subject string) ([]model.Symptom, error) {
	cur, err := r.symptoms.Find(ctx, bson.M{"subject": subject},
		options.Find().SetSort(bson.D{{Key: "recorded_on", Value: -1}}).SetLimit(maxSymptomsRead))
	if err != nil {
		return nil, err
	}
	var out []model.Symptom
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Cycle) CreatePeriod(ctx context.Context, period *model.Period) error {
	id, err := r.nextID(ctx, model.CollectionPeriods)
	if err != nil {
		return err
	}
	period.ID = id
	period.CreatedAt = time.Now().UTC()
	_, err = r.periods.InsertOne(ctx, period)
	return err
}

func (r *Cycle) UpdatePeriod(ctx context.Context, subject string, id int64, p model.Period) error {
	result, err := r.periods.UpdateOne(ctx,
		bson.M{"_id": id, "subject": subject},
		bson.M{"$set": bson.M{"started_on": p.StartedOn, "ended_on": p.EndedOn, "flow": p.Flow, "notes": p.Notes}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Cycle) CreateSymptom(ctx context.Context, symptom *model.Symptom) error {
	id, err := r.nextID(ctx, model.CollectionSymptoms)
	if err != nil {
		return err
	}
	symptom.ID = id
	symptom.CreatedAt = time.Now().UTC()
	_, err = r.symptoms.InsertOne(ctx, symptom)
	return err
}

// insertPeriods writes a batch of periods, drawing every id from the counter in
// a single round trip. A bulk import would otherwise cost one counter round
// trip per record, which dominates the cost of a file with thousands of them.
func (r *Cycle) insertPeriods(ctx context.Context, ps []model.Period) error {
	if len(ps) == 0 {
		return nil
	}
	first, err := r.reserveIDs(ctx, model.CollectionPeriods, len(ps))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	docs := make([]any, len(ps))
	for i := range ps {
		ps[i].ID = first + int64(i)
		ps[i].CreatedAt = now
		docs[i] = &ps[i]
	}
	_, err = r.periods.InsertMany(ctx, docs)
	return err
}

// insertSymptoms writes a batch of check-ins, with the same single-round-trip
// id allocation as insertPeriods.
func (r *Cycle) insertSymptoms(ctx context.Context, ss []model.Symptom) error {
	if len(ss) == 0 {
		return nil
	}
	first, err := r.reserveIDs(ctx, model.CollectionSymptoms, len(ss))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	docs := make([]any, len(ss))
	for i := range ss {
		ss[i].ID = first + int64(i)
		ss[i].CreatedAt = now
		docs[i] = &ss[i]
	}
	_, err = r.symptoms.InsertMany(ctx, docs)
	return err
}

// PeriodStarts returns the start date of every period the subject already has.
// The import handler diffs against it so re-importing the same file is a no-op
// instead of duplicating the whole history.
func (r *Cycle) PeriodStarts(ctx context.Context, subject string) ([]string, error) {
	cur, err := r.periods.Find(ctx, bson.M{"subject": subject},
		options.Find().
			SetProjection(bson.M{"started_on": 1}).
			SetSort(bson.D{{Key: "started_on", Value: -1}}).
			SetLimit(maxPeriodsRead))
	if err != nil {
		return nil, err
	}
	// Decoded into a typed field rather than a map: a projected date read
	// through bson.M arrives as bson.DateTime, and a type assertion against
	// time.Time there fails on every document and silently returns nothing.
	var docs []struct {
		StartedOn time.Time `bson:"started_on"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.StartedOn.Format(dateLayout))
	}
	return out, nil
}

// SymptomKeys returns one key per check-in the subject already has, in the form
// SymptomKey builds. A day can hold several kinds, so a slice of days would
// lose the distinction; a slice of keys does not.
func (r *Cycle) SymptomKeys(ctx context.Context, subject string) ([]string, error) {
	cur, err := r.symptoms.Find(ctx, bson.M{"subject": subject},
		options.Find().
			SetProjection(bson.M{"recorded_on": 1, "kind": 1}).
			SetSort(bson.D{{Key: "recorded_on", Value: -1}}).
			SetLimit(maxSymptomsRead))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		RecordedOn time.Time `bson:"recorded_on"`
		Kind       string    `bson:"kind"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, SymptomKey(d.RecordedOn.Format(dateLayout), d.Kind))
	}
	return out, nil
}

// SymptomKey is the natural identity of a check-in: the day it applies to plus
// the kind. A member never logs the same kind twice for one day through the
// UI, so the pair is a safe duplicate test during an import.
func SymptomKey(recordedOn, kind string) string {
	return recordedOn + "\x00" + kind
}

// ReplaceRecords erases the subject's periods and check-ins and writes the
// supplied ones, atomically. Requires a replica set, as DeleteMember does.
func (r *Cycle) ReplaceRecords(ctx context.Context, subject string, ps []model.Period, ss []model.Symptom) error {
	sess, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer sess.EndSession(ctx)
	_, err = sess.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		if err := r.deleteRecords(ctx, subject); err != nil {
			return nil, err
		}
		if err := r.insertPeriods(ctx, ps); err != nil {
			return nil, err
		}
		return nil, r.insertSymptoms(ctx, ss)
	})
	return err
}

// AppendRecords writes the supplied periods and check-ins alongside whatever
// the subject already has, atomically.
func (r *Cycle) AppendRecords(ctx context.Context, ps []model.Period, ss []model.Symptom) error {
	sess, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer sess.EndSession(ctx)
	_, err = sess.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		if err := r.insertPeriods(ctx, ps); err != nil {
			return nil, err
		}
		return nil, r.insertSymptoms(ctx, ss)
	})
	return err
}

func (r *Cycle) deleteRecords(ctx context.Context, subject string) error {
	if _, err := r.periods.DeleteMany(ctx, bson.M{"subject": subject}); err != nil {
		return err
	}
	_, err := r.symptoms.DeleteMany(ctx, bson.M{"subject": subject})
	return err
}

func (r *Cycle) UpdateSymptom(ctx context.Context, subject string, id int64, s model.Symptom) error {
	result, err := r.symptoms.UpdateOne(ctx,
		bson.M{"_id": id, "subject": subject},
		bson.M{"$set": bson.M{"recorded_on": s.RecordedOn, "kind": s.Kind, "severity": s.Severity, "notes": s.Notes}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Cycle) DeleteSymptom(ctx context.Context, subject string, id int64) error {
	result, err := r.symptoms.DeleteOne(ctx, bson.M{"_id": id, "subject": subject})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePeriod removes a period outright. Scoped to the subject so one member can
// never delete another's record by guessing an id.
func (r *Cycle) DeletePeriod(ctx context.Context, subject string, id int64) error {
	result, err := r.periods.DeleteOne(ctx, bson.M{"_id": id, "subject": subject})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateInvite issues a short-lived numeric code an owner can hand to a partner.
func (r *Cycle) CreateInvite(ctx context.Context, owner string) (model.Invite, error) {
	code, err := randomDigits(6)
	if err != nil {
		return model.Invite{}, err
	}
	inv := model.Invite{Code: code, OwnerSubject: owner, ExpiresAt: time.Now().UTC().Add(15 * time.Minute)}
	if _, err := r.invites.InsertOne(ctx, inv); err != nil {
		return model.Invite{}, err
	}
	return inv, nil
}

// RedeemInvite links partnerSubject to the invite's owner and consumes the code.
// Fails with ErrNotFound if the code is unknown, already used, or expired.
func (r *Cycle) RedeemInvite(ctx context.Context, code, partnerSubject string) (string, error) {
	sess, err := r.client.StartSession()
	if err != nil {
		return "", err
	}
	defer sess.EndSession(ctx)
	owner, err := sess.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		var inv model.Invite
		err := r.invites.FindOneAndDelete(ctx, bson.M{"_id": code, "expires_at": bson.M{"$gt": time.Now().UTC()}}).Decode(&inv)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if inv.OwnerSubject == partnerSubject {
			return nil, errors.New("cannot link to your own account")
		}
		id, err := r.nextID(ctx, model.CollectionPartnerLinks)
		if err != nil {
			return nil, err
		}
		_, err = r.partnerLinks.InsertOne(ctx, model.PartnerLink{
			ID: id, OwnerSubject: inv.OwnerSubject, PartnerSubject: partnerSubject, CreatedAt: time.Now().UTC(),
		})
		if mongo.IsDuplicateKeyError(err) {
			return inv.OwnerSubject, nil
		}
		if err != nil {
			return nil, err
		}
		return inv.OwnerSubject, nil
	})
	if err != nil {
		return "", err
	}
	return owner.(string), nil
}

// PartnersOf lists who an owner has shared their status with.
func (r *Cycle) PartnersOf(ctx context.Context, owner string) ([]model.PartnerLink, error) {
	cur, err := r.partnerLinks.Find(ctx, bson.M{"owner_subject": owner})
	if err != nil {
		return nil, err
	}
	var out []model.PartnerLink
	err = cur.All(ctx, &out)
	return out, err
}

// SharedWithMe lists the owners who have shared their status with partnerSubject.
func (r *Cycle) SharedWithMe(ctx context.Context, partnerSubject string) ([]model.PartnerLink, error) {
	cur, err := r.partnerLinks.Find(ctx, bson.M{"partner_subject": partnerSubject})
	if err != nil {
		return nil, err
	}
	var out []model.PartnerLink
	err = cur.All(ctx, &out)
	return out, err
}

func (r *Cycle) RevokePartner(ctx context.Context, owner, partnerSubject string) error {
	result, err := r.partnerLinks.DeleteOne(ctx, bson.M{"owner_subject": owner, "partner_subject": partnerSubject})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteMember removes a member, all their periods/symptoms, and every partner
// link involving them (either side) atomically. Requires the mongo deployment to
// run as a replica set (multi-document transactions are not supported on a standalone node).
func (r *Cycle) DeleteMember(ctx context.Context, subject string) error {
	sess, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer sess.EndSession(ctx)
	_, err = sess.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		if _, err := r.periods.DeleteMany(ctx, bson.M{"subject": subject}); err != nil {
			return nil, err
		}
		if _, err := r.symptoms.DeleteMany(ctx, bson.M{"subject": subject}); err != nil {
			return nil, err
		}
		if _, err := r.partnerLinks.DeleteMany(ctx, bson.M{"$or": bson.A{
			bson.M{"owner_subject": subject}, bson.M{"partner_subject": subject},
		}}); err != nil {
			return nil, err
		}
		if _, err := r.members.DeleteOne(ctx, bson.M{"_id": subject}); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

func randomDigits(n int) (string, error) {
	max := 1
	for i := 0; i < n; i++ {
		max *= 10
	}
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	v := (int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])) % max
	if v < 0 {
		v = -v
	}
	return fmt.Sprintf("%0*d", n, v), nil
}

// nextID hands out a Postgres-serial-like auto-incrementing id via a counters collection,
// since Mongo document ids have no built-in sequential integer type.
func (r *Cycle) nextID(ctx context.Context, counterName string) (int64, error) {
	return r.reserveIDs(ctx, counterName, 1)
}

// reserveIDs claims n consecutive ids and returns the lowest. Bulk inserts use
// it so a file of thousands of records costs one counter round trip instead of
// one per document.
func (r *Cycle) reserveIDs(ctx context.Context, counterName string, n int) (int64, error) {
	var counter model.Counter
	err := r.counters.FindOneAndUpdate(ctx,
		bson.M{"_id": counterName},
		bson.M{"$inc": bson.M{"seq": int64(n)}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&counter)
	if err != nil {
		return 0, err
	}
	return counter.Seq - int64(n) + 1, nil
}
