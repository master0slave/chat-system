// Package realtime fans events out to WebSocket connections in this process (ADR 0004).
package realtime

import (
	"sync"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

// DefaultBufferSize is how many events a connection may fall behind before the Hub drops it.
const DefaultBufferSize = 16

const agentsTopic = "agents"

type subscriber struct {
	ch chan models.Event
}

// Hub implements usecases.MessageBroadcaster. Publish never blocks: a subscriber whose
// buffer is full is removed and its channel closed, so one slow client cannot hold up a case.
type Hub struct {
	mu         sync.Mutex
	bufferSize int
	topics     map[string]map[*subscriber]struct{}
}

var _ usecases.MessageBroadcaster = (*Hub)(nil)

func NewHub(bufferSize int) *Hub {
	return &Hub{bufferSize: bufferSize, topics: map[string]map[*subscriber]struct{}{}}
}

func (h *Hub) PublishCase(caseID string, e models.Event) { h.publish(caseTopic(caseID), e) }

func (h *Hub) PublishAgents(e models.Event) { h.publish(agentsTopic, e) }

// SubscribeCase returns a channel of events for one case and a cancel function.
// The channel is closed when cancel is called or when the Hub drops a slow subscriber.
func (h *Hub) SubscribeCase(caseID string) (<-chan models.Event, func()) {
	return h.subscribe(caseTopic(caseID))
}

// SubscribeAgents returns a channel of case-list events for agents. See SubscribeCase.
func (h *Hub) SubscribeAgents() (<-chan models.Event, func()) {
	return h.subscribe(agentsTopic)
}

func caseTopic(caseID string) string { return "case:" + caseID }

func (h *Hub) subscribe(topic string) (<-chan models.Event, func()) {
	s := &subscriber{ch: make(chan models.Event, h.bufferSize)}
	h.mu.Lock()
	if h.topics[topic] == nil {
		h.topics[topic] = map[*subscriber]struct{}{}
	}
	h.topics[topic][s] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.remove(topic, s)
	}
	return s.ch, cancel
}

// remove must be called with h.mu held. It is safe to call more than once.
func (h *Hub) remove(topic string, s *subscriber) {
	subs := h.topics[topic]
	if _, ok := subs[s]; !ok {
		return
	}
	delete(subs, s)
	close(s.ch)
	if len(subs) == 0 {
		delete(h.topics, topic)
	}
}

func (h *Hub) publish(topic string, e models.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.topics[topic] {
		select {
		case s.ch <- e:
		default:
			h.remove(topic, s)
		}
	}
}
