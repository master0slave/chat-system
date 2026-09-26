package usecases

import (
	"context"

	"supportchat/business/models"
)

// Driving ports: what handlers may call. Each one is implemented in its own file.

type OpenCase interface {
	OpenCase(ctx context.Context, actor models.User, question string) (models.Case, models.Message, error)
}

type JoinCase interface {
	JoinCase(ctx context.Context, actor models.User, caseID string) (models.Case, error)
}

type SendMessage interface {
	SendMessage(ctx context.Context, actor models.User, caseID, body string) (models.Message, error)
}

type CloseCase interface {
	CloseCase(ctx context.Context, actor models.User, caseID string) (models.Case, error)
}

type ListCases interface {
	// ListCases returns the cases the actor may see, newest UpdatedAt first. An empty status means any status.
	ListCases(ctx context.Context, actor models.User, status models.CaseStatus) ([]models.Case, error)
}

type GetCase interface {
	GetCase(ctx context.Context, actor models.User, caseID string) (models.Case, error)
}

type ListMessages interface {
	// ListMessages returns messages older than beforeID (all if empty), newest first.
	// limit 0 means DefaultMessageLimit; values above MaxMessageLimit are capped.
	ListMessages(ctx context.Context, actor models.User, caseID, beforeID string, limit int) ([]models.Message, error)
}

type CaseService interface {
	OpenCase
	JoinCase
	SendMessage
	CloseCase
	ListCases
	GetCase
	ListMessages
}
