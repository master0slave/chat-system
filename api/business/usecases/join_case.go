package usecases

import (
	"context"
	"errors"

	"supportchat/business/models"
)

func (s *caseService) JoinCase(ctx context.Context, actor models.User, caseID string) (models.Case, error) {
	if actor.Role != models.RoleAgent {
		return models.Case{}, models.ErrForbidden
	}

	now := s.Clock.Now()
	var wasWaiting bool
	c, err := s.updateCase(ctx, caseID, func(c *models.Case) error {
		if c.Status == models.StatusClosed {
			return models.ErrCaseClosed
		}
		if c.HasParticipant(actor.ID) {
			return errNoChange
		}
		wasWaiting = c.Status == models.StatusWaiting
		c.Participants = append(c.Participants, models.Participant{
			UserID: actor.ID, Name: actor.Name, Role: actor.Role, JoinedAt: now,
		})
		c.Status = models.StatusOpen
		c.UpdatedAt = now
		return nil
	})
	if errors.Is(err, errNoChange) {
		return c, nil
	}
	if err != nil {
		return models.Case{}, err
	}

	joined := c.Participants[len(c.Participants)-1]
	m := s.newMessage(c.ID, actor, models.KindSystem, actor.Name+" joined the case", now)
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Case{}, err
	}
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventParticipantJoined, CaseID: c.ID, Data: joined})
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventMessageCreated, CaseID: c.ID, Data: m})
	if wasWaiting {
		s.Broadcaster.PublishAgents(models.Event{Type: models.EventCaseStatusChanged, CaseID: c.ID, Data: c})
	}
	return c, nil
}
