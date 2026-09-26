package usecases

import (
	"context"
	"errors"

	"supportchat/business/models"
)

func (s *caseService) SendMessage(ctx context.Context, actor models.User, caseID, body string) (models.Message, error) {
	body, err := models.NormalizeBody(body)
	if err != nil {
		return models.Message{}, err
	}
	c, err := s.Cases.Get(ctx, caseID)
	if err != nil {
		return models.Message{}, err
	}
	if err := canSend(c, actor); err != nil {
		return models.Message{}, err
	}

	now := s.Clock.Now()
	// TouchForParticipant re-checks "open and a participant" inside the same atomic write,
	// so a message cannot slip in after a close that saved first, and senders never block each other.
	err = s.Cases.TouchForParticipant(ctx, caseID, actor.ID, now)
	if errors.Is(err, models.ErrConflict) {
		if c, err = s.Cases.Get(ctx, caseID); err != nil {
			return models.Message{}, err
		}
		if err := canSend(c, actor); err != nil {
			return models.Message{}, err
		}
		return models.Message{}, models.ErrConflict
	}
	if err != nil {
		return models.Message{}, err
	}

	m := s.newMessage(caseID, actor, models.KindText, body, now)
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Message{}, err
	}
	s.Broadcaster.PublishCase(caseID, models.Event{Type: models.EventMessageCreated, CaseID: caseID, Data: m})
	return m, nil
}

func canSend(c models.Case, actor models.User) error {
	if !c.HasParticipant(actor.ID) {
		return models.ErrForbidden
	}
	if c.Status == models.StatusClosed {
		return models.ErrCaseClosed
	}
	return nil
}
