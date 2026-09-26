package repositories_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"supportchat/repositories"
)

var t0 = time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)

// testDB connects to the Mongo started by `make up` and gives each test its own database.
func testDB(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := repositories.Connect(ctx, uri)
	require.NoError(t, err, "cannot reach MongoDB at %s; run `make up`", uri)

	db := client.Database(fmt.Sprintf("test_%d", time.Now().UnixNano()))
	require.NoError(t, repositories.EnsureIndexes(ctx, db))
	t.Cleanup(func() {
		ctx := context.Background()
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	})
	return db
}
