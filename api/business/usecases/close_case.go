package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) CloseCase(ctx context.Context, actor models.User, caseID string) (models.Case, error) {
	if actor.Role != models.RoleAgent {
		return models.Case{}, models.ErrForbidden
	}

	now := s.Clock.Now()
	c, err := s.updateCase(ctx, caseID, func(c *models.Case) error {
		if !c.HasParticipant(actor.ID) {
			return models.ErrForbidden
		}
		if c.Status == models.StatusClosed {
			return models.ErrCaseClosed
		}
		c.Status = models.StatusClosed
		c.ClosedAt = &now
		c.UpdatedAt = now
		return nil
	})
	if err != nil {
		return models.Case{}, err
	}

	m := s.newMessage(c.ID, actor, models.KindSystem, "Case closed by "+actor.Name, now)
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Case{}, err
	}
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventMessageCreated, CaseID: c.ID, Data: m})
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventCaseClosed, CaseID: c.ID, Data: c})
	s.Broadcaster.PublishAgents(models.Event{Type: models.EventCaseStatusChanged, CaseID: c.ID, Data: c})
	return c, nil
}
