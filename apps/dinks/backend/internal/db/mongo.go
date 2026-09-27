// Package db owns the MongoDB connection lifecycle.
package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"dinks/internal/logging"
)

// Connect dials MongoDB and verifies the connection with a ping, so a
// misconfigured URI fails at startup rather than on the first request.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, err
	}
	logging.Log.Info("mongo connected", "uri", logging.RedactURI(uri))
	return client, nil
}
