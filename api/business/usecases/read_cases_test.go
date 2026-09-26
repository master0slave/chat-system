package usecases_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

func TestListCases(t *testing.T) {
	e := newEnv()
	first := e.openCase(t, ann, "first")
	e.clock.now = t0.Add(time.Minute)
	catsCase := e.openCase(t, cat, "cat's question")
	e.clock.now = t0.Add(2 * time.Minute)
	e.join(t, bob, first.ID) // first is now the most recently updated

	agentView, err := e.svc.ListCases(context.Background(), bob, "")
	require.NoError(t, err)
	assert.Equal(t, []string{first.ID, catsCase.ID}, ids(agentView), "agents see all cases, newest update first")

	annView, err := e.svc.ListCases(context.Background(), ann, "")
	require.NoError(t, err)
	assert.Equal(t, []string{first.ID}, ids(annView), "customers see only their own cases")

	waiting, err := e.svc.ListCases(context.Background(), bob, models.StatusWaiting)
	require.NoError(t, err)
	assert.Equal(t, []string{catsCase.ID}, ids(waiting))

	_, err = e.svc.ListCases(context.Background(), bob, "archived")
	assert.ErrorIs(t, err, models.ErrInvalidInput)
}

func TestGetCase(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")

	c, err := e.svc.GetCase(context.Background(), ann, opened.ID)
	require.NoError(t, err)
	assert.Equal(t, opened.ID, c.ID)

	_, err = e.svc.GetCase(context.Background(), bob, opened.ID)
	assert.NoError(t, err, "agents can view any case, even before joining")

	_, err = e.svc.GetCase(context.Background(), cat, opened.ID)
	assert.ErrorIs(t, err, models.ErrForbidden)

	_, err = e.svc.GetCase(context.Background(), ann, "missing")
	assert.ErrorIs(t, err, models.ErrNotFound)
}

func TestListMessagesPagesNewestFirst(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "message 0")
	for i := 1; i <= 4; i++ {
		_, err := e.svc.SendMessage(context.Background(), ann, opened.ID, fmt.Sprintf("message %d", i))
		require.NoError(t, err)
	}

	page1, err := e.svc.ListMessages(context.Background(), ann, opened.ID, "", 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"message 4", "message 3"}, bodies(page1))

	page2, err := e.svc.ListMessages(context.Background(), ann, opened.ID, page1[1].ID, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"message 2", "message 1"}, bodies(page2))
}

func TestListMessagesLimits(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	for i := range usecases.MaxMessageLimit + 10 {
		_, err := e.svc.SendMessage(context.Background(), ann, opened.ID, fmt.Sprint(i))
		require.NoError(t, err)
	}

	def, err := e.svc.ListMessages(context.Background(), ann, opened.ID, "", 0)
	require.NoError(t, err)
	assert.Len(t, def, usecases.DefaultMessageLimit)

	capped, err := e.svc.ListMessages(context.Background(), ann, opened.ID, "", 1000)
	require.NoError(t, err)
	assert.Len(t, capped, usecases.MaxMessageLimit)

	_, err = e.svc.ListMessages(context.Background(), ann, opened.ID, "", -1)
	assert.ErrorIs(t, err, models.ErrInvalidInput)

	_, err = e.svc.ListMessages(context.Background(), cat, opened.ID, "", 0)
	assert.ErrorIs(t, err, models.ErrForbidden)
}

func ids(cs []models.Case) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}
