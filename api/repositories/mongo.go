// Package repositories implements the driven repository ports with MongoDB.
package repositories

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	casesCollection    = "cases"
	messagesCollection = "messages"
)

// Connect opens a client and pings the server, so a wrong URI fails at startup.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return client, nil
}

// EnsureIndexes creates the indexes the queries in this package rely on. It is safe to call on every start.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(casesCollection).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "updatedAt", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "updatedAt", Value: -1}}},
		{Keys: bson.D{{Key: "customerId", Value: 1}, {Key: "updatedAt", Value: -1}}},
	})
	if err != nil {
		return err
	}
	_, err = db.Collection(messagesCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "caseId", Value: 1}, {Key: "_id", Value: -1}},
	})
	return err
}
