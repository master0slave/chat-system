package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
	"supportchat/repositories"
)

func sampleCase(id, customerID string, status models.CaseStatus, updatedAt time.Time) models.Case {
	return models.Case{
		ID:         id,
		Subject:    "help",
		CustomerID: customerID,
		Participants: []models.Participant{
			{UserID: customerID, Name: "Ann", Role: models.RoleCustomer, JoinedAt: t0},
		},
		Status:    status,
		CreatedAt: t0,
		UpdatedAt: updatedAt,
		Version:   1,
	}
}

func TestCaseRepositoryInsertAndGet(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))
	ctx := context.Background()
	closedAt := t0.Add(time.Hour)
	c := sampleCase("c1", "customer:ann", models.StatusClosed, t0)
	c.ClosedAt = &closedAt

	require.NoError(t, repo.Insert(ctx, c))
	got, err := repo.Get(ctx, "c1")

	require.NoError(t, err)
	assert.Equal(t, c, got)
}

func TestCaseRepositoryGetUnknown(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))

	_, err := repo.Get(context.Background(), "missing")

	assert.ErrorIs(t, err, models.ErrNotFound)
}

func TestCaseRepositoryUpdateChecksVersion(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))
	ctx := context.Background()
	c := sampleCase("c1", "customer:ann", models.StatusWaiting, t0)
	require.NoError(t, repo.Insert(ctx, c))

	c.Status = models.StatusOpen
	require.NoError(t, repo.Update(ctx, c))

	stored, err := repo.Get(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, models.StatusOpen, stored.Status)
	assert.Equal(t, 2, stored.Version)

	stale := c // still version 1
	stale.Subject = "overwritten"
	assert.ErrorIs(t, repo.Update(ctx, stale), models.ErrConflict)

	assert.ErrorIs(t, repo.Update(ctx, sampleCase("missing", "x", models.StatusOpen, t0)), models.ErrNotFound)
}

func TestCaseRepositoryListFiltersAndSorts(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))
	ctx := context.Background()
	require.NoError(t, repo.Insert(ctx, sampleCase("old", "customer:ann", models.StatusWaiting, t0)))
	require.NoError(t, repo.Insert(ctx, sampleCase("new", "customer:ann", models.StatusOpen, t0.Add(2*time.Minute))))
	require.NoError(t, repo.Insert(ctx, sampleCase("cats", "customer:cat", models.StatusWaiting, t0.Add(time.Minute))))

	all, err := repo.List(ctx, usecases.CaseFilter{})
	require.NoError(t, err)
	assert.Equal(t, []string{"new", "cats", "old"}, caseIDs(all))

	anns, err := repo.List(ctx, usecases.CaseFilter{CustomerID: "customer:ann"})
	require.NoError(t, err)
	assert.Equal(t, []string{"new", "old"}, caseIDs(anns))

	waiting, err := repo.List(ctx, usecases.CaseFilter{Status: models.StatusWaiting})
	require.NoError(t, err)
	assert.Equal(t, []string{"cats", "old"}, caseIDs(waiting))

	none, err := repo.List(ctx, usecases.CaseFilter{CustomerID: "customer:nobody"})
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

func caseIDs(cs []models.Case) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func TestCaseRepositoryAddParticipant(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))
	ctx := context.Background()
	require.NoError(t, repo.Insert(ctx, sampleCase("c1", "customer:ann", models.StatusWaiting, t0)))
	bob := models.Participant{UserID: "agent:bob", Name: "Bob", Role: models.RoleAgent, JoinedAt: t0.Add(time.Minute)}

	before, err := repo.AddParticipant(ctx, "c1", bob)

	require.NoError(t, err)
	assert.Equal(t, models.StatusWaiting, before.Status, "returns the case as it was before the change")
	assert.Equal(t, 1, before.Version)
	stored, err := repo.Get(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, models.StatusOpen, stored.Status)
	assert.Equal(t, bob, stored.Participants[1])
	assert.Equal(t, t0.Add(time.Minute), stored.UpdatedAt)
	assert.Equal(t, 2, stored.Version)

	_, err = repo.AddParticipant(ctx, "c1", bob)
	assert.ErrorIs(t, err, models.ErrConflict, "already a participant")

	_, err = repo.AddParticipant(ctx, "missing", bob)
	assert.ErrorIs(t, err, models.ErrNotFound)

	require.NoError(t, repo.Insert(ctx, sampleCase("closed", "customer:ann", models.StatusClosed, t0)))
	_, err = repo.AddParticipant(ctx, "closed", bob)
	assert.ErrorIs(t, err, models.ErrConflict, "closed case")
}

func TestCaseRepositoryTouchForParticipant(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))
	ctx := context.Background()
	require.NoError(t, repo.Insert(ctx, sampleCase("c1", "customer:ann", models.StatusOpen, t0)))
	later := t0.Add(time.Hour)

	require.NoError(t, repo.TouchForParticipant(ctx, "c1", "customer:ann", later))

	stored, err := repo.Get(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, later, stored.UpdatedAt)
	assert.Equal(t, 1, stored.Version, "touching does not compete with other writers")

	assert.ErrorIs(t, repo.TouchForParticipant(ctx, "c1", "agent:dan", later), models.ErrConflict, "not a participant")
	assert.ErrorIs(t, repo.TouchForParticipant(ctx, "missing", "customer:ann", later), models.ErrNotFound)
	require.NoError(t, repo.Insert(ctx, sampleCase("closed", "customer:ann", models.StatusClosed, t0)))
	assert.ErrorIs(t, repo.TouchForParticipant(ctx, "closed", "customer:ann", later), models.ErrConflict, "closed case")
}
