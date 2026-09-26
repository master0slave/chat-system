package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) GetCase(ctx context.Context, actor models.User, caseID string) (models.Case, error) {
	c, err := s.Cases.Get(ctx, caseID)
	if err != nil {
		return models.Case{}, err
	}
	if !c.CanView(actor) {
		return models.Case{}, models.ErrForbidden
	}
	return c, nil
}
