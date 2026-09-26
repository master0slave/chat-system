package usecases_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestParticipantsSendMessages(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	e.clock.now = t0.Add(2 * time.Minute)

	m, err := e.svc.SendMessage(context.Background(), bob, opened.ID, " Click 'Forgot password' ")

	require.NoError(t, err)
	assert.Equal(t, "Click 'Forgot password'", m.Body)
	assert.Equal(t, models.KindText, m.Kind)
	assert.Equal(t, bob.ID, m.SenderID)
	assert.Equal(t, "Bob", m.SenderName)
	assert.Equal(t, models.RoleAgent, m.SenderRole)
	assert.Equal(t, t0.Add(2*time.Minute), e.cases.stored(t, opened.ID).UpdatedAt)
	assert.Contains(t, e.messages.inCase(opened.ID), m)
	assert.Equal(t, []string{"case:" + opened.ID + " message.created"}, e.events.types())

	_, err = e.svc.SendMessage(context.Background(), ann, opened.ID, "Thanks!")
	assert.NoError(t, err, "the customer can reply too")
}

func TestSendMessageErrors(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	closed := e.openCase(t, ann, "old question")
	e.join(t, bob, closed.ID)
	e.close(t, bob, closed.ID)

	cases := []struct {
		name   string
		actor  models.User
		caseID string
		body   string
		want   error
	}{
		{"agent who has not joined", dan, opened.ID, "hi", models.ErrForbidden},
		{"another customer", cat, opened.ID, "hi", models.ErrForbidden},
		{"closed case", ann, closed.ID, "hi", models.ErrCaseClosed},
		{"empty body", ann, opened.ID, "  ", models.ErrInvalidInput},
		{"body too long", ann, opened.ID, strings.Repeat("a", 2001), models.ErrInvalidInput},
		{"unknown case", ann, "missing", "hi", models.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.svc.SendMessage(context.Background(), tc.actor, tc.caseID, tc.body)
			assert.ErrorIs(t, err, tc.want)
		})
	}
	assert.Empty(t, e.events.types())
}

func TestSendMessageLosesRaceWithClose(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	// Bob closes the case between Ann's read and Ann's write.
	e.cases.beforeUpdate = func() { e.close(t, bob, opened.ID) }

	_, err := e.svc.SendMessage(context.Background(), ann, opened.ID, "one more thing")

	assert.ErrorIs(t, err, models.ErrCaseClosed)
}
