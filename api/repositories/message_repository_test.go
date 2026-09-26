package repositories_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
	"supportchat/pkg/ids"
	"supportchat/repositories"
)

func TestMessageRepositoryInsertAndList(t *testing.T) {
	repo := repositories.NewMongoMessageRepository(testDB(t))
	ctx := context.Background()
	m := models.Message{
		ID: "m1", CaseID: "c1", SenderID: "customer:ann", SenderName: "Ann",
		SenderRole: models.RoleCustomer, Kind: models.KindText, Body: "hello", CreatedAt: t0,
	}
	require.NoError(t, repo.Insert(ctx, m))
	require.NoError(t, repo.Insert(ctx, models.Message{ID: "m2", CaseID: "other-case", Body: "x", CreatedAt: t0}))

	got, err := repo.List(ctx, usecases.MessageQuery{CaseID: "c1", Limit: 10})

	require.NoError(t, err)
	assert.Equal(t, []models.Message{m}, got)
}

// Review Focus: several messages in the same millisecond must page without gaps or repeats.
func TestMessageRepositoryPagesMessagesFromTheSameMillisecond(t *testing.T) {
	repo := repositories.NewMongoMessageRepository(testDB(t))
	ctx := context.Background()
	gen := ids.UUIDv7{}
	for i := range 5 {
		require.NoError(t, repo.Insert(ctx, models.Message{
			ID: gen.NewID(), CaseID: "c1", Body: fmt.Sprint(i), CreatedAt: t0, // same timestamp for all
		}))
	}

	var seen []string
	before := ""
	for {
		page, err := repo.List(ctx, usecases.MessageQuery{CaseID: "c1", BeforeID: before, Limit: 2})
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}
		for _, m := range page {
			seen = append(seen, m.Body)
		}
		before = page[len(page)-1].ID
	}

	assert.Equal(t, []string{"4", "3", "2", "1", "0"}, seen)
}

func TestMessageRepositoryEmptyListIsNotNil(t *testing.T) {
	repo := repositories.NewMongoMessageRepository(testDB(t))

	got, err := repo.List(context.Background(), usecases.MessageQuery{CaseID: "none", Limit: 10})

	require.NoError(t, err)
	assert.NotNil(t, got)
}
