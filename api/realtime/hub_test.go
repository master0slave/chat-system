package realtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/realtime"
)

func event(caseID, body string) models.Event {
	return models.Event{Type: models.EventMessageCreated, CaseID: caseID, Data: body}
}

func receive(t *testing.T, ch <-chan models.Event) models.Event {
	t.Helper()
	select {
	case e, ok := <-ch:
		require.True(t, ok, "channel was closed")
		return e
	case <-time.After(time.Second):
		t.Fatal("no event received")
		return models.Event{}
	}
}

func assertNothingWaiting(t *testing.T, ch <-chan models.Event) {
	t.Helper()
	select {
	case e := <-ch:
		t.Fatalf("unexpected event %+v", e)
	default:
	}
}

func TestPublishCaseReachesEverySubscriberOfThatCaseOnly(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	ann, cancelAnn := hub.SubscribeCase("c1")
	defer cancelAnn()
	bob, cancelBob := hub.SubscribeCase("c1")
	defer cancelBob()
	other, cancelOther := hub.SubscribeCase("c2")
	defer cancelOther()

	hub.PublishCase("c1", event("c1", "hi"))

	assert.Equal(t, event("c1", "hi"), receive(t, ann))
	assert.Equal(t, event("c1", "hi"), receive(t, bob))
	assertNothingWaiting(t, other)
}

func TestPublishAgentsReachesAgentSubscribersOnly(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	agents, cancelAgents := hub.SubscribeAgents()
	defer cancelAgents()
	caseSub, cancelCase := hub.SubscribeCase("c1")
	defer cancelCase()

	created := models.Event{Type: models.EventCaseCreated, CaseID: "c1"}
	hub.PublishAgents(created)

	assert.Equal(t, created, receive(t, agents))
	assertNothingWaiting(t, caseSub)
}

func TestCancelClosesChannelAndCanBeCalledTwice(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	ch, cancel := hub.SubscribeCase("c1")

	cancel()
	cancel()
	hub.PublishCase("c1", event("c1", "after cancel"))

	_, ok := <-ch
	assert.False(t, ok)
}

// Review Focus: a client that stops reading must not block the others in the case.
func TestSlowSubscriberIsDroppedWithoutBlockingOthers(t *testing.T) {
	hub := realtime.NewHub(2)
	slow, cancelSlow := hub.SubscribeCase("c1")
	defer cancelSlow()
	fast, cancelFast := hub.SubscribeCase("c1")
	defer cancelFast()

	for i := range 3 { // one more than the buffer
		hub.PublishCase("c1", event("c1", string(rune('a'+i))))
		receive(t, fast)
	}

	// slow still holds the 2 buffered events, then its channel is closed.
	assert.Equal(t, event("c1", "a"), receive(t, slow))
	assert.Equal(t, event("c1", "b"), receive(t, slow))
	_, ok := <-slow
	assert.False(t, ok, "slow subscriber should be dropped")

	hub.PublishCase("c1", event("c1", "d"))
	assert.Equal(t, event("c1", "d"), receive(t, fast), "fast subscriber keeps receiving")
}
