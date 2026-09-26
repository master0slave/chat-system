package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) OpenCase(ctx context.Context, actor models.User, question string) (models.Case, models.Message, error) {
	if actor.Role != models.RoleCustomer {
		return models.Case{}, models.Message{}, models.ErrForbidden
	}
	body, err := models.NormalizeBody(question)
	if err != nil {
		return models.Case{}, models.Message{}, err
	}

	now := s.Clock.Now()
	c := models.Case{
		ID:         s.IDs.NewID(),
		Subject:    models.SubjectFrom(body),
		CustomerID: actor.ID,
		Participants: []models.Participant{
			{UserID: actor.ID, Name: actor.Name, Role: actor.Role, JoinedAt: now},
		},
		Status:    models.StatusWaiting,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
	m := s.newMessage(c.ID, actor, models.KindText, body, now)

	if err := s.Cases.Insert(ctx, c); err != nil {
		return models.Case{}, models.Message{}, err
	}
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Case{}, models.Message{}, err
	}
	s.Broadcaster.PublishAgents(models.Event{Type: models.EventCaseCreated, CaseID: c.ID, Data: c})
	return c, m, nil
}
