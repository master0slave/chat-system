package usecases

import (
	"context"
	"errors"

	"supportchat/business/models"
)

// Temporary: caseService must satisfy CaseService before every use case exists.
// Each later task deletes the stub for the use case it implements, and Task 7 deletes this file.

var errNotImplemented = errors.New("not implemented")

func (s *caseService) ListCases(context.Context, models.User, models.CaseStatus) ([]models.Case, error) {
	return nil, errNotImplemented
}

func (s *caseService) GetCase(context.Context, models.User, string) (models.Case, error) {
	return models.Case{}, errNotImplemented
}

func (s *caseService) ListMessages(context.Context, models.User, string, string, int) ([]models.Message, error) {
	return nil, errNotImplemented
}
