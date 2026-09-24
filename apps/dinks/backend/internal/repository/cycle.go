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

func (r *Cycle) Periods(ctx context.Context, subject string) ([]model.Period, error) {
	cur, err := r.periods.Find(ctx, bson.M{"subject": subject},
		options.Find().SetSort(bson.D{{Key: "started_on", Value: -1}}).SetLimit(100))
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
		options.Find().SetSort(bson.D{{Key: "recorded_on", Value: -1}}).SetLimit(100))
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
	var counter model.Counter
	err := r.counters.FindOneAndUpdate(ctx,
		bson.M{"_id": counterName},
		bson.M{"$inc": bson.M{"seq": int64(1)}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&counter)
	if err != nil {
		return 0, err
	}
	return counter.Seq, nil
}
