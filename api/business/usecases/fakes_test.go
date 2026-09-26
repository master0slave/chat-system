package usecases_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

var (
	ann = models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}
	cat = models.User{ID: "customer:cat", Name: "Cat", Role: models.RoleCustomer}
	bob = models.User{ID: "agent:bob", Name: "Bob", Role: models.RoleAgent}
	dan = models.User{ID: "agent:dan", Name: "Dan", Role: models.RoleAgent}

	t0 = time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
)

// fakeCaseRepo is an in-memory CaseRepository with the same version check as the Mongo one.
type fakeCaseRepo struct {
	mu    sync.Mutex
	cases map[string]models.Case
	// beforeUpdate, if set, runs inside Update before the version check.
	// Tests use it to simulate another request saving first.
	beforeUpdate func()
}

func newFakeCaseRepo() *fakeCaseRepo { return &fakeCaseRepo{cases: map[string]models.Case{}} }

func (r *fakeCaseRepo) Insert(_ context.Context, c models.Case) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cases[c.ID] = clone(c)
	return nil
}

func (r *fakeCaseRepo) Get(_ context.Context, id string) (models.Case, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.cases[id]
	if !ok {
		return models.Case{}, models.ErrNotFound
	}
	return clone(c), nil
}

func (r *fakeCaseRepo) Update(_ context.Context, c models.Case) error {
	if hook := r.beforeUpdate; hook != nil {
		r.beforeUpdate = nil
		hook()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.cases[c.ID]
	if !ok {
		return models.ErrNotFound
	}
	if stored.Version != c.Version {
		return models.ErrConflict
	}
	c = clone(c)
	c.Version++
	r.cases[c.ID] = c
	return nil
}

func (r *fakeCaseRepo) List(_ context.Context, f usecases.CaseFilter) ([]models.Case, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Case
	for _, c := range r.cases {
		if (f.CustomerID == "" || c.CustomerID == f.CustomerID) && (f.Status == "" || c.Status == f.Status) {
			out = append(out, clone(c))
		}
	}
	slices.SortFunc(out, func(a, b models.Case) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return out, nil
}

func (r *fakeCaseRepo) stored(t *testing.T, id string) models.Case {
	t.Helper()
	c, err := r.Get(context.Background(), id)
	require.NoError(t, err)
	return c
}

func clone(c models.Case) models.Case {
	c.Participants = slices.Clone(c.Participants)
	return c
}

type fakeMessageRepo struct {
	mu       sync.Mutex
	messages []models.Message
}

func (r *fakeMessageRepo) Insert(_ context.Context, m models.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, m)
	return nil
}

func (r *fakeMessageRepo) List(_ context.Context, q usecases.MessageQuery) ([]models.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Message
	for _, m := range slices.Backward(r.messages) {
		if m.CaseID == q.CaseID && (q.BeforeID == "" || m.ID < q.BeforeID) && len(out) < q.Limit {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *fakeMessageRepo) inCase(caseID string) []models.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Message
	for _, m := range r.messages {
		if m.CaseID == caseID {
			out = append(out, m)
		}
	}
	return out
}

type published struct {
	Topic string // "case:<id>" or "agents"
	Event models.Event
}

type recordingBroadcaster struct {
	mu     sync.Mutex
	events []published
}

func (b *recordingBroadcaster) PublishCase(caseID string, e models.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, published{Topic: "case:" + caseID, Event: e})
}

func (b *recordingBroadcaster) PublishAgents(e models.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, published{Topic: "agents", Event: e})
}

// types returns "topic type" pairs, which keeps assertions short.
func (b *recordingBroadcaster) types() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, p := range b.events {
		out = append(out, p.Topic+" "+string(p.Event.Type))
	}
	return out
}

func (b *recordingBroadcaster) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = nil
}

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

// sequentialIDs returns id-0001, id-0002, … which sort in creation order like UUIDv7.
type sequentialIDs struct {
	mu sync.Mutex
	n  int
}

func (g *sequentialIDs) NewID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return fmt.Sprintf("id-%04d", g.n)
}

type env struct {
	cases    *fakeCaseRepo
	messages *fakeMessageRepo
	events   *recordingBroadcaster
	clock    *fixedClock
	svc      usecases.CaseService
}

func newEnv() *env {
	e := &env{
		cases:    newFakeCaseRepo(),
		messages: &fakeMessageRepo{},
		events:   &recordingBroadcaster{},
		clock:    &fixedClock{now: t0},
	}
	e.svc = usecases.NewCaseService(usecases.CaseDeps{
		Cases:       e.cases,
		Messages:    e.messages,
		Broadcaster: e.events,
		Clock:       e.clock,
		IDs:         &sequentialIDs{},
	})
	return e
}

// openCase is a Given step: Ann opens a case, and the recorded events are cleared.
func (e *env) openCase(t *testing.T, customer models.User, question string) models.Case {
	t.Helper()
	c, _, err := e.svc.OpenCase(context.Background(), customer, question)
	require.NoError(t, err)
	e.events.reset()
	return c
}

func (e *env) join(t *testing.T, agent models.User, caseID string) {
	t.Helper()
	_, err := e.svc.JoinCase(context.Background(), agent, caseID)
	require.NoError(t, err)
	e.events.reset()
}

func (e *env) close(t *testing.T, agent models.User, caseID string) {
	t.Helper()
	_, err := e.svc.CloseCase(context.Background(), agent, caseID)
	require.NoError(t, err)
	e.events.reset()
}

func bodies(ms []models.Message) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Body)
	}
	return out
}

