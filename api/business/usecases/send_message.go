package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) SendMessage(ctx context.Context, actor models.User, caseID, body string) (models.Message, error) {
	body, err := models.NormalizeBody(body)
	if err != nil {
		return models.Message{}, err
	}

	now := s.Clock.Now()
	// Touching UpdatedAt through updateCase makes "is the case still open?" and the save
	// one versioned step, so a message cannot slip in after a close that saved first.
	c, err := s.updateCase(ctx, caseID, func(c *models.Case) error {
		if !c.HasParticipant(actor.ID) {
			return models.ErrForbidden
		}
		if c.Status == models.StatusClosed {
			return models.ErrCaseClosed
		}
		c.UpdatedAt = now
		return nil
	})
	if err != nil {
		return models.Message{}, err
	}

	m := s.newMessage(c.ID, actor, models.KindText, body, now)
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Message{}, err
	}
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventMessageCreated, CaseID: c.ID, Data: m})
	return m, nil
}
