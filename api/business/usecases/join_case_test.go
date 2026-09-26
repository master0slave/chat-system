package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestFirstAgentJoinOpensTheCase(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.clock.now = t0.Add(time.Minute)

	c, err := e.svc.JoinCase(context.Background(), bob, opened.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusOpen, c.Status)
	assert.True(t, c.HasParticipant(bob.ID))
	assert.Equal(t, t0.Add(time.Minute), c.UpdatedAt)
	assert.Equal(t, c, e.cases.stored(t, opened.ID))

	msgs := e.messages.inCase(opened.ID)
	last := msgs[len(msgs)-1]
	assert.Equal(t, models.KindSystem, last.Kind)
	assert.Equal(t, "Bob joined the case", last.Body)

	assert.Equal(t, []string{
		"case:" + opened.ID + " participant.joined",
		"case:" + opened.ID + " message.created",
		"agents case.status_changed",
	}, e.events.types())
}

func TestSecondAgentJoinKeepsCaseOpenAndDoesNotRepeatStatusEvent(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)

	c, err := e.svc.JoinCase(context.Background(), dan, opened.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusOpen, c.Status)
	assert.Len(t, c.Participants, 3)
	assert.NotContains(t, e.events.types(), "agents case.status_changed")
}

func TestJoinTwiceHasNoEffect(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	before := e.messages.inCase(opened.ID)

	c, err := e.svc.JoinCase(context.Background(), bob, opened.ID)

	require.NoError(t, err)
	assert.Len(t, c.Participants, 2)
	assert.Equal(t, before, e.messages.inCase(opened.ID))
	assert.Empty(t, e.events.types())
}

func TestJoinCaseErrors(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")

	_, err := e.svc.JoinCase(context.Background(), ann, opened.ID)
	assert.ErrorIs(t, err, models.ErrForbidden, "customers cannot join")

	_, err = e.svc.JoinCase(context.Background(), bob, "missing")
	assert.ErrorIs(t, err, models.ErrNotFound)
}

// Review Focus: two agents join the same waiting case at the same moment.
func TestConcurrentJoinsKeepBothAgents(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	// Dan's join saves between Bob's read and Bob's write.
	e.cases.beforeWrite = func() { e.join(t, dan, opened.ID) }

	c, err := e.svc.JoinCase(context.Background(), bob, opened.ID)

	require.NoError(t, err)
	assert.True(t, c.HasParticipant(bob.ID))
	assert.True(t, c.HasParticipant(dan.ID))
	assert.Len(t, e.cases.stored(t, opened.ID).Participants, 3)
}

func TestJoinLosesRaceWithClose(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	// Bob closes the case between Dan's read and Dan's write.
	e.cases.beforeWrite = func() { e.close(t, bob, opened.ID) }

	_, err := e.svc.JoinCase(context.Background(), dan, opened.ID)

	assert.ErrorIs(t, err, models.ErrCaseClosed)
	assert.False(t, e.cases.stored(t, opened.ID).HasParticipant(dan.ID))
}

// A double click: the same agent's first join request saves between the second one's read and write.
func TestJoinRacingItselfHasNoEffect(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.cases.beforeWrite = func() { e.join(t, bob, opened.ID) }

	c, err := e.svc.JoinCase(context.Background(), bob, opened.ID)

	require.NoError(t, err)
	assert.Len(t, c.Participants, 2)
	assert.Len(t, e.cases.stored(t, opened.ID).Participants, 2)
	assert.Empty(t, e.events.types(), "only the first request announces the join")
}
