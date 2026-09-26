package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestJoinedAgentClosesCase(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	closedAt := t0.Add(time.Hour)
	e.clock.now = closedAt

	c, err := e.svc.CloseCase(context.Background(), bob, opened.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusClosed, c.Status)
	require.NotNil(t, c.ClosedAt)
	assert.Equal(t, closedAt, *c.ClosedAt)
	assert.Equal(t, models.StatusClosed, e.cases.stored(t, opened.ID).Status)

	msgs := e.messages.inCase(opened.ID)
	assert.Equal(t, "Case closed by Bob", msgs[len(msgs)-1].Body)
	assert.Equal(t, models.KindSystem, msgs[len(msgs)-1].Kind)

	assert.Equal(t, []string{
		"case:" + opened.ID + " message.created",
		"case:" + opened.ID + " case.closed",
		"agents case.status_changed",
	}, e.events.types())
}

func TestCloseCaseErrors(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)

	_, err := e.svc.CloseCase(context.Background(), ann, opened.ID)
	assert.ErrorIs(t, err, models.ErrForbidden, "customers cannot close")

	_, err = e.svc.CloseCase(context.Background(), dan, opened.ID)
	assert.ErrorIs(t, err, models.ErrForbidden, "agents must join before closing")

	_, err = e.svc.CloseCase(context.Background(), bob, "missing")
	assert.ErrorIs(t, err, models.ErrNotFound)

	e.close(t, bob, opened.ID)
	_, err = e.svc.CloseCase(context.Background(), bob, opened.ID)
	assert.ErrorIs(t, err, models.ErrCaseClosed, "closing twice")
}

func TestClosedCaseCannotBeJoined(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	e.close(t, bob, opened.ID)

	_, err := e.svc.JoinCase(context.Background(), dan, opened.ID)

	assert.ErrorIs(t, err, models.ErrCaseClosed)
}
