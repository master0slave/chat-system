package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) ListCases(ctx context.Context, actor models.User, status models.CaseStatus) ([]models.Case, error) {
	if status != "" && !status.Valid() {
		return nil, models.ErrInvalidInput
	}
	filter := CaseFilter{Status: status}
	switch actor.Role {
	case models.RoleAgent:
	case models.RoleCustomer:
		filter.CustomerID = actor.ID
	default:
		return nil, models.ErrForbidden
	}
	return s.Cases.List(ctx, filter)
}
