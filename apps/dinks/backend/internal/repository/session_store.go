package repository

import (
	"context"
	"errors"
	"time"

	"dinks/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// SessionStore implements scs.Store on top of Mongo, backing the alexedwards/scs
// session manager. Expired documents are reaped by the TTL index created in Migrate.
type SessionStore struct {
	sessions *mongo.Collection
}

func NewSessionStore(client *mongo.Client, dbName string) *SessionStore {
	return &SessionStore{sessions: client.Database(dbName).Collection(model.CollectionSessions)}
}

func (s *SessionStore) Find(token string) ([]byte, bool, error) {
	var sess model.Session
	err := s.sessions.FindOne(context.Background(), bson.M{"_id": token}).Decode(&sess)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if sess.Expiry.Before(time.Now()) {
		return nil, false, nil
	}
	return sess.Data, true, nil
}

func (s *SessionStore) Commit(token string, b []byte, expiry time.Time) error {
	_, err := s.sessions.UpdateByID(context.Background(), token,
		bson.M{"$set": bson.M{"data": b, "expiry": expiry}},
		options.UpdateOne().SetUpsert(true))
	return err
}

func (s *SessionStore) Delete(token string) error {
	_, err := s.sessions.DeleteOne(context.Background(), bson.M{"_id": token})
	return err
}
