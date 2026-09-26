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
	c, err := s.Cases.Get(ctx, caseID)
	if err != nil {
		return models.Case{}, err
	}
	if done, err := alreadyJoined(c, actor); done || err != nil {
		return c, err
	}

	now := s.Clock.Now()
	joined := models.Participant{UserID: actor.ID, Name: actor.Name, Role: actor.Role, JoinedAt: now}
	before, err := s.Cases.AddParticipant(ctx, caseID, joined)
	if errors.Is(err, models.ErrConflict) {
		// Someone closed the case, or this agent joined in a parallel request. Say which.
		if c, err = s.Cases.Get(ctx, caseID); err != nil {
			return models.Case{}, err
		}
		if done, err := alreadyJoined(c, actor); done || err != nil {
			return c, err
		}
		return models.Case{}, models.ErrConflict
	}
	if err != nil {
		return models.Case{}, err
	}

	c = before
	c.Participants = append(c.Participants, joined)
	c.Status = models.StatusOpen
	c.UpdatedAt = now
	c.Version++

	m := s.newMessage(c.ID, actor, models.KindSystem, actor.Name+" joined the case", now)
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Case{}, err
	}
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventParticipantJoined, CaseID: c.ID, Data: joined})
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventMessageCreated, CaseID: c.ID, Data: m})
	if before.Status == models.StatusWaiting {
		s.Broadcaster.PublishAgents(models.Event{Type: models.EventCaseStatusChanged, CaseID: c.ID, Data: c})
	}
	return c, nil
}

// alreadyJoined reports whether joining c needs no write: the case is closed (an error),
// or the agent is already a participant (joining twice has no effect).
func alreadyJoined(c models.Case, agent models.User) (bool, error) {
	if c.Status == models.StatusClosed {
		return true, models.ErrCaseClosed
	}
	return c.HasParticipant(agent.ID), nil
}
