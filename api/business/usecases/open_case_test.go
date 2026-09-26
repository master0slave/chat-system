package usecases_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestOpenCase(t *testing.T) {
	e := newEnv()

	c, m, err := e.svc.OpenCase(context.Background(), ann, "  How do I reset my password?  ")

	require.NoError(t, err)
	assert.Equal(t, models.StatusWaiting, c.Status)
	assert.Equal(t, "How do I reset my password?", c.Subject)
	assert.Equal(t, ann.ID, c.CustomerID)
	assert.Equal(t, []models.Participant{{UserID: ann.ID, Name: "Ann", Role: models.RoleCustomer, JoinedAt: t0}}, c.Participants)
	assert.Equal(t, t0, c.CreatedAt)
	assert.Equal(t, t0, c.UpdatedAt)

	assert.Equal(t, c.ID, m.CaseID)
	assert.Equal(t, models.KindText, m.Kind)
	assert.Equal(t, "How do I reset my password?", m.Body)
	assert.Equal(t, ann.ID, m.SenderID)

	assert.Equal(t, c, e.cases.stored(t, c.ID))
	assert.Equal(t, []models.Message{m}, e.messages.inCase(c.ID))
	assert.Equal(t, []string{"agents case.created"}, e.events.types())
}

func TestOpenCaseRejectsAgents(t *testing.T) {
	e := newEnv()

	_, _, err := e.svc.OpenCase(context.Background(), bob, "hello")

	assert.ErrorIs(t, err, models.ErrForbidden)
	assert.Empty(t, e.events.types())
}

func TestOpenCaseRejectsEmptyQuestion(t *testing.T) {
	e := newEnv()

	_, _, err := e.svc.OpenCase(context.Background(), ann, "   ")

	assert.ErrorIs(t, err, models.ErrInvalidInput)
}
