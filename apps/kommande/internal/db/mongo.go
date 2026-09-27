package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"kommande/internal/logging"
)

func Connect(uri string) (*mongo.Client, error) {
	// In v2, Connect takes only options (no context)
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	logging.Log.Info("connected to mongodb", "uri", logging.RedactURI(uri))
	return client, nil
}
