package usecases

import (
	"context"
	"time"

	"supportchat/business/models"
)

// Driven ports: what the case use cases need from the outside world.

type CaseFilter struct {
	CustomerID string            // empty means every customer
	Status     models.CaseStatus // empty means every status
}

type CaseRepository interface {
	Insert(ctx context.Context, c models.Case) error
	// Get returns models.ErrNotFound when the case does not exist.
	Get(ctx context.Context, id string) (models.Case, error)
	// Update saves c only if the stored version equals c.Version, and stores c.Version+1.
	// It returns models.ErrConflict when another request saved first, and models.ErrNotFound for an unknown id.
	Update(ctx context.Context, c models.Case) error
	// List returns matching cases, newest UpdatedAt first.
	List(ctx context.Context, f CaseFilter) ([]models.Case, error)
}

type MessageQuery struct {
	CaseID   string
	BeforeID string // empty means start from the newest message
	Limit    int
}

type MessageRepository interface {
	Insert(ctx context.Context, m models.Message) error
	// List returns messages whose ID is lower than BeforeID, newest first.
	// This works because IDGenerator returns IDs that sort in creation order.
	List(ctx context.Context, q MessageQuery) ([]models.Message, error)
}

// MessageBroadcaster notifies connected clients. It must not block: the database is the source of truth.
type MessageBroadcaster interface {
	PublishCase(caseID string, e models.Event)
	PublishAgents(e models.Event)
}

type Clock interface {
	Now() time.Time
}

// IDGenerator returns unique IDs that sort (as strings) in creation order.
type IDGenerator interface {
	NewID() string
}
