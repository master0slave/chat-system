package usecases

import (
	"context"

	"supportchat/business/models"
)

const (
	DefaultMessageLimit = 50
	MaxMessageLimit     = 100
)

func (s *caseService) ListMessages(ctx context.Context, actor models.User, caseID, beforeID string, limit int) ([]models.Message, error) {
	if limit < 0 {
		return nil, models.ErrInvalidInput
	}
	if limit == 0 {
		limit = DefaultMessageLimit
	}
	limit = min(limit, MaxMessageLimit)

	if _, err := s.GetCase(ctx, actor, caseID); err != nil {
		return nil, err
	}
	return s.Messages.List(ctx, MessageQuery{CaseID: caseID, BeforeID: beforeID, Limit: limit})
}
